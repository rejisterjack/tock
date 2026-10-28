package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rejisterjack/tock/internal/app"
	"github.com/rejisterjack/tock/internal/config"
	"github.com/rejisterjack/tock/internal/db"
	"github.com/rejisterjack/tock/internal/demo"
	"github.com/rejisterjack/tock/internal/httpapi"
	"github.com/rejisterjack/tock/internal/notify"
	"github.com/rejisterjack/tock/internal/store/pg"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(runHealthcheck())
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := db.Migrate(ctx, cfg.DatabaseURL); err != nil {
		log.Error("migrate", "err", err)
		os.Exit(1)
	}
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database", "err", err)
		os.Exit(1)
	}
	repo := pg.New(pool)
	defer repo.Close()

	notifier := notify.Guard{
		Demo: cfg.Demo,
		Next: notify.Multi{
			Email: notify.Email{
				Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUser,
				Password: cfg.SMTPPassword, From: cfg.SMTPFrom,
			},
			Slack: notify.Slack{},
			Hook:  notify.Webhook{},
		},
	}
	svc := &app.Service{
		Repo: repo, Notifier: notifier, BaseURL: cfg.BaseURL, Demo: cfg.Demo,
		Retention: cfg.Retention(), Log: log,
	}
	if cfg.Demo {
		if err := demo.Seed(ctx, repo, time.Now().UTC()); err != nil {
			log.Error("demo seed", "err", err)
			os.Exit(1)
		}
		go demo.Run(ctx, repo, log)
	}
	go runScheduler(ctx, svc, cfg.TickInterval, log)

	api, err := httpapi.New(cfg, svc, log, version)
	if err != nil {
		log.Error("templates", "err", err)
		os.Exit(1)
	}
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shut, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shut); err != nil {
			log.Error("shutdown", "err", err)
		}
	}()
	log.Info("listening", "addr", cfg.Addr, "demo", cfg.Demo, "version", version)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}

func runScheduler(ctx context.Context, svc *app.Service, every time.Duration, log *slog.Logger) {
	tick := func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("scheduler panic", "err", rec)
			}
		}()
		if err := svc.Tick(ctx); err != nil && ctx.Err() == nil {
			log.Error("tick", "err", err)
		}
	}
	tick()
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tick()
		}
	}
}

func runHealthcheck() int {
	addr := os.Getenv("TOCK_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	client := &http.Client{Timeout: 2 * time.Second}
	res, err := client.Get("http://" + net.JoinHostPort(host, port) + "/healthz")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, res.Status)
		return 1
	}
	return 0
}
