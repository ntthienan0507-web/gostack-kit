package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ntthienan0507-web/go-api-template/pkg/config"
)

func runDB() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go-api-template db <command>")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  test               Test database connection")
		fmt.Println("  tables             List all tables")
		fmt.Println("  columns <tbl>      Show columns of a table")
		fmt.Println("  explain <sql>      Run EXPLAIN ANALYZE on a query")
		fmt.Println("  slow-queries       Show top 20 slowest queries (pg_stat_statements)")
		fmt.Println("  index-usage        Show unused/low-usage indexes")
		fmt.Println("  table-stats        Show table sizes, row counts, dead tuples")
		fmt.Println("  locks              Show current lock waits")
		fmt.Println("  connections        Show active connections by state")
		fmt.Println("  outbox-maintain    Create next partition + drop old ones")
		fmt.Println("  outbox-partitions  List outbox partitions with sizes")
		fmt.Println("  outbox-failed      Show unresolved failed messages")
		fmt.Println("  outbox-retry <id>  Retry a specific failed message")
		fmt.Println("  outbox-retry-all   Retry ALL unresolved failed messages")
		fmt.Println("  query <sql>        Run ad-hoc SQL query")
		fmt.Println("  shell              Open psql shell")
		os.Exit(1)
	}

	cfg := mustLoadConfig()

	switch os.Args[2] {
	case "test":
		dbTest(cfg)

	// --- Query performance ---

	case "explain":
		requireArg(4, "db explain \"SELECT ...\"")
		query := strings.Join(os.Args[3:], " ")
		psqlQuery(cfg, fmt.Sprintf("EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) %s", query))

	case "slow-queries":
		psqlQuery(cfg, `SELECT calls, round(total_exec_time::numeric, 2) AS total_ms, round(mean_exec_time::numeric, 2) AS avg_ms, round(max_exec_time::numeric, 2) AS max_ms, rows, left(query, 120) AS query FROM pg_stat_statements ORDER BY mean_exec_time DESC LIMIT 20;`)

	case "index-usage":
		psqlQuery(cfg, `SELECT schemaname || '.' || relname AS table, indexrelname AS index, idx_scan AS scans, pg_size_pretty(pg_relation_size(indexrelid)) AS size FROM pg_stat_user_indexes WHERE idx_scan < 10 ORDER BY pg_relation_size(indexrelid) DESC LIMIT 20;`)

	case "table-stats":
		psqlQuery(cfg, `SELECT relname AS table, pg_size_pretty(pg_total_relation_size(relid)) AS total_size, n_live_tup AS live_rows, n_dead_tup AS dead_rows, round(100.0 * n_dead_tup / NULLIF(n_live_tup + n_dead_tup, 0), 1) AS dead_pct, last_vacuum, last_autovacuum FROM pg_stat_user_tables ORDER BY pg_total_relation_size(relid) DESC;`)

	case "locks":
		psqlQuery(cfg, `SELECT blocked_locks.pid AS blocked_pid, blocked_activity.usename AS blocked_user, left(blocked_activity.query, 80) AS blocked_query, blocking_locks.pid AS blocking_pid, blocking_activity.usename AS blocking_user, left(blocking_activity.query, 80) AS blocking_query FROM pg_catalog.pg_locks blocked_locks JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid JOIN pg_catalog.pg_locks blocking_locks ON blocking_locks.locktype = blocked_locks.locktype AND blocking_locks.relation = blocked_locks.relation AND blocking_locks.pid != blocked_locks.pid JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid WHERE NOT blocked_locks.granted;`)

	case "connections":
		psqlQuery(cfg, `SELECT state, usename, count(*) AS count, max(now() - state_change) AS max_duration FROM pg_stat_activity WHERE pid != pg_backend_pid() GROUP BY state, usename ORDER BY count DESC;`)

	// --- Schema ---

	case "tables":
		psqlQuery(cfg, "SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename;")

	case "columns":
		requireArg(4, "db columns <table_name>")
		psqlQuery(cfg, fmt.Sprintf(
			"SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns WHERE table_name = '%s' ORDER BY ordinal_position;",
			os.Args[3],
		))

	// --- Outbox management ---

	case "outbox-maintain":
		psqlQuery(cfg, "SELECT create_outbox_partition();")
		psqlQuery(cfg, "SELECT * FROM drop_old_outbox_partitions(3);")

	case "outbox-partitions":
		psqlQuery(cfg, "SELECT tablename, pg_size_pretty(pg_total_relation_size(tablename::regclass)) AS size FROM pg_tables WHERE tablename LIKE 'outbox_%' ORDER BY tablename;")

	case "outbox-failed":
		psqlQuery(cfg, "SELECT id, topic, key, retry_count, last_error, created_at, source_partition FROM outbox_failed WHERE NOT resolved ORDER BY created_at;")

	case "outbox-retry":
		requireArg(4, "db outbox-retry <id>")
		psqlQuery(cfg, fmt.Sprintf("SELECT retry_outbox_failed(%s);", os.Args[3]))
		fmt.Printf("Failed message %s re-inserted into outbox as pending\n", os.Args[3])

	case "outbox-retry-all":
		psqlQuery(cfg, "SELECT retry_all_outbox_failed();")

	// --- Raw access ---

	case "query":
		requireArg(4, "db query \"SELECT ...\"")
		psqlQuery(cfg, strings.Join(os.Args[3:], " "))

	case "shell":
		psqlShell(cfg)

	default:
		fatal("Unknown db command: %s", os.Args[2])
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
