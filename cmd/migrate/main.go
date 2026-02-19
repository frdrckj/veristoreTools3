package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/verifone/veristoretools3/internal/config"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Check for subcommands: "verify" compares v2 and v3 row counts.
	if len(os.Args) > 1 && os.Args[1] == "verify" {
		runVerifyCommand()
		return
	}

	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	db, err := sql.Open("mysql", cfg.Database.DSN())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal().Err(err).Msg("failed to ping database")
	}
	log.Info().Str("database", cfg.Database.Name).Msg("connected to database")

	migrationsDir := "migrations"
	if err := runMigrations(db, migrationsDir); err != nil {
		log.Fatal().Err(err).Msg("migration failed")
	}

	log.Info().Msg("all migrations completed successfully")
}

// runVerifyCommand loads config and runs the migration verification that
// compares row counts between veristoretools2 and veristoretools3 databases.
func runVerifyCommand() {
	configPath := "config.yaml"
	if len(os.Args) > 2 {
		configPath = os.Args[2]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	// Build DSNs: v3 uses the configured database, v2 uses the same
	// connection settings but with "veristoretools2" as the database name.
	v3DSN := cfg.Database.DSN()

	v2Cfg := cfg.Database
	v2Cfg.Name = "veristoretools2"
	v2DSN := v2Cfg.DSN()

	log.Info().Str("v2", "veristoretools2").Str("v3", cfg.Database.Name).Msg("verifying migration")

	if err := runVerify(v2DSN, v3DSN); err != nil {
		log.Fatal().Err(err).Msg("verification failed")
	}
}

func runMigrations(db *sql.DB, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) == ".sql" {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	for _, file := range files {
		path := filepath.Join(dir, file)
		log.Info().Str("file", file).Msg("running migration")

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration file %s: %w", file, err)
		}

		statements := splitStatements(string(content))
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("execute migration %s: %w\nStatement: %s", file, err, stmt)
			}
		}

		log.Info().Str("file", file).Msg("migration completed")
	}

	return nil
}

func splitStatements(content string) []string {
	var statements []string
	var current strings.Builder

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip comment-only lines
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		current.WriteString(line)
		current.WriteString("\n")
		if strings.HasSuffix(trimmed, ";") {
			statements = append(statements, current.String())
			current.Reset()
		}
	}

	// Add any remaining content
	if remaining := strings.TrimSpace(current.String()); remaining != "" {
		statements = append(statements, remaining)
	}

	return statements
}
