//go:build integration

package testutil

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// migrationsDir returns the absolute path to db/migrations from the project root.
func migrationsDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "db", "migrations")
}

// startPostgresContainer spins up a PostgreSQL 16 container and runs migrations.
// Returns the connection string. Container is terminated via t.Cleanup.
func startPostgresContainer(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate postgres container: %v", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get postgres connection string: %v", err)
	}

	sqlDB, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("failed to open sql connection for migrations: %v", err)
	}
	defer sqlDB.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("failed to set goose dialect: %v", err)
	}
	if err := goose.Up(sqlDB, migrationsDir()); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return connStr
}

// NewPostgresContainer starts a PostgreSQL 16 container, runs goose migrations,
// and returns a *pgxpool.Pool connected to the container. The container is
// terminated automatically via t.Cleanup.
func NewPostgresContainer(t *testing.T) *pgxpool.Pool {
	t.Helper()
	connStr := startPostgresContainer(t)

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		t.Fatalf("failed to create pgxpool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	return pool
}

// NewGormDB starts a PostgreSQL 16 container, runs goose migrations,
// and returns a *gorm.DB connected to the container. The container is
// terminated automatically via t.Cleanup.
func NewGormDB(t *testing.T) *gorm.DB {
	t.Helper()
	connStr := startPostgresContainer(t)

	db, err := gorm.Open(postgres.Open(connStr), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open gorm connection: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	})

	return db
}

// NewRedisContainer starts a Redis container and returns a *redis.Client
// connected to it. The container is terminated automatically via t.Cleanup.
func NewRedisContainer(t *testing.T) *redis.Client {
	t.Helper()
	ctx := context.Background()

	redisContainer, err := tcredis.Run(ctx,
		"redis:7-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").
				WithStartupTimeout(15*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start redis container: %v", err)
	}
	t.Cleanup(func() {
		if err := redisContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate redis container: %v", err)
		}
	})

	connStr, err := redisContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("failed to get redis connection string: %v", err)
	}

	opts, err := redis.ParseURL(connStr)
	if err != nil {
		t.Fatalf("failed to parse redis URL: %v", err)
	}

	client := redis.NewClient(opts)
	t.Cleanup(func() { client.Close() })

	return client
}
