package cmd

import (
	"fmt"
	"os"

	"github.com/pressly/goose/v3"
)

func runMigrate() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: gostack-kit migrate <command>")
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
			fatal("Usage: gostack-kit migrate create <name>")
		}
		must(goose.Create(db, migrationsDir, os.Args[3], "sql"))
		fmt.Printf("Created migration: %s\n", os.Args[3])

	case "up-to":
		if len(os.Args) < 4 {
			fatal("Usage: gostack-kit migrate up-to <version>")
		}
		must(goose.UpTo(db, migrationsDir, parseInt64(os.Args[3])))

	case "down-to":
		if len(os.Args) < 4 {
			fatal("Usage: gostack-kit migrate down-to <version>")
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
