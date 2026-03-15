package app

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/ntthienan0507-web/gostack-kit/pkg/broker"
	"github.com/ntthienan0507-web/gostack-kit/pkg/config"
	"github.com/ntthienan0507-web/gostack-kit/pkg/crypto"
	"github.com/ntthienan0507-web/gostack-kit/pkg/external/elasticsearch"
	"github.com/ntthienan0507-web/gostack-kit/pkg/external/firebase"
	"github.com/ntthienan0507-web/gostack-kit/pkg/external/icewarp"
	"github.com/ntthienan0507-web/gostack-kit/pkg/external/sendgrid"
	"github.com/ntthienan0507-web/gostack-kit/pkg/external/stripe"
)

// Services holds all external service clients.
// Initialized based on which env vars are configured — unconfigured services are nil.
// Modules check for nil before using: if app.Services.SendGrid != nil { ... }
type Services struct {
	Encryptor     *crypto.Encryptor
	SendGrid      *sendgrid.Client
	Stripe        *stripe.Client
	IceWarp       *icewarp.Client
	Firebase      *firebase.Client
	Elasticsearch *elasticsearch.Client
	KafkaProducer broker.Producer
	KafkaConsumer broker.Consumer
}

// initServices creates external service clients based on config.
// Only services with configured credentials are initialized.
// Missing config = nil client = service not available (not an error).
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

	// SendGrid
	if cfg.SendGridAPIKey != "" {
		s.SendGrid = sendgrid.New(sendgrid.Config{
			BaseURL:   cfg.SendGridURL,
			APIKey:    cfg.SendGridAPIKey,
			FromEmail: cfg.SendGridFrom,
		}, logger)
		logger.Info("sendgrid client initialized")
	}

	// Stripe
	if cfg.StripeSecretKey != "" {
		s.Stripe = stripe.New(stripe.Config{
			BaseURL:   cfg.StripeURL,
			SecretKey: cfg.StripeSecretKey,
		}, logger)
		logger.Info("stripe client initialized")
	}

	// IceWarp
	if cfg.IceWarpURL != "" && cfg.IceWarpUsername != "" {
		s.IceWarp = icewarp.New(icewarp.Config{
			URL:      cfg.IceWarpURL,
			Username: cfg.IceWarpUsername,
			Password: cfg.IceWarpPassword,
			From:     cfg.IceWarpFrom,
		}, logger)
		logger.Info("icewarp client initialized")
	}

	// Firebase
	if cfg.FirebaseCredentialsFile != "" {
		fb, err := firebase.New(ctx, firebase.Config{
			CredentialsFile: cfg.FirebaseCredentialsFile,
			ProjectID:       cfg.FirebaseProjectID,
		}, logger)
		if err != nil {
			return nil, fmt.Errorf("init firebase: %w", err)
		}
		s.Firebase = fb
		logger.Info("firebase client initialized")
	}

	// Elasticsearch
	if cfg.ElasticURLs != "" {
		es, err := elasticsearch.New(elasticsearch.Config{
			URLs:     cfg.ElasticURLList(),
			Username: cfg.ElasticUsername,
			Password: cfg.ElasticPassword,
			APIKey:   cfg.ElasticAPIKey,
		}, logger)
		if err != nil {
			return nil, fmt.Errorf("init elasticsearch: %w", err)
		}
		s.Elasticsearch = es
		logger.Info("elasticsearch client initialized")
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

// shutdownServices cleanly releases external service resources.
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
