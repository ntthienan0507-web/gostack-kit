package broker

import (
	"crypto/tls"

	"github.com/IBM/sarama"
)

// Config holds Kafka connection configuration.
type Config struct {
	Brokers              []string   `mapstructure:"brokers"`
	ConsumerGroup        string     `mapstructure:"consumer_group"`
	TLS                  bool       `mapstructure:"tls"`
	SASL                 SASLConfig `mapstructure:"sasl"`
	EnableCircuitBreaker bool       `mapstructure:"enable_circuit_breaker"`
}

// SASLConfig holds SASL authentication settings.
type SASLConfig struct {
	Enable    bool   `mapstructure:"enable"`
	Mechanism string `mapstructure:"mechanism"` // SCRAM-SHA-256, SCRAM-SHA-512, PLAIN
	Username  string `mapstructure:"username"`
	Password  string `mapstructure:"password"`
}

// applySASL configures SASL authentication on the sarama config.
func applySASL(saramaCfg *sarama.Config, cfg SASLConfig) {
	if !cfg.Enable {
		return
	}

	saramaCfg.Net.SASL.Enable = true
	saramaCfg.Net.SASL.User = cfg.Username
	saramaCfg.Net.SASL.Password = cfg.Password

	switch cfg.Mechanism {
	case "SCRAM-SHA-256":
		saramaCfg.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
		saramaCfg.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
			return &scramClient{HashGeneratorFcn: SHA256}
		}
	case "SCRAM-SHA-512":
		saramaCfg.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
		saramaCfg.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
			return &scramClient{HashGeneratorFcn: SHA512}
		}
	default:
		saramaCfg.Net.SASL.Mechanism = sarama.SASLTypePlaintext
	}
}

// newSaramaConfig creates a base sarama.Config from Config.
func newSaramaConfig(cfg Config) *sarama.Config {
	saramaCfg := sarama.NewConfig()
	saramaCfg.Version = sarama.V2_6_0_0

	if cfg.TLS {
		saramaCfg.Net.TLS.Enable = true
		saramaCfg.Net.TLS.Config = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	applySASL(saramaCfg, cfg.SASL)

	return saramaCfg
}
