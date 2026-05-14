package storecommon

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/szczursonn/online-activity-tracker/internal/db"
)

const errPrefix = "db/storecommon: "

func UpsertHeartbeat(tx *db.Tx, key string, ts time.Time) error {
	if err := tx.Exec(`
		INSERT INTO heartbeat (key, timestamp)
		VALUES (?1, ?2)
		ON CONFLICT(key)
		DO UPDATE SET timestamp = MAX(timestamp, ?2);
	`, key, ts.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"upserting \"%s\" heartbeat: %w", key, err)
	}

	return nil
}

func GetHeartbeat(tx *db.Tx, key string) (time.Time, error) {
	var tsMillis int64
	if err := tx.QueryRow(`
		SELECT timestamp
		FROM heartbeat
		WHERE key = ?;
	`, key).Scan(&tsMillis); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, fmt.Errorf(errPrefix+"getting \"%s\" heartbeat: %w", key, err)
	}

	return time.UnixMilli(tsMillis), nil
}
