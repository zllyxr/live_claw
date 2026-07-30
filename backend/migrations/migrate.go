package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed *.sql
var files embed.FS

func Apply(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("reserve migration connection: %w", err)
	}
	defer conn.Close() //nolint:errcheck

	var acquired sql.NullInt64
	if err = conn.QueryRowContext(
		ctx, "SELECT GET_LOCK('claw_v2_schema_migrations', 30)",
	).Scan(&acquired); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	if !acquired.Valid || acquired.Int64 != 1 {
		return fmt.Errorf("acquire migration lock: lock wait timed out")
	}
	defer releaseMigrationLock(conn)

	if _, err = conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version varchar(100) NOT NULL,
		checksum char(64) NOT NULL,
		applied_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		PRIMARY KEY (version)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci`); err != nil {
		return fmt.Errorf("ensure migration table: %w", err)
	}

	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return fmt.Errorf("list embedded migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		body, readErr := files.ReadFile(name)
		if readErr != nil {
			return fmt.Errorf("read migration %s: %w", name, readErr)
		}
		sum := sha256.Sum256(body)
		checksum := hex.EncodeToString(sum[:])

		var existing string
		err = conn.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE version=?", name).Scan(&existing)
		switch {
		case err == nil && existing != checksum:
			return fmt.Errorf("migration %s checksum changed after apply", name)
		case err == nil:
			continue
		case err != sql.ErrNoRows:
			return fmt.Errorf("query migration %s: %w", name, err)
		}

		if _, err = conn.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err = conn.ExecContext(ctx,
			"INSERT INTO schema_migrations(version,checksum) VALUES(?,?)", filepath.Base(name), checksum,
		); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
	}
	return nil
}

func releaseMigrationLock(conn *sql.Conn) {
	releaseContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var released sql.NullInt64
	err := conn.QueryRowContext(
		releaseContext, "SELECT RELEASE_LOCK('claw_v2_schema_migrations')",
	).Scan(&released)
	if err != nil || !released.Valid || released.Int64 != 1 {
		// Returning a connection that may still own a named lock to the pool
		// could block every future migration run.
		_ = conn.Raw(func(any) error {
			return driver.ErrBadConn
		})
	}
}
