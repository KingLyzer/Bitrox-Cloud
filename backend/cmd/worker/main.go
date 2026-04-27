package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	appcalendar "cloud/backend/internal/application/calendar"
	appfiles "cloud/backend/internal/application/files"
	"cloud/backend/internal/infrastructure/config"
	"cloud/backend/internal/infrastructure/logger"
	infraPg "cloud/backend/internal/infrastructure/postgres"
	infraRedis "cloud/backend/internal/infrastructure/redis"
	"cloud/backend/internal/infrastructure/storage/localfs"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", slog.Any("error", err))
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel, cfg.LogFormat)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pgPool, err := infraPg.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("failed to connect postgres", slog.Any("error", err))
		os.Exit(1)
	}
	defer pgPool.Close()

	if err := infraPg.EnsurePhase4Schema(ctx, pgPool); err != nil {
		log.Error("failed to ensure runtime schema", slog.Any("error", err))
		os.Exit(1)
	}

	redisClient, err := infraRedis.Connect(ctx, cfg.RedisURL)
	if err != nil {
		log.Error("failed to connect redis", slog.Any("error", err))
		os.Exit(1)
	}
	defer redisClient.Close()

	filesRepo := infraPg.NewFilesRepository(pgPool)
	calendarRepo := infraPg.NewCalendarRepository(pgPool)
	objectStorage := localfs.New(cfg.StorageLocalRoot)
	reaper := appfiles.NewReaper(filesRepo, objectStorage, appfiles.RealClock{}, appfiles.ReaperConfig{
		Interval:         cfg.UploadReaperInterval,
		BatchSize:        cfg.UploadReaperBatchSize,
		DeleteMaxRetries: 3,
	}, log)
	reminderProcessor := appcalendar.NewReminderProcessor(calendarRepo, cfg.ReminderScanInterval, cfg.ReminderScanBatchSize, log)

	go reaper.Run(ctx)
	go reminderProcessor.Run(ctx)

	log.Info(
		"worker started",
		slog.Duration("upload_reaper_interval", cfg.UploadReaperInterval),
		slog.Int("upload_reaper_batch_size", cfg.UploadReaperBatchSize),
		slog.Duration("reminder_scan_interval", cfg.ReminderScanInterval),
		slog.Int("reminder_scan_batch_size", cfg.ReminderScanBatchSize),
	)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()
	_ = shutdownCtx

	log.Info("worker shutdown complete")
}
