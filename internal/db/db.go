package db

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"net/url"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const errPrefix = "db: "

type DB struct {
	sqlWriter *sql.DB
	sqlReader *sql.DB
}

func Open(ctx context.Context, dbPath string) (*DB, error) {
	if dbPath == "" {
		dbPath = "./oat.db"
	}

	writeDSN, readDSN, err := makeSQLiteDSNs(dbPath)
	if err != nil {
		return nil, err
	}

	db := &DB{}
	db.sqlWriter, err = sql.Open("sqlite", writeDSN)
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"failed to open writer db: %w", err)
	}
	db.sqlWriter.SetMaxOpenConns(1)

	db.sqlReader, err = sql.Open("sqlite", readDSN)
	if err != nil {
		db.sqlWriter.Close()
		return nil, fmt.Errorf(errPrefix+"failed to open reader db: %w", err)
	}

	return db, nil
}

func makeSQLiteDSNs(dbFilePath string) (string, string, error) {
	abs, err := filepath.Abs(dbFilePath)
	if err != nil {
		return "", "", err
	}
	p := filepath.ToSlash(abs)
	// windows: "C:/foo" -> "/C:/foo"
	if len(p) > 1 && p[1] == ':' {
		p = "/" + p
	}

	u := url.URL{
		Scheme: "file",
		Path:   p,
	}

	q := url.Values{
		"_pragma": []string{"foreign_keys(ON)", "busy_timeout(5000)", "journal_mode(WAL)", "synchronous(NORMAL)"},
		"_txlock": []string{"immediate"},
	}
	u.RawQuery = q.Encode()
	writeDSN := u.String()

	delete(q, "_txlock")
	q["mode"] = []string{"ro"}
	u.RawQuery = q.Encode()

	return writeDSN, u.String(), nil
}

func (db *DB) Vacuum(ctx context.Context) error {
	if _, err := db.sqlWriter.ExecContext(ctx, `VACUUM;`); err != nil {
		return fmt.Errorf(errPrefix+"executing vacuum: %w", err)
	}

	return nil
}

func (db *DB) CheckpointTruncate(ctx context.Context) error {
	if _, err := db.sqlWriter.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE);`); err != nil {
		return fmt.Errorf(errPrefix+"executing wal checkpoint truncate: %w", err)
	}

	return nil
}

func (db *DB) VacuumInto(ctx context.Context, dest string) error {
	if _, err := db.sqlWriter.ExecContext(ctx, `VACUUM INTO ?;`, dest); err != nil {
		return fmt.Errorf(errPrefix+"executing vacuum into: %w", err)
	}

	return nil
}

func (db *DB) Close() {
	db.sqlWriter.Close()
	db.sqlReader.Close()
}

func AssertStorableAsInteger(val uint64) error {
	if val > math.MaxInt64 {
		return fmt.Errorf(errPrefix+"uint64 value %d exceeds INTEGER max value %d", val, math.MaxInt64)
	}

	return nil
}
