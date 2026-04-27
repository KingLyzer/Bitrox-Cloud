package files

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	domainfiles "cloud/backend/internal/domain/files"
	"cloud/backend/internal/infrastructure/storage"
)

const (
	defaultReaperInterval   = time.Minute
	defaultReaperBatchSize  = 200
	defaultDeleteMaxRetries = 3
)

type ReaperConfig struct {
	Interval         time.Duration
	BatchSize        int
	DeleteMaxRetries int
}

type Reaper struct {
	repo    Repository
	storage storage.ObjectStorage
	clock   Clock
	cfg     ReaperConfig
	log     *slog.Logger
}

func NewReaper(repo Repository, objectStorage storage.ObjectStorage, clock Clock, cfg ReaperConfig, log *slog.Logger) *Reaper {
	if log == nil {
		log = slog.Default()
	}
	if clock == nil {
		clock = RealClock{}
	}
	if cfg.Interval <= 0 {
		cfg.Interval = defaultReaperInterval
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultReaperBatchSize
	}
	if cfg.DeleteMaxRetries <= 0 {
		cfg.DeleteMaxRetries = defaultDeleteMaxRetries
	}
	return &Reaper{
		repo:    repo,
		storage: objectStorage,
		clock:   clock,
		cfg:     cfg,
		log:     log,
	}
}

func (r *Reaper) Run(ctx context.Context) {
	ticker := time.NewTicker(r.cfg.Interval)
	defer ticker.Stop()

	for {
		if err := r.RunOnce(ctx); err != nil && !errorsIsContext(err) {
			r.log.Error("upload reaper pass failed", slog.Any("error", err))
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Reaper) RunOnce(ctx context.Context) error {
	now := r.clock.Now().UTC()
	expired, err := r.repo.ListExpiredActiveUploadSessions(ctx, now, r.cfg.BatchSize)
	if err != nil {
		return fmt.Errorf("list expired upload sessions: %w", err)
	}

	for _, session := range expired {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		claimed, err := r.repo.MarkUploadSessionCleanupInProgress(ctx, session.OwnerUserID, session.ID, now)
		if err != nil {
			if err == domainfiles.ErrUploadSessionNotFound {
				continue
			}
			r.log.Warn("failed to claim expired upload session cleanup",
				slog.Any("error", err),
				slog.String("upload_session_id", session.ID.String()),
			)
			continue
		}

		cleanupErr := r.cleanupSessionObjects(ctx, claimed)
		if err := r.repo.MarkUploadSessionCleanupResult(ctx, claimed.OwnerUserID, claimed.ID, r.clock.Now().UTC(), cleanupErr); err != nil {
			r.log.Error("failed to persist upload cleanup result",
				slog.Any("error", err),
				slog.String("upload_session_id", claimed.ID.String()),
			)
		}
		if cleanupErr != nil {
			r.log.Warn("upload cleanup failed",
				slog.Any("error", cleanupErr),
				slog.String("upload_session_id", claimed.ID.String()),
			)
		}
	}
	return nil
}

func (r *Reaper) cleanupSessionObjects(ctx context.Context, session domainfiles.UploadSession) error {
	chunks, err := r.repo.ListUploadChunks(ctx, session.ID)
	if err != nil {
		return fmt.Errorf("list chunks for cleanup: %w", err)
	}

	for _, chunk := range chunks {
		if err := r.deleteWithRetry(ctx, chunk.StagingKey); err != nil {
			return fmt.Errorf("delete staged chunk %s: %w", chunk.StagingKey, err)
		}
	}
	if session.ObjectKey != nil && strings.TrimSpace(*session.ObjectKey) != "" {
		if err := r.deleteWithRetry(ctx, strings.TrimSpace(*session.ObjectKey)); err != nil {
			return fmt.Errorf("delete session object key %s: %w", strings.TrimSpace(*session.ObjectKey), err)
		}
	}
	return nil
}

func (r *Reaper) deleteWithRetry(ctx context.Context, key string) error {
	var lastErr error
	for attempt := 0; attempt < r.cfg.DeleteMaxRetries; attempt++ {
		if err := r.storage.Delete(ctx, key); err == nil {
			return nil
		} else {
			lastErr = err
		}
		backoff := time.Duration(attempt+1) * 200 * time.Millisecond
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
	return lastErr
}

func errorsIsContext(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
