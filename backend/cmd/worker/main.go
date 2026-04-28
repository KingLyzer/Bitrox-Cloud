package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	filesService := appfiles.NewService(filesRepo, objectStorage, appfiles.RealClock{}, appfiles.Config{
		UploadSessionTTL:        cfg.UploadSessionTTL,
		MaxChunkBytes:           cfg.UploadMaxChunkBytes,
		MaxFileBytes:            cfg.UploadMaxFileBytes,
		MaxActiveUploadSessions: cfg.UploadMaxActiveSessions,
		MaxUserStagedBytes:      cfg.UploadMaxStagedBytes,
	}, log)
	reaper := appfiles.NewReaper(filesRepo, objectStorage, appfiles.RealClock{}, appfiles.ReaperConfig{
		Interval:         cfg.UploadReaperInterval,
		BatchSize:        cfg.UploadReaperBatchSize,
		DeleteMaxRetries: 3,
	}, log)
	reminderProcessor := appcalendar.NewReminderProcessor(calendarRepo, cfg.ReminderScanInterval, cfg.ReminderScanBatchSize, log)

	go reaper.Run(ctx)
	go reminderProcessor.Run(ctx)
	go func() {
		ticker := time.NewTicker(cfg.TrashPurgeInterval)
		defer ticker.Stop()

		purge := func() {
			cutoff := time.Now().UTC().Add(-cfg.TrashRetention)
			purged, purgeErr := filesService.PurgeDeletedNodesBefore(ctx, cutoff, 200)
			if purgeErr != nil {
				log.Warn("trash purge cycle failed", slog.Any("error", purgeErr))
				return
			}
			if purged > 0 {
				log.Info("trash purge completed", slog.Int("purged", purged), slog.Time("cutoff", cutoff))
			}
		}

		purge()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				purge()
			}
		}
	}()

	log.Info(
		"worker started",
		slog.Duration("upload_reaper_interval", cfg.UploadReaperInterval),
		slog.Int("upload_reaper_batch_size", cfg.UploadReaperBatchSize),
		slog.Duration("reminder_scan_interval", cfg.ReminderScanInterval),
		slog.Int("reminder_scan_batch_size", cfg.ReminderScanBatchSize),
		slog.Duration("trash_purge_interval", cfg.TrashPurgeInterval),
		slog.Duration("trash_retention", cfg.TrashRetention),
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
