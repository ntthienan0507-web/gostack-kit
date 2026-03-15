package app

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/ntthienan0507-web/gostack-kit/pkg/broker"
	"github.com/ntthienan0507-web/gostack-kit/pkg/config"
	"github.com/ntthienan0507-web/gostack-kit/pkg/crypto"
)

type Services struct {
	Encryptor     *crypto.Encryptor
	KafkaProducer broker.Producer
	KafkaConsumer broker.Consumer
	external      map[string]any
}

type serviceInitFunc func(ctx context.Context, cfg *config.Config, logger *zap.Logger, s *Services) error

var optionalServiceInits []serviceInitFunc

func registerOptionalService(fn serviceInitFunc) {
	optionalServiceInits = append(optionalServiceInits, fn)
}

func (s *Services) register(name string, svc any) {
	if s.external == nil {
		s.external = make(map[string]any)
	}
	s.external[name] = svc
}

func (s *Services) lookup(name string) any {
	if s.external == nil {
		return nil
	}
	return s.external[name]
}

func initServices(ctx context.Context, cfg *config.Config, logger *zap.Logger) (*Services, error) {
	s := &Services{}

	// Encryption
	if cfg.EncryptionKey != "" {
		enc, err := crypto.NewEncryptor(cfg.EncryptionKey)
		if err != nil {
			return nil, fmt.Errorf("init encryptor: %w", err)
		}
		s.Encryptor = enc
		logger.Info("encryption enabled")
	}

	// Optional external services (registered by services_*.go files)
	for _, initFn := range optionalServiceInits {
		if err := initFn(ctx, cfg, logger, s); err != nil {
			return nil, err
		}
	}

	// Kafka Producer
	if brokers := cfg.KafkaBrokerList(); len(brokers) > 0 {
		producer, err := broker.NewProducer(broker.Config{
			Brokers:              brokers,
			TLS:                  cfg.KafkaTLS,
			EnableCircuitBreaker: true,
			SASL: broker.SASLConfig{
				Enable:    cfg.KafkaSASLEnable,
				Mechanism: cfg.KafkaSASLMechanism,
				Username:  cfg.KafkaSASLUsername,
				Password:  cfg.KafkaSASLPassword,
			},
		}, logger)
		if err != nil {
			logger.Warn("kafka producer init failed — events will not be published", zap.Error(err))
		} else {
			s.KafkaProducer = producer
			logger.Info("kafka producer initialized")
		}

		// Kafka Consumer
		consumer, err := broker.NewConsumer(broker.ConsumerConfig{
			Config: broker.Config{
				Brokers:       brokers,
				ConsumerGroup: cfg.KafkaConsumerGroup,
				TLS:           cfg.KafkaTLS,
				SASL: broker.SASLConfig{
					Enable:    cfg.KafkaSASLEnable,
					Mechanism: cfg.KafkaSASLMechanism,
					Username:  cfg.KafkaSASLUsername,
					Password:  cfg.KafkaSASLPassword,
				},
			},
			Workers: cfg.WorkerCount,
		}, logger)
		if err != nil {
			logger.Warn("kafka consumer init failed", zap.Error(err))
		} else {
			s.KafkaConsumer = consumer
			logger.Info("kafka consumer initialized")
		}
	}

	return s, nil
}

func (s *Services) shutdown(logger *zap.Logger) {
	if s == nil {
		return
	}
	if s.KafkaProducer != nil {
		if err := s.KafkaProducer.Close(); err != nil {
			logger.Error("close kafka producer", zap.Error(err))
		}
	}
	if s.KafkaConsumer != nil {
		if err := s.KafkaConsumer.Close(); err != nil {
			logger.Error("close kafka consumer", zap.Error(err))
		}
	}
}
