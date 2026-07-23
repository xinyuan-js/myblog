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

	"github.com/example/myblog/apps/api/internal/config"
	_ "github.com/go-sql-driver/mysql"
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
	defer func() {
		releaseContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(releaseContext, `SELECT RELEASE_LOCK('myblog_schema_migrations')`)
	}()

	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			checksum BINARY(32) NOT NULL,
			applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migration_attempts (
			version VARCHAR(255) PRIMARY KEY,
			checksum BINARY(32) NOT NULL,
			started_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`); err != nil {
		return fmt.Errorf("create migration attempt table: %w", err)
	}
	// If the process stopped after recording a completed migration but before
	// deleting its attempt marker, the matching checksum proves it is safe to
	// clean up. Every other marker may represent partially committed MySQL DDL
	// and must stop startup instead of blindly re-running non-transactional SQL.
	if _, err := conn.ExecContext(ctx, `
		DELETE attempt
		FROM schema_migration_attempts AS attempt
		INNER JOIN schema_migrations AS applied
			ON applied.version = attempt.version AND applied.checksum = attempt.checksum
	`); err != nil {
		return fmt.Errorf("clean completed migration attempts: %w", err)
	}
	var incompleteVersion string
	err = conn.QueryRowContext(ctx, `
		SELECT version
		FROM schema_migration_attempts
		ORDER BY started_at ASC, version ASC
		LIMIT 1
	`).Scan(&incompleteVersion)
	if err == nil {
		return fmt.Errorf(
			"migration %s may be partially applied; restore the pre-upgrade backup or inspect the schema before clearing its attempt marker",
			incompleteVersion,
		)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check incomplete migration attempts: %w", err)
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

		if _, err := conn.ExecContext(ctx,
			`INSERT INTO schema_migration_attempts (version, checksum) VALUES (?, ?)`,
			entry.Name(), checksum[:],
		); err != nil {
			return fmt.Errorf("record migration %s attempt: %w", entry.Name(), err)
		}
		if _, err := conn.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf(
				"apply migration %s: %w; the attempt marker was retained because MySQL DDL may have partially committed",
				entry.Name(), err,
			)
		}
		recordTx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin recording migration %s: %w", entry.Name(), err)
		}
		if _, err := recordTx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, checksum) VALUES (?, ?)`, entry.Name(), checksum[:]); err != nil {
			_ = recordTx.Rollback()
			return fmt.Errorf("record migration %s: %w", entry.Name(), err)
		}
		if _, err := recordTx.ExecContext(ctx,
			`DELETE FROM schema_migration_attempts WHERE version = ?`, entry.Name()); err != nil {
			_ = recordTx.Rollback()
			return fmt.Errorf("clear migration %s attempt: %w", entry.Name(), err)
		}
		if err := recordTx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s record: %w", entry.Name(), err)
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
