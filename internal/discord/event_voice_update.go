package discord

import (
	"log/slog"
	"time"

	"github.com/disgoorg/disgo/events"
	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storediscord"
)

func (t *Tracker) handleVoiceStateUpdateEvent(event *events.GuildVoiceStateUpdate) {
	ts := time.Now()

	p := &storediscord.SyncVoiceParam{
		UserID:  uint64(event.VoiceState.UserID),
		GuildID: uint64(event.VoiceState.GuildID),
	}

	if event.VoiceState.ChannelID != nil {
		p.ChannelID = uint64(*event.VoiceState.ChannelID)
	}

	t.enqueueTask(func() { t.applyVoiceStateUpdateEvent(p, ts) })
}

func (t *Tracker) applyVoiceStateUpdateEvent(p *storediscord.SyncVoiceParam, ts time.Time) {
	if err := t.db.WriteTx(t.ctx, func(tx *db.Tx) error {
		if !t.hasContinuity {
			heartbeatTs, err := storediscord.GetHeartbeat(tx)
			if err != nil {
				return err
			}

			if err := storediscord.CloseAllSessions(tx, heartbeatTs); err != nil {
				return err
			}
		}

		if err := storediscord.SyncVoice(tx, ts, p); err != nil {
			return err
		}

		if err := storediscord.UpsertHeartbeat(tx, ts); err != nil {
			return err
		}

		return nil
	}); err != nil {
		t.hasContinuity = false
		t.logger.Error("Failed to apply voice state update event", slog.Uint64("guildID", p.GuildID), slog.Uint64("userID", p.UserID), slog.Any("err", err))
	} else {
		t.hasContinuity = true
		t.logger.Debug("Applied voice state update event", slog.Uint64("guildID", p.GuildID), slog.Uint64("userID", p.UserID))
	}
}
