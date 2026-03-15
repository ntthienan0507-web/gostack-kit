package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

const (
	defaultWorkers         = 4
	defaultBufferPerWorker = 256
	defaultFetchMinBytes   = 1
	defaultFetchMaxWait    = 500 * time.Millisecond
	defaultFetchMaxBytes   = 1024 * 1024 // 1 MB
	defaultCommitInterval  = 5 * time.Second
)

// ConsumerConfig extends Config with consumer-specific settings.
type ConsumerConfig struct {
	Config           `mapstructure:",squash"`
	Workers          int           `mapstructure:"workers"`
	BufferPerWorker  int           `mapstructure:"buffer_per_worker"`
	FetchMinBytes    int32         `mapstructure:"fetch_min_bytes"`
	FetchMaxWait     time.Duration `mapstructure:"fetch_max_wait"`
	FetchMaxBytes    int32         `mapstructure:"fetch_max_bytes"`
}

type topicSubscription struct {
	handler    Handler
	dispatcher *dispatcher
	batcher    *Batcher
}

// kafkaConsumer implements Consumer using sarama's consumer group.
type kafkaConsumer struct {
	cfg           ConsumerConfig
	logger        *zap.Logger
	subscriptions map[Topic]*topicSubscription
	group         sarama.ConsumerGroup
	cancel        context.CancelFunc
	ctx           context.Context
	mu            sync.Mutex
	started       bool
}

// NewConsumer creates a new Kafka consumer.
func NewConsumer(cfg ConsumerConfig, logger *zap.Logger) (Consumer, error) {
	if cfg.Workers <= 0 {
		cfg.Workers = defaultWorkers
	}
	if cfg.BufferPerWorker <= 0 {
		cfg.BufferPerWorker = defaultBufferPerWorker
	}
	if cfg.FetchMinBytes <= 0 {
		cfg.FetchMinBytes = defaultFetchMinBytes
	}
	if cfg.FetchMaxWait <= 0 {
		cfg.FetchMaxWait = defaultFetchMaxWait
	}
	if cfg.FetchMaxBytes <= 0 {
		cfg.FetchMaxBytes = defaultFetchMaxBytes
	}

	saramaCfg := newSaramaConfig(cfg.Config)
	saramaCfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyRoundRobin(),
	}
	saramaCfg.Consumer.Offsets.AutoCommit.Enable = false
	saramaCfg.Consumer.Fetch.Min = cfg.FetchMinBytes
	saramaCfg.Consumer.MaxWaitTime = cfg.FetchMaxWait
	saramaCfg.Consumer.Fetch.Max = cfg.FetchMaxBytes

	group, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.ConsumerGroup, saramaCfg)
	if err != nil {
		return nil, fmt.Errorf("broker: failed to create consumer group: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &kafkaConsumer{
		cfg:           cfg,
		logger:        logger,
		subscriptions: make(map[Topic]*topicSubscription),
		group:         group,
		ctx:           ctx,
		cancel:        cancel,
	}, nil
}

// Subscribe registers a handler for a topic. The handler is invoked through
// a key-sharded dispatcher for per-key ordering.
func (c *kafkaConsumer) Subscribe(topic Topic, handler Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()

	d := newDispatcher(c.cfg.Workers, c.cfg.BufferPerWorker, handler, c.logger)
	c.subscriptions[topic] = &topicSubscription{
		handler:    handler,
		dispatcher: d,
	}

	c.logger.Info("subscribed to topic", zap.String("topic", string(topic)))
}

// SubscribeBatch registers a batch handler for a topic. Messages are
// accumulated by a Batcher and dispatched through the key-sharded dispatcher.
func (c *kafkaConsumer) SubscribeBatch(topic Topic, handler BatchHandler, opts BatchOpts) {
	c.mu.Lock()
	defer c.mu.Unlock()

	batcher := NewBatcher(handler, opts, c.logger)
	d := newDispatcher(c.cfg.Workers, c.cfg.BufferPerWorker, batcher.AsHandler(), c.logger)

	c.subscriptions[topic] = &topicSubscription{
		dispatcher: d,
		batcher:    batcher,
	}

	c.logger.Info("subscribed to topic (batch)",
		zap.String("topic", string(topic)),
		zap.Int("batch_size", opts.Size),
		zap.Duration("batch_interval", opts.Interval),
	)
}

// Start begins consuming messages. Blocks until Close is called.
func (c *kafkaConsumer) Start() error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return fmt.Errorf("broker: consumer already started")
	}
	c.started = true
	c.mu.Unlock()

	topics := make([]string, 0, len(c.subscriptions))
	for t := range c.subscriptions {
		topics = append(topics, string(t))
	}

	handler := &consumerGroupHandler{
		subscriptions: c.subscriptions,
		logger:        c.logger,
	}

	for {
		if err := c.group.Consume(c.ctx, topics, handler); err != nil {
			c.logger.Error("consumer group error", zap.Error(err))
		}

		if c.ctx.Err() != nil {
			return nil
		}
	}
}

// Close gracefully shuts down the consumer.
func (c *kafkaConsumer) Close() error {
	c.cancel()

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, sub := range c.subscriptions {
		if sub.batcher != nil {
			sub.batcher.Shutdown()
		}
		if sub.dispatcher != nil {
			sub.dispatcher.Shutdown()
		}
	}

	return c.group.Close()
}

// consumerGroupHandler implements sarama.ConsumerGroupHandler.
type consumerGroupHandler struct {
	subscriptions map[Topic]*topicSubscription
	logger        *zap.Logger
}

func (h *consumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

// ConsumeClaim processes messages from a partition claim.
// Auto-commit is disabled; offsets are committed periodically.
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	ticker := time.NewTicker(defaultCommitInterval)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}

			topic := Topic(msg.Topic)
			sub, exists := h.subscriptions[topic]
			if !exists {
				h.logger.Warn("no subscription for topic", zap.String("topic", msg.Topic))
				continue
			}

			headers := make(map[string]string)
			for _, hdr := range msg.Headers {
				headers[string(hdr.Key)] = string(hdr.Value)
			}

			brokerMsg := &Message{
				Topic:     topic,
				Key:       string(msg.Key),
				Value:     json.RawMessage(msg.Value),
				Headers:   headers,
				Timestamp: msg.Timestamp,
				Offset:    msg.Offset,
				Partition: msg.Partition,
			}

			sub.dispatcher.Dispatch(brokerMsg)
			session.MarkMessage(msg, "")

		case <-ticker.C:
			session.Commit()

		case <-session.Context().Done():
			return nil
		}
	}
}
