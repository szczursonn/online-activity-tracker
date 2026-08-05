package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
)

const migrationsDir = "migrations"

//go:embed migrations/*.sql
var migrationsFS embed.FS

type migration struct {
	name string
	sql  string
}

type MigrationHooks struct {
	OnStatus func(status MigrationStatus)
	OnStart  func(name string, left int)
	OnDone   func(name string)
}

type MigrationStatus struct {
	LastApplied  string
	PendingCount int
}

func loadMigrations() ([]*migration, error) {
	fsEntries, err := migrationsFS.ReadDir(migrationsDir)
	if err != nil {
		return nil, err
	}

	migrations := make([]*migration, 0, len(fsEntries))
	for _, fsEntry := range fsEntries {
		fileName := fsEntry.Name()
		if fsEntry.IsDir() || path.Ext(fileName) != ".sql" {
			continue
		}

		buf, err := migrationsFS.ReadFile(path.Join(migrationsDir, fileName))
		if err != nil {
			return nil, err
		}

		if len(buf) == 0 {
			return nil, fmt.Errorf("migration file %q is empty", fileName)
		}

		migrations = append(migrations, &migration{
			name: strings.TrimSuffix(fileName, path.Ext(fileName)),
			sql:  string(buf),
		})
	}

	slices.SortFunc(migrations, func(a *migration, b *migration) int {
		return strings.Compare(a.name, b.name)
	})

	return migrations, nil
}

// pendingMigrations returns the migrations after lastApplied. lastApplied is "" on a database that
// has never been migrated, in which case every migration is pending.
func pendingMigrations(migrations []*migration, lastApplied string) ([]*migration, error) {
	if lastApplied == "" {
		return migrations, nil
	}

	lastAppliedIdx := slices.IndexFunc(migrations, func(m *migration) bool {
		return m.name == lastApplied
	})
	if lastAppliedIdx == -1 {
		knownNames := make([]string, 0, len(migrations))
		for _, migration := range migrations {
			knownNames = append(knownNames, migration.name)
		}

		return nil, fmt.Errorf("database is at unknown migration %q, this build only knows [%s] (is the app outdated?)", lastApplied, strings.Join(knownNames, ", "))
	}

	return migrations[lastAppliedIdx+1:], nil
}

func readLastAppliedMigrationName(tx *Tx) (string, error) {
	var lastAppliedName sql.NullString
	if err := tx.QueryRow(`
		SELECT last_applied_name
		FROM migration_status
		LIMIT 1;
	`).Scan(&lastAppliedName); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("getting last applied migration name: %w", err)
		}
	}

	return lastAppliedName.String, nil
}

func (db *DB) RunMigrations(ctx context.Context, hooks MigrationHooks) error {
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

		lastAppliedName, err := readLastAppliedMigrationName(tx)
		if err != nil {
			return err
		}

		migrationsToRun, err := pendingMigrations(migrations, lastAppliedName)
		if err != nil {
			return err
		}

		if hooks.OnStatus != nil {
			hooks.OnStatus(MigrationStatus{
				LastApplied:  lastAppliedName,
				PendingCount: len(migrationsToRun),
			})
		}

		if len(migrationsToRun) == 0 {
			return nil
		}

		for i, migration := range migrationsToRun {
			if hooks.OnStart != nil {
				hooks.OnStart(migration.name, len(migrationsToRun)-i)
			}

			if err := tx.Exec(migration.sql); err != nil {
				return fmt.Errorf("executing migration %q sql: %w", migration.name, err)
			}

			if hooks.OnDone != nil {
				hooks.OnDone(migration.name)
			}
		}

		// the whole run is a single transaction, so only the final position has to be recorded
		newLastAppliedName := migrationsToRun[len(migrationsToRun)-1].name
		if err := tx.Exec(`DELETE FROM migration_status;`); err != nil {
			return fmt.Errorf("clearing migration status: %w", err)
		}
		if err := tx.Exec(`INSERT INTO migration_status VALUES (?);`, newLastAppliedName); err != nil {
			return fmt.Errorf("setting last applied migration name %q: %w", newLastAppliedName, err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf(errPrefix+"in write tx: %w", err)
	}

	return nil
}

func (db *DB) MigrationStatus(ctx context.Context) (MigrationStatus, error) {
	migrations, err := loadMigrations()
	if err != nil {
		return MigrationStatus{}, fmt.Errorf(errPrefix+"loading migrations: %w", err)
	}

	var status MigrationStatus
	if err := db.ReadTx(ctx, func(tx *Tx) error {
		var tableExists int
		if err := tx.QueryRow(`
			SELECT 1
			FROM sqlite_master
			WHERE type = 'table' AND name = 'migration_status'
			LIMIT 1;
		`).Scan(&tableExists); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("checking for \"migration_status\" table: %w", err)
			}
		}

		if tableExists == 1 {
			lastAppliedName, err := readLastAppliedMigrationName(tx)
			if err != nil {
				return err
			}

			status.LastApplied = lastAppliedName
		}

		pending, err := pendingMigrations(migrations, status.LastApplied)
		if err != nil {
			return err
		}

		status.PendingCount = len(pending)

		return nil
	}); err != nil {
		return MigrationStatus{}, fmt.Errorf(errPrefix+"in read tx: %w", err)
	}

	return status, nil
}
