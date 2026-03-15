// Package broker provides Kafka producer/consumer with transactional outbox pattern.
package broker

import (
	"encoding/json"
	"fmt"
	"time"
)

// Message represents a Kafka message.
type Message struct {
	Topic     Topic             `json:"topic"`
	Key       string            `json:"key"`
	Value     json.RawMessage   `json:"value"`
	Headers   map[string]string `json:"headers,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	Offset    int64             `json:"offset"`
	Partition int32             `json:"partition"`
}

// IdempotencyKey returns a unique identifier for this message based on its
// topic, partition, and offset. This key is stable and can be used for
// consumer-side deduplication.
func (m *Message) IdempotencyKey() string {
	return fmt.Sprintf("%s/%d/%d", m.Topic, m.Partition, m.Offset)
}

// Handler processes a single Kafka message.
// Implementations should be idempotent — the same message may be delivered
// more than once due to consumer rebalancing or retries.
type Handler func(msg *Message) error

// BatchHandler processes a batch of Kafka messages atomically.
// Implementations should be idempotent. Use IdempotencyKey() or the
// IdempotentBatchHandler wrapper to deduplicate within a batch.
type BatchHandler func(msgs []*Message) error

// BatchOpts configures batch consumption behavior.
type BatchOpts struct {
	// Size is the maximum number of messages per batch.
	Size int
	// Interval is the maximum time to wait before flushing an incomplete batch.
	Interval time.Duration
}

// Producer publishes messages to Kafka topics.
type Producer interface {
	// Publish sends a message to the given topic with the specified key and value.
	// The key is validated against the topic's registered key strategy.
	Publish(topic Topic, key string, value []byte, headers map[string]string) error
	// Close shuts down the producer and flushes pending messages.
	Close() error
}

// Consumer subscribes to Kafka topics and processes messages.
type Consumer interface {
	// Subscribe registers a handler for the given topic.
	Subscribe(topic Topic, handler Handler)
	// SubscribeBatch registers a batch handler for the given topic.
	SubscribeBatch(topic Topic, handler BatchHandler, opts BatchOpts)
	// Start begins consuming messages. Blocks until Close is called or an error occurs.
	Start() error
	// Close gracefully shuts down the consumer.
	Close() error
}
