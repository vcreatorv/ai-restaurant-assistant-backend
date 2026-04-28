package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/example/ai-restaurant-assistant-backend/cmd/app/app"
)

var (
	// Version версия сборки
	Version = "dev"
	// Commit короткий хэш коммита
	Commit = "unknown"
	// BuildDate дата сборки
	BuildDate = "unknown"
)

func main() {
	configPath := flag.String("config", app.DefaultConfigPath, "path to yaml config")
	flag.Parse()

	slog.Info("starting",
		"version", Version,
		"commit", Commit,
		"build_date", BuildDate,
	)

	cfg, err := app.LoadConfig(*configPath)
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	a, err := app.New(cfg)
	if err != nil {
		slog.Error("init", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("listening", "addr", cfg.HTTP.Addr)
	if err := a.Run(ctx); err != nil {
		slog.Error("run", "err", err)
		os.Exit(1)
	}
	slog.Info("stopped")
}
