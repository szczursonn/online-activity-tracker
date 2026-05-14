package discord

import (
	"log/slog"
	"time"

	"github.com/disgoorg/disgo/events"
	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storediscord"
)

func (t *Tracker) handlePresenceUpdateEvent(event *events.PresenceUpdate) {
	ts := time.Now()

	p := &storediscord.SyncPresenceParam{
		DesktopStatus: discordOnlineStatusToInt(event.Presence.ClientStatus.Desktop),
		MobileStatus:  discordOnlineStatusToInt(event.Presence.ClientStatus.Mobile),
		WebStatus:     discordOnlineStatusToInt(event.Presence.ClientStatus.Web),
		UserID:        uint64(event.PresenceUser.ID),
		GuildID:       uint64(event.Presence.GuildID),
		Activities:    make([]storediscord.SyncPresenceParamActivity, 0, len(event.Presence.Activities)),
	}

	if cachedMember, ok := t.client.Caches.Member(event.Presence.GuildID, event.PresenceUser.ID); ok {
		p.UserExtraInfo = &storediscord.SyncPresenceParamUserExtraInfo{
			Name:      cachedMember.User.EffectiveName(),
			AvatarURL: cachedMember.User.EffectiveAvatarURL(),
		}
	}

	if cachedGuild, ok := t.client.Caches.Guild(event.Presence.GuildID); ok {
		var iconURL string
		if iconURLPtr := cachedGuild.IconURL(); iconURLPtr != nil {
			iconURL = *iconURLPtr
		}

		p.GuildExtraInfo = &storediscord.SyncPresenceParamGuildExtraInfo{
			Name:    cachedGuild.Name,
			IconURL: iconURL,
		}
	}

	for _, activity := range event.Presence.Activities {
		var details string
		if activity.Details != nil {
			details = *activity.Details
		}

		var state string
		if activity.State != nil {
			state = *activity.State
		}

		p.Activities = append(p.Activities, storediscord.SyncPresenceParamActivity{
			Name:    activity.Name,
			Details: details,
			State:   state,
		})
	}

	t.enqueueTask(func() { t.applyPresenceUpdateEvent(p, ts) })
}

func (t *Tracker) applyPresenceUpdateEvent(p *storediscord.SyncPresenceParam, ts time.Time) {
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

		if err := storediscord.SyncPresence(tx, ts, p); err != nil {
			return err
		}

		if err := storediscord.UpsertHeartbeat(tx, ts); err != nil {
			return err
		}

		return nil
	}); err != nil {
		t.hasContinuity = false
		t.logger.Error("Failed to apply presence update event", slog.Uint64("guildID", p.GuildID), slog.Uint64("userID", p.UserID), slog.Any("err", err))
	} else {
		t.hasContinuity = true
		t.logger.Debug("Applied presence update event", slog.Uint64("guildID", p.GuildID), slog.Uint64("userID", p.UserID))
	}
}
