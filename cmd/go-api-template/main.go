package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"

	"github.com/ntthienan0507-web/go-api-template/internal/app"
	"github.com/ntthienan0507-web/go-api-template/internal/config"
	"github.com/ntthienan0507-web/go-api-template/internal/database"
	"github.com/ntthienan0507-web/go-api-template/pkg/logger"
)

const migrationsDir = "db/migrations"

// @title           go-api-template API
// @version         1.0
// @description     API server for go-api-template
// @BasePath        /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		runServe()
	case "migrate":
		runMigrate()
	case "db":
		runDB()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

// ============================================
// serve
// ============================================

func runServe() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := mustLoadConfig()

	log, err := logger.New(cfg.LogFormat, cfg.LogLevel)
	if err != nil {
		fatal("init logger: %v", err)
	}

	// Auto-migrate on startup
	if err := database.RunMigrations(cfg.DSN(), migrationsDir, log); err != nil {
		log.Error("migration failed", zap.Error(err))
	}

	application, err := app.New(ctx, cfg, log)
	if err != nil {
		log.Fatal("init app failed", zap.Error(err))
	}
	defer application.Shutdown()

	if err := application.Run(); err != nil {
		log.Fatal("server error", zap.Error(err))
	}
}

// ============================================
// migrate
// ============================================

func runMigrate() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go-api-template migrate <command>")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  up              Run all pending migrations")
		fmt.Println("  status          Show migration status")
		fmt.Println("  down            Rollback last migration")
		fmt.Println("  create <name>   Create new migration file")
		fmt.Println("  up-to <ver>     Migrate up to version")
		fmt.Println("  down-to <ver>   Rollback down to version")
		fmt.Println("  version         Print current migration version")
		fmt.Println("  fix             Fix migration sequence numbers")
		os.Exit(1)
	}

	cfg := mustLoadConfig()
	db := openDB(cfg)
	defer db.Close()
	setGoose()

	switch os.Args[2] {
	case "up":
		must(goose.Up(db, migrationsDir))
		fmt.Println("Migrations applied successfully")

	case "status":
		must(goose.Status(db, migrationsDir))

	case "down":
		must(goose.Down(db, migrationsDir))
		fmt.Println("Rolled back one migration")

	case "create":
		if len(os.Args) < 4 {
			fatal("Usage: go-api-template migrate create <name>")
		}
		must(goose.Create(db, migrationsDir, os.Args[3], "sql"))
		fmt.Printf("Created migration: %s\n", os.Args[3])

	case "up-to":
		if len(os.Args) < 4 {
			fatal("Usage: go-api-template migrate up-to <version>")
		}
		must(goose.UpTo(db, migrationsDir, parseInt64(os.Args[3])))

	case "down-to":
		if len(os.Args) < 4 {
			fatal("Usage: go-api-template migrate down-to <version>")
		}
		must(goose.DownTo(db, migrationsDir, parseInt64(os.Args[3])))

	case "version":
		ver, err := goose.GetDBVersion(db)
		if err != nil {
			fatal("get version: %v", err)
		}
		fmt.Printf("Current version: %d\n", ver)

	case "fix":
		must(goose.Fix(migrationsDir))
		fmt.Println("Fixed migration sequence numbers")

	default:
		fatal("Unknown migrate command: %s", os.Args[2])
	}
}

// ============================================
// db
// ============================================

func runDB() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go-api-template db <command>")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  test            Test database connection")
		fmt.Println("  tables          List all tables")
		fmt.Println("  columns <tbl>   Show columns of a table")
		fmt.Println("  query <sql>     Run ad-hoc SQL query")
		fmt.Println("  shell           Open psql shell")
		os.Exit(1)
	}

	cfg := mustLoadConfig()

	switch os.Args[2] {
	case "test":
		dbTest(cfg)

	case "tables":
		psqlQuery(cfg, "SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename;")

	case "columns":
		if len(os.Args) < 4 {
			fatal("Usage: go-api-template db columns <table_name>")
		}
		psqlQuery(cfg, fmt.Sprintf(
			"SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns WHERE table_name = '%s' ORDER BY ordinal_position;",
			os.Args[3],
		))

	case "query":
		if len(os.Args) < 4 {
			fatal("Usage: go-api-template db query \"SELECT ...\"")
		}
		psqlQuery(cfg, strings.Join(os.Args[3:], " "))

	case "shell":
		psqlShell(cfg)

	default:
		fatal("Unknown db command: %s", os.Args[2])
	}
}

// ============================================
// Helpers
// ============================================

func mustLoadConfig() *config.Config {
	cfg, err := config.Load(".")
	if err != nil {
		fatal("load config: %v", err)
	}
	return cfg
}

func openDB(cfg *config.Config) *sql.DB {
	db, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		fatal("open db: %v", err)
	}
	return db
}

func setGoose() {
	if err := goose.SetDialect("postgres"); err != nil {
		fatal("set dialect: %v", err)
	}
}

func dbTest(cfg *config.Config) {
	fmt.Printf("Connecting to %s@%s:%d/%s ...\n", cfg.DBUser, cfg.DBHost, cfg.DBPort, cfg.DBName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DSN())
	if err != nil {
		fatal("FAIL: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		fatal("FAIL: ping: %v", err)
	}

	var version string
	if err := pool.QueryRow(ctx, "SELECT version()").Scan(&version); err != nil {
		fatal("FAIL: query: %v", err)
	}

	fmt.Println("OK: connected successfully")
	fmt.Printf("    %s\n", version)
}

func psqlQuery(cfg *config.Config, query string) {
	cmd := exec.Command("psql",
		"-U", cfg.DBUser,
		"-d", cfg.DBName,
		"-h", cfg.DBHost,
		"-p", fmt.Sprintf("%d", cfg.DBPort),
		"-c", query,
	)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", cfg.DBPassword))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal("psql: %v", err)
	}
}

func psqlShell(cfg *config.Config) {
	cmd := exec.Command("psql",
		"-U", cfg.DBUser,
		"-d", cfg.DBName,
		"-h", cfg.DBHost,
		"-p", fmt.Sprintf("%d", cfg.DBPort),
	)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", cfg.DBPassword))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal("psql: %v", err)
	}
}

func parseInt64(s string) int64 {
	var v int64
	_, err := fmt.Sscanf(s, "%d", &v)
	if err != nil {
		fatal("invalid version number: %s", s)
	}
	return v
}

func must(err error) {
	if err != nil {
		fatal("%v", err)
	}
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func printUsage() {
	fmt.Println("Usage: go-api-template <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  serve      Start the API server")
	fmt.Println("  migrate    Database migration operations")
	fmt.Println("  db         Database utility operations")
}
