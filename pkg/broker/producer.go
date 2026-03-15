package broker

import (
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/ntthienan0507-web/gostack-kit/pkg/circuitbreaker"
	"go.uber.org/zap"
)

// ProduceError is returned when a publish fails. Retryable indicates whether
// the caller should retry the operation.
type ProduceError struct {
	Topic     Topic
	Key       string
	Err       error
	Retryable bool
}

func (e *ProduceError) Error() string {
	return fmt.Sprintf("broker: failed to publish to %s (key=%s, retryable=%v): %v",
		e.Topic, e.Key, e.Retryable, e.Err)
}

func (e *ProduceError) Unwrap() error {
	return e.Err
}

// kafkaProducer implements Producer using a sarama SyncProducer.
type kafkaProducer struct {
	producer sarama.SyncProducer
	cb       *circuitbreaker.CircuitBreaker
	logger   *zap.Logger
}

// NewProducer creates a new Kafka producer.
func NewProducer(cfg Config, logger *zap.Logger) (Producer, error) {
	saramaCfg := newSaramaConfig(cfg)
	saramaCfg.Producer.RequiredAcks = sarama.WaitForAll
	saramaCfg.Producer.Retry.Max = 3
	saramaCfg.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer(cfg.Brokers, saramaCfg)
	if err != nil {
		return nil, fmt.Errorf("broker: failed to create producer: %w", err)
	}

	p := &kafkaProducer{
		producer: producer,
		logger:   logger,
	}

	if cfg.EnableCircuitBreaker {
		p.cb = circuitbreaker.New(circuitbreaker.DefaultConfig("kafka-producer"), logger)
	}

	return p, nil
}

// Publish sends a message to the given topic. The key is validated against
// the topic's registered key strategy before sending.
func (p *kafkaProducer) Publish(topic Topic, key string, value []byte, headers map[string]string) error {
	if err := ValidateKey(topic, key); err != nil {
		return &ProduceError{
			Topic:     topic,
			Key:       key,
			Err:       err,
			Retryable: false,
		}
	}

	msg := &sarama.ProducerMessage{
		Topic:     string(topic),
		Value:     sarama.ByteEncoder(value),
		Timestamp: time.Now(),
	}

	if key != "" {
		msg.Key = sarama.StringEncoder(key)
	}

	if len(headers) > 0 {
		for k, v := range headers {
			msg.Headers = append(msg.Headers, sarama.RecordHeader{
				Key:   []byte(k),
				Value: []byte(v),
			})
		}
	}

	send := func() error {
		_, _, err := p.producer.SendMessage(msg)
		return err
	}

	var err error
	if p.cb != nil {
		err = circuitbreaker.ExecuteVoid(p.cb, send)
	} else {
		err = send()
	}

	if err != nil {
		retryable := IsRetryableKafkaError(err)
		p.logger.Error("failed to publish message",
			zap.String("topic", string(topic)),
			zap.String("key", key),
			zap.Bool("retryable", retryable),
			zap.Error(err),
		)
		return &ProduceError{
			Topic:     topic,
			Key:       key,
			Err:       err,
			Retryable: retryable,
		}
	}

	p.logger.Debug("message published",
		zap.String("topic", string(topic)),
		zap.String("key", key),
	)
	return nil
}

// Close shuts down the producer.
func (p *kafkaProducer) Close() error {
	return p.producer.Close()
}
