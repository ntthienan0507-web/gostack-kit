package cmd

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/ntthienan0507-web/go-api-template/pkg/cron"
	"github.com/ntthienan0507-web/go-api-template/pkg/logger"
)

func runCron() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := mustLoadConfig()

	log, err := logger.New(cfg.LogFormat, cfg.LogLevel)
	if err != nil {
		fatal("init logger: %v", err)
	}

	scheduler := cron.NewScheduler(log)

	// Register jobs here
	// scheduler.Register("cleanup", "0 3 * * *", cleanupJob)

	log.Info("starting cron service")
	scheduler.Start(ctx)
}
