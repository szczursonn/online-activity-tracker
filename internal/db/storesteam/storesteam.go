package storesteam

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storecommon"
)

const errPrefix = "db/storesteam: "

const heartbeatKey = "steam"

func GetHeartbeat(tx *db.Tx) (time.Time, error) {
	return storecommon.GetHeartbeat(tx, heartbeatKey)
}

func UpsertHeartbeat(tx *db.Tx, ts time.Time) error {
	return storecommon.UpsertHeartbeat(tx, heartbeatKey, ts)
}

func InsertOrEnableUser(tx *db.Tx, userID uint64) error {
	if userID == 0 {
		return fmt.Errorf(errPrefix + "user id is required")
	}

	if err := db.AssertStorableAsInteger(userID); err != nil {
		return err
	}

	if err := tx.Exec(`
		INSERT INTO steam_user (id, enabled)
		VALUES (?, 1)
		ON CONFLICT(id)
		DO UPDATE SET enabled = 1;
	`, int64(userID)); err != nil {
		return fmt.Errorf(errPrefix+"inserting or enabling %d: %w", userID, err)
	}

	return nil
}

func DisableUser(tx *db.Tx, userID uint64, closeSessionsAt time.Time) error {
	if userID == 0 {
		return fmt.Errorf(errPrefix + "user id is required")
	}

	if err := tx.Exec(`
		UPDATE steam_session_persona_state
		SET last_observed_at = MAX(?2, first_observed_at)
		WHERE last_observed_at IS NULL and user_id = ?1;
	`, int64(userID), closeSessionsAt.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"closing persona state sessions for user %d: %w", userID, err)
	}

	if err := tx.Exec(`
		UPDATE steam_session_app
		SET last_observed_at = MAX(?2, first_observed_at)
		WHERE last_observed_at IS NULL and user_id = ?1;
	`, int64(userID), closeSessionsAt.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"closing app sessions for user %d: %w", userID, err)
	}

	if err := tx.Exec(`
		UPDATE steam_user
		SET enabled = 0
		WHERE id = ?;
	`, int64(userID)); err != nil {
		return fmt.Errorf(errPrefix+"disabling user %d: %w", userID, err)
	}

	return nil
}

func UpdateUserExtraInfo(tx *db.Tx, userID uint64, name string, profileURL string, avatarURL string, updatedAt time.Time) error {
	if err := tx.Exec(`
		UPDATE steam_user
		SET name = ?, profile_url = ?, avatar_url = ?, extra_info_updated_at = ?
		WHERE id = ?;
	`, name, profileURL, avatarURL, updatedAt.UnixMilli(), int64(userID)); err != nil {
		return fmt.Errorf(errPrefix+"updating user %d extra info: %w", userID, err)
	}

	return nil
}

func GetEnabledUserIDs(tx *db.Tx) ([]uint64, error) {
	userIDs, err := func() ([]uint64, error) {
		rows, err := tx.Query(`
			SELECT id
			FROM steam_user
			WHERE enabled = 1;
		`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var userIDs []uint64
		for rows.Next() {
			var userID int64
			if err := rows.Scan(&userID); err != nil {
				return nil, err
			}

			userIDs = append(userIDs, uint64(userID))
		}

		return userIDs, rows.Err()
	}()
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"getting enabled user ids: %w", err)
	}

	return userIDs, nil
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
			FROM steam_user
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

func UpdateAppExtraInfo(tx *db.Tx, appID uint64, name string, headerImageURL string, updatedAt time.Time) error {
	if err := tx.Exec(`
		UPDATE steam_app
		SET name = ?, header_image_url = ?, extra_info_updated_at = ?
		WHERE id = ?;
	`, name, headerImageURL, updatedAt.UnixMilli(), int64(appID)); err != nil {
		return fmt.Errorf(errPrefix+"updating app %d extra info: %w", appID, err)
	}

	return nil
}

func GetLeastUpdatedExtraInfoAppID(tx *db.Tx, maxUpdatedAt time.Time) (uint64, error) {
	var appID int64

	if err := tx.QueryRow(`
		SELECT id
		FROM steam_app
		WHERE extra_info_updated_at IS NULL OR extra_info_updated_at < ?
		ORDER BY extra_info_updated_at ASC
		LIMIT 1;
	`, maxUpdatedAt.UnixMilli()).Scan(&appID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, db.ErrNotFound
		}

		return 0, fmt.Errorf(errPrefix+"getting least updated extra info app id: %w", err)
	}

	return uint64(appID), nil
}

type GetAppPageDataResult struct {
	ID             uint64 `json:"id,string"`
	Name           string `json:"name"`
	HeaderImageURL string `json:"header_image_url"`
}

func GetAppPageData(tx *db.Tx, appID uint64) (*GetAppPageDataResult, error) {
	res := &GetAppPageDataResult{
		ID: appID,
	}

	if err := tx.QueryRow(`
		SELECT name, header_image_url
		FROM steam_app
		WHERE id = ?;
	`, int64(appID)).Scan(&res.Name, &res.HeaderImageURL); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, db.ErrNotFound
		}

		return nil, fmt.Errorf(errPrefix+"getting app %d: %w", appID, err)
	}

	return res, nil
}

func CloseAllSessions(tx *db.Tx, closeTs time.Time) error {
	if err := tx.Exec(`
		UPDATE steam_session_persona_state
		SET last_observed_at = MAX(?1, first_observed_at)
		WHERE last_observed_at IS NULL;
	`, closeTs.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"closing all persona state sessions: %w", err)
	}

	if err := tx.Exec(`
		UPDATE steam_session_app
		SET last_observed_at = MAX(?1, first_observed_at)
		WHERE last_observed_at IS NULL;
	`, closeTs.UnixMilli()); err != nil {
		return fmt.Errorf(errPrefix+"closing all app sessions: %w", err)
	}

	return nil
}

func SyncPersonaStateSessions(tx *db.Tx, closeTs time.Time, openTs time.Time, newPersonaStateByUserID map[uint64]*int) error {
	if err := func() error {
		type openPersonaStateSession struct {
			id           int64
			personaState int
		}
		openSessionByUserID, err := func() (map[uint64]openPersonaStateSession, error) {
			rows, err := tx.Query(`
				SELECT id, user_id, persona_state
				FROM steam_session_persona_state
				WHERE last_observed_at IS NULL;
			`)
			if err != nil {
				return nil, err
			}
			defer rows.Close()

			m := make(map[uint64]openPersonaStateSession)
			for rows.Next() {
				var id int64
				var userID int64
				var personaState int
				if err := rows.Scan(&id, &userID, &personaState); err != nil {
					return nil, err
				}

				m[uint64(userID)] = openPersonaStateSession{
					id:           id,
					personaState: personaState,
				}
			}

			return m, rows.Err()
		}()
		if err != nil {
			return fmt.Errorf("getting all open persona state sessions: %w", err)
		}

		for userID, openSession := range openSessionByUserID {
			newPersonaState, isUserPartOfChangeSet := newPersonaStateByUserID[userID]
			if !isUserPartOfChangeSet {
				continue
			}

			if newPersonaState == nil || *newPersonaState != openSession.personaState {
				if err := tx.Exec(`
					UPDATE steam_session_persona_state
					SET last_observed_at = MAX(?2, first_observed_at)
					WHERE id = ?1;
				`, openSession.id, closeTs.UnixMilli()); err != nil {
					return fmt.Errorf("closing persona state session %d for user %d: %w", openSession.id, userID, err)
				}
			}
		}

		for userID, newPersonaState := range newPersonaStateByUserID {
			if newPersonaState == nil {
				continue
			}

			openSession, hasOpenSession := openSessionByUserID[userID]
			if !hasOpenSession || *newPersonaState != openSession.personaState {
				if err := tx.Exec(`
					INSERT INTO steam_session_persona_state (user_id, persona_state, first_observed_at)
					VALUES (?, ?, ?);
				`, int64(userID), *newPersonaState, openTs.UnixMilli()); err != nil {
					return fmt.Errorf("opening persona state session for user %d: %w", userID, err)
				}
			}
		}

		return nil
	}(); err != nil {
		return fmt.Errorf(errPrefix+"syncing persona state sessions: %w", err)
	}

	return nil
}

type GetUserPageDataResult struct {
	Heartbeat            time.Time                                  `json:"heartbeat,omitzero"`
	User                 GetUserPageDataResultUser                  `json:"user"`
	PersonaStateSessions []GetUserPageDataResultPersonaStateSession `json:"persona_state_sessions"`
	AppSessions          []GetUserPageDataResultAppSession          `json:"app_sessions"`
}

type GetUserPageDataResultUser struct {
	ID         uint64 `json:"id,string"`
	Name       string `json:"name,omitempty"`
	ProfileURL string `json:"profile_url,omitempty"`
	AvatarURL  string `json:"avatar_url,omitempty"`
}

type GetUserPageDataResultPersonaStateSession struct {
	ID              int64     `json:"id,string"`
	PersonaState    int       `json:"persona_state"`
	FirstObservedAt time.Time `json:"first_observed_at,omitzero"`
	LastObservedAt  time.Time `json:"last_observed_at,omitzero"`
}

type GetUserPageDataResultAppSession struct {
	ID              int64     `json:"id,string"`
	AppID           uint64    `json:"app_id,string"`
	AppName         string    `json:"app_name,omitempty"`
	FirstObservedAt time.Time `json:"first_observed_at,omitzero"`
	LastObservedAt  time.Time `json:"last_observed_at,omitzero"`
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
			SELECT name, profile_url, avatar_url
			FROM steam_user
			WHERE id = ?;
		`, userID).Scan(&res.User.Name, &res.User.ProfileURL, &res.User.AvatarURL); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, db.ErrNotFound
		}

		return nil, fmt.Errorf(errPrefix+"getting user extra info %d: %w", userID, err)
	}

	res.PersonaStateSessions, err = func() ([]GetUserPageDataResultPersonaStateSession, error) {
		rows, err := tx.Query(`
			SELECT id, persona_state, first_observed_at, last_observed_at
			FROM steam_session_persona_state
			WHERE user_id = ?;
		`, int64(userID))
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		entries := []GetUserPageDataResultPersonaStateSession{}
		for rows.Next() {
			var id int64
			var personaState int
			var firstObservedAtNum int64
			var lastObservedAtNumPtr *int64
			if err := rows.Scan(&id, &personaState, &firstObservedAtNum, &lastObservedAtNumPtr); err != nil {
				return nil, err
			}

			var lastObservedAt time.Time
			if lastObservedAtNumPtr != nil {
				lastObservedAt = time.UnixMilli(*lastObservedAtNumPtr)
			}

			entries = append(entries, GetUserPageDataResultPersonaStateSession{
				ID:              id,
				PersonaState:    personaState,
				FirstObservedAt: time.UnixMilli(firstObservedAtNum),
				LastObservedAt:  lastObservedAt,
			})
		}

		return entries, rows.Err()
	}()
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"getting all persona state sessions for user %d: %w", userID, err)
	}

	res.AppSessions, err = func() ([]GetUserPageDataResultAppSession, error) {
		rows, err := tx.Query(`
			SELECT ssa.id, sa.id, sa.name, ssa.first_observed_at, ssa.last_observed_at
			FROM steam_session_app ssa
			LEFT JOIN steam_app sa ON ssa.app_id = sa.id
			WHERE user_id = ?;
		`, int64(userID))
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		entries := []GetUserPageDataResultAppSession{}
		for rows.Next() {
			var id int64
			var appID int64
			var appName string
			var firstObservedAtNum int64
			var lastObservedAtNumPtr *int64
			if err := rows.Scan(&id, &appID, &appName, &firstObservedAtNum, &lastObservedAtNumPtr); err != nil {
				return nil, err
			}

			var lastObservedAt time.Time
			if lastObservedAtNumPtr != nil {
				lastObservedAt = time.UnixMilli(*lastObservedAtNumPtr)
			}

			entries = append(entries, GetUserPageDataResultAppSession{
				ID:              id,
				AppID:           uint64(appID),
				AppName:         appName,
				FirstObservedAt: time.UnixMilli(firstObservedAtNum),
				LastObservedAt:  lastObservedAt,
			})
		}

		return entries, rows.Err()
	}()
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"getting all app sessions with app info for user %d: %w", userID, err)
	}

	return res, nil
}

func SyncAppSessions(tx *db.Tx, closeTs time.Time, openTs time.Time, newAppIDByUserID map[uint64]*uint64) error {
	if err := func() error {
		type openAppSession struct {
			id    int64
			appID uint64
		}
		openSessionByUserID, err := func() (map[uint64]openAppSession, error) {
			rows, err := tx.Query(`
				SELECT id, user_id, app_id
				FROM steam_session_app
				WHERE last_observed_at IS NULL;
			`)
			if err != nil {
				return nil, err
			}
			defer rows.Close()

			m := map[uint64]openAppSession{}
			for rows.Next() {
				var id int64
				var userID int64
				var appID int64
				if err := rows.Scan(&id, &userID, &appID); err != nil {
					return nil, err
				}

				m[uint64(userID)] = openAppSession{
					id:    id,
					appID: uint64(appID),
				}
			}

			return m, rows.Err()
		}()
		if err != nil {
			return fmt.Errorf("getting all open app sessions: %w", err)
		}

		for userID, openSession := range openSessionByUserID {
			newAppID, isUserPartOfChangeSet := newAppIDByUserID[userID]
			if !isUserPartOfChangeSet {
				continue
			}

			if newAppID == nil || *newAppID != openSession.appID {
				if err := tx.Exec(`
					UPDATE steam_session_app
					SET last_observed_at = MAX(?2, first_observed_at)
					WHERE id = ?1;
				`, openSession.id, closeTs.UnixMilli()); err != nil {
					return fmt.Errorf("closing app session %d for user %d: %w", openSession.id, userID, err)
				}
			}
		}

		appEnsuredInsertedByID := make(map[uint64]struct{})

		for userID, newAppID := range newAppIDByUserID {
			if newAppID == nil || *newAppID == 0 {
				continue
			}

			openAppSession, hasOpenAppSession := openSessionByUserID[userID]
			if !hasOpenAppSession || openAppSession.appID != *newAppID {
				if _, ok := appEnsuredInsertedByID[*newAppID]; !ok {
					if err := db.AssertStorableAsInteger(*newAppID); err != nil {
						return err
					}

					if err := tx.Exec(`
						INSERT INTO steam_app (id)
						VALUES (?)
						ON CONFLICT(id)
						DO NOTHING;
					`, int64(*newAppID)); err != nil {
						return fmt.Errorf("ensuring app %d inserted: %w", *newAppID, err)
					}
					appEnsuredInsertedByID[*newAppID] = struct{}{}
				}

				if err := tx.Exec(`
						INSERT INTO steam_session_app (user_id, app_id, first_observed_at)
						VALUES (?, ?, ?);
					`, int64(userID), int64(*newAppID), openTs.UnixMilli()); err != nil {
					return fmt.Errorf("opening app session for user %d and user %d: %w", userID, *newAppID, err)
				}
			}
		}

		return nil
	}(); err != nil {
		return fmt.Errorf(errPrefix+"syncing app sessions: %w", err)
	}

	return nil
}
