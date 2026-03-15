package broker

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProcessedEvent tracks consumed Kafka messages for idempotency.
// Uses the message's IdempotencyKey (topic/partition/offset) as the unique key.
type ProcessedEvent struct {
	ID             uint      `gorm:"primaryKey;autoIncrement"`
	IdempotencyKey string    `gorm:"type:varchar(255);uniqueIndex;not null"`
	Topic          string    `gorm:"type:varchar(255);not null;index"`
	ProcessedAt    time.Time `gorm:"autoCreateTime"`
}

// TableName specifies the table name for GORM.
func (ProcessedEvent) TableName() string {
	return "processed_events"
}

// IdempotentHandler wraps a Handler with deduplication. Before processing a
// message, it checks whether the message's IdempotencyKey has already been
// recorded. If so, the message is skipped silently.
func IdempotentHandler(db *gorm.DB, handler Handler, logger *zap.Logger) Handler {
	return func(msg *Message) error {
		key := msg.IdempotencyKey()

		// Try to insert — ON CONFLICT DO NOTHING.
		result := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&ProcessedEvent{
			IdempotencyKey: key,
			Topic:          string(msg.Topic),
		})

		if result.Error != nil {
			return fmt.Errorf("broker: idempotency check failed: %w", result.Error)
		}

		// If no row was inserted, the event was already processed.
		if result.RowsAffected == 0 {
			logger.Debug("skipping duplicate message",
				zap.String("idempotency_key", key),
				zap.String("topic", string(msg.Topic)),
			)
			return nil
		}

		return handler(msg)
	}
}

// IdempotentBatchHandler wraps a BatchHandler with deduplication. It filters
// out messages that have already been processed, then passes only new messages
// to the underlying handler. Uses bulk insert with ON CONFLICT DO NOTHING.
func IdempotentBatchHandler(db *gorm.DB, handler BatchHandler, logger *zap.Logger) BatchHandler {
	return func(msgs []*Message) error {
		if len(msgs) == 0 {
			return nil
		}

		// Build events for bulk insert.
		events := make([]ProcessedEvent, len(msgs))
		keyIndex := make(map[string]int, len(msgs))
		for i, msg := range msgs {
			key := msg.IdempotencyKey()
			events[i] = ProcessedEvent{
				IdempotencyKey: key,
				Topic:          string(msg.Topic),
			}
			keyIndex[key] = i
		}

		// Bulk insert with ON CONFLICT DO NOTHING.
		result := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&events)
		if result.Error != nil {
			return fmt.Errorf("broker: idempotency bulk check failed: %w", result.Error)
		}

		// If all rows were inserted, no duplicates — pass full batch.
		if result.RowsAffected == int64(len(msgs)) {
			return handler(msgs)
		}

		// Some duplicates exist — find which keys were actually inserted
		// by querying for existing keys.
		keys := make([]string, 0, len(msgs))
		for _, e := range events {
			keys = append(keys, e.IdempotencyKey)
		}

		var existing []ProcessedEvent
		if err := db.Where("idempotency_key IN ?", keys).Find(&existing).Error; err != nil {
			return fmt.Errorf("broker: failed to query processed events: %w", err)
		}

		// Build set of already-processed keys (those that existed before this batch).
		// Since we just inserted, we need to compare timestamps or use the insert result.
		// Simpler approach: re-query and filter by those that were just created.
		newMsgs := make([]*Message, 0, len(msgs))
		existingKeys := make(map[string]bool, len(existing))
		for _, e := range existing {
			existingKeys[e.IdempotencyKey] = true
		}

		// All existing keys include both old and newly inserted.
		// We inserted result.RowsAffected new ones. Filter: if a key exists
		// and was part of our batch, it was either newly inserted or already present.
		// We just pass through only the ones that got inserted (RowsAffected count).
		// Since ON CONFLICT DO NOTHING, the newly inserted ones have IDs assigned.
		// We rely on: newly inserted events have non-zero IDs from Create.
		insertedKeys := make(map[string]bool)
		for _, e := range events {
			if e.ID != 0 {
				insertedKeys[e.IdempotencyKey] = true
			}
		}

		for _, msg := range msgs {
			if insertedKeys[msg.IdempotencyKey()] {
				newMsgs = append(newMsgs, msg)
			} else {
				logger.Debug("skipping duplicate message in batch",
					zap.String("idempotency_key", msg.IdempotencyKey()),
					zap.String("topic", string(msg.Topic)),
				)
			}
		}

		if len(newMsgs) == 0 {
			return nil
		}

		return handler(newMsgs)
	}
}

// CleanupProcessedEvents removes processed events older than the retention
// period. Should be called periodically to prevent unbounded table growth.
func CleanupProcessedEvents(ctx context.Context, db *gorm.DB, retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention)

	result := db.WithContext(ctx).
		Where("processed_at < ?", cutoff).
		Delete(&ProcessedEvent{})

	if result.Error != nil {
		return 0, fmt.Errorf("broker: failed to cleanup processed events: %w", result.Error)
	}

	return result.RowsAffected, nil
}
