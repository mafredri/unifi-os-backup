package backup

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddress   string
	BackupDirectory string
	Consoles        []ConsoleConfig
}

type ConsoleConfig struct {
	Name           string
	URL            string
	Username       string
	Password       string
	Targets        []string
	Interval       time.Duration
	HealthMaxAge   time.Duration
	HTTPTimeout    time.Duration
	SkipTLSVerify  bool
	DailyKeep      int
	WeeklyKeep     int
	WeeklyInterval time.Duration
}

var validTargets = map[string]bool{"": true, "users": true, "uos": true, "network": true, "protect": true, "innerspace": true}

func LoadConfig() (Config, error) {
	c := Config{
		ListenAddress: "0.0.0.0:8080", BackupDirectory: "/backups",
	}
	if c.ListenAddress = getenv("LISTEN_ADDRESS", c.ListenAddress); c.ListenAddress == "" {
		return c, errors.New("LISTEN_ADDRESS must not be empty")
	}
	c.BackupDirectory = getenv("BACKUP_DIRECTORY", c.BackupDirectory)
	names := splitList(os.Getenv("CONSOLES"))
	if len(names) == 0 {
		return c, errors.New("CONSOLES is required and must list at least one console")
	}
	seen := map[string]bool{}
	for _, rawName := range names {
		name := strings.ToLower(strings.TrimSpace(rawName))
		if !validConsoleName(name) || seen[name] {
			return c, fmt.Errorf("invalid or duplicate console name %q; use letters, numbers, and underscores", rawName)
		}
		seen[name] = true
		prefix := "CONSOLE_" + strings.ToUpper(name)
		cc := ConsoleConfig{Name: name, URL: strings.TrimRight(strings.TrimSpace(os.Getenv(prefix+"_URL")), "/")}
		var err error
		cc.Username, err = secretValue(prefix+"_USERNAME", prefix+"_USERNAME_FILE")
		if err != nil {
			return c, err
		}
		cc.Password, err = secretValue(prefix+"_PASSWORD", prefix+"_PASSWORD_FILE")
		if err != nil {
			return c, err
		}
		u, e := url.Parse(cc.URL)
		if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return c, fmt.Errorf("console %q has an invalid %s_URL", name, prefix)
		}
		if cc.Username == "" || cc.Password == "" {
			return c, fmt.Errorf("console %q requires username/password or username/password file variables", name)
		}
		cc.Interval, err = durationValue(prefix+"_BACKUP_INTERVAL", 24*time.Hour)
		if err != nil {
			return c, err
		}
		cc.HealthMaxAge, err = durationValue(prefix+"_HEALTH_MAX_AGE", 48*time.Hour)
		if err != nil {
			return c, err
		}
		cc.HTTPTimeout, err = durationValue(prefix+"_HTTP_TIMEOUT", 30*time.Minute)
		if err != nil {
			return c, err
		}
		cc.WeeklyInterval, err = durationValue(prefix+"_WEEKLY_INTERVAL", 7*24*time.Hour)
		if err != nil {
			return c, err
		}
		cc.DailyKeep, err = integerValue(prefix+"_RETENTION_DAILY", 14)
		if err != nil {
			return c, err
		}
		cc.WeeklyKeep, err = integerValue(prefix+"_RETENTION_WEEKLY", 12)
		if err != nil {
			return c, err
		}
		cc.SkipTLSVerify, err = parseBool(prefix+"_SKIP_TLS_VERIFY", false)
		if err != nil {
			return c, err
		}
		targets := splitList(os.Getenv(prefix + "_TARGETS"))
		if len(targets) == 0 {
			cc.Targets = []string{""}
		} else {
			for _, target := range targets {
				target = strings.ToLower(target)
				if target == "full" {
					target = ""
				}
				if !validTargets[target] {
					return c, fmt.Errorf("console %q has invalid target %q", name, target)
				}
				cc.Targets = append(cc.Targets, target)
			}
		}
		c.Consoles = append(c.Consoles, cc)
	}
	for _, console := range c.Consoles {
		if console.DailyKeep == 0 && console.WeeklyKeep == 0 {
			return c, fmt.Errorf("console %q must retain at least one backup", console.Name)
		}
	}
	if err := os.MkdirAll(filepath.Clean(c.BackupDirectory), 0750); err != nil {
		return c, fmt.Errorf("create backup directory: %w", err)
	}
	return c, nil
}

func TLSConfig(skipVerify bool) *tls.Config {
	return &tls.Config{InsecureSkipVerify: skipVerify} // #nosec G402: explicitly configured per console for self-signed consoles.
}
func getenv(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
func parseBool(name string, fallback bool) (bool, error) {
	v := os.Getenv(name)
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be boolean: %q", name, v)
	}
	return b, nil
}

func durationValue(name string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration: %q", name, value)
	}
	return duration, nil
}

func integerValue(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	integer, err := strconv.Atoi(value)
	if err != nil || integer < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer: %q", name, value)
	}
	return integer, nil
}

func splitList(value string) []string {
	var values []string
	for item := range strings.SplitSeq(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			values = append(values, item)
		}
	}
	return values
}

func validConsoleName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

func secretValue(valueName, fileName string) (string, error) {
	value, file := os.Getenv(valueName), os.Getenv(fileName)
	if value != "" && file != "" {
		return "", fmt.Errorf("set only one of %s and %s", valueName, fileName)
	}
	if file == "" {
		return value, nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", fileName, err)
	}
	return strings.TrimRight(string(data), "\r\n"), nil
}
