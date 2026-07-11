package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFilename(t *testing.T) {
	for _, tc := range []struct{ header, want string }{
		{"unifi_os_backup_1.unf", "unifi_os_backup_1.unf"},
		{"attachment; filename=backup.unf", "backup.unf"},
		{"../../escape", ""},
		{"", ""},
	} {
		if got := filename(tc.header); got != tc.want {
			t.Errorf("filename(%q) = %q, want %q", tc.header, got, tc.want)
		}
	}
}

func TestManagedBackupFilename(t *testing.T) {
	for _, tc := range []struct {
		target, name string
		want         bool
	}{
		{"", "unifi_os_backup_1783761947693_uuid.unifi", true},
		{"network", "network_backup_07.11.2026_10-13-AM_v10.4.57.unf", true},
		{"protect", "unifi_protect_backup.v7.1.87.202607111314532.zip", true},
		{"uos", "unifi_os_backup_for_uos_1783764805982_uuid.unifi", true},
		{"network", "unifi_os_backup_1783761947693_uuid.unifi", false},
	} {
		if got := managedBackupFilename(tc.target, tc.name); got != tc.want {
			t.Errorf("managedBackupFilename(%q, %q) = %t, want %t", tc.target, tc.name, got, tc.want)
		}
	}
}

func TestPruneKeepsDailyAndWeeklyRepresentatives(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "c", "full")
	if err := os.MkdirAll(dir, 0750); err != nil {
		t.Fatal(err)
	}
	for i := range 30 {
		path := filepath.Join(dir, "unifi_os_backup_"+time.Now().Add(-time.Duration(i)*24*time.Hour).Format("20060102-150405")+".unf")
		if err := os.WriteFile(path, []byte("backup"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, time.Now().Add(-time.Duration(i)*24*time.Hour), time.Now().Add(-time.Duration(i)*24*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	s := Store{Root: root}
	if err := s.Prune("c", "", Retention{DailyKeep: 3, WeeklyKeep: 2, WeeklyInterval: 7 * 24 * time.Hour}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("kept %d files, want 4", len(entries))
	}
}

func TestSaveDoesNotOverwriteSameFilename(t *testing.T) {
	root := t.TempDir()
	store := Store{Root: root}
	retention := Retention{DailyKeep: 2, WeeklyKeep: 0, WeeklyInterval: 7 * 24 * time.Hour}
	name := "unifi_protect_backup.v7.1.87.202607111314532.zip"
	if _, err := store.Save("c", "protect", name, strings.NewReader("one"), retention); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save("c", "protect", name, strings.NewReader("two"), retention); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "c", "protect"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("kept %d files, want 2", len(entries))
	}
}
