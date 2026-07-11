package backup

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthRequiresRecentSuccess(t *testing.T) {
	a := NewService(Config{Consoles: []ConsoleConfig{{Name: "home", Targets: []string{""}, HealthMaxAge: time.Hour}}}, slog.Default())
	r := httptest.NewRecorder()
	a.health(r, nil)
	if r.Code != http.StatusServiceUnavailable {
		t.Fatalf("initial health = %d", r.Code)
	}
	a.status["home/"] = jobStatus{LastSuccess: time.Now()}
	r = httptest.NewRecorder()
	a.health(r, nil)
	if r.Code != http.StatusOK {
		t.Fatalf("healthy status = %d", r.Code)
	}
	a.status["home/"] = jobStatus{LastSuccess: time.Now().Add(-2 * time.Hour), LastError: "login failed"}
	r = httptest.NewRecorder()
	a.health(r, nil)
	if r.Code != http.StatusServiceUnavailable {
		t.Fatalf("stale health = %d", r.Code)
	}
}
