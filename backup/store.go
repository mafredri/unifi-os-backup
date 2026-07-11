package backup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type Store struct {
	Root string
}

type Retention struct {
	DailyKeep      int
	WeeklyKeep     int
	WeeklyInterval time.Duration
}

func (s Store) Save(console, target, name string, r io.Reader, retention Retention) (string, error) {
	dir := filepath.Join(s.Root, console, targetDir(target))
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, ".partial-*")
	if err != nil {
		return "", err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, err = io.Copy(f, r); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", fmt.Errorf("write temporary backup: %w", err)
	}
	info, err := os.Stat(temp)
	if err != nil || info.Size() == 0 {
		return "", fmt.Errorf("downloaded backup is empty")
	}
	dst := filepath.Join(dir, uniqueName(dir, name))
	if err := os.Rename(temp, dst); err != nil {
		return "", fmt.Errorf("archive backup: %w", err)
	}
	if err := s.Prune(console, target, retention); err != nil {
		return dst, err
	}
	return dst, nil
}

type archive struct {
	path string
	when time.Time
}

func (s Store) Prune(console, target string, retention Retention) error {
	files, err := s.archives(console, target)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	keep := map[string]bool{}
	for i := range min(len(files), retention.DailyKeep) {
		keep[files[i].path] = true
	}
	// Weekly representatives are selected across the whole history. A representative
	// inside the daily window is already kept, so it counts toward WeeklyKeep without
	// creating a second copy.
	if retention.WeeklyKeep > 0 {
		newest := files[0].when
		buckets := map[int]archive{}
		for _, f := range files {
			age := newest.Sub(f.when)
			bucket := int(age / retention.WeeklyInterval)
			if old, exists := buckets[bucket]; !exists || f.when.After(old.when) {
				buckets[bucket] = f
			}
		}
		var reps []archive
		for _, f := range buckets {
			reps = append(reps, f)
		}
		slices.SortFunc(reps, func(a, b archive) int { return b.when.Compare(a.when) })
		for i := range min(len(reps), retention.WeeklyKeep) {
			if !keep[reps[i].path] {
				keep[reps[i].path] = true
			}
		}
	}
	for _, f := range files {
		if !keep[f.path] {
			if err := os.Remove(f.path); err != nil {
				return fmt.Errorf("remove old backup %q: %w", f.path, err)
			}
		}
	}
	return nil
}

func (s Store) Latest(console, target string) (time.Time, bool, error) {
	files, err := s.archives(console, target)
	if err != nil || len(files) == 0 {
		return time.Time{}, false, err
	}
	return files[0].when, true, nil
}

func (s Store) archives(console, target string) ([]archive, error) {
	dir := filepath.Join(s.Root, console, targetDir(target))
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	files := make([]archive, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") || !managedBackupFilename(target, e.Name()) {
			continue
		}
		when, ok := backupTime(e.Info)
		if ok {
			files = append(files, archive{filepath.Join(dir, e.Name()), when})
		}
	}
	slices.SortFunc(files, func(a, b archive) int { return b.when.Compare(a.when) })
	return files, nil
}

func targetDir(target string) string {
	if target == "" {
		return "full"
	}
	return target
}

func managedBackupFilename(target, name string) bool {
	if target == "" {
		return strings.HasPrefix(name, "unifi_os_backup_") && !strings.HasPrefix(name, "unifi_os_backup_for_")
	}
	if strings.HasPrefix(name, "unifi_os_backup_for_"+target+"_") {
		return true
	}
	switch target {
	case "network":
		return strings.HasPrefix(name, "network_backup_")
	case "protect":
		return strings.HasPrefix(name, "unifi_protect_backup.")
	default:
		return strings.HasPrefix(name, "unifi_os_backup_")
	}
}

func uniqueName(dir, name string) string {
	if _, err := os.Lstat(filepath.Join(dir, name)); os.IsNotExist(err) {
		return name
	}
	extension := filepath.Ext(name)
	stem := strings.TrimSuffix(name, extension)
	return fmt.Sprintf("%s_download_%d%s", stem, time.Now().UnixNano(), extension)
}

func backupTime(info func() (os.FileInfo, error)) (time.Time, bool) {
	fi, err := info()
	if err != nil {
		return time.Time{}, false
	}
	return fi.ModTime(), true
}
