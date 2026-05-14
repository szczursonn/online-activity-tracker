package discord

import (
	"log/slog"
	"time"

	"github.com/disgoorg/disgo/events"
	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storediscord"
)

func (t *Tracker) handleGuildMemberLeaveEvent(event *events.GuildMemberLeave) {
	ts := time.Now()
	guildID := uint64(event.GuildID)
	userID := uint64(event.User.ID)
	t.enqueueTask(func() { t.applyGuildMemberUntrackableEvent(guildID, userID, ts) })
}

func (t *Tracker) applyGuildMemberUntrackableEvent(guildID uint64, userID uint64, ts time.Time) {
	if err := t.db.WriteTx(t.ctx, func(tx *db.Tx) error {
		if t.hasContinuity {
			if err := storediscord.CloseAllSessionsForGuildAndUser(tx, guildID, userID, ts); err != nil {
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
		t.logger.Error("Failed to apply guild member leave event", slog.Uint64("guildID", guildID), slog.Uint64("userID", userID), slog.Any("err", err))
	} else {
		t.hasContinuity = true
		t.logger.Debug("Applied guild member leave event", slog.Uint64("guildID", guildID), slog.Uint64("userID", userID))
	}
}
