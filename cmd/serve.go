package cmd

import (
	"context"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/ntthienan0507-web/go-api-template/pkg/app"
	"github.com/ntthienan0507-web/go-api-template/pkg/database"
	"github.com/ntthienan0507-web/go-api-template/pkg/logger"
)

func runServe() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := mustLoadConfig()

	log, err := logger.New(cfg.LogFormat, cfg.LogLevel)
	if err != nil {
		fatal("init logger: %v", err)
	}

	// Auto-migrate on startup
	if err := database.RunMigrations(cfg.DSN(), migrationsDir, log); err != nil {
		log.Error("migration failed", zap.Error(err))
	}

	application, err := app.New(ctx, cfg, log)
	if err != nil {
		log.Fatal("init app failed", zap.Error(err))
	}
	defer application.Shutdown()

	if err := application.Run(ctx); err != nil {
		log.Fatal("server error", zap.Error(err))
	}
}
