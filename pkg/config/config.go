package config

import (
	"fmt"
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
	DBDriver   string `mapstructure:"DB_DRIVER"` // "sqlc" | "gorm"
	DBHost     string `mapstructure:"DB_HOST"`
	DBPort     int    `mapstructure:"DB_PORT"`
	DBUser     string `mapstructure:"DB_USER"`
	DBPassword string `mapstructure:"DB_PASSWORD"`
	DBName     string `mapstructure:"DB_NAME"`
	DBSSLMode  string `mapstructure:"DB_SSL_MODE"`
	DBMaxConns int32  `mapstructure:"DB_MAX_CONNS"`
	DBMinConns int32  `mapstructure:"DB_MIN_CONNS"`

	// Auth
	AuthProvider string        `mapstructure:"AUTH_PROVIDER"`
	JWTSecret    string        `mapstructure:"JWT_SECRET"`
	JWTExpiry    time.Duration `mapstructure:"JWT_EXPIRY"`

	// Keycloak (only when AUTH_PROVIDER=keycloak)
	KeycloakHost         string `mapstructure:"KEYCLOAK_HOST"`
	KeycloakRealm        string `mapstructure:"KEYCLOAK_REALM"`
	KeycloakClientID     string `mapstructure:"KEYCLOAK_CLIENT_ID"`
	KeycloakClientSecret string `mapstructure:"KEYCLOAK_CLIENT_SECRET"`

	// Logging
	LogLevel  string `mapstructure:"LOG_LEVEL"`
	LogFormat string `mapstructure:"LOG_FORMAT"`
}

// DSN returns the PostgreSQL connection string.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName, c.DBSSLMode,
	)
}

// Load reads config from .env file + environment variables.
// Env vars OVERRIDE file values.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.AddConfigPath(path)
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AutomaticEnv()

	// Defaults
	v.SetDefault("SERVER_PORT", 8080)
	v.SetDefault("SERVER_MODE", "debug")
	v.SetDefault("DB_DRIVER", "sqlc")
	v.SetDefault("DB_SSL_MODE", "disable")
	v.SetDefault("DB_MAX_CONNS", 20)
	v.SetDefault("DB_MIN_CONNS", 2)
	v.SetDefault("AUTH_PROVIDER", "jwt")
	v.SetDefault("JWT_EXPIRY", "24h")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("LOG_FORMAT", "console")

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
