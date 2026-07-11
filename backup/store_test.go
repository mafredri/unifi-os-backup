package backup

import (
	"os"
	"path/filepath"
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
