package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/chungnguyen/go-api-template/internal/app"
	"github.com/chungnguyen/go-api-template/internal/config"
	"github.com/chungnguyen/go-api-template/internal/database"
	"github.com/chungnguyen/go-api-template/pkg/logger"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Config — no globals
	cfg, err := config.Load(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	// Logger — no globals
	log, err := logger.New(cfg.LogFormat, cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init logger: %v\n", err)
		os.Exit(1)
	}

	// Programmatic migration — no shell-out
	if err := database.RunMigrations(cfg.DSN(), "db/migrations", log); err != nil {
		log.Error("migration failed", zap.Error(err))
		// Don't fatal — DB might not be available in dev; just warn
	}

	// App (DI container) — no globals
	application, err := app.New(ctx, cfg, log)
	if err != nil {
		log.Fatal("init app failed", zap.Error(err))
	}
	defer application.Shutdown()

	if err := application.Run(); err != nil {
		log.Fatal("server error", zap.Error(err))
	}
}
