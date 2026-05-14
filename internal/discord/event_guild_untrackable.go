package discord

import (
	"log/slog"
	"time"

	"github.com/disgoorg/disgo/events"
	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storediscord"
)

func (t *Tracker) handleGuildUnavailableEvent(event *events.GuildUnavailable) {
	ts := time.Now()
	guildID := uint64(event.GuildID)
	t.enqueueTask(func() { t.applyGuildUntrackableEvent(guildID, ts) })
}

func (t *Tracker) handleGuildLeaveEvent(event *events.GuildLeave) {
	ts := time.Now()
	guildID := uint64(event.GuildID)
	t.enqueueTask(func() { t.applyGuildUntrackableEvent(guildID, ts) })
}

func (t *Tracker) applyGuildUntrackableEvent(guildID uint64, ts time.Time) {
	if err := t.db.WriteTx(t.ctx, func(tx *db.Tx) error {
		if t.hasContinuity {
			if err := storediscord.CloseAllSessionsForGuild(tx, guildID, ts); err != nil {
				return err
			}
		} else {
			heartbeatTs, err := storediscord.GetHeartbeat(tx)
			if err != nil {
				return err
			}

			if err := storediscord.CloseAllSessions(tx, heartbeatTs); err != nil {
				return err
			}
		}

		if err := storediscord.UpsertHeartbeat(tx, ts); err != nil {
			return err
		}

		return nil
	}); err != nil {
		t.hasContinuity = false
		t.logger.Error("Failed to apply guild unavailable/leave event", slog.Uint64("guildID", guildID), slog.Any("err", err))
	} else {
		t.hasContinuity = true
		t.logger.Debug("Applied guild unavailable/leave event", slog.Uint64("guildID", guildID))
	}
}
