package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
)

// initConfig holds the user's stack selection.
type initConfig struct {
	ProjectName string
	DBDriver    string // "sqlc" | "gorm" | "mongo"

	// Infrastructure
	EnableRedis         bool
	EnableKafka         bool
	EnableElasticsearch bool
	EnableOTEL          bool
	EnableEncryption    bool

	// Features
	EnableCron       bool
	EnableWebSocket  bool
	EnableKubernetes bool

	// External services
	EnableSendGrid bool
	EnableStripe   bool
	EnableIceWarp  bool
	EnableFirebase bool

	// Code quality
	EnableSonar bool
}

func runInit() {
	cfg := &initConfig{}

	// Check for flags
	if len(os.Args) >= 3 {
		switch os.Args[2] {
		case "--all":
			cfg.ProjectName = "myapp"
			cfg.DBDriver = "gorm"
			cfg.EnableRedis = true
			cfg.EnableKafka = true
			cfg.EnableElasticsearch = true
			cfg.EnableOTEL = true
			cfg.EnableEncryption = true
			cfg.EnableCron = true
			cfg.EnableWebSocket = true
			cfg.EnableKubernetes = true
			cfg.EnableSendGrid = true
			cfg.EnableStripe = true
			cfg.EnableIceWarp = true
			cfg.EnableFirebase = true
			cfg.EnableSonar = true
			generateAll(cfg)
			return

		case "--minimal":
			cfg.ProjectName = "myapp"
			cfg.DBDriver = "gorm"
			cfg.EnableRedis = true
			generateAll(cfg)
			return
		}
	}

	// Interactive mode
	var infraChoices []string
	var featureChoices []string
	var serviceChoices []string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project name").
				Value(&cfg.ProjectName).
				Placeholder("myapp"),

			huh.NewSelect[string]().
				Title("Database driver (pick one)").
				Options(
					huh.NewOption("GORM — ORM, recommended", "gorm"),
					huh.NewOption("SQLC — raw SQL, type-safe generated code", "sqlc"),
					huh.NewOption("MongoDB — document store", "mongo"),
				).
				Value(&cfg.DBDriver),
		).Title("  Project"),

		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Infrastructure (space to select)").
				Options(
					huh.NewOption("Redis — cache, sessions, pub/sub", "redis"),
					huh.NewOption("Kafka — event streaming, outbox pattern", "kafka"),
					huh.NewOption("Elasticsearch — search & analytics", "elasticsearch"),
					huh.NewOption("OpenTelemetry — distributed tracing", "otel"),
					huh.NewOption("Encryption — AES-256 for PII fields", "encryption"),
				).
				Value(&infraChoices),
		).Title("  Infrastructure"),

		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Features (space to select)").
				Options(
					huh.NewOption("Cron — scheduled background jobs", "cron"),
					huh.NewOption("WebSocket — real-time communication", "ws"),
					huh.NewOption("Kubernetes — deployment manifests", "k8s"),
				).
				Value(&featureChoices),
		).Title("  Features"),

		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("External services (space to select)").
				Options(
					huh.NewOption("SendGrid — transactional email", "sendgrid"),
					huh.NewOption("Stripe — payment processing", "stripe"),
					huh.NewOption("IceWarp — mail server (XML API)", "icewarp"),
					huh.NewOption("Firebase — push notifications + auth", "firebase"),
				).
				Value(&serviceChoices),

			huh.NewConfirm().
				Title("Enable SonarQube?").
				Description("Code quality & security analysis").
				Value(&cfg.EnableSonar),
		).Title("  Services & Quality"),
	)

	if err := form.Run(); err != nil {
		if err.Error() == "user aborted" {
			fmt.Println("Setup cancelled.")
			return
		}
		fatal("form error: %v", err)
	}

	if cfg.ProjectName == "" {
		cfg.ProjectName = "myapp"
	}

	// Map choices
	for _, c := range infraChoices {
		switch c {
		case "redis":
			cfg.EnableRedis = true
		case "kafka":
			cfg.EnableKafka = true
		case "elasticsearch":
			cfg.EnableElasticsearch = true
		case "otel":
			cfg.EnableOTEL = true
		case "encryption":
			cfg.EnableEncryption = true
		}
	}
	for _, c := range featureChoices {
		switch c {
		case "cron":
			cfg.EnableCron = true
		case "ws":
			cfg.EnableWebSocket = true
		case "k8s":
			cfg.EnableKubernetes = true
		}
	}
	for _, c := range serviceChoices {
		switch c {
		case "sendgrid":
			cfg.EnableSendGrid = true
		case "stripe":
			cfg.EnableStripe = true
		case "icewarp":
			cfg.EnableIceWarp = true
		case "firebase":
			cfg.EnableFirebase = true
		}
	}

	generateAll(cfg)
}

func generateAll(cfg *initConfig) {
	fmt.Println()
	generateEnv(cfg)
	generateComposeOverride(cfg)
	cleanupUnused(cfg)
	printSummary(cfg)
}

// cleanupUnused removes files and packages not needed based on selection.
func cleanupUnused(cfg *initConfig) {
	var removed []string

	rm := func(paths ...string) {
		for _, p := range paths {
			// Support glob patterns
			matches, err := filepath.Glob(p)
			if err != nil {
				continue
			}
			for _, m := range matches {
				if err := os.RemoveAll(m); err == nil {
					removed = append(removed, m)
				}
			}
		}
	}

	// --- Database driver exclusion ---
	switch cfg.DBDriver {
	case "gorm":
		// Remove SQLC-specific files
		rm("sqlc.yaml", "db/queries", "db/sqlc")
		rm("modules/*/repository_sqlc.go")
		// Remove Mongo-specific files
		rm("modules/*/repository_mongo.go")
		rm("pkg/database/mongo.go")

	case "sqlc":
		// Remove GORM-specific files
		rm("modules/*/repository_gorm.go")
		rm("pkg/database/gorm.go", "pkg/database/transaction.go", "pkg/database/transaction_test.go")
		// Remove Mongo-specific files
		rm("modules/*/repository_mongo.go")
		rm("pkg/database/mongo.go")

	case "mongo":
		// Remove PostgreSQL-specific files
		rm("sqlc.yaml", "db/queries", "db/sqlc")
		rm("modules/*/repository_sqlc.go")
		rm("modules/*/repository_gorm.go")
		rm("pkg/database/postgres.go", "pkg/database/gorm.go")
		rm("pkg/database/store.go", "pkg/database/migration.go")
		rm("pkg/database/transaction.go", "pkg/database/transaction_test.go")
		rm("db/migrations") // Mongo doesn't use SQL migrations
	}

	// --- Kafka ---
	if !cfg.EnableKafka {
		rm("pkg/broker")
		rm("modules/*/events.go")
		rm("db/migrations/*outbox*", "db/migrations/*processed_events*")
	}

	// --- Cron ---
	if !cfg.EnableCron {
		rm("pkg/cron", "cmd/cron.go")
	}

	// --- WebSocket ---
	if !cfg.EnableWebSocket {
		rm("pkg/ws")
	}

	// --- Kubernetes ---
	if !cfg.EnableKubernetes {
		rm("deployments")
	}

	// --- OpenTelemetry ---
	if !cfg.EnableOTEL {
		rm("pkg/tracing")
	}

	// --- Encryption ---
	if !cfg.EnableEncryption {
		rm("pkg/crypto")
	}

	// --- External services ---
	if !cfg.EnableSendGrid {
		rm("pkg/external/sendgrid")
	}
	if !cfg.EnableStripe {
		rm("pkg/external/stripe")
	}
	if !cfg.EnableIceWarp {
		rm("pkg/external/icewarp")
	}
	if !cfg.EnableFirebase {
		rm("pkg/external/firebase")
	}
	if !cfg.EnableElasticsearch {
		rm("pkg/external/elasticsearch")
	}

	// --- SonarQube ---
	if !cfg.EnableSonar {
		rm("sonar-project.properties", "scripts/setup-sonar.sh")
	}

	// --- Circuit breaker (only useful with Kafka or httpclient heavy usage) ---
	if !cfg.EnableKafka && !cfg.EnableSendGrid && !cfg.EnableStripe && !cfg.EnableIceWarp {
		rm("pkg/circuitbreaker")
	}

	// --- Redis ---
	if !cfg.EnableRedis {
		rm("pkg/database/redis.go", "pkg/cache", "pkg/distlock")
	}

	// Clean up empty pkg/external/ if nothing enabled
	if !cfg.EnableSendGrid && !cfg.EnableStripe && !cfg.EnableIceWarp && !cfg.EnableFirebase && !cfg.EnableElasticsearch {
		rm("pkg/external")
		rm("pkg/httpclient/codec.go", "pkg/httpclient/service.go", "pkg/httpclient/token.go")
	}

	if len(removed) > 0 {
		fmt.Printf("  ✓ Cleaned up %d unused files/dirs\n", len(removed))
	}
}

// --- File generation (same as before) ---

func generateEnv(cfg *initConfig) {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Generated by: go-api-template init\n# Project: %s\n\n", cfg.ProjectName))
	b.WriteString("# Server\nSERVER_PORT=8080\nSERVER_MODE=debug\n\n")

	// Database
	b.WriteString(fmt.Sprintf("# Database\nDB_DRIVER=%s\n", cfg.DBDriver))
	if cfg.DBDriver == "mongo" {
		b.WriteString(fmt.Sprintf("MONGO_URI=mongodb://localhost:27017\nMONGO_DB_NAME=%s\n", cfg.ProjectName))
	} else {
		b.WriteString(fmt.Sprintf("DB_HOST=localhost\nDB_PORT=5432\nDB_USER=postgres\nDB_PASSWORD=postgres\nDB_NAME=%s\nDB_SSL_MODE=disable\nDB_MAX_CONNS=10\nDB_MIN_CONNS=2\n", cfg.ProjectName))
	}

	if cfg.EnableRedis {
		b.WriteString("\n# Redis\nREDIS_URL=redis://localhost:6379/0\nREDIS_POOL_SIZE=10\nREDIS_MIN_IDLE=2\n")
	}

	jwtSecret := randomHex(32)
	b.WriteString(fmt.Sprintf("\n# Auth\nAUTH_PROVIDER=jwt\nJWT_SECRET=%s\nJWT_EXPIRY=24h\n", jwtSecret))

	if cfg.EnableEncryption {
		b.WriteString(fmt.Sprintf("\n# Encryption (AES-256-GCM)\nENCRYPTION_KEY=%s\n", randomHex(32)))
	}
	if cfg.EnableSendGrid {
		b.WriteString("\n# SendGrid\nSENDGRID_URL=https://api.sendgrid.com\nSENDGRID_API_KEY=\nSENDGRID_FROM=noreply@example.com\n")
	}
	if cfg.EnableStripe {
		b.WriteString("\n# Stripe\nSTRIPE_URL=https://api.stripe.com\nSTRIPE_SECRET_KEY=\n")
	}
	if cfg.EnableIceWarp {
		b.WriteString("\n# IceWarp\nICEWARP_URL=\nICEWARP_USERNAME=\nICEWARP_PASSWORD=\nICEWARP_FROM=\n")
	}
	if cfg.EnableFirebase {
		b.WriteString("\n# Firebase\nFIREBASE_CREDENTIALS_FILE=\nFIREBASE_PROJECT_ID=\n")
	}
	if cfg.EnableElasticsearch {
		b.WriteString("\n# Elasticsearch\nELASTIC_URLS=http://localhost:9200\n")
	}
	if cfg.EnableKafka {
		b.WriteString(fmt.Sprintf("\n# Kafka\nKAFKA_BROKERS=localhost:9092\nKAFKA_CONSUMER_GROUP=%s-group\n", cfg.ProjectName))
	}
	if cfg.EnableOTEL {
		b.WriteString(fmt.Sprintf("\n# OpenTelemetry\nOTEL_ENABLED=true\nOTEL_SERVICE_NAME=%s\nOTEL_ENDPOINT=localhost:4318\n", cfg.ProjectName))
	}

	b.WriteString("\n# Workers\nWORKER_COUNT=4\nWORKER_QUEUE_SIZE=100\n")
	b.WriteString("\n# Logging\nLOG_LEVEL=debug\nLOG_FORMAT=console\n")

	os.WriteFile(".env", []byte(b.String()), 0644)
	fmt.Println("  ✓ .env generated")
}

func generateComposeOverride(cfg *initConfig) {
	var b strings.Builder
	var volumes []string

	b.WriteString("# Generated by: go-api-template init\nservices:\n")

	if cfg.EnableKafka {
		b.WriteString(`  kafka:
    image: bitnami/kafka:3.7
    restart: unless-stopped
    ports:
      - "9092:9092"
    environment:
      KAFKA_CFG_NODE_ID: 0
      KAFKA_CFG_PROCESS_ROLES: controller,broker
      KAFKA_CFG_CONTROLLER_QUORUM_VOTERS: 0@kafka:9093
      KAFKA_CFG_LISTENERS: PLAINTEXT://:9092,CONTROLLER://:9093
      KAFKA_CFG_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9092
      KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP: CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT
      KAFKA_CFG_CONTROLLER_LISTENER_NAMES: CONTROLLER
    volumes:
      - kafka_data:/bitnami/kafka
`)
		volumes = append(volumes, "kafka_data")
	}

	if cfg.EnableElasticsearch {
		b.WriteString(`  elasticsearch:
    image: elasticsearch:8.13.0
    restart: unless-stopped
    ports:
      - "9200:9200"
    environment:
      discovery.type: single-node
      xpack.security.enabled: "false"
      ES_JAVA_OPTS: "-Xms512m -Xmx512m"
    volumes:
      - es_data:/usr/share/elasticsearch/data
`)
		volumes = append(volumes, "es_data")
	}

	if cfg.EnableSonar {
		b.WriteString(`  sonarqube:
    image: sonarqube:10-community
    restart: unless-stopped
    ports:
      - "9000:9000"
    environment:
      SONAR_ES_BOOTSTRAP_CHECKS_DISABLE: "true"
    volumes:
      - sonar_data:/opt/sonarqube/data
`)
		volumes = append(volumes, "sonar_data")
	}

	if len(volumes) > 0 {
		b.WriteString("\nvolumes:\n")
		for _, v := range volumes {
			b.WriteString(fmt.Sprintf("  %s:\n", v))
		}
	}

	os.WriteFile("docker-compose.override.yml", []byte(b.String()), 0644)
	fmt.Println("  ✓ docker-compose.override.yml generated")
}

func printSummary(cfg *initConfig) {
	fmt.Println()
	fmt.Println("  ══════════════════════════════════════")
	fmt.Println("  Setup complete!")
	fmt.Println("  ══════════════════════════════════════")
	fmt.Printf("\n  Project:   %s\n  Database:  %s\n\n  Enabled:\n", cfg.ProjectName, cfg.DBDriver)

	checks := []struct {
		on   bool
		name string
	}{
		{cfg.EnableRedis, "Redis"},
		{cfg.EnableKafka, "Kafka + Outbox Pattern"},
		{cfg.EnableElasticsearch, "Elasticsearch"},
		{cfg.EnableOTEL, "OpenTelemetry"},
		{cfg.EnableEncryption, "Encryption (AES-256)"},
		{cfg.EnableCron, "Cron Scheduler"},
		{cfg.EnableWebSocket, "WebSocket"},
		{cfg.EnableKubernetes, "Kubernetes Manifests"},
		{cfg.EnableSendGrid, "SendGrid"},
		{cfg.EnableStripe, "Stripe"},
		{cfg.EnableIceWarp, "IceWarp"},
		{cfg.EnableFirebase, "Firebase"},
		{cfg.EnableSonar, "SonarQube"},
	}

	for _, c := range checks {
		if c.on {
			fmt.Printf("    ✓ %s\n", c.name)
		}
	}

	fmt.Println("\n  Next steps:")
	fmt.Println("    1. docker compose up -d")
	fmt.Println("    2. make build")
	fmt.Println("    3. ./bin/go-api-template serve")
	if cfg.EnableSonar {
		fmt.Println("    4. ./scripts/setup-sonar.sh")
	}
	fmt.Println()
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
