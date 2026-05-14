package discord

import (
	"log/slog"
	"time"

	"github.com/disgoorg/disgo/events"
	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storediscord"
)

func (t *Tracker) handleReadyEvent(event *events.Ready) {
	ts := time.Now()
	t.logger.Info("Logged into Discord", slog.String("userID", event.User.ID.String()), slog.String("username", event.User.Username))
	t.enqueueTask(func() { t.applyReadyEvent(ts) })
}

func (t *Tracker) applyReadyEvent(ts time.Time) {
	if err := t.db.WriteTx(t.ctx, func(tx *db.Tx) error {
		heartbeatTs, err := storediscord.GetHeartbeat(tx)
		if err != nil {
			return err
		}

		if err := storediscord.CloseAllSessions(tx, heartbeatTs); err != nil {
			return err
		}

		if err := storediscord.UpsertHeartbeat(tx, ts); err != nil {
			return err
		}

		return nil
	}); err != nil {
		t.hasContinuity = false
		t.logger.Error("Failed to apply ready event", slog.Any("err", err))
	} else {
		t.hasContinuity = true
		t.logger.Debug("Applied ready event")
	}
}
