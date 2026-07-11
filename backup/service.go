package backup

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type jobStatus struct {
	LastSuccess time.Time
	LastAttempt time.Time
	LastError   string
}
type Service struct {
	cfg        Config
	log        *slog.Logger
	downloader Downloader
	store      Store
	mu         sync.Mutex
	status     map[string]jobStatus
}

func NewService(cfg Config, logger *slog.Logger) *Service {
	return &Service{cfg: cfg, log: logger, store: Store{Root: cfg.BackupDirectory}, status: map[string]jobStatus{}}
}

func (a *Service) Run(ctx context.Context) error {
	server := &http.Server{Addr: a.cfg.ListenAddress, Handler: a.handler(), ReadHeaderTimeout: 5 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		a.log.Info("http server listening", "address", a.cfg.ListenAddress)
		errCh <- server.ListenAndServe()
	}()
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	for _, console := range a.cfg.Consoles {
		go a.runConsole(ctx, console)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errCh:
			if err == http.ErrServerClosed {
				return nil
			}
			return err
		}
	}
}

func (a *Service) runConsole(ctx context.Context, console ConsoleConfig) {
	a.runConsoleOnce(ctx, console)
	ticker := time.NewTicker(console.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.runConsoleOnce(ctx, console)
		}
	}
}

func (a *Service) runConsoleOnce(ctx context.Context, console ConsoleConfig) {
	for _, target := range console.Targets {
		a.runJob(ctx, console, target)
	}
}

func (a *Service) runJob(ctx context.Context, console ConsoleConfig, target string) {
	key := console.Name + "/" + target
	now := time.Now()
	a.mu.Lock()
	state := a.status[key]
	state.LastAttempt = now
	a.status[key] = state
	a.mu.Unlock()
	latest, exists, err := a.store.Latest(console.Name, target)
	if err != nil {
		a.finishFailure(key, console, target, fmt.Errorf("inspect existing backups: %w", err))
		return
	}
	if exists && time.Since(latest) < console.Interval {
		a.mu.Lock()
		state = a.status[key]
		state.LastSuccess = latest
		state.LastError = ""
		a.status[key] = state
		a.mu.Unlock()
		a.log.Info("backup still current", "console", console.Name, "target", target, "last_archived", latest)
		return
	}
	name, body, err := a.downloader.Download(ctx, console, target)
	if err == nil {
		defer body.Close()
		_, err = a.store.Save(console.Name, target, name, body, Retention{DailyKeep: console.DailyKeep, WeeklyKeep: console.WeeklyKeep, WeeklyInterval: console.WeeklyInterval})
	}
	a.mu.Lock()
	state = a.status[key]
	if err != nil {
		state.LastError = err.Error()
		a.log.Error("backup failed", "console", console.Name, "target", target, "error", err)
	} else {
		state.LastSuccess = time.Now()
		state.LastError = ""
		a.log.Info("backup archived", "console", console.Name, "target", target, "filename", name)
	}
	a.status[key] = state
	a.mu.Unlock()
}

func (a *Service) finishFailure(key string, console ConsoleConfig, target string, err error) {
	a.mu.Lock()
	state := a.status[key]
	state.LastError = err.Error()
	a.status[key] = state
	a.mu.Unlock()
	a.log.Error("backup failed", "console", console.Name, "target", target, "error", err)
}

func (a *Service) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /readyz", a.health)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "not found", http.StatusNotFound) })
	return mux
}
func (a *Service) health(w http.ResponseWriter, _ *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	unhealthy := len(a.status) == 0
	for _, c := range a.cfg.Consoles {
		for _, t := range c.Targets {
			key := c.Name + "/" + t
			state, ok := a.status[key]
			if !ok || state.LastSuccess.IsZero() || now.Sub(state.LastSuccess) > c.HealthMaxAge {
				unhealthy = true
			}
		}
	}
	if unhealthy {
		http.Error(w, "unhealthy", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}
