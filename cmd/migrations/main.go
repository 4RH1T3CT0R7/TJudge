package main

import (
	"fmt"
	"log"
	"os"

	"github.com/bmstu-itstech/tjudge/internal/config"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Загружаем конфигурацию
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Создаём экземпляр migrate
	m, err := migrate.New(
		"file://migrations",
		cfg.Database.DSNURL(),
	)
	if err != nil {
		log.Fatalf("Failed to create migrate instance: %v", err)
	}
	defer m.Close()

	command := os.Args[1]

	switch command {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("Failed to apply migrations: %v", err)
		}
		fmt.Println("Migrations applied successfully")

	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("Failed to rollback migrations: %v", err)
		}
		fmt.Println("Migrations rolled back successfully")

	case "force":
		if len(os.Args) < 3 {
			log.Fatal("Version number required for force command")
		}
		var version int
		if _, err := fmt.Sscanf(os.Args[2], "%d", &version); err != nil {
			log.Fatalf("Invalid version number: %v", err)
		}
		if err := m.Force(version); err != nil {
			log.Fatalf("Failed to force version: %v", err)
		}
		fmt.Printf("Forced version to %d\n", version)

	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("Failed to get version: %v", err)
		}
		fmt.Printf("Current version: %d (dirty: %t)\n", version, dirty)

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: migrate <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  up      - Apply all pending migrations")
	fmt.Println("  down    - Rollback all migrations")
	fmt.Println("  force N - Force database version to N")
	fmt.Println("  version - Show current migration version")
}
