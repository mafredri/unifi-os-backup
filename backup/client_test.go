package backup

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDownloadLoginAndTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth/login" {
			if r.Method != http.MethodPost {
				t.Errorf("login method = %s", r.Method)
			}
			http.SetCookie(w, &http.Cookie{Name: "SESSION", Value: "ok"})
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path != "/api/backup/download" || r.URL.Query().Get("target") != "network" || r.Header.Get("Cookie") != "SESSION=ok" {
			t.Errorf("unexpected download request: %s cookie=%q", r.URL.String(), r.Header.Get("Cookie"))
			http.Error(w, "bad", 400)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("filename", "network_backup_07.11.2026_10-13-AM_v10.4.57.unf")
		_, _ = w.Write([]byte("data"))
	}))
	defer server.Close()
	d := Downloader{}
	name, body, err := d.Download(context.Background(), ConsoleConfig{URL: server.URL, Username: "u", Password: "p"}, "network")
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	data, _ := io.ReadAll(body)
	if name != "network_backup_07.11.2026_10-13-AM_v10.4.57.unf" || string(data) != "data" {
		t.Fatalf("got %q %q", name, data)
	}
}
