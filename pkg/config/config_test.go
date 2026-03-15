package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("DB_DRIVER", "gorm")
	t.Setenv("AUTH_PROVIDER", "keycloak")

	cfg, err := Load(t.TempDir())

	assert.NoError(t, err)
	assert.Equal(t, 9090, cfg.ServerPort)
	assert.Equal(t, "gorm", cfg.DBDriver)
	assert.Equal(t, "keycloak", cfg.AuthProvider)
}

// validBaseConfig returns a Config that passes all validation.
func validBaseConfig() *Config {
	return &Config{
		ServerPort:    8080,
		ServerMode:    "debug",
		DBDriver:      "sqlc",
		DBHost:        "localhost",
		DBPort:        5432,
		DBName:        "myapp",
		DBMaxConns:    10,
		DBMinConns:    2,
		AuthProvider:  "jwt",
		JWTSecret:     "this-secret-is-at-least-32-chars!",
		JWTExpiry:     24 * time.Hour,
		RedisPoolSize: 10,
		RedisMinIdle:  2,
		WorkerCount:   4,
		WorkerQueueSize: 100,
		LogLevel:      "info",
		LogFormat:     "console",
		OTELSampler:   1.0,
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := validBaseConfig()

	err := cfg.Validate()

	require.NoError(t, err)
}

func TestValidate_InvalidServerPort(t *testing.T) {
	for _, port := range []int{0, -1, 70000} {
		cfg := validBaseConfig()
		cfg.ServerPort = port

		err := cfg.Validate()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "SERVER_PORT")
	}
}

func TestValidate_InvalidServerMode(t *testing.T) {
	cfg := validBaseConfig()
	cfg.ServerMode = "production"

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SERVER_MODE")
}

func TestValidate_InvalidDBDriver(t *testing.T) {
	cfg := validBaseConfig()
	cfg.DBDriver = "mysql"

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "DB_DRIVER")
}

func TestValidate_MongoSkipsPostgresFields(t *testing.T) {
	cfg := validBaseConfig()
	cfg.DBDriver = "mongo"
	cfg.DBHost = ""
	cfg.DBPort = 0
	cfg.DBName = ""

	err := cfg.Validate()

	require.NoError(t, err)
}

func TestValidate_MissingDBHost(t *testing.T) {
	cfg := validBaseConfig()
	cfg.DBHost = ""

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "DB_HOST")
}

func TestValidate_InvalidDBPort(t *testing.T) {
	cfg := validBaseConfig()
	cfg.DBPort = 0

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "DB_PORT")
}

func TestValidate_MissingDBName(t *testing.T) {
	cfg := validBaseConfig()
	cfg.DBName = ""

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "DB_NAME")
}

func TestValidate_DBMinConnsExceedsMax(t *testing.T) {
	cfg := validBaseConfig()
	cfg.DBMaxConns = 5
	cfg.DBMinConns = 10

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "DB_MIN_CONNS")
}

func TestValidate_JWTSecretTooShort(t *testing.T) {
	cfg := validBaseConfig()
	cfg.JWTSecret = "short"

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestValidate_JWTSecretMissing(t *testing.T) {
	cfg := validBaseConfig()
	cfg.JWTSecret = ""

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestValidate_RS256_MissingKeyFiles(t *testing.T) {
	cfg := validBaseConfig()
	cfg.JWTAlgorithm = "RS256"
	cfg.JWTSecret = ""

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_PRIVATE_KEY_FILE")
	assert.Contains(t, err.Error(), "JWT_PUBLIC_KEY_FILE")
}

func TestValidate_RS256_Valid(t *testing.T) {
	cfg := validBaseConfig()
	cfg.JWTAlgorithm = "RS256"
	cfg.JWTSecret = ""
	cfg.JWTPrivateKeyFile = "/keys/private.pem"
	cfg.JWTPublicKeyFile = "/keys/public.pem"

	err := cfg.Validate()

	require.NoError(t, err)
}

func TestValidate_InvalidJWTAlgorithm(t *testing.T) {
	cfg := validBaseConfig()
	cfg.JWTAlgorithm = "ES256"

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_ALGORITHM")
}

func TestValidate_JWTExpiryNegative(t *testing.T) {
	cfg := validBaseConfig()
	cfg.JWTExpiry = -time.Hour

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_EXPIRY")
}

func TestValidate_InvalidAuthProvider(t *testing.T) {
	cfg := validBaseConfig()
	cfg.AuthProvider = "oauth2"

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "AUTH_PROVIDER")
}

func TestValidate_KeycloakMissingFields(t *testing.T) {
	cfg := validBaseConfig()
	cfg.AuthProvider = "keycloak"
	cfg.JWTSecret = ""

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "KEYCLOAK_HOST")
	assert.Contains(t, err.Error(), "KEYCLOAK_REALM")
	assert.Contains(t, err.Error(), "KEYCLOAK_CLIENT_ID")
}

func TestValidate_KeycloakValid(t *testing.T) {
	cfg := validBaseConfig()
	cfg.AuthProvider = "keycloak"
	cfg.JWTSecret = ""
	cfg.KeycloakHost = "https://keycloak.example.com"
	cfg.KeycloakRealm = "myrealm"
	cfg.KeycloakClientID = "myapp"

	err := cfg.Validate()

	require.NoError(t, err)
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := validBaseConfig()
	cfg.LogLevel = "verbose"

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "LOG_LEVEL")
}

func TestValidate_InvalidLogFormat(t *testing.T) {
	cfg := validBaseConfig()
	cfg.LogFormat = "text"

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "LOG_FORMAT")
}

func TestValidate_OTELSamplerOutOfRange(t *testing.T) {
	cfg := validBaseConfig()
	cfg.OTELSampler = 2.0

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "OTEL_SAMPLER")
}

func TestValidate_EncryptionKeyTooShort(t *testing.T) {
	cfg := validBaseConfig()
	cfg.EncryptionKey = "too-short"

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ENCRYPTION_KEY")
}

func TestValidate_KafkaSASLMissingUsername(t *testing.T) {
	cfg := validBaseConfig()
	cfg.KafkaSASLEnable = true
	cfg.KafkaSASLMechanism = "PLAIN"

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "KAFKA_SASL_USERNAME")
}

func TestValidate_KafkaSASLInvalidMechanism(t *testing.T) {
	cfg := validBaseConfig()
	cfg.KafkaSASLEnable = true
	cfg.KafkaSASLMechanism = "GSSAPI"
	cfg.KafkaSASLUsername = "user"

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "KAFKA_SASL_MECHANISM")
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := validBaseConfig()
	cfg.ServerPort = 0
	cfg.DBDriver = "mysql"
	cfg.LogLevel = "verbose"

	err := cfg.Validate()

	require.Error(t, err)
	// All 3 errors reported in one message
	lines := strings.Split(err.Error(), "\n")
	assert.GreaterOrEqual(t, len(lines), 3)
}

func TestValidate_WorkerCountZero(t *testing.T) {
	cfg := validBaseConfig()
	cfg.WorkerCount = 0

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "WORKER_COUNT")
}
