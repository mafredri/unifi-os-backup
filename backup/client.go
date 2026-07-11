package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

type Downloader struct{}

func (d Downloader) Download(ctx context.Context, console ConsoleConfig, target string) (name string, body io.ReadCloser, err error) {
	client := &http.Client{Timeout: console.HTTPTimeout, Transport: &http.Transport{TLSClientConfig: TLSConfig(console.SkipTLSVerify)}}
	login, err := json.Marshal(map[string]string{"username": console.Username, "password": console.Password})
	if err != nil {
		return "", nil, err
	}
	loginReq, err := http.NewRequestWithContext(ctx, http.MethodPost, console.URL+"/api/auth/login", strings.NewReader(string(login)))
	if err != nil {
		return "", nil, err
	}
	loginReq.Header.Set("Content-Type", "application/json")
	jar := newCookieJar()
	client.Jar = jar
	resp, err := client.Do(loginReq)
	if err != nil {
		return "", nil, fmt.Errorf("login: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("login returned HTTP %s", resp.Status)
	}
	endpoint := console.URL + "/api/backup/download"
	if target != "" {
		endpoint += "?target=" + url.QueryEscape(target)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", nil, err
	}
	resp, err = client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("download: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return "", nil, fmt.Errorf("download returned HTTP %s", resp.Status)
	}
	contentType := resp.Header.Get("Content-Type")
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if mediaType != "application/octet-stream" && mediaType != "application/zip" && mediaType != "application/x-tar" {
		resp.Body.Close()
		return "", nil, fmt.Errorf("download returned unexpected content type %q", contentType)
	}
	name = filename(resp.Header.Get("filename"))
	if name == "" {
		name = filename(resp.Header.Get("Content-Disposition"))
	}
	if name == "" {
		resp.Body.Close()
		return "", nil, fmt.Errorf("download did not provide a safe filename")
	}
	if !strings.HasPrefix(name, "unifi_os_backup_") {
		resp.Body.Close()
		return "", nil, fmt.Errorf("download returned unexpected filename %q", name)
	}
	return name, resp.Body, nil
}

func filename(header string) string {
	if strings.Contains(header, "filename=") {
		_, params, err := mime.ParseMediaType(header)
		if err == nil {
			header = params["filename"]
		}
	}
	header = strings.Trim(strings.TrimSpace(header), `"'`)
	if strings.ContainsAny(header, `/\\`) {
		return ""
	}
	name := filepath.Base(header)
	if name == "." || name == "" || name == ".." || strings.ContainsAny(name, `/\\`) || strings.ContainsAny(name, "\x00\r\n") {
		return ""
	}
	return name
}

type cookieJar struct{ cookies []*http.Cookie }

func newCookieJar() *cookieJar { return &cookieJar{} }
func (j *cookieJar) SetCookies(_ *url.URL, cookies []*http.Cookie) {
	j.cookies = append(j.cookies, cookies...)
}
func (j *cookieJar) Cookies(_ *url.URL) []*http.Cookie { return j.cookies }
