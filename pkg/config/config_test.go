package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDSN(t *testing.T) {
	cfg := &Config{
		DBUser:     "postgres",
		DBPassword: "secret",
		DBHost:     "localhost",
		DBPort:     5432,
		DBName:     "mydb",
		DBSSLMode:  "disable",
	}

	dsn := cfg.DSN()

	assert.Equal(t, "postgres://postgres:secret@localhost:5432/mydb?sslmode=disable", dsn)
}

func TestMongoURI_Explicit(t *testing.T) {
	cfg := &Config{
		MongoURI_: "mongodb://user:pass@host:27017",
	}

	assert.Equal(t, "mongodb://user:pass@host:27017", cfg.MongoURI())
}

func TestMongoURI_Fallback(t *testing.T) {
	cfg := &Config{
		DBHost: "mongo-host",
		DBPort: 27017,
	}

	assert.Equal(t, "mongodb://mongo-host:27017", cfg.MongoURI())
}

func TestLoad_Defaults(t *testing.T) {
	// Load with a path that has no .env file — should use defaults
	cfg, err := Load(t.TempDir())

	assert.NoError(t, err)
	assert.Equal(t, 8080, cfg.ServerPort)
	assert.Equal(t, "debug", cfg.ServerMode)
	assert.Equal(t, "sqlc", cfg.DBDriver)
	assert.Equal(t, "disable", cfg.DBSSLMode)
	assert.Equal(t, int32(10), cfg.DBMaxConns)
	assert.Equal(t, int32(2), cfg.DBMinConns)
	assert.Equal(t, "myapp", cfg.MongoDBName)
	assert.Equal(t, "jwt", cfg.AuthProvider)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "console", cfg.LogFormat)
}

func TestLoad_FromEnvFile(t *testing.T) {
	// Set env vars to override defaults
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("DB_DRIVER", "gorm")
	t.Setenv("AUTH_PROVIDER", "keycloak")

	cfg, err := Load(t.TempDir())

	assert.NoError(t, err)
	assert.Equal(t, 9090, cfg.ServerPort)
	assert.Equal(t, "gorm", cfg.DBDriver)
	assert.Equal(t, "keycloak", cfg.AuthProvider)
}
