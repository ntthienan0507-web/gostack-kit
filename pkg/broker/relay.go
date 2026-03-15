package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/ntthienan0507-web/go-api-template/pkg/retry"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Relay polls the outbox table and publishes pending entries to Kafka.
type Relay struct {
	db       *gorm.DB
	producer Producer
	logger   *zap.Logger
	pollInterval time.Duration
	batchSize    int
}

// RelayConfig configures the outbox relay.
type RelayConfig struct {
	PollInterval time.Duration
	BatchSize    int
}

// NewRelay creates a new outbox Relay.
func NewRelay(db *gorm.DB, producer Producer, logger *zap.Logger, cfg RelayConfig) *Relay {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 1 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}

	return &Relay{
		db:           db,
		producer:     producer,
		logger:       logger,
		pollInterval: cfg.PollInterval,
		batchSize:    cfg.BatchSize,
	}
}

// Run starts the relay loop. It blocks until the context is cancelled.
func (r *Relay) Run(ctx context.Context) error {
	r.logger.Info("outbox relay started",
		zap.Duration("poll_interval", r.pollInterval),
		zap.Int("batch_size", r.batchSize),
	)

	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("outbox relay stopped")
			return nil
		case <-ticker.C:
			if err := r.poll(ctx); err != nil {
				r.logger.Error("outbox relay poll error", zap.Error(err))
			}
		}
	}
}

func (r *Relay) poll(ctx context.Context) error {
	var entries []OutboxEntry
	err := r.db.WithContext(ctx).
		Where("status = ? AND (next_retry_at IS NULL OR next_retry_at <= ?)", StatusPending, time.Now()).
		Order("id ASC").
		Limit(r.batchSize).
		Find(&entries).Error
	if err != nil {
		return fmt.Errorf("broker: failed to query outbox: %w", err)
	}

	for i := range entries {
		if ctx.Err() != nil {
			return nil
		}
		r.publishWithRetry(ctx, &entries[i])
	}

	return nil
}

func (r *Relay) publishWithRetry(ctx context.Context, entry *OutboxEntry) {
	var headers map[string]string
	if len(entry.Headers) > 0 {
		if err := json.Unmarshal(entry.Headers, &headers); err != nil {
			r.logger.Error("failed to unmarshal outbox headers",
				zap.Uint("entry_id", entry.ID),
				zap.Error(err),
			)
			r.markFailed(entry, fmt.Errorf("invalid headers JSON: %w", err))
			return
		}
	}

	err := retry.Do(ctx, retry.DefaultConfig, func() error {
		return r.producer.Publish(Topic(entry.Topic), entry.Key, entry.Payload, headers)
	})

	if err != nil {
		var produceErr *ProduceError
		if errors.As(err, &produceErr) && !produceErr.Retryable {
			r.markFailed(entry, err)
			return
		}

		// Retryable error — schedule for later retry.
		r.scheduleRetry(entry, err)
		return
	}

	r.markPublished(entry)
}

func (r *Relay) markPublished(entry *OutboxEntry) {
	now := time.Now()
	err := r.db.Model(entry).Updates(map[string]interface{}{
		"status":       StatusPublished,
		"published_at": now,
		"last_error":   "",
	}).Error
	if err != nil {
		r.logger.Error("failed to mark outbox entry as published",
			zap.Uint("entry_id", entry.ID),
			zap.Error(err),
		)
	}
}

func (r *Relay) markFailed(entry *OutboxEntry, publishErr error) {
	err := r.db.Model(entry).Updates(map[string]interface{}{
		"status":     StatusFailed,
		"last_error": publishErr.Error(),
	}).Error
	if err != nil {
		r.logger.Error("failed to mark outbox entry as failed",
			zap.Uint("entry_id", entry.ID),
			zap.Error(err),
		)
	}

	r.logger.Warn("outbox entry permanently failed",
		zap.Uint("entry_id", entry.ID),
		zap.String("topic", entry.Topic),
		zap.Error(publishErr),
	)
}

func (r *Relay) scheduleRetry(entry *OutboxEntry, publishErr error) {
	entry.RetryCount++

	if entry.RetryCount >= DefaultMaxRetries {
		r.markFailed(entry, fmt.Errorf("max retries (%d) exceeded: %w", DefaultMaxRetries, publishErr))
		return
	}

	// Exponential backoff: 2^retryCount seconds, capped at 5 minutes.
	delay := time.Duration(math.Pow(2, float64(entry.RetryCount))) * time.Second
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	nextRetry := time.Now().Add(delay)

	err := r.db.Model(entry).Updates(map[string]interface{}{
		"retry_count":   entry.RetryCount,
		"last_error":    publishErr.Error(),
		"next_retry_at": nextRetry,
	}).Error
	if err != nil {
		r.logger.Error("failed to schedule outbox retry",
			zap.Uint("entry_id", entry.ID),
			zap.Error(err),
		)
	}

	r.logger.Info("outbox entry scheduled for retry",
		zap.Uint("entry_id", entry.ID),
		zap.Int("retry_count", entry.RetryCount),
		zap.Time("next_retry_at", nextRetry),
	)
}

// RetryFailed resets all failed outbox entries back to pending status
// so they will be picked up by the next poll cycle.
func (r *Relay) RetryFailed(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).
		Model(&OutboxEntry{}).
		Where("status = ?", StatusFailed).
		Updates(map[string]interface{}{
			"status":        StatusPending,
			"retry_count":   0,
			"last_error":    "",
			"next_retry_at": nil,
		})

	if result.Error != nil {
		return 0, fmt.Errorf("broker: failed to reset failed entries: %w", result.Error)
	}

	r.logger.Info("reset failed outbox entries", zap.Int64("count", result.RowsAffected))
	return result.RowsAffected, nil
}
