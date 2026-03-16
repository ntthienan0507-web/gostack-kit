package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
// Loaded once at startup, passed via constructors — NO global vars.
type Config struct {
	// Server
	ServerPort  int    `mapstructure:"SERVER_PORT"`
	ServerMode  string `mapstructure:"SERVER_MODE"`
	CORSOrigins string `mapstructure:"CORS_ORIGINS"` // comma-separated allowed origins, "*" for all

	// Database
	DBDriver   string `mapstructure:"DB_DRIVER"` // "sqlc" | "gorm" | "mongo"
	DBHost     string `mapstructure:"DB_HOST"`
	DBPort     int    `mapstructure:"DB_PORT"`
	DBUser     string `mapstructure:"DB_USER"`
	DBPassword string `mapstructure:"DB_PASSWORD"`
	DBName     string `mapstructure:"DB_NAME"`
	DBSSLMode  string `mapstructure:"DB_SSL_MODE"`
	DBMaxConns int32  `mapstructure:"DB_MAX_CONNS"`
	DBMinConns int32  `mapstructure:"DB_MIN_CONNS"`

	// MongoDB (only when DB_DRIVER=mongo)
	MongoURI_   string `mapstructure:"MONGO_URI"`
	MongoDBName string `mapstructure:"MONGO_DB_NAME"`

	// Redis
	RedisURL      string `mapstructure:"REDIS_URL"`
	RedisPoolSize int    `mapstructure:"REDIS_POOL_SIZE"`
	RedisMinIdle  int    `mapstructure:"REDIS_MIN_IDLE"`

	// Encryption
	EncryptionKey string `mapstructure:"ENCRYPTION_KEY"`

	// Auth
	AuthProvider   string        `mapstructure:"AUTH_PROVIDER"`
	JWTSecret      string        `mapstructure:"JWT_SECRET"`
	JWTExpiry      time.Duration `mapstructure:"JWT_EXPIRY"`
	JWTAlgorithm   string        `mapstructure:"JWT_ALGORITHM"`        // "HS256" (default) | "RS256"
	JWTPrivateKeyFile string     `mapstructure:"JWT_PRIVATE_KEY_FILE"` // path to PEM-encoded RSA private key
	JWTPublicKeyFile  string     `mapstructure:"JWT_PUBLIC_KEY_FILE"`  // path to PEM-encoded RSA public key
	JWTKeyID       string        `mapstructure:"JWT_KEY_ID"`           // key ID (kid) for key rotation

	// Keycloak (only when AUTH_PROVIDER=keycloak)
	KeycloakHost         string `mapstructure:"KEYCLOAK_HOST"`
	KeycloakRealm        string `mapstructure:"KEYCLOAK_REALM"`
	KeycloakClientID     string `mapstructure:"KEYCLOAK_CLIENT_ID"`
	KeycloakClientSecret string `mapstructure:"KEYCLOAK_CLIENT_SECRET"`

	// OAuth2/OIDC (only when AUTH_PROVIDER=oauth2)
	OAuth2IssuerURL   string `mapstructure:"OAUTH2_ISSUER_URL"`   // OIDC issuer for discovery
	OAuth2ClientID    string `mapstructure:"OAUTH2_CLIENT_ID"`
	OAuth2ClientSecret string `mapstructure:"OAUTH2_CLIENT_SECRET"`

	// SAML 2.0 (only when AUTH_PROVIDER=saml)
	SAMLSPEntityID       string `mapstructure:"SAML_SP_ENTITY_ID"`       // SP entity ID / audience
	SAMLSPACS            string `mapstructure:"SAML_SP_ACS_URL"`         // Assertion Consumer Service URL
	SAMLSPCertFile       string `mapstructure:"SAML_SP_CERT_FILE"`       // SP certificate PEM
	SAMLSPKeyFile        string `mapstructure:"SAML_SP_KEY_FILE"`        // SP private key PEM
	SAMLIDPMetadataURL   string `mapstructure:"SAML_IDP_METADATA_URL"`   // IdP metadata URL (auto-fetch)
	SAMLIDPMetadataFile  string `mapstructure:"SAML_IDP_METADATA_FILE"`  // IdP metadata XML file (offline)

	// gRPC
	GRPCEnabled bool `mapstructure:"GRPC_ENABLED"`
	GRPCPort    int  `mapstructure:"GRPC_PORT"`

	// Network
	OutboundIP string `mapstructure:"OUTBOUND_IP"`

	// External services — SendGrid
	SendGridURL    string `mapstructure:"SENDGRID_URL"`
	SendGridAPIKey string `mapstructure:"SENDGRID_API_KEY"`
	SendGridFrom   string `mapstructure:"SENDGRID_FROM"`

	// External services — Stripe
	StripeURL       string `mapstructure:"STRIPE_URL"`
	StripeSecretKey string `mapstructure:"STRIPE_SECRET_KEY"`

	// External services — IceWarp
	IceWarpURL      string `mapstructure:"ICEWARP_URL"`
	IceWarpUsername string `mapstructure:"ICEWARP_USERNAME"`
	IceWarpPassword string `mapstructure:"ICEWARP_PASSWORD"`
	IceWarpFrom     string `mapstructure:"ICEWARP_FROM"`

	// Firebase
	FirebaseCredentialsFile string `mapstructure:"FIREBASE_CREDENTIALS_FILE"`
	FirebaseProjectID       string `mapstructure:"FIREBASE_PROJECT_ID"`

	// Elasticsearch
	ElasticURLs     string `mapstructure:"ELASTIC_URLS"`
	ElasticUsername string `mapstructure:"ELASTIC_USERNAME"`
	ElasticPassword string `mapstructure:"ELASTIC_PASSWORD"`
	ElasticAPIKey   string `mapstructure:"ELASTIC_API_KEY"`

	// Kafka
	KafkaBrokers       string `mapstructure:"KAFKA_BROKERS"`
	KafkaConsumerGroup string `mapstructure:"KAFKA_CONSUMER_GROUP"`
	KafkaTLS           bool   `mapstructure:"KAFKA_TLS"`
	KafkaSASLEnable    bool   `mapstructure:"KAFKA_SASL_ENABLE"`
	KafkaSASLMechanism string `mapstructure:"KAFKA_SASL_MECHANISM"`
	KafkaSASLUsername  string `mapstructure:"KAFKA_SASL_USERNAME"`
	KafkaSASLPassword  string `mapstructure:"KAFKA_SASL_PASSWORD"`

	// Worker pool
	WorkerCount     int `mapstructure:"WORKER_COUNT"`
	WorkerQueueSize int `mapstructure:"WORKER_QUEUE_SIZE"`

	// Logging
	LogLevel  string `mapstructure:"LOG_LEVEL"`
	LogFormat string `mapstructure:"LOG_FORMAT"`

	// OpenTelemetry
	OTELEnabled     bool    `mapstructure:"OTEL_ENABLED"`
	OTELServiceName string  `mapstructure:"OTEL_SERVICE_NAME"`
	OTELEndpoint    string  `mapstructure:"OTEL_ENDPOINT"`
	OTELSampler     float64 `mapstructure:"OTEL_SAMPLER"`
}

// Validate checks that config values are within expected ranges and required
// fields are present. Called at startup to fail fast on misconfiguration.
func (c *Config) Validate() error {
	var errs []string
	fail := func(msg string) { errs = append(errs, msg) }

	// Server
	if c.ServerPort < 1 || c.ServerPort > 65535 {
		fail(fmt.Sprintf("SERVER_PORT=%d: must be 1–65535", c.ServerPort))
	}
	switch c.ServerMode {
	case "debug", "release", "test":
	default:
		fail(fmt.Sprintf("SERVER_MODE=%q: must be debug, release, or test", c.ServerMode))
	}

	// Database
	switch c.DBDriver {
	case "sqlc", "gorm", "mongo":
	default:
		fail(fmt.Sprintf("DB_DRIVER=%q: must be sqlc, gorm, or mongo", c.DBDriver))
	}
	if c.DBDriver != "mongo" {
		if c.DBHost == "" {
			fail("DB_HOST is required")
		}
		if c.DBPort < 1 || c.DBPort > 65535 {
			fail(fmt.Sprintf("DB_PORT=%d: must be 1–65535", c.DBPort))
		}
		if c.DBName == "" {
			fail("DB_NAME is required")
		}
		if c.DBMaxConns < 1 {
			fail(fmt.Sprintf("DB_MAX_CONNS=%d: must be >= 1", c.DBMaxConns))
		}
		if c.DBMinConns < 0 || c.DBMinConns > c.DBMaxConns {
			fail(fmt.Sprintf("DB_MIN_CONNS=%d: must be 0–DB_MAX_CONNS(%d)", c.DBMinConns, c.DBMaxConns))
		}
	}

	// Auth
	switch c.AuthProvider {
	case "jwt":
		switch c.JWTAlgorithm {
		case "HS256", "":
			if c.JWTSecret == "" {
				fail("JWT_SECRET is required when AUTH_PROVIDER=jwt and JWT_ALGORITHM=HS256")
			} else if len(c.JWTSecret) < 32 {
				fail(fmt.Sprintf("JWT_SECRET length=%d: must be >= 32 characters", len(c.JWTSecret)))
			}
		case "RS256":
			if c.JWTPrivateKeyFile == "" {
				fail("JWT_PRIVATE_KEY_FILE is required for RS256")
			}
			if c.JWTPublicKeyFile == "" {
				fail("JWT_PUBLIC_KEY_FILE is required for RS256")
			}
		default:
			fail(fmt.Sprintf("JWT_ALGORITHM=%q: must be HS256 or RS256", c.JWTAlgorithm))
		}
		if c.JWTExpiry <= 0 {
			fail(fmt.Sprintf("JWT_EXPIRY=%s: must be positive", c.JWTExpiry))
		}
	case "keycloak":
		if c.KeycloakHost == "" {
			fail("KEYCLOAK_HOST is required when AUTH_PROVIDER=keycloak")
		}
		if c.KeycloakRealm == "" {
			fail("KEYCLOAK_REALM is required when AUTH_PROVIDER=keycloak")
		}
		if c.KeycloakClientID == "" {
			fail("KEYCLOAK_CLIENT_ID is required when AUTH_PROVIDER=keycloak")
		}
	case "oauth2":
		if c.OAuth2IssuerURL == "" {
			fail("OAUTH2_ISSUER_URL is required when AUTH_PROVIDER=oauth2")
		}
		if c.OAuth2ClientID == "" {
			fail("OAUTH2_CLIENT_ID is required when AUTH_PROVIDER=oauth2")
		}
	case "saml":
		if c.SAMLSPEntityID == "" {
			fail("SAML_SP_ENTITY_ID is required when AUTH_PROVIDER=saml")
		}
		if c.SAMLSPACS == "" {
			fail("SAML_SP_ACS_URL is required when AUTH_PROVIDER=saml")
		}
		if c.SAMLSPCertFile == "" {
			fail("SAML_SP_CERT_FILE is required when AUTH_PROVIDER=saml")
		}
		if c.SAMLSPKeyFile == "" {
			fail("SAML_SP_KEY_FILE is required when AUTH_PROVIDER=saml")
		}
		if c.SAMLIDPMetadataURL == "" && c.SAMLIDPMetadataFile == "" {
			fail("SAML_IDP_METADATA_URL or SAML_IDP_METADATA_FILE is required when AUTH_PROVIDER=saml")
		}
	default:
		fail(fmt.Sprintf("AUTH_PROVIDER=%q: must be jwt, keycloak, oauth2, or saml", c.AuthProvider))
	}

	// gRPC
	if c.GRPCEnabled {
		if c.GRPCPort < 1 || c.GRPCPort > 65535 {
			fail(fmt.Sprintf("GRPC_PORT=%d: must be 1–65535", c.GRPCPort))
		}
		if c.GRPCPort == c.ServerPort {
			fail(fmt.Sprintf("GRPC_PORT=%d: must differ from SERVER_PORT=%d", c.GRPCPort, c.ServerPort))
		}
	}

	// Redis
	if c.RedisPoolSize < 1 {
		fail(fmt.Sprintf("REDIS_POOL_SIZE=%d: must be >= 1", c.RedisPoolSize))
	}
	if c.RedisMinIdle < 0 || c.RedisMinIdle > c.RedisPoolSize {
		fail(fmt.Sprintf("REDIS_MIN_IDLE=%d: must be 0–REDIS_POOL_SIZE(%d)", c.RedisMinIdle, c.RedisPoolSize))
	}

	// Encryption
	if c.EncryptionKey != "" && len(c.EncryptionKey) < 32 {
		fail(fmt.Sprintf("ENCRYPTION_KEY length=%d: must be >= 32 characters when set", len(c.EncryptionKey)))
	}

	// Workers
	if c.WorkerCount < 1 {
		fail(fmt.Sprintf("WORKER_COUNT=%d: must be >= 1", c.WorkerCount))
	}
	if c.WorkerQueueSize < 1 {
		fail(fmt.Sprintf("WORKER_QUEUE_SIZE=%d: must be >= 1", c.WorkerQueueSize))
	}

	// Logging
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		fail(fmt.Sprintf("LOG_LEVEL=%q: must be debug, info, warn, or error", c.LogLevel))
	}
	switch c.LogFormat {
	case "console", "json":
	default:
		fail(fmt.Sprintf("LOG_FORMAT=%q: must be console or json", c.LogFormat))
	}

	// OpenTelemetry
	if c.OTELSampler < 0 || c.OTELSampler > 1 {
		fail(fmt.Sprintf("OTEL_SAMPLER=%.2f: must be 0.0–1.0", c.OTELSampler))
	}

	// Kafka SASL
	if c.KafkaSASLEnable {
		switch c.KafkaSASLMechanism {
		case "PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512":
		default:
			fail(fmt.Sprintf("KAFKA_SASL_MECHANISM=%q: must be PLAIN, SCRAM-SHA-256, or SCRAM-SHA-512", c.KafkaSASLMechanism))
		}
		if c.KafkaSASLUsername == "" {
			fail("KAFKA_SASL_USERNAME is required when KAFKA_SASL_ENABLE=true")
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// DSN returns the PostgreSQL connection string.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName, c.DBSSLMode,
	)
}

// MongoURI returns the MongoDB connection string.
func (c *Config) MongoURI() string {
	if c.MongoURI_ != "" {
		return c.MongoURI_
	}
	return fmt.Sprintf("mongodb://%s:%d", c.DBHost, c.DBPort)
}

// ElasticURLList returns Elasticsearch URLs as a slice.
func (c *Config) ElasticURLList() []string {
	if c.ElasticURLs == "" {
		return nil
	}
	urls := strings.Split(c.ElasticURLs, ",")
	for i := range urls {
		urls[i] = strings.TrimSpace(urls[i])
	}
	return urls
}

// KafkaBrokerList returns Kafka broker addresses as a slice.
func (c *Config) KafkaBrokerList() []string {
	if c.KafkaBrokers == "" {
		return nil
	}
	brokers := strings.Split(c.KafkaBrokers, ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}
	return brokers
}

// Load reads config from .env file + environment variables.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.AddConfigPath(path)
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AutomaticEnv()

	v.SetDefault("SERVER_PORT", 8080)
	v.SetDefault("SERVER_MODE", "debug")
	v.SetDefault("CORS_ORIGINS", "*")
	v.SetDefault("DB_DRIVER", "sqlc")
	v.SetDefault("DB_SSL_MODE", "disable")
	v.SetDefault("DB_MAX_CONNS", 10)
	v.SetDefault("DB_MIN_CONNS", 2)
	v.SetDefault("MONGO_DB_NAME", "myapp")
	v.SetDefault("REDIS_URL", "redis://localhost:6379/0")
	v.SetDefault("REDIS_POOL_SIZE", 10)
	v.SetDefault("REDIS_MIN_IDLE", 2)
	v.SetDefault("AUTH_PROVIDER", "jwt")
	v.SetDefault("JWT_EXPIRY", "24h")
	v.SetDefault("JWT_ALGORITHM", "HS256")
	v.SetDefault("GRPC_ENABLED", false)
	v.SetDefault("GRPC_PORT", 9090)
	v.SetDefault("SENDGRID_URL", "https://api.sendgrid.com")
	v.SetDefault("STRIPE_URL", "https://api.stripe.com")
	v.SetDefault("KAFKA_BROKERS", "localhost:9092")
	v.SetDefault("KAFKA_CONSUMER_GROUP", "myapp-group")
	v.SetDefault("WORKER_COUNT", 4)
	v.SetDefault("WORKER_QUEUE_SIZE", 100)
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("LOG_FORMAT", "console")
	v.SetDefault("OTEL_ENABLED", false)
	v.SetDefault("OTEL_SERVICE_NAME", "gostack-kit")
	v.SetDefault("OTEL_ENDPOINT", "localhost:4318")
	v.SetDefault("OTEL_SAMPLER", 1.0)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &cfg, nil
}
