package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigUsesPerConsoleSecretFiles(t *testing.T) {
	dir := t.TempDir()
	usernameFile := filepath.Join(dir, "username")
	passwordFile := filepath.Join(dir, "password")
	if err := os.WriteFile(usernameFile, []byte("admin\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(passwordFile, []byte("secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BACKUP_DIRECTORY", filepath.Join(dir, "backups"))
	t.Setenv("CONSOLES", "HOME,office")
	t.Setenv("CONSOLE_HOME_URL", "https://home.example")
	t.Setenv("CONSOLE_HOME_USERNAME_FILE", usernameFile)
	t.Setenv("CONSOLE_HOME_PASSWORD_FILE", passwordFile)
	t.Setenv("CONSOLE_HOME_TARGETS", "full,network")
	t.Setenv("CONSOLE_HOME_BACKUP_INTERVAL", "6h")
	t.Setenv("CONSOLE_HOME_HEALTH_MAX_AGE", "18h")
	t.Setenv("CONSOLE_HOME_HTTP_TIMEOUT", "2m")
	t.Setenv("CONSOLE_HOME_SKIP_TLS_VERIFY", "true")
	t.Setenv("CONSOLE_HOME_RETENTION_DAILY", "7")
	t.Setenv("CONSOLE_HOME_RETENTION_WEEKLY", "4")
	t.Setenv("CONSOLE_HOME_WEEKLY_INTERVAL", "168h")
	t.Setenv("CONSOLE_OFFICE_URL", "https://office.example")
	t.Setenv("CONSOLE_OFFICE_USERNAME", "admin")
	t.Setenv("CONSOLE_OFFICE_PASSWORD", "direct-secret")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Consoles) != 2 || cfg.Consoles[0].Name != "home" || cfg.Consoles[0].Password != "secret" {
		t.Fatalf("unexpected consoles: %+v", cfg.Consoles)
	}
	if len(cfg.Consoles[0].Targets) != 2 || cfg.Consoles[0].Targets[0] != "" || cfg.Consoles[0].Targets[1] != "network" {
		t.Fatalf("unexpected targets: %+v", cfg.Consoles[0].Targets)
	}
	if cfg.Consoles[0].Interval != 6*time.Hour || cfg.Consoles[0].HealthMaxAge != 18*time.Hour || cfg.Consoles[0].HTTPTimeout != 2*time.Minute || !cfg.Consoles[0].SkipTLSVerify || cfg.Consoles[0].DailyKeep != 7 || cfg.Consoles[0].WeeklyKeep != 4 || cfg.Consoles[0].WeeklyInterval != 168*time.Hour {
		t.Fatalf("unexpected per-console policy: %+v", cfg.Consoles[0])
	}
	if cfg.Consoles[1].SkipTLSVerify {
		t.Fatal("TLS verification is not disabled by default")
	}
}

func TestLoadConfigRejectsCredentialValueAndFileTogether(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BACKUP_DIRECTORY", dir)
	t.Setenv("CONSOLES", "home")
	t.Setenv("CONSOLE_HOME_URL", "https://home.example")
	t.Setenv("CONSOLE_HOME_USERNAME", "admin")
	t.Setenv("CONSOLE_HOME_PASSWORD", "secret")
	t.Setenv("CONSOLE_HOME_PASSWORD_FILE", filepath.Join(dir, "password"))

	if _, err := LoadConfig(); err == nil {
		t.Fatal("LoadConfig accepted both password forms")
	}
}
