package calendar

import (
	"context"
	"log/slog"
	"time"
)

type ReminderProcessorRepository interface {
	ProcessDueReminders(ctx context.Context, now time.Time, batchSize int) (int, error)
}

type ReminderProcessor struct {
	repo      ReminderProcessorRepository
	interval  time.Duration
	batchSize int
	log       *slog.Logger
}

func NewReminderProcessor(repo ReminderProcessorRepository, interval time.Duration, batchSize int, log *slog.Logger) *ReminderProcessor {
	if batchSize <= 0 {
		batchSize = 200
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if log == nil {
		log = slog.Default()
	}
	return &ReminderProcessor{
		repo:      repo,
		interval:  interval,
		batchSize: batchSize,
		log:       log,
	}
}

func (p *ReminderProcessor) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.processOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.processOnce(ctx)
		}
	}
}

func (p *ReminderProcessor) processOnce(ctx context.Context) {
	processed, err := p.repo.ProcessDueReminders(ctx, time.Now().UTC(), p.batchSize)
	if err != nil {
		p.log.Warn("calendar reminder scan failed", slog.Any("error", err))
		return
	}
	if processed > 0 {
		p.log.Info("calendar reminders processed", slog.Int("processed", processed))
	}
}
