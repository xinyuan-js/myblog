package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/example/myblog/apps/api/internal/config"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Open(ctx context.Context, cfg config.Config) (*sql.DB, error) {
	db, err := sql.Open("mysql", cfg.DatabaseDSN)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	db.SetMaxOpenConns(cfg.DatabaseMaxOpen)
	db.SetMaxIdleConns(cfg.DatabaseMaxIdle)
	db.SetConnMaxLifetime(cfg.DatabaseMaxLife)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Close()

	var acquired int
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK('myblog_schema_migrations', 10)`).Scan(&acquired); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	if acquired != 1 {
		return errors.New("acquire migration lock: timed out")
	}
	defer conn.ExecContext(context.Background(), `SELECT RELEASE_LOCK('myblog_schema_migrations')`)

	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			checksum BINARY(32) NOT NULL,
			applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		body, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		checksum := sha256.Sum256(body)
		var stored []byte
		err = conn.QueryRowContext(ctx, `SELECT checksum FROM schema_migrations WHERE version = ?`, entry.Name()).Scan(&stored)
		switch {
		case err == nil:
			if len(stored) != len(checksum) || !equalBytes(stored, checksum[:]) {
				return fmt.Errorf("migration %s was modified after being applied", entry.Name())
			}
			continue
		case !errors.Is(err, sql.ErrNoRows):
			return fmt.Errorf("check migration %s: %w", entry.Name(), err)
		}

		if _, err := conn.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}
		if _, err := conn.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, checksum) VALUES (?, ?)`, entry.Name(), checksum[:]); err != nil {
			return fmt.Errorf("record migration %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var different byte
	for i := range a {
		different |= a[i] ^ b[i]
	}
	return different == 0
}
