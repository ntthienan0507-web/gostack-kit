package broker

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const (
	StatusPending   = "pending"
	StatusPublished = "published"
	StatusFailed    = "failed"

	DefaultMaxRetries = 5
)

// OutboxEntry is a GORM model for the transactional outbox pattern.
// Messages are written to this table within the same database transaction
// as the business operation, then a Relay publishes them to Kafka.
type OutboxEntry struct {
	ID          uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	Topic       string          `gorm:"type:varchar(255);not null;index" json:"topic"`
	Key         string          `gorm:"type:varchar(255);not null" json:"key"`
	Payload     json.RawMessage `gorm:"type:jsonb;not null" json:"payload"`
	Headers     json.RawMessage `gorm:"type:jsonb" json:"headers,omitempty"`
	Status      string          `gorm:"type:varchar(20);not null;default:'pending';index" json:"status"`
	RetryCount  int             `gorm:"not null;default:0" json:"retry_count"`
	LastError   string          `gorm:"type:text" json:"last_error,omitempty"`
	NextRetryAt *time.Time      `gorm:"index" json:"next_retry_at,omitempty"`
	CreatedAt   time.Time       `gorm:"autoCreateTime" json:"created_at"`
	PublishedAt *time.Time      `json:"published_at,omitempty"`
}

// TableName specifies the table name for GORM.
func (OutboxEntry) TableName() string {
	return "outbox_entries"
}

// WriteOutbox inserts a new outbox entry within the given transaction.
// The key is validated against the topic's registered key strategy.
func WriteOutbox(tx *gorm.DB, topic Topic, key string, payload []byte, headers map[string]string) error {
	if err := ValidateKey(topic, key); err != nil {
		return fmt.Errorf("broker: outbox validation failed: %w", err)
	}

	var headersJSON json.RawMessage
	if len(headers) > 0 {
		var err error
		headersJSON, err = json.Marshal(headers)
		if err != nil {
			return fmt.Errorf("broker: failed to marshal headers: %w", err)
		}
	}

	entry := OutboxEntry{
		Topic:   string(topic),
		Key:     key,
		Payload: payload,
		Headers: headersJSON,
		Status:  StatusPending,
	}

	if err := tx.Create(&entry).Error; err != nil {
		return fmt.Errorf("broker: failed to write outbox entry: %w", err)
	}

	return nil
}
