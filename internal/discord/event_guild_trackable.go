package discord

import (
	"log/slog"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storediscord"
)

type GuildSnapshotVoiceState struct {
	ChannelID uint64
}

func (t *Tracker) handleGuildJoinEvent(event *events.GuildJoin) {
	ts := time.Now()
	snap := buildGuildSnapshot(&event.Guild)
	t.enqueueTask(func() { t.applyGuildTrackableEvent(snap, ts) })
}

func (t *Tracker) handleGuildAvailableEvent(event *events.GuildAvailable) {
	ts := time.Now()
	snap := buildGuildSnapshot(&event.Guild)
	t.enqueueTask(func() { t.applyGuildTrackableEvent(snap, ts) })
}

func (t *Tracker) handleGuildReadyEvent(event *events.GuildReady) {
	ts := time.Now()
	snap := buildGuildSnapshot(&event.Guild)
	t.enqueueTask(func() { t.applyGuildTrackableEvent(snap, ts) })
}

func buildGuildSnapshot(gatewayGuild *discord.GatewayGuild) *storediscord.ResetGuildStateParam {
	p := &storediscord.ResetGuildStateParam{
		ID:                  uint64(gatewayGuild.ID),
		Name:                gatewayGuild.Name,
		VoiceChannelsByID:   make(map[uint64]storediscord.ResetGuildStateVoiceChannel, len(gatewayGuild.Channels)),
		MembersByUserID:     make(map[uint64]storediscord.ResetGuildStateUser, len(gatewayGuild.Members)),
		PresencesByUserID:   make(map[uint64]storediscord.ResetGuildStatePresence, len(gatewayGuild.Presences)),
		VoiceStatesByUserID: make(map[uint64]storediscord.ResetGuildStateVoiceState, len(gatewayGuild.VoiceStates)),
	}

	if iconURLPtr := gatewayGuild.IconURL(); iconURLPtr != nil {
		p.IconURL = *iconURLPtr
	}

	for _, channel := range gatewayGuild.Channels {
		if t := channel.Type(); t != discord.ChannelTypeGuildVoice && t != discord.ChannelTypeGuildStageVoice {
			continue
		}

		p.VoiceChannelsByID[uint64(channel.ID())] = storediscord.ResetGuildStateVoiceChannel{
			Name: channel.Name(),
		}
	}

	for _, presence := range gatewayGuild.Presences {
		activities := make([]storediscord.ResetGuildStateActivity, 0, len(presence.Activities))
		for _, activity := range presence.Activities {
			var details string
			if activity.Details != nil {
				details = *activity.Details
			}

			var state string
			if activity.State != nil {
				state = *activity.State
			}

			activities = append(activities, storediscord.ResetGuildStateActivity{
				Name:    activity.Name,
				Details: details,
				State:   state,
			})
		}

		p.PresencesByUserID[uint64(presence.PresenceUser.ID)] = storediscord.ResetGuildStatePresence{
			DesktopStatus: discordOnlineStatusToInt(presence.ClientStatus.Desktop),
			MobileStatus:  discordOnlineStatusToInt(presence.ClientStatus.Mobile),
			WebStatus:     discordOnlineStatusToInt(presence.ClientStatus.Web),
			Activities:    activities,
		}
	}

	for _, member := range gatewayGuild.Members {
		p.MembersByUserID[uint64(member.User.ID)] = storediscord.ResetGuildStateUser{
			Name:      member.User.EffectiveName(),
			AvatarURL: member.User.EffectiveAvatarURL(),
		}

		if _, ok := p.PresencesByUserID[uint64(member.User.ID)]; !ok {
			// if the member is offline, presence is not transmitted
			p.PresencesByUserID[uint64(member.User.ID)] = storediscord.ResetGuildStatePresence{
				DesktopStatus: discordOnlineStatusToInt(discord.OnlineStatusOffline),
				MobileStatus:  discordOnlineStatusToInt(discord.OnlineStatusOffline),
				WebStatus:     discordOnlineStatusToInt(discord.OnlineStatusOffline),
			}
		}
	}

	for _, vs := range gatewayGuild.VoiceStates {
		if vs.ChannelID == nil {
			continue
		}

		p.VoiceStatesByUserID[uint64(vs.UserID)] = storediscord.ResetGuildStateVoiceState{
			ChannelID: uint64(*vs.ChannelID),
		}
	}

	return p
}

func (t *Tracker) applyGuildTrackableEvent(p *storediscord.ResetGuildStateParam, ts time.Time) {
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

		if err := storediscord.ResetGuildState(tx, ts, p); err != nil {
			return err
		}

		if err := storediscord.UpsertHeartbeat(tx, ts); err != nil {
			return err
		}

		return nil
	}); err != nil {
		t.hasContinuity = false
		t.logger.Error("Failed to apply guild available/join/ready event", slog.Uint64("guildID", p.ID), slog.Any("err", err))
	} else {
		t.hasContinuity = true
		t.logger.Debug("Applied guild available/join/ready event", slog.Uint64("guildID", p.ID))
	}
}
