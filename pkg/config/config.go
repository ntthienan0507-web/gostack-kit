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
	ServerPort int    `mapstructure:"SERVER_PORT"`
	ServerMode string `mapstructure:"SERVER_MODE"`

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
	AuthProvider string        `mapstructure:"AUTH_PROVIDER"`
	JWTSecret    string        `mapstructure:"JWT_SECRET"`
	JWTExpiry    time.Duration `mapstructure:"JWT_EXPIRY"`

	// Keycloak (only when AUTH_PROVIDER=keycloak)
	KeycloakHost         string `mapstructure:"KEYCLOAK_HOST"`
	KeycloakRealm        string `mapstructure:"KEYCLOAK_REALM"`
	KeycloakClientID     string `mapstructure:"KEYCLOAK_CLIENT_ID"`
	KeycloakClientSecret string `mapstructure:"KEYCLOAK_CLIENT_SECRET"`

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
	v.SetDefault("SENDGRID_URL", "https://api.sendgrid.com")
	v.SetDefault("STRIPE_URL", "https://api.stripe.com")
	v.SetDefault("KAFKA_BROKERS", "localhost:9092")
	v.SetDefault("KAFKA_CONSUMER_GROUP", "myapp-group")
	v.SetDefault("WORKER_COUNT", 4)
	v.SetDefault("WORKER_QUEUE_SIZE", 100)
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("LOG_FORMAT", "console")
	v.SetDefault("OTEL_ENABLED", false)
	v.SetDefault("OTEL_SERVICE_NAME", "go-api-template")
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
