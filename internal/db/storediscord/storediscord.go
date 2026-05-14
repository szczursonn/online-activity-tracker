package storediscord

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storecommon"
)

const errPrefix = "db/storediscord: "

const heartbeatKey = "discord"

func GetHeartbeat(tx *db.Tx) (time.Time, error) {
	return storecommon.GetHeartbeat(tx, heartbeatKey)
}

func UpsertHeartbeat(tx *db.Tx, ts time.Time) error {
	return storecommon.UpsertHeartbeat(tx, heartbeatKey, ts)
}

func InsertOrEnableUser(tx *db.Tx, userID uint64, presenceGuildID uint64, closeSessionsAt time.Time) error {
	if userID == 0 {
		return fmt.Errorf(errPrefix + "user id is required")
	}

	if err := db.AssertStorableAsInteger(userID); err != nil {
		return err
	}

	if err := ensureGuildInserted(tx, presenceGuildID); err != nil {
		return err
	}

	if err := tx.Exec(`
		UPDATE discord_session_status
		SET end_observed_at = MAX(?3, start_observed_at)
		WHERE end_observed_at IS NULL AND user_id = ?1 AND guild_id != ?2;
	`, int64(userID), int64(presenceGuildID), closeSessionsAt.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"closing all status sessions for user %d: %w", userID, err)
	}

	if err := tx.Exec(`
		UPDATE discord_session_activity
		SET end_observed_at = MAX(?3, start_observed_at)
		WHERE end_observed_at IS NULL AND user_id = ?1 AND guild_id != ?2;
	`, int64(userID), int64(presenceGuildID), closeSessionsAt.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"closing all activity sessions for user %d: %w", userID, err)
	}

	if err := tx.Exec(`
		INSERT INTO discord_user (id, presence_guild_id, enabled)
		VALUES (?1, ?2, 1)
		ON CONFLICT(id)
		DO UPDATE SET presence_guild_id = ?2, enabled = 1;
	`, int64(userID), int64(presenceGuildID)); err != nil {
		return fmt.Errorf(errPrefix+"inserting or enabling user %d: %w", userID, err)
	}

	return nil
}

func DisableUser(tx *db.Tx, userID uint64, closeSessionsAt time.Time) error {
	if userID == 0 {
		return fmt.Errorf(errPrefix + "user id is required")
	}

	if err := db.AssertStorableAsInteger(userID); err != nil {
		return err
	}

	if err := tx.Exec(`
		UPDATE discord_session_status
		SET end_observed_at = MAX(?2, start_observed_at)
		WHERE end_observed_at IS NULL AND user_id = ?1;
	`, int64(userID), closeSessionsAt.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"closing all status sessions for user %d: %w", userID, err)
	}

	if err := tx.Exec(`
		UPDATE discord_session_activity
		SET end_observed_at = MAX(?2, start_observed_at)
		WHERE end_observed_at IS NULL AND user_id = ?1;
	`, int64(userID), closeSessionsAt.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"closing all activity sessions for user %d: %w", userID, err)
	}

	if err := tx.Exec(`
		UPDATE discord_session_voice
		SET end_observed_at = MAX(?2, start_observed_at)
		WHERE end_observed_at IS NULL AND user_id = ?1;
	`, int64(userID), closeSessionsAt.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"closing all voice sessions for user %d: %w", userID, err)
	}

	if err := tx.Exec(`
		UPDATE discord_user
		SET enabled = 0
		WHERE id = ?;
	`, int64(userID)); err != nil {
		return fmt.Errorf(errPrefix+"disabling user %d: %w", userID, err)
	}

	return nil
}

type GetUsersPageDataResult struct {
	Users []GetUsersPageDataResultUser `json:"users"`
}

type GetUsersPageDataResultUser struct {
	ID        uint64 `json:"id,string"`
	Name      string `json:"name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

func GetUsersPageData(tx *db.Tx) (*GetUsersPageDataResult, error) {
	res := &GetUsersPageDataResult{}

	var err error
	res.Users, err = func() ([]GetUsersPageDataResultUser, error) {
		rows, err := tx.Query(`
			SELECT id, name, avatar_url
			FROM discord_user
			ORDER BY name COLLATE NOCASE ASC, id ASC;
		`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var res []GetUsersPageDataResultUser
		for rows.Next() {
			var userID int64
			var name string
			var avatarURL string
			if err := rows.Scan(&userID, &name, &avatarURL); err != nil {
				return nil, err
			}

			res = append(res, GetUsersPageDataResultUser{
				ID:        uint64(userID),
				Name:      name,
				AvatarURL: avatarURL,
			})
		}

		return res, rows.Err()
	}()
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"getting all users: %w", err)
	}

	return res, nil
}

func CloseAllSessions(tx *db.Tx, ts time.Time) error {
	if err := tx.Exec(`
		UPDATE discord_session_status
		SET end_observed_at = MAX(?1, start_observed_at)
		WHERE end_observed_at IS NULL;
	`, ts.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"closing all status sessions: %w", err)
	}

	if err := tx.Exec(`
		UPDATE discord_session_activity
		SET end_observed_at = MAX(?1, start_observed_at)
		WHERE end_observed_at IS NULL;
	`, ts.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"closing all activity sessions: %w", err)
	}

	if err := tx.Exec(`
		UPDATE discord_session_voice
		SET end_observed_at = MAX(?1, start_observed_at)
		WHERE end_observed_at IS NULL;
	`, ts.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"closing all voice sessions: %w", err)
	}

	return nil
}

func CloseAllSessionsForGuild(tx *db.Tx, guildID uint64, ts time.Time) error {
	if err := tx.Exec(`
		UPDATE discord_session_status
		SET end_observed_at = MAX(?2, start_observed_at)
		WHERE end_observed_at IS NULL AND guild_id = ?1;
	`, int64(guildID), ts.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"closing all status sessions for guild %d: %w", guildID, err)
	}

	if err := tx.Exec(`
		UPDATE discord_session_activity
		SET end_observed_at = MAX(?2, start_observed_at)
		WHERE end_observed_at IS NULL AND guild_id = ?1;
	`, int64(guildID), ts.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"closing all activity sessions for guild %d: %w", guildID, err)
	}

	if err := tx.Exec(`
		UPDATE discord_session_voice
		SET end_observed_at = MAX(?2, start_observed_at)
		WHERE end_observed_at IS NULL AND channel_id IN (
			SELECT id
			FROM discord_channel
			WHERE guild_id = ?1
		);
	`, int64(guildID), ts.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"closing all voice sessions for guild %d: %w", guildID, err)
	}

	return nil
}

func CloseAllSessionsForGuildAndUser(tx *db.Tx, guildID uint64, userID uint64, ts time.Time) error {
	if err := tx.Exec(`
		UPDATE discord_session_status
		SET end_observed_at = MAX(?3, start_observed_at)
		WHERE end_observed_at IS NULL AND user_id = ?2 AND guild_id = ?1;
	`, int64(guildID), int64(userID), ts.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"closing all status sessions for guild %d and user %d: %w", guildID, userID, err)
	}

	if err := tx.Exec(`
		UPDATE discord_session_activity
		SET end_observed_at = MAX(?3, start_observed_at)
		WHERE end_observed_at IS NULL AND user_id = ?2 AND guild_id = ?1;
	`, int64(guildID), int64(userID), ts.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"closing all activity sessions for guild %d and user %d: %w", guildID, userID, err)
	}

	if err := tx.Exec(`
		UPDATE discord_session_voice
		SET end_observed_at = MAX(?3, start_observed_at)
		WHERE end_observed_at IS NULL AND user_id = ?2 AND channel_id IN (
			SELECT id
			FROM discord_channel
			WHERE guild_id = ?1
		);
	`, int64(guildID), int64(userID), ts.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"closing all voice sessions for guild %d and user %d: %w", guildID, userID, err)
	}

	return nil
}

type SyncPresenceParam struct {
	DesktopStatus  int
	MobileStatus   int
	WebStatus      int
	UserID         uint64
	UserExtraInfo  *SyncPresenceParamUserExtraInfo
	GuildID        uint64
	GuildExtraInfo *SyncPresenceParamGuildExtraInfo
	Activities     []SyncPresenceParamActivity
}

type SyncPresenceParamActivity struct {
	Name    string
	Details string
	State   string
}

type SyncPresenceParamUserExtraInfo struct {
	Name      string
	AvatarURL string
}

type SyncPresenceParamGuildExtraInfo struct {
	Name    string
	IconURL string
}

func SyncPresence(tx *db.Tx, ts time.Time, p *SyncPresenceParam) error {
	var dummy int
	if err := tx.QueryRow(`
			SELECT 1
			FROM discord_user
			WHERE id = ? AND enabled = 1 AND presence_guild_id = ?;
		`, int64(p.UserID), int64(p.GuildID)).Scan(&dummy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}

		return fmt.Errorf(errPrefix+"checking user enabled for presence in guild %d: %w", p.GuildID, err)
	}

	if p.UserExtraInfo != nil {
		if err := updateUserExtraInfo(tx, p.UserID, p.UserExtraInfo.Name, p.UserExtraInfo.AvatarURL, ts); err != nil {
			return err
		}
	}

	if p.GuildExtraInfo == nil {
		if err := ensureGuildInserted(tx, p.GuildID); err != nil {
			return err
		}
	} else {
		if err := upsertGuild(tx, p.GuildID, p.GuildExtraInfo.Name, p.GuildExtraInfo.IconURL, ts); err != nil {
			return err
		}
	}

	statusSession := &struct {
		id            int64
		desktopStatus int
		mobileStatus  int
		webStatus     int
	}{}
	if err := tx.QueryRow(`
			SELECT id, status_desktop, status_mobile, status_web
			FROM discord_session_status
			WHERE end_observed_at IS NULL AND user_id = ?;
		`, int64(p.UserID)).Scan(&statusSession.id, &statusSession.desktopStatus, &statusSession.mobileStatus, &statusSession.webStatus); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf(errPrefix+"getting open status session for user %d: %w", p.UserID, err)
		}

		statusSession = nil
	}

	if statusSession != nil && (statusSession.desktopStatus != p.DesktopStatus || statusSession.mobileStatus != p.MobileStatus || statusSession.webStatus != p.WebStatus) {
		if err := closeStatusSession(tx, statusSession.id, ts); err != nil {
			return err
		}
		statusSession = nil
	}

	if statusSession == nil {
		if err := openStatusSession(tx, p.GuildID, p.UserID, p.DesktopStatus, p.MobileStatus, p.WebStatus, ts); err != nil {
			return err
		}
	}

	type openActivitySessionRow struct {
		id      int64
		name    string
		details string
		state   string
	}
	activitySessions, err := func() ([]openActivitySessionRow, error) {
		rows, err := tx.Query(`
			SELECT dsa.id, dan.name, COALESCE(dad.details, ''), COALESCE(das.state, '')
			FROM discord_session_activity dsa
			LEFT JOIN discord_activity_name dan ON dan.id = dsa.name_id
			LEFT JOIN discord_activity_details dad ON dad.id = dsa.details_id
			LEFT JOIN discord_activity_state das ON das.id = dsa.state_id
			WHERE dsa.end_observed_at IS NULL AND dsa.user_id = ?;
		`, int64(p.UserID))
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var result []openActivitySessionRow
		for rows.Next() {
			var entry openActivitySessionRow
			if err := rows.Scan(&entry.id, &entry.name, &entry.details, &entry.state); err != nil {
				return nil, err
			}

			result = append(result, entry)
		}

		return result, rows.Err()
	}()
	if err != nil {
		return fmt.Errorf(errPrefix+"getting open activity sessions for user %d: %w", p.UserID, err)
	}

	for _, activitySession := range activitySessions {
		shouldEnd := true
		for _, activity := range p.Activities {
			if activity.Name == activitySession.name && activity.Details == activitySession.details && activity.State == activitySession.state {
				shouldEnd = false
				break
			}
		}

		if shouldEnd {
			if err := closeActivitySessionByID(tx, activitySession.id, ts); err != nil {
				return err
			}
		}
	}

	for _, activity := range p.Activities {
		shouldStart := true
		for _, activitySession := range activitySessions {
			if activity.Name == activitySession.name && activity.Details == activitySession.details && activity.State == activitySession.state {
				shouldStart = false
				break
			}
		}

		if shouldStart {
			if err := openActivitySession(tx, p.GuildID, p.UserID, activity.Name, activity.Details, activity.State, ts); err != nil {
				return err
			}
		}
	}

	return nil
}

type SyncVoiceParam struct {
	UserID           uint64
	UserExtraInfo    *SyncVoiceParamUserExtraInfo
	GuildID          uint64
	GuildExtraInfo   *SyncVoiceParamGuildExtraInfo
	ChannelID        uint64
	ChannelExtraInfo *SyncVoiceParamChannelExtraInfo
}

type SyncVoiceParamUserExtraInfo struct {
	Name      string
	AvatarURL string
}

type SyncVoiceParamGuildExtraInfo struct {
	Name    string
	IconURL string
}

type SyncVoiceParamChannelExtraInfo struct {
	Name string
}

func SyncVoice(tx *db.Tx, ts time.Time, p *SyncVoiceParam) error {
	var dummy int
	if err := tx.QueryRow(`
			SELECT 1
			FROM discord_user
			WHERE id = ? AND enabled = 1;
		`, int64(p.UserID)).Scan(&dummy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}

		return fmt.Errorf(errPrefix+"checking user %d enabled for voice: %w", p.UserID, err)
	}

	if p.UserExtraInfo != nil {
		if err := updateUserExtraInfo(tx, p.UserID, p.UserExtraInfo.Name, p.UserExtraInfo.AvatarURL, ts); err != nil {
			return err
		}
	}

	if p.GuildExtraInfo == nil {
		if err := ensureGuildInserted(tx, p.GuildID); err != nil {
			return err
		}
	} else {
		if err := upsertGuild(tx, p.GuildID, p.GuildExtraInfo.Name, p.GuildExtraInfo.IconURL, ts); err != nil {
			return err
		}
	}

	if p.ChannelID != 0 {
		if p.ChannelExtraInfo == nil {
			if err := ensureChannelInserted(tx, p.ChannelID, p.GuildID); err != nil {
				return err
			}
		} else {
			if err := upsertChannel(tx, p.ChannelID, p.GuildID, p.ChannelExtraInfo.Name, ts); err != nil {
				return err
			}
		}
	}

	var openVoiceSessionID int64
	var openVoiceSessionChannelIDInt64 int64
	if err := tx.QueryRow(`
			SELECT id, channel_id
			FROM discord_session_voice
			WHERE end_observed_at IS NULL AND user_id = ?2 AND channel_id IN (
				SELECT id
				FROM discord_channel
				WHERE guild_id = ?1
			);
		`, int64(p.GuildID), int64(p.UserID)).Scan(&openVoiceSessionID, &openVoiceSessionChannelIDInt64); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf(errPrefix+"getting open voice session channel id for guild %d and user %d: %w", p.GuildID, p.UserID, err)
	}
	openVoiceSessionChannelID := uint64(openVoiceSessionChannelIDInt64)

	if openVoiceSessionChannelID != 0 && p.ChannelID != openVoiceSessionChannelID {
		if err := tx.Exec(`
				UPDATE discord_session_voice
				SET end_observed_at = MAX(?2, start_observed_at)
				WHERE id = ?1;
			`, openVoiceSessionID, ts.UnixMilli()); err != nil {
			return fmt.Errorf(errPrefix+"closing voice session %d: %w", openVoiceSessionID, err)
		}
		openVoiceSessionChannelID = 0
	}

	if openVoiceSessionChannelID == 0 && p.ChannelID != 0 {
		if err := openVoiceSession(tx, p.ChannelID, p.UserID, ts); err != nil {
			return err
		}
	}

	return nil
}

type ResetGuildStateParam struct {
	ID                  uint64
	Name                string
	IconURL             string
	VoiceChannelsByID   map[uint64]ResetGuildStateVoiceChannel
	MembersByUserID     map[uint64]ResetGuildStateUser
	PresencesByUserID   map[uint64]ResetGuildStatePresence
	VoiceStatesByUserID map[uint64]ResetGuildStateVoiceState
}

type ResetGuildStateVoiceChannel struct {
	Name string
}

type ResetGuildStateUser struct {
	Name      string
	AvatarURL string
}

type ResetGuildStatePresence struct {
	DesktopStatus int
	MobileStatus  int
	WebStatus     int
	Activities    []ResetGuildStateActivity
}

type ResetGuildStateActivity struct {
	Name    string
	Details string
	State   string
}

type ResetGuildStateVoiceState struct {
	ChannelID uint64
}

func ResetGuildState(tx *db.Tx, ts time.Time, p *ResetGuildStateParam) error {
	if err := CloseAllSessionsForGuild(tx, p.ID, ts); err != nil {
		return err
	}

	type enabledUser struct {
		ID              uint64
		PresenceGuildID uint64
	}
	enabledUsers, err := func() ([]enabledUser, error) {
		rows, err := tx.Query(`
			SELECT id, presence_guild_id
			FROM discord_user
			WHERE enabled = 1;
		`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var result []enabledUser
		for rows.Next() {
			var userID int64
			var presenceGuildID int64
			if err := rows.Scan(&userID, &presenceGuildID); err != nil {
				return nil, err
			}

			result = append(result, enabledUser{
				ID:              uint64(userID),
				PresenceGuildID: uint64(presenceGuildID),
			})
		}

		return result, rows.Err()
	}()
	if err != nil {
		return fmt.Errorf(errPrefix+"getting all enabled users: %w", err)
	}

	guildUpserted := false
	upsertGuild := func() error {
		if guildUpserted {
			return nil
		}

		if err := upsertGuild(tx, p.ID, p.Name, p.IconURL, ts); err != nil {
			return err
		}

		guildUpserted = true
		return nil
	}
	upsertedChannelIDs := make(map[uint64]struct{})

	for _, enabledUser := range enabledUsers {
		if member, ok := p.MembersByUserID[enabledUser.ID]; ok {
			if err := updateUserExtraInfo(tx, enabledUser.ID, member.Name, member.AvatarURL, ts); err != nil {
				return err
			}
		}

		if enabledUser.PresenceGuildID == p.ID {
			if err := upsertGuild(); err != nil {
				return err
			}

			if presence, ok := p.PresencesByUserID[enabledUser.ID]; ok {
				if err := openStatusSession(tx, p.ID, enabledUser.ID, presence.DesktopStatus, presence.MobileStatus, presence.WebStatus, ts); err != nil {
					return err
				}

				for _, activity := range presence.Activities {
					if err := openActivitySession(tx, p.ID, enabledUser.ID, activity.Name, activity.Details, activity.State, ts); err != nil {
						return err
					}
				}
			}
		}

		if voiceState, ok := p.VoiceStatesByUserID[enabledUser.ID]; ok && voiceState.ChannelID != 0 {
			if err := upsertGuild(); err != nil {
				return err
			}

			if _, ok := upsertedChannelIDs[voiceState.ChannelID]; !ok {
				channel, hasChannel := p.VoiceChannelsByID[voiceState.ChannelID]
				if hasChannel {
					if err := upsertChannel(tx, voiceState.ChannelID, p.ID, channel.Name, ts); err != nil {
						return err
					}
				} else {
					if err := ensureChannelInserted(tx, voiceState.ChannelID, p.ID); err != nil {
						return err
					}
				}

				upsertedChannelIDs[voiceState.ChannelID] = struct{}{}
			}

			if err := openVoiceSession(tx, voiceState.ChannelID, enabledUser.ID, ts); err != nil {
				return err
			}
		}
	}

	return nil
}

type GetAllVoiceSessionsForUserResultEntry struct {
	ID              int64
	ChannelID       uint64
	ChannelName     string
	GuildID         uint64
	GuildName       string
	StartObservedAt int64
	EndObservedAt   int64
}

func GetAllVoiceSessionsForUser(tx *db.Tx, userID uint64) ([]GetAllVoiceSessionsForUserResultEntry, error) {
	res, err := func() ([]GetAllVoiceSessionsForUserResultEntry, error) {
		rows, err := tx.Query(`
			SELECT dsv.id, dc.id, dc.name, dg.id, dg.name, dsv.start_observed_at, dsv.end_observed_at
			FROM discord_session_voice dsv
			LEFT JOIN discord_channel dc ON dsv.channel_id = dc.id
			LEFT JOIN discord_guild dg ON dc.guild_id = dg.id
			WHERE dsv.user_id = ?;
		`, int64(userID))
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		entries := []GetAllVoiceSessionsForUserResultEntry{}
		for rows.Next() {
			var id int64
			var channelID int64
			var channelName string
			var guildID int64
			var guildName string
			var startObservedAt int64
			var endObservedAtPtr *int64
			if err := rows.Scan(&id, &channelID, &channelName, &guildID, &guildName, &startObservedAt, &endObservedAtPtr); err != nil {
				return nil, err
			}

			var endObservedAt int64
			if endObservedAtPtr != nil {
				endObservedAt = *endObservedAtPtr
			}

			entries = append(entries, GetAllVoiceSessionsForUserResultEntry{
				ID:              id,
				ChannelID:       uint64(channelID),
				ChannelName:     channelName,
				GuildID:         uint64(guildID),
				GuildName:       guildName,
				StartObservedAt: startObservedAt,
				EndObservedAt:   endObservedAt,
			})
		}

		return entries, rows.Err()
	}()
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"getting all voice sessions for user %d: %w", userID, err)
	}

	return res, nil
}

type GetUserPageDataResult struct {
	Heartbeat        time.Time                              `json:"heartbeat,omitzero"`
	User             GetUserPageDataResultUser              `json:"user"`
	StatusSessions   []GetUserPageDataResultStatusSession   `json:"status_sessions"`
	ActivitySessions []GetUserPageDataResultActivitySession `json:"activity_sessions"`
	VoiceSessions    []GetUserPageDataResultVoiceSession    `json:"voice_sessions"`
}

type GetUserPageDataResultUser struct {
	ID        uint64 `json:"id,string"`
	Name      string `json:"name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

type GetUserPageDataResultStatusSession struct {
	ID              int64     `json:"id,string"`
	DesktopStatus   int       `json:"desktop_status"`
	MobileStatus    int       `json:"mobile_status"`
	WebStatus       int       `json:"web_status"`
	StartObservedAt time.Time `json:"start_observed_at,omitzero"`
	EndObservedAt   time.Time `json:"end_observed_at,omitzero"`
}

type GetUserPageDataResultActivitySession struct {
	ID              int64     `json:"id,string"`
	Name            string    `json:"name"`
	Details         string    `json:"details,omitzero"`
	State           string    `json:"state,omitzero"`
	StartObservedAt time.Time `json:"start_observed_at,omitzero"`
	EndObservedAt   time.Time `json:"end_observed_at,omitzero"`
}

type GetUserPageDataResultVoiceSession struct {
	ID              int64     `json:"id,string"`
	ChannelID       uint64    `json:"channel_id,string"`
	ChannelName     string    `json:"channel_name,omitzero"`
	GuildID         uint64    `json:"guild_id,string"`
	GuildName       string    `json:"guild_name,omitzero"`
	StartObservedAt time.Time `json:"start_observed_at,omitzero"`
	EndObservedAt   time.Time `json:"end_observed_at,omitzero"`
}

func GetUserPageData(tx *db.Tx, userID uint64) (*GetUserPageDataResult, error) {
	res := &GetUserPageDataResult{
		User: GetUserPageDataResultUser{
			ID: userID,
		},
	}

	var err error
	res.Heartbeat, err = GetHeartbeat(tx)
	if err != nil {
		return nil, err
	}

	if err := tx.QueryRow(`
			SELECT name, avatar_url
			FROM discord_user
			WHERE id = ?;
		`, userID).Scan(&res.User.Name, &res.User.AvatarURL); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, db.ErrNotFound
		}

		return nil, fmt.Errorf(errPrefix+"getting user extra info %d: %w", userID, err)
	}

	res.StatusSessions, err = func() ([]GetUserPageDataResultStatusSession, error) {
		rows, err := tx.Query(`
			SELECT id, status_desktop, status_mobile, status_web, start_observed_at, end_observed_at
			FROM discord_session_status
			WHERE user_id = ?;
		`, int64(userID))
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		entries := []GetUserPageDataResultStatusSession{}
		for rows.Next() {
			var id int64
			var desktopStatus int
			var mobileStatus int
			var webStatus int
			var startObservedAtNum int64
			var endObservedAtNumPtr *int64
			if err := rows.Scan(&id, &desktopStatus, &mobileStatus, &webStatus, &startObservedAtNum, &endObservedAtNumPtr); err != nil {
				return nil, err
			}

			var endObservedAt time.Time
			if endObservedAtNumPtr != nil {
				endObservedAt = time.UnixMilli(*endObservedAtNumPtr)
			}

			entries = append(entries, GetUserPageDataResultStatusSession{
				ID:              id,
				DesktopStatus:   desktopStatus,
				MobileStatus:    mobileStatus,
				WebStatus:       webStatus,
				StartObservedAt: time.UnixMilli(startObservedAtNum),
				EndObservedAt:   endObservedAt,
			})
		}

		return entries, rows.Err()
	}()
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"getting all status sessions for user %d: %w", userID, err)
	}

	res.ActivitySessions, err = func() ([]GetUserPageDataResultActivitySession, error) {
		rows, err := tx.Query(`
			SELECT dsa.id, dan.name, COALESCE(dad.details, ''), COALESCE(das.state, ''), dsa.start_observed_at, dsa.end_observed_at
			FROM discord_session_activity dsa
			LEFT JOIN discord_activity_name dan ON dan.id = dsa.name_id
			LEFT JOIN discord_activity_details dad ON dad.id = dsa.details_id
			LEFT JOIN discord_activity_state das ON das.id = dsa.state_id
			WHERE dsa.user_id = ?;
		`, int64(userID))
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		entries := []GetUserPageDataResultActivitySession{}
		for rows.Next() {
			var id int64
			var name string
			var details string
			var state string
			var startObservedAtNum int64
			var endObservedAtNumPtr *int64
			if err := rows.Scan(&id, &name, &details, &state, &startObservedAtNum, &endObservedAtNumPtr); err != nil {
				return nil, err
			}

			var endObservedAt time.Time
			if endObservedAtNumPtr != nil {
				endObservedAt = time.UnixMilli(*endObservedAtNumPtr)
			}

			entries = append(entries, GetUserPageDataResultActivitySession{
				ID:              id,
				Name:            name,
				Details:         details,
				State:           state,
				StartObservedAt: time.UnixMilli(startObservedAtNum),
				EndObservedAt:   endObservedAt,
			})
		}

		return entries, rows.Err()
	}()
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"getting all activity sessions for user %d: %w", userID, err)
	}

	res.VoiceSessions, err = func() ([]GetUserPageDataResultVoiceSession, error) {
		rows, err := tx.Query(`
			SELECT dsv.id, dc.id, dc.name, dg.id, dg.name, dsv.start_observed_at, dsv.end_observed_at
			FROM discord_session_voice dsv
			LEFT JOIN discord_channel dc ON dsv.channel_id = dc.id
			LEFT JOIN discord_guild dg ON dc.guild_id = dg.id
			WHERE dsv.user_id = ?;
		`, int64(userID))
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		entries := []GetUserPageDataResultVoiceSession{}
		for rows.Next() {
			var id int64
			var channelID int64
			var channelName string
			var guildID int64
			var guildName string
			var startObservedAtNum int64
			var endObservedAtNumPtr *int64
			if err := rows.Scan(&id, &channelID, &channelName, &guildID, &guildName, &startObservedAtNum, &endObservedAtNumPtr); err != nil {
				return nil, err
			}

			var endObservedAt time.Time
			if endObservedAtNumPtr != nil {
				endObservedAt = time.UnixMilli(*endObservedAtNumPtr)
			}

			entries = append(entries, GetUserPageDataResultVoiceSession{
				ID:              id,
				ChannelID:       uint64(channelID),
				ChannelName:     channelName,
				GuildID:         uint64(guildID),
				GuildName:       guildName,
				StartObservedAt: time.UnixMilli(startObservedAtNum),
				EndObservedAt:   endObservedAt,
			})
		}

		return entries, rows.Err()
	}()
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"getting all voice sessions for user %d: %w", userID, err)
	}

	return res, nil
}

func ensureGuildInserted(tx *db.Tx, guildID uint64) error {
	if guildID == 0 {
		return fmt.Errorf(errPrefix + "guild id is required")
	}

	if err := tx.Exec(`
		INSERT INTO discord_guild (id)
		VALUES (?)
		ON CONFLICT(id)
		DO NOTHING;
	`, int64(guildID)); err != nil {
		return fmt.Errorf(errPrefix+"ensuring guild %d inserted: %w", guildID, err)
	}

	return nil
}

func upsertGuild(tx *db.Tx, guildID uint64, name string, iconURL string, updatedAt time.Time) error {
	if err := db.AssertStorableAsInteger(guildID); err != nil {
		return err
	}

	if err := tx.Exec(`
		INSERT INTO discord_guild (id, name, icon_url, extra_info_updated_at)
		VALUES (?1, ?2, ?3, ?4)
		ON CONFLICT(id)
		DO UPDATE SET name = ?2, icon_url = ?3, extra_info_updated_at = ?4;
	`, int64(guildID), name, iconURL, updatedAt.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"upserting guild %d: %w", guildID, err)
	}

	return nil
}

func updateUserExtraInfo(tx *db.Tx, userID uint64, name string, avatarURL string, ts time.Time) error {
	if err := tx.Exec(`
		UPDATE discord_user
		SET name = ?, avatar_url = ?, extra_info_updated_at = ?
		WHERE id = ?;
	`, name, avatarURL, ts.UnixMilli(), int64(userID)); err != nil {
		return fmt.Errorf(errPrefix+"updating user %d extra info: %w", userID, err)
	}

	return nil
}

func ensureChannelInserted(tx *db.Tx, channelID uint64, guildID uint64) error {
	if err := db.AssertStorableAsInteger(channelID); err != nil {
		return err
	}

	if err := tx.Exec(`
		INSERT INTO discord_channel (id, guild_id)
		VALUES (?, ?)
		ON CONFLICT(id)
		DO NOTHING;
	`, int64(channelID), int64(guildID)); err != nil {
		return fmt.Errorf(errPrefix+"ensuring channel %d inserted: %w", channelID, err)
	}

	return nil
}

func upsertChannel(tx *db.Tx, channelID uint64, guildID uint64, name string, updatedAt time.Time) error {
	if err := db.AssertStorableAsInteger(channelID); err != nil {
		return err
	}

	if err := tx.Exec(`
		INSERT INTO discord_channel (id, guild_id, name, extra_info_updated_at)
		VALUES (?1, ?2, ?3, ?4)
		ON CONFLICT(id)
		DO UPDATE SET guild_id = ?2, name = ?3, extra_info_updated_at = ?4;
	`, int64(channelID), int64(guildID), name, updatedAt.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"upserting channel %d: %w", channelID, err)
	}

	return nil
}

func openStatusSession(tx *db.Tx, guildID uint64, userID uint64, desktopStatus int, mobileStatus int, webStatus int, ts time.Time) error {
	if err := tx.Exec(`
		INSERT INTO discord_session_status (guild_id, user_id, status_desktop, status_mobile, status_web, start_observed_at)
		VALUES (?, ?, ?, ?, ?, ?);
	`, int64(guildID), int64(userID), desktopStatus, mobileStatus, webStatus, ts.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"opening status session for guild %d and user %d: %w", guildID, userID, err)
	}

	return nil
}

func closeStatusSession(tx *db.Tx, id int64, ts time.Time) error {
	if err := tx.Exec(`
		UPDATE discord_session_status
		SET end_observed_at = MAX(?2, start_observed_at)
		WHERE id = ?1 AND end_observed_at IS NULL;
	`, id, ts.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"closing status session %d: %w", id, err)
	}

	return nil
}

func openActivitySession(tx *db.Tx, guildID uint64, userID uint64, name string, details string, state string, ts time.Time) error {
	var nameID uint64
	if err := tx.QueryRow(`
			INSERT INTO discord_activity_name (name) VALUES (?1)
			ON CONFLICT DO NOTHING;

			SELECT id FROM discord_activity_name WHERE name = ?1;
		`, name).Scan(&nameID); err != nil {
		return fmt.Errorf(errPrefix+"upserting activity name %s: %w", name, err)
	}

	var detailsID *uint64
	if details != "" {
		if err := tx.QueryRow(`
			INSERT INTO discord_activity_details (details) VALUES (?1)
			ON CONFLICT DO NOTHING;

			SELECT id FROM discord_activity_details WHERE details = ?1;
		`, details).Scan(&detailsID); err != nil {
			return fmt.Errorf(errPrefix+"upserting activity details %s: %w", details, err)
		}
	}

	var stateID *uint64
	if state != "" {
		if err := tx.QueryRow(`
			INSERT INTO discord_activity_state (state) VALUES (?1)
			ON CONFLICT DO NOTHING;

			SELECT id FROM discord_activity_state WHERE state = ?1;
		`, state).Scan(&stateID); err != nil {
			return fmt.Errorf(errPrefix+"upserting activity state %s: %w", state, err)
		}
	}

	if err := tx.Exec(`
		INSERT INTO discord_session_activity (guild_id, user_id, name_id, details_id, state_id, start_observed_at)
		VALUES (?, ?, ?, ?, ?, ?);
	`, int64(guildID), int64(userID), nameID, detailsID, stateID, ts.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"opening activity session for guild %d and user %d: %w", guildID, userID, err)
	}

	return nil
}

func closeActivitySessionByID(tx *db.Tx, id int64, ts time.Time) error {
	if err := tx.Exec(`
		UPDATE discord_session_activity
		SET end_observed_at = MAX(?2, start_observed_at)
		WHERE id = ?1 AND end_observed_at IS NULL;
	`, id, ts.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"closing activity session %d: %w", id, err)
	}

	return nil
}

func openVoiceSession(tx *db.Tx, channelID uint64, userID uint64, ts time.Time) error {
	if err := tx.Exec(`
		INSERT INTO discord_session_voice (channel_id, user_id, start_observed_at)
		VALUES (?, ?, ?);
	`, int64(channelID), int64(userID), ts.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"opening voice session for channel %d and user %d: %w", channelID, userID, err)
	}

	return nil
}
