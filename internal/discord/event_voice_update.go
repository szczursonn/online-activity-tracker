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

	if cachedMember, ok := t.client.Caches.Member(event.VoiceState.GuildID, event.VoiceState.UserID); ok {
		p.UserExtraInfo = &storediscord.SyncVoiceParamUserExtraInfo{
			Name:      cachedMember.User.EffectiveName(),
			AvatarURL: cachedMember.User.EffectiveAvatarURL(),
		}
	}

	if cachedGuild, ok := t.client.Caches.Guild(event.VoiceState.GuildID); ok {
		var iconURL string
		if iconURLPtr := cachedGuild.IconURL(); iconURLPtr != nil {
			iconURL = *iconURLPtr
		}

		p.GuildExtraInfo = &storediscord.SyncVoiceParamGuildExtraInfo{
			Name:    cachedGuild.Name,
			IconURL: iconURL,
		}
	}

	if event.VoiceState.ChannelID != nil {
		p.ChannelID = uint64(*event.VoiceState.ChannelID)
		if cachedChannel, ok := t.client.Caches.Channel(*event.VoiceState.ChannelID); ok {
			p.ChannelExtraInfo = &storediscord.SyncVoiceParamChannelExtraInfo{
				Name: cachedChannel.Name(),
			}
		}
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
