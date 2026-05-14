package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrNotFound = errors.New(errPrefix + "not found")

type Tx struct {
	ctx   context.Context
	sqlTx *sql.Tx
}

func (db *DB) ReadTx(ctx context.Context, f func(tx *Tx) error) error {
	return runTx(ctx, db.sqlReader, f)
}

func (db *DB) WriteTx(ctx context.Context, f func(tx *Tx) error) error {
	return runTx(ctx, db.sqlWriter, f)
}

func runTx(ctx context.Context, db *sql.DB, f func(tx *Tx) error) error {
	sqlTx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf(errPrefix+"beginning transaction: %w", err)
	}
	defer sqlTx.Rollback()

	if err := f(&Tx{
		ctx:   ctx,
		sqlTx: sqlTx,
	}); err != nil {
		return fmt.Errorf(errPrefix+"running transaction: %w", err)
	}

	if err := sqlTx.Commit(); err != nil {
		return fmt.Errorf(errPrefix+"commiting transaction: %w", err)
	}

	return nil
}

func (tx *Tx) Exec(query string, args ...any) error {
	_, err := tx.sqlTx.ExecContext(tx.ctx, query, args...)
	return err
}

func (tx *Tx) Query(query string, args ...any) (*sql.Rows, error) {
	return tx.sqlTx.QueryContext(tx.ctx, query, args...)
}

func (tx *Tx) QueryRow(query string, args ...any) *sql.Row {
	return tx.sqlTx.QueryRowContext(tx.ctx, query, args...)
}
