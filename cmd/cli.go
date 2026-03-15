package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/ntthienan0507-web/gostack-kit/pkg/config"
)

const migrationsDir = "db/migrations"

// Run is the CLI entry point.
func Run() {
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
	case "cron":
		runCron()
	case "init":
		runInit()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: gostack-kit <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  serve      Start the API server")
	fmt.Println("  migrate    Database migration operations")
	fmt.Println("  db         Database utility operations")
	fmt.Println("  cron       Start the cron scheduler")
	fmt.Println("  init       Interactive project setup (select stacks, generate .env)")
	fmt.Println("  init --minimal   DB + Redis only")
	fmt.Println("  init --all       Enable all stacks")
}

// --- Shared helpers ---

func mustLoadConfig() *config.Config {
	cfg, err := config.Load(".")
	if err != nil {
		fatal("load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		fatal("%v", err)
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

func psqlQuery(cfg *config.Config, query string) {
	c := exec.Command("psql",
		"-U", cfg.DBUser,
		"-d", cfg.DBName,
		"-h", cfg.DBHost,
		"-p", fmt.Sprintf("%d", cfg.DBPort),
		"-c", query,
	)
	c.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", cfg.DBPassword))
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		fatal("psql: %v", err)
	}
}

func psqlShell(cfg *config.Config) {
	c := exec.Command("psql",
		"-U", cfg.DBUser,
		"-d", cfg.DBName,
		"-h", cfg.DBHost,
		"-p", fmt.Sprintf("%d", cfg.DBPort),
	)
	c.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", cfg.DBPassword))
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		fatal("psql: %v", err)
	}
}

func requireArg(minArgs int, usage string) {
	if len(os.Args) < minArgs {
		fatal("Usage: gostack-kit %s", usage)
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

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
