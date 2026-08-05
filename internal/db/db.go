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

func Open(ctx context.Context, dbFilePath string) (*DB, error) {
	if dbFilePath == "" {
		dbFilePath = "./oat.db"
	}

	writeDSN, readDSN, err := makeDSNs(dbFilePath)
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"preparing dsn: %w", err)
	}

	sqlWriter, err := sql.Open("sqlite", writeDSN)
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"opening write handle: %w", err)
	}
	sqlWriter.SetMaxOpenConns(1)
	sqlWriter.SetMaxIdleConns(1)

	if err := sqlWriter.PingContext(ctx); err != nil {
		sqlWriter.Close()
		return nil, fmt.Errorf(errPrefix+"pinging write handle: %w", err)
	}

	sqlReader, err := sql.Open("sqlite", readDSN)
	if err != nil {
		sqlWriter.Close()
		return nil, fmt.Errorf(errPrefix+"failed to open reader db: %w", err)
	}

	return &DB{
		sqlReader: sqlReader,
		sqlWriter: sqlWriter,
	}, nil
}

var writeDSNRawQuery = url.Values{
	"_pragma": []string{"foreign_keys(ON)", "busy_timeout(5000)", "journal_mode(WAL)", "synchronous(NORMAL)"},
	"_txlock": []string{"immediate"},
}.Encode()

var readDSNRawQuery = url.Values{
	"_pragma": []string{
		"busy_timeout(5000)",
		"journal_mode(WAL)",
		"query_only(1)",
	},
	"_txlock": []string{"deferred"},
}.Encode()

func makeDSNs(dbFilePath string) (string, string, error) {
	abs, err := filepath.Abs(dbFilePath)
	if err != nil {
		return "", "", err
	}
	resolvedFilePath := filepath.ToSlash(abs)
	// windows: "C:/foo" -> "/C:/foo"
	if len(resolvedFilePath) > 1 && resolvedFilePath[1] == ':' {
		resolvedFilePath = "/" + resolvedFilePath
	}

	u := url.URL{
		Scheme: "file",
		Path:   resolvedFilePath,
	}

	u.RawQuery = writeDSNRawQuery
	writeDSN := u.String()

	u.RawQuery = readDSNRawQuery
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
