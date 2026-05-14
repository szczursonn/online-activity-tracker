package db

import (
	"context"
	"database/sql"
	"embed"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type migration struct {
	name string
	sql  string
}

func loadMigrations() ([]*migration, error) {
	fsEntries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}

	migrations := make([]*migration, 0, len(fsEntries))
	for _, fsEntry := range fsEntries {
		migration, err := func() (*migration, error) {
			fileName := fsEntry.Name()
			f, err := migrationsFS.Open(path.Join("migrations", fileName))
			if err != nil {
				return nil, err
			}
			defer f.Close()

			buf, err := io.ReadAll(f)
			if err != nil {
				return nil, err
			}

			return &migration{
				name: strings.TrimSuffix(fileName, filepath.Ext(fileName)),
				sql:  string(buf),
			}, nil
		}()
		if err != nil {
			return nil, err
		}

		migrations = append(migrations, migration)
	}

	slices.SortFunc(migrations, func(a *migration, b *migration) int {
		if a.name > b.name {
			return 1
		}

		if a.name < b.name {
			return -1
		}

		return 0
	})

	return migrations, nil
}

func (db *DB) RunMigrations(ctx context.Context) error {
	migrations, err := loadMigrations()
	if err != nil {
		return fmt.Errorf(errPrefix+"loading migrations: %w", err)
	}

	if err := db.WriteTx(ctx, func(tx *Tx) error {
		if err := tx.Exec(`
			CREATE TABLE IF NOT EXISTS "migration_status" (
				"last_applied_name"		TEXT,
				PRIMARY KEY("last_applied_name")
			) WITHOUT ROWID,STRICT;
    	`); err != nil {
			return fmt.Errorf("creating if not exists \"migration_status\" table: %w", err)
		}

		// schema version = index+1 of last applied migration
		var lastAppliedMigrationName string
		if err := tx.QueryRow(`
			SELECT last_applied_name
			FROM migration_status;
		`).Scan(&lastAppliedMigrationName); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("getting last applied migration name: %w", err)
			}
		}

		var migrationsToRun []*migration
		if lastAppliedMigrationName == "" {
			if err := tx.Exec(`
				INSERT INTO migration_status
				VALUES ("0000_dummy");
			`); err != nil {
				return fmt.Errorf("inserting dummy last applied migration: %w", err)
			}

			migrationsToRun = migrations
		} else {
			lastAppliedMigrationIdx := slices.IndexFunc(migrations, func(m *migration) bool {
				return m.name == lastAppliedMigrationName
			})

			if lastAppliedMigrationIdx == -1 {
				return fmt.Errorf("last applied migration (%s) does not exist (is the app outdated?)", lastAppliedMigrationName)
			}

			migrationsToRun = migrations[lastAppliedMigrationIdx+1:]
		}

		if len(migrationsToRun) == 0 {
			return nil
		}

		for _, migration := range migrationsToRun {
			if err := tx.Exec(migration.sql); err != nil {
				return fmt.Errorf("executing migration \"%s\" sql: %w", migration.name, err)
			}

			if err := tx.Exec(`
				UPDATE migration_status
				SET last_applied_name = ?;
			`, migration.name); err != nil {
				return fmt.Errorf("updating db last applied migration name \"%s\": %w", migration.name, err)
			}
		}

		return nil
	}); err != nil {
		return fmt.Errorf(errPrefix+"in write tx: %w", err)
	}

	return nil
}
