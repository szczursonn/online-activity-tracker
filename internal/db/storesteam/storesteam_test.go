package storesteam_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storesteam"
)

/*
⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️
(PARTIALLY) AI-GENERATED
⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️⚠️
*/

// ---------- harness ----------

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := database.RunMigrations(t.Context()); err != nil {
		t.Fatalf("db.RunMigrations: %v", err)
	}
	return database
}

func writeTx(t *testing.T, database *db.DB, fn func(tx *db.Tx) error) {
	t.Helper()
	if err := database.WriteTx(context.Background(), fn); err != nil {
		t.Fatalf("WriteTx: %v", err)
	}
}

func readTx(t *testing.T, database *db.DB, fn func(tx *db.Tx) error) {
	t.Helper()
	if err := database.ReadTx(context.Background(), fn); err != nil {
		t.Fatalf("ReadTx: %v", err)
	}
}

func enableUser(t *testing.T, database *db.DB, userID uint64) {
	t.Helper()
	writeTx(t, database, func(tx *db.Tx) error {
		return storesteam.InsertOrEnableUser(tx, userID)
	})
}

func disableUser(t *testing.T, database *db.DB, userID uint64, ts time.Time) {
	t.Helper()
	writeTx(t, database, func(tx *db.Tx) error {
		return storesteam.DisableUser(tx, userID, ts)
	})
}

func intPtr(v int) *int             { return &v }
func uint64Ptr(v uint64) *uint64    { return &v }

// ---------- row inspection ----------

type personaRow struct {
	id           int64
	userID       uint64
	personaState int
	start        int64
	end          sql.NullInt64
}

func listPersonaRows(t *testing.T, database *db.DB, userID uint64) []personaRow {
	t.Helper()
	var rows []personaRow
	readTx(t, database, func(tx *db.Tx) error {
		r, err := tx.Query(`
			SELECT id, user_id, persona_state, first_observed_at, last_observed_at
			FROM steam_session_persona_state WHERE user_id = ? ORDER BY id;
		`, int64(userID))
		if err != nil {
			return err
		}
		defer r.Close()
		for r.Next() {
			var p personaRow
			var uid int64
			if err := r.Scan(&p.id, &uid, &p.personaState, &p.start, &p.end); err != nil {
				return err
			}
			p.userID = uint64(uid)
			rows = append(rows, p)
		}
		return r.Err()
	})
	return rows
}

type appSessionRow struct {
	id     int64
	userID uint64
	appID  uint64
	start  int64
	end    sql.NullInt64
}

func listAppRows(t *testing.T, database *db.DB, userID uint64) []appSessionRow {
	t.Helper()
	var rows []appSessionRow
	readTx(t, database, func(tx *db.Tx) error {
		r, err := tx.Query(`
			SELECT id, user_id, app_id, first_observed_at, last_observed_at
			FROM steam_session_app WHERE user_id = ? ORDER BY id;
		`, int64(userID))
		if err != nil {
			return err
		}
		defer r.Close()
		for r.Next() {
			var a appSessionRow
			var uid, aid int64
			if err := r.Scan(&a.id, &uid, &aid, &a.start, &a.end); err != nil {
				return err
			}
			a.userID, a.appID = uint64(uid), uint64(aid)
			rows = append(rows, a)
		}
		return r.Err()
	})
	return rows
}

func countOpen[T personaRow | appSessionRow](rows []T) int {
	open := 0
	for _, r := range rows {
		switch v := any(r).(type) {
		case personaRow:
			if !v.end.Valid {
				open++
			}
		case appSessionRow:
			if !v.end.Valid {
				open++
			}
		}
	}
	return open
}

func userEnabled(t *testing.T, database *db.DB, userID uint64) (int, bool) {
	t.Helper()
	var enabled int
	found := false
	readTx(t, database, func(tx *db.Tx) error {
		err := tx.QueryRow(`SELECT enabled FROM steam_user WHERE id = ?;`, int64(userID)).Scan(&enabled)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		return nil
	})
	return enabled, found
}

func countSteamAppRows(t *testing.T, database *db.DB, appID uint64) int {
	t.Helper()
	var n int
	readTx(t, database, func(tx *db.Tx) error {
		return tx.QueryRow(`SELECT COUNT(*) FROM steam_app WHERE id = ?;`, int64(appID)).Scan(&n)
	})
	return n
}

func steamAppName(t *testing.T, database *db.DB, appID uint64) string {
	t.Helper()
	var name string
	readTx(t, database, func(tx *db.Tx) error {
		return tx.QueryRow(`SELECT name FROM steam_app WHERE id = ?;`, int64(appID)).Scan(&name)
	})
	return name
}

// ---------- InsertOrEnableUser ----------

func TestInsertOrEnableUser_NewUser(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	enabled, ok := userEnabled(t, d, 1)
	if !ok || enabled != 1 {
		t.Fatalf("expected user 1 enabled=1, got enabled=%d ok=%v", enabled, ok)
	}
}

func TestInsertOrEnableUser_ReEnableKeepsExtraInfo(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.UpdateUserExtraInfo(tx, 1, "alice", "https://p", "https://a", time.UnixMilli(100))
	})
	disableUser(t, d, 1, time.UnixMilli(200))

	if enabled, _ := userEnabled(t, d, 1); enabled != 0 {
		t.Fatalf("expected disabled, got enabled=%d", enabled)
	}

	enableUser(t, d, 1)

	if enabled, _ := userEnabled(t, d, 1); enabled != 1 {
		t.Fatalf("expected re-enabled, got enabled=%d", enabled)
	}

	var page *storesteam.GetUserPageDataResult
	readTx(t, d, func(tx *db.Tx) error {
		var err error
		page, err = storesteam.GetUserPageData(tx, 1)
		return err
	})
	if page.User.Name != "alice" || page.User.ProfileURL != "https://p" || page.User.AvatarURL != "https://a" {
		t.Errorf("re-enable should preserve extra info, got %+v", page.User)
	}
}

func TestInsertOrEnableUser_ReEnableDoesNotCloseOpenSessions(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(1000), time.UnixMilli(1000),
			map[uint64]*int{1: intPtr(1)})
	})

	before := listPersonaRows(t, d, 1)
	if len(before) != 1 || before[0].end.Valid {
		t.Fatalf("setup: expected 1 open persona row, got %+v", before)
	}

	// Calling InsertOrEnableUser on an already-enabled user must be a no-op
	// against existing sessions (unlike storediscord which closes wrong-guild rows).
	enableUser(t, d, 1)

	after := listPersonaRows(t, d, 1)
	if len(after) != 1 || after[0].id != before[0].id || after[0].end.Valid {
		t.Errorf("InsertOrEnableUser must not touch existing sessions: before=%+v after=%+v", before, after)
	}
}

func TestInsertOrEnableUser_ZeroIDReturnsError(t *testing.T) {
	d := newTestDB(t)
	err := d.WriteTx(context.Background(), func(tx *db.Tx) error {
		return storesteam.InsertOrEnableUser(tx, 0)
	})
	if err == nil {
		t.Fatal("expected error for userID=0, got nil")
	}
}

// ---------- DisableUser ----------

func TestDisableUser_ClosesOpenSessionsAndDisables(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		if err := storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(100), time.UnixMilli(100),
			map[uint64]*int{1: intPtr(1)}); err != nil {
			return err
		}
		return storesteam.SyncAppSessions(tx, time.UnixMilli(100), time.UnixMilli(100),
			map[uint64]*uint64{1: uint64Ptr(500)})
	})

	disableUser(t, d, 1, time.UnixMilli(2000))

	if enabled, _ := userEnabled(t, d, 1); enabled != 0 {
		t.Errorf("expected enabled=0, got %d", enabled)
	}
	for _, r := range listPersonaRows(t, d, 1) {
		if !r.end.Valid || r.end.Int64 != 2000 {
			t.Errorf("persona row should be closed at 2000: %+v", r)
		}
	}
	for _, r := range listAppRows(t, d, 1) {
		if !r.end.Valid || r.end.Int64 != 2000 {
			t.Errorf("app row should be closed at 2000: %+v", r)
		}
	}
}

func TestDisableUser_MaxClampOnLateTimestamp(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		if err := storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(1000), time.UnixMilli(1000),
			map[uint64]*int{1: intPtr(1)}); err != nil {
			return err
		}
		return storesteam.SyncAppSessions(tx, time.UnixMilli(1000), time.UnixMilli(1000),
			map[uint64]*uint64{1: uint64Ptr(500)})
	})

	// closeSessionsAt earlier than first_observed_at — must clamp to start.
	disableUser(t, d, 1, time.UnixMilli(500))

	for _, r := range listPersonaRows(t, d, 1) {
		if !r.end.Valid || r.end.Int64 != r.start {
			t.Errorf("persona row end should be clamped to start: %+v", r)
		}
	}
	for _, r := range listAppRows(t, d, 1) {
		if !r.end.Valid || r.end.Int64 != r.start {
			t.Errorf("app row end should be clamped to start: %+v", r)
		}
	}
}

func TestDisableUser_OtherUsersUntouched(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)
	enableUser(t, d, 2)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(1000), time.UnixMilli(1000),
			map[uint64]*int{1: intPtr(1), 2: intPtr(1)})
	})

	disableUser(t, d, 1, time.UnixMilli(2000))

	u1 := listPersonaRows(t, d, 1)
	if len(u1) != 1 || !u1[0].end.Valid {
		t.Errorf("u1 persona row should be closed: %+v", u1)
	}
	u2 := listPersonaRows(t, d, 2)
	if len(u2) != 1 || u2[0].end.Valid {
		t.Errorf("u2 persona row should remain open: %+v", u2)
	}
	if enabled, _ := userEnabled(t, d, 2); enabled != 1 {
		t.Errorf("u2 should remain enabled, got %d", enabled)
	}
}

func TestDisableUser_ZeroIDReturnsError(t *testing.T) {
	d := newTestDB(t)
	err := d.WriteTx(context.Background(), func(tx *db.Tx) error {
		return storesteam.DisableUser(tx, 0, time.UnixMilli(0))
	})
	if err == nil {
		t.Fatal("expected error for userID=0, got nil")
	}
}

// ---------- extra info / page-data getters ----------

func TestUpdateUserExtraInfo_VisibleInGetUsersPageData(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)
	enableUser(t, d, 2)
	enableUser(t, d, 3)
	enableUser(t, d, 4)

	writeTx(t, d, func(tx *db.Tx) error {
		if err := storesteam.UpdateUserExtraInfo(tx, 1, "Bob", "", "av1", time.UnixMilli(10)); err != nil {
			return err
		}
		if err := storesteam.UpdateUserExtraInfo(tx, 2, "alice", "", "av2", time.UnixMilli(10)); err != nil {
			return err
		}
		if err := storesteam.UpdateUserExtraInfo(tx, 3, "Charlie", "", "av3", time.UnixMilli(10)); err != nil {
			return err
		}
		// User 4 left with the default empty name.
		return nil
	})

	var res *storesteam.GetUsersPageDataResult
	readTx(t, d, func(tx *db.Tx) error {
		var err error
		res, err = storesteam.GetUsersPageData(tx)
		return err
	})
	if len(res.Users) != 4 {
		t.Fatalf("expected 4 users, got %+v", res.Users)
	}
	// Order: empty name first (id 4), then alice (2), Bob (1), Charlie (3) — case-insensitive.
	wantOrder := []uint64{4, 2, 1, 3}
	for i, u := range res.Users {
		if u.ID != wantOrder[i] {
			t.Errorf("user %d: want id=%d, got %+v", i, wantOrder[i], u)
		}
	}
	if res.Users[1].Name != "alice" || res.Users[1].AvatarURL != "av2" {
		t.Errorf("alice fields wrong: %+v", res.Users[1])
	}
}

func TestGetEnabledUserIDs_ExcludesDisabled(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)
	enableUser(t, d, 2)
	enableUser(t, d, 3)
	disableUser(t, d, 2, time.UnixMilli(0))

	var ids []uint64
	readTx(t, d, func(tx *db.Tx) error {
		var err error
		ids, err = storesteam.GetEnabledUserIDs(tx)
		return err
	})
	got := map[uint64]bool{}
	for _, id := range ids {
		got[id] = true
	}
	if got[2] {
		t.Errorf("disabled user 2 should not appear in enabled list: %+v", ids)
	}
	if !got[1] || !got[3] {
		t.Errorf("expected users 1 and 3 in enabled list: %+v", ids)
	}
}

func TestUpdateAppExtraInfo_VisibleInGetAppPageData(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)
	// Get an app row inserted via a sync.
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(0), time.UnixMilli(0),
			map[uint64]*uint64{1: uint64Ptr(500)})
	})

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.UpdateAppExtraInfo(tx, 500, "Counter-Strike", "https://img", time.UnixMilli(100))
	})

	var page *storesteam.GetAppPageDataResult
	readTx(t, d, func(tx *db.Tx) error {
		var err error
		page, err = storesteam.GetAppPageData(tx, 500)
		return err
	})
	if page.ID != 500 || page.Name != "Counter-Strike" || page.HeaderImageURL != "https://img" {
		t.Errorf("unexpected page: %+v", page)
	}
}

func TestGetAppPageData_NotFoundReturnsErrNotFound(t *testing.T) {
	d := newTestDB(t)
	err := d.ReadTx(context.Background(), func(tx *db.Tx) error {
		_, err := storesteam.GetAppPageData(tx, 999)
		if !errors.Is(err, db.ErrNotFound) {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected db.ErrNotFound, got %v", err)
	}
}

func TestGetLeastUpdatedExtraInfoAppID_PrefersNullThenOldest_NotFoundWhenAllFresh(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	// Insert three app rows: 100, 200, 300.
	writeTx(t, d, func(tx *db.Tx) error {
		if err := storesteam.SyncAppSessions(tx, time.UnixMilli(0), time.UnixMilli(0),
			map[uint64]*uint64{1: uint64Ptr(100)}); err != nil {
			return err
		}
		if err := storesteam.SyncAppSessions(tx, time.UnixMilli(1), time.UnixMilli(1),
			map[uint64]*uint64{1: uint64Ptr(200)}); err != nil {
			return err
		}
		return storesteam.SyncAppSessions(tx, time.UnixMilli(2), time.UnixMilli(2),
			map[uint64]*uint64{1: uint64Ptr(300)})
	})

	// Set extra_info_updated_at for 100 and 200, leave 300 NULL.
	writeTx(t, d, func(tx *db.Tx) error {
		if err := storesteam.UpdateAppExtraInfo(tx, 100, "A", "", time.UnixMilli(100)); err != nil {
			return err
		}
		return storesteam.UpdateAppExtraInfo(tx, 200, "B", "", time.UnixMilli(200))
	})

	// NULL beats non-null in ASC.
	var id uint64
	readTx(t, d, func(tx *db.Tx) error {
		var err error
		id, err = storesteam.GetLeastUpdatedExtraInfoAppID(tx, time.UnixMilli(1000))
		return err
	})
	if id != 300 {
		t.Errorf("expected 300 (NULL extra info), got %d", id)
	}

	// Now fill 300 with the largest timestamp; the smallest remaining is 100.
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.UpdateAppExtraInfo(tx, 300, "C", "", time.UnixMilli(300))
	})
	readTx(t, d, func(tx *db.Tx) error {
		var err error
		id, err = storesteam.GetLeastUpdatedExtraInfoAppID(tx, time.UnixMilli(1000))
		return err
	})
	if id != 100 {
		t.Errorf("expected 100 (oldest non-null), got %d", id)
	}

	// All apps are fresher than maxUpdatedAt=50 → ErrNotFound.
	err := d.ReadTx(context.Background(), func(tx *db.Tx) error {
		_, err := storesteam.GetLeastUpdatedExtraInfoAppID(tx, time.UnixMilli(50))
		if !errors.Is(err, db.ErrNotFound) {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected db.ErrNotFound, got %v", err)
	}
}

func TestGetUserPageData_NotFoundReturnsErrNotFound(t *testing.T) {
	d := newTestDB(t)
	err := d.ReadTx(context.Background(), func(tx *db.Tx) error {
		_, err := storesteam.GetUserPageData(tx, 999)
		if !errors.Is(err, db.ErrNotFound) {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected db.ErrNotFound, got %v", err)
	}
}

func TestGetUserPageData_ReturnsHeartbeatAndSessions(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		if err := storesteam.UpsertHeartbeat(tx, time.UnixMilli(5000)); err != nil {
			return err
		}
		if err := storesteam.UpdateUserExtraInfo(tx, 1, "alice", "https://p", "https://a", time.UnixMilli(100)); err != nil {
			return err
		}
		// One persona session: opened at 100, closed by switching to a different state at 200.
		if err := storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(0), time.UnixMilli(100),
			map[uint64]*int{1: intPtr(1)}); err != nil {
			return err
		}
		if err := storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(200), time.UnixMilli(200),
			map[uint64]*int{1: intPtr(2)}); err != nil {
			return err
		}
		// One app session, then app extra info to populate AppName via LEFT JOIN.
		if err := storesteam.SyncAppSessions(tx, time.UnixMilli(0), time.UnixMilli(100),
			map[uint64]*uint64{1: uint64Ptr(500)}); err != nil {
			return err
		}
		return storesteam.UpdateAppExtraInfo(tx, 500, "Counter-Strike", "", time.UnixMilli(100))
	})

	var page *storesteam.GetUserPageDataResult
	readTx(t, d, func(tx *db.Tx) error {
		var err error
		page, err = storesteam.GetUserPageData(tx, 1)
		return err
	})

	if !page.Heartbeat.Equal(time.UnixMilli(5000)) {
		t.Errorf("heartbeat: want 5000, got %v", page.Heartbeat)
	}
	if page.User.ID != 1 || page.User.Name != "alice" || page.User.ProfileURL != "https://p" || page.User.AvatarURL != "https://a" {
		t.Errorf("user: %+v", page.User)
	}
	if len(page.PersonaStateSessions) != 2 {
		t.Errorf("expected 2 persona sessions, got %+v", page.PersonaStateSessions)
	}
	openPersona, closedPersona := 0, 0
	for _, s := range page.PersonaStateSessions {
		if s.LastObservedAt.IsZero() {
			openPersona++
		} else {
			closedPersona++
		}
	}
	if openPersona != 1 || closedPersona != 1 {
		t.Errorf("expected 1 open + 1 closed persona, got open=%d closed=%d", openPersona, closedPersona)
	}
	if len(page.AppSessions) != 1 || page.AppSessions[0].AppID != 500 || page.AppSessions[0].AppName != "Counter-Strike" {
		t.Errorf("app sessions wrong: %+v", page.AppSessions)
	}
	if !page.AppSessions[0].LastObservedAt.IsZero() {
		t.Errorf("app session should be open, got %+v", page.AppSessions[0])
	}
}

// ---------- heartbeat ----------

func TestHeartbeat_UpsertThenGetRoundtrip(t *testing.T) {
	d := newTestDB(t)
	writeTx(t, d, func(tx *db.Tx) error { return storesteam.UpsertHeartbeat(tx, time.UnixMilli(1234)) })

	var got time.Time
	readTx(t, d, func(tx *db.Tx) error {
		var err error
		got, err = storesteam.GetHeartbeat(tx)
		return err
	})
	if !got.Equal(time.UnixMilli(1234)) {
		t.Errorf("want 1234, got %v", got)
	}
}

func TestHeartbeat_NeverMovesBackwards(t *testing.T) {
	d := newTestDB(t)
	writeTx(t, d, func(tx *db.Tx) error { return storesteam.UpsertHeartbeat(tx, time.UnixMilli(2000)) })
	writeTx(t, d, func(tx *db.Tx) error { return storesteam.UpsertHeartbeat(tx, time.UnixMilli(1000)) })

	var got time.Time
	readTx(t, d, func(tx *db.Tx) error {
		var err error
		got, err = storesteam.GetHeartbeat(tx)
		return err
	})
	if !got.Equal(time.UnixMilli(2000)) {
		t.Errorf("heartbeat moved backwards: want 2000, got %v", got)
	}
}

func TestHeartbeat_UnsetReturnsEpoch(t *testing.T) {
	d := newTestDB(t)
	var got time.Time
	readTx(t, d, func(tx *db.Tx) error {
		var err error
		got, err = storesteam.GetHeartbeat(tx)
		return err
	})
	if !got.Equal(time.UnixMilli(0)) {
		t.Errorf("expected Unix epoch from unset heartbeat, got %v", got)
	}
}

// ---------- CloseAllSessions ----------

func TestCloseAllSessions_ClosesPersonaAndApp(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)
	enableUser(t, d, 2)

	writeTx(t, d, func(tx *db.Tx) error {
		if err := storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(100), time.UnixMilli(100),
			map[uint64]*int{1: intPtr(1), 2: intPtr(1)}); err != nil {
			return err
		}
		return storesteam.SyncAppSessions(tx, time.UnixMilli(100), time.UnixMilli(100),
			map[uint64]*uint64{1: uint64Ptr(500), 2: uint64Ptr(600)})
	})

	writeTx(t, d, func(tx *db.Tx) error { return storesteam.CloseAllSessions(tx, time.UnixMilli(2000)) })

	for _, uid := range []uint64{1, 2} {
		for _, r := range listPersonaRows(t, d, uid) {
			if !r.end.Valid || r.end.Int64 != 2000 {
				t.Errorf("persona row for u%d should end at 2000: %+v", uid, r)
			}
		}
		for _, r := range listAppRows(t, d, uid) {
			if !r.end.Valid || r.end.Int64 != 2000 {
				t.Errorf("app row for u%d should end at 2000: %+v", uid, r)
			}
		}
	}
}

func TestCloseAllSessions_MaxClamp(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		if err := storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(1000), time.UnixMilli(1000),
			map[uint64]*int{1: intPtr(1)}); err != nil {
			return err
		}
		return storesteam.SyncAppSessions(tx, time.UnixMilli(1000), time.UnixMilli(1000),
			map[uint64]*uint64{1: uint64Ptr(500)})
	})

	writeTx(t, d, func(tx *db.Tx) error { return storesteam.CloseAllSessions(tx, time.UnixMilli(0)) })

	for _, r := range listPersonaRows(t, d, 1) {
		if !r.end.Valid || r.end.Int64 != r.start {
			t.Errorf("persona end should be clamped to start: %+v", r)
		}
	}
	for _, r := range listAppRows(t, d, 1) {
		if !r.end.Valid || r.end.Int64 != r.start {
			t.Errorf("app end should be clamped to start: %+v", r)
		}
	}
}

func TestCloseAllSessions_DoesNotReopenClosedRows(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(100), time.UnixMilli(100),
			map[uint64]*int{1: intPtr(1)})
	})
	writeTx(t, d, func(tx *db.Tx) error { return storesteam.CloseAllSessions(tx, time.UnixMilli(2000)) })
	writeTx(t, d, func(tx *db.Tx) error { return storesteam.CloseAllSessions(tx, time.UnixMilli(3000)) })

	rows := listPersonaRows(t, d, 1)
	if len(rows) != 1 || !rows[0].end.Valid || rows[0].end.Int64 != 2000 {
		t.Errorf("second CloseAllSessions must not move end timestamp: %+v", rows)
	}
}

// ---------- SyncPersonaStateSessions ----------

func TestSyncPersonaState_UserNotInChangesetUntouched(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)
	enableUser(t, d, 2)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(100), time.UnixMilli(100),
			map[uint64]*int{1: intPtr(1)})
	})
	before := listPersonaRows(t, d, 1)

	// User 2 only in the next changeset; user 1's open session must not be touched.
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(200), time.UnixMilli(200),
			map[uint64]*int{2: intPtr(1)})
	})

	after := listPersonaRows(t, d, 1)
	if len(after) != 1 || after[0].id != before[0].id || after[0].end.Valid {
		t.Errorf("u1 must be untouched: before=%+v after=%+v", before, after)
	}
}

func TestSyncPersonaState_NilClosesOpenWithoutOpeningNew(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(100), time.UnixMilli(100),
			map[uint64]*int{1: intPtr(1)})
	})
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(200), time.UnixMilli(200),
			map[uint64]*int{1: nil})
	})

	rows := listPersonaRows(t, d, 1)
	if len(rows) != 1 || !rows[0].end.Valid || rows[0].end.Int64 != 200 {
		t.Errorf("expected single closed row, got %+v", rows)
	}
}

func TestSyncPersonaState_SameValueIsNoOp(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(100), time.UnixMilli(100),
			map[uint64]*int{1: intPtr(1)})
	})
	before := listPersonaRows(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(200), time.UnixMilli(200),
			map[uint64]*int{1: intPtr(1)})
	})

	after := listPersonaRows(t, d, 1)
	if len(after) != 1 || after[0].id != before[0].id || after[0].end.Valid {
		t.Errorf("same value should not churn rows: before=%+v after=%+v", before, after)
	}
}

func TestSyncPersonaState_DifferentValueClosesAndOpens(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(100), time.UnixMilli(100),
			map[uint64]*int{1: intPtr(1)})
	})
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(200), time.UnixMilli(250),
			map[uint64]*int{1: intPtr(3)})
	})

	rows := listPersonaRows(t, d, 1)
	if len(rows) != 2 {
		t.Fatalf("expected 2 persona rows, got %+v", rows)
	}
	if rows[0].personaState != 1 || !rows[0].end.Valid || rows[0].end.Int64 != 200 {
		t.Errorf("first row should be state=1 closed at 200: %+v", rows[0])
	}
	if rows[1].personaState != 3 || rows[1].end.Valid || rows[1].start != 250 {
		t.Errorf("second row should be state=3 open starting at 250: %+v", rows[1])
	}
}

func TestSyncPersonaState_OpensWhenNoOpenSession(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(100), time.UnixMilli(100),
			map[uint64]*int{1: intPtr(2)})
	})

	rows := listPersonaRows(t, d, 1)
	if len(rows) != 1 || rows[0].personaState != 2 || rows[0].end.Valid || rows[0].start != 100 {
		t.Errorf("expected one open row state=2 at 100, got %+v", rows)
	}
}

func TestSyncPersonaState_MaxClampOnLateTimestamp(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(1000), time.UnixMilli(1000),
			map[uint64]*int{1: intPtr(1)})
	})
	writeTx(t, d, func(tx *db.Tx) error {
		// closeTs before first_observed_at — must clamp.
		return storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(500), time.UnixMilli(600),
			map[uint64]*int{1: intPtr(2)})
	})

	rows := listPersonaRows(t, d, 1)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %+v", rows)
	}
	if !rows[0].end.Valid || rows[0].end.Int64 != rows[0].start {
		t.Errorf("first row end should be clamped to start: %+v", rows[0])
	}
}

// ---------- SyncAppSessions ----------

func TestSyncApp_UserNotInChangesetUntouched(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)
	enableUser(t, d, 2)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(100), time.UnixMilli(100),
			map[uint64]*uint64{1: uint64Ptr(500)})
	})
	before := listAppRows(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(200), time.UnixMilli(200),
			map[uint64]*uint64{2: uint64Ptr(500)})
	})

	after := listAppRows(t, d, 1)
	if len(after) != 1 || after[0].id != before[0].id || after[0].end.Valid {
		t.Errorf("u1 should be untouched: before=%+v after=%+v", before, after)
	}
}

func TestSyncApp_NilClosesOpenWithoutOpeningNew(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(100), time.UnixMilli(100),
			map[uint64]*uint64{1: uint64Ptr(500)})
	})
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(200), time.UnixMilli(200),
			map[uint64]*uint64{1: nil})
	})

	rows := listAppRows(t, d, 1)
	if len(rows) != 1 || !rows[0].end.Valid || rows[0].end.Int64 != 200 {
		t.Errorf("expected single closed row, got %+v", rows)
	}
}

func TestSyncApp_ZeroAppIDClosesOpenWithoutOpeningNew(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(100), time.UnixMilli(100),
			map[uint64]*uint64{1: uint64Ptr(500)})
	})
	// *newAppID == 0: close path runs (0 != 500), open path skips on the `*newAppID == 0` guard.
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(200), time.UnixMilli(200),
			map[uint64]*uint64{1: uint64Ptr(0)})
	})

	rows := listAppRows(t, d, 1)
	if len(rows) != 1 || !rows[0].end.Valid || rows[0].end.Int64 != 200 {
		t.Errorf("expected single closed row, got %+v", rows)
	}
}

func TestSyncApp_SameAppIsNoOp(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(100), time.UnixMilli(100),
			map[uint64]*uint64{1: uint64Ptr(500)})
	})
	before := listAppRows(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(200), time.UnixMilli(200),
			map[uint64]*uint64{1: uint64Ptr(500)})
	})

	after := listAppRows(t, d, 1)
	if len(after) != 1 || after[0].id != before[0].id || after[0].end.Valid {
		t.Errorf("same app should not churn rows: before=%+v after=%+v", before, after)
	}
}

func TestSyncApp_DifferentAppClosesAndOpens(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(100), time.UnixMilli(100),
			map[uint64]*uint64{1: uint64Ptr(500)})
	})
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(200), time.UnixMilli(250),
			map[uint64]*uint64{1: uint64Ptr(600)})
	})

	rows := listAppRows(t, d, 1)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %+v", rows)
	}
	if rows[0].appID != 500 || !rows[0].end.Valid || rows[0].end.Int64 != 200 {
		t.Errorf("first row should be app=500 closed at 200: %+v", rows[0])
	}
	if rows[1].appID != 600 || rows[1].end.Valid || rows[1].start != 250 {
		t.Errorf("second row should be app=600 open at 250: %+v", rows[1])
	}
}

func TestSyncApp_OpensWhenNoOpenSession(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(100), time.UnixMilli(100),
			map[uint64]*uint64{1: uint64Ptr(500)})
	})

	rows := listAppRows(t, d, 1)
	if len(rows) != 1 || rows[0].appID != 500 || rows[0].end.Valid || rows[0].start != 100 {
		t.Errorf("expected one open row app=500 at 100, got %+v", rows)
	}
}

func TestSyncApp_InsertsSteamAppRowFirstTime(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(100), time.UnixMilli(100),
			map[uint64]*uint64{1: uint64Ptr(500)})
	})
	if countSteamAppRows(t, d, 500) != 1 {
		t.Fatalf("expected steam_app row for 500 to exist")
	}

	// Set extra info, then open a fresh session for the same app via close+reopen.
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.UpdateAppExtraInfo(tx, 500, "CS", "img", time.UnixMilli(150))
	})
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(200), time.UnixMilli(200),
			map[uint64]*uint64{1: uint64Ptr(600)})
	})
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(300), time.UnixMilli(300),
			map[uint64]*uint64{1: uint64Ptr(500)})
	})

	if got := steamAppName(t, d, 500); got != "CS" {
		t.Errorf("INSERT … ON CONFLICT DO NOTHING must not clobber name, got %q", got)
	}
}

func TestSyncApp_MultiUserSameAppInsertsAppOnce(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)
	enableUser(t, d, 2)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(100), time.UnixMilli(100),
			map[uint64]*uint64{1: uint64Ptr(500), 2: uint64Ptr(500)})
	})

	if n := countSteamAppRows(t, d, 500); n != 1 {
		t.Errorf("expected exactly 1 steam_app row, got %d", n)
	}
	if len(listAppRows(t, d, 1)) != 1 || len(listAppRows(t, d, 2)) != 1 {
		t.Errorf("expected one session per user for the shared app")
	}
}

func TestSyncApp_MaxClampOnLateTimestamp(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(1000), time.UnixMilli(1000),
			map[uint64]*uint64{1: uint64Ptr(500)})
	})
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(500), time.UnixMilli(600),
			map[uint64]*uint64{1: uint64Ptr(600)})
	})

	rows := listAppRows(t, d, 1)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %+v", rows)
	}
	if !rows[0].end.Valid || rows[0].end.Int64 != rows[0].start {
		t.Errorf("close path must clamp end to start: %+v", rows[0])
	}
}

// ---------- multi-operation flows ----------

func TestFlow_UserPersonaLifecycle(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	// Step 1: come online.
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(10), time.UnixMilli(10),
			map[uint64]*int{1: intPtr(1)})
	})
	if rows := listPersonaRows(t, d, 1); countOpen(rows) != 1 {
		t.Fatalf("step 1: want 1 open persona row, got %+v", rows)
	}

	// Step 2: state change online → away.
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(20), time.UnixMilli(20),
			map[uint64]*int{1: intPtr(3)})
	})
	rows := listPersonaRows(t, d, 1)
	if len(rows) != 2 || !rows[0].end.Valid || rows[0].end.Int64 != 20 || rows[1].end.Valid || rows[1].personaState != 3 {
		t.Fatalf("step 2: %+v", rows)
	}

	// Step 3: offline (nil).
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(30), time.UnixMilli(30),
			map[uint64]*int{1: nil})
	})
	if countOpen(listPersonaRows(t, d, 1)) != 0 {
		t.Fatalf("step 3: all rows should be closed")
	}

	// Step 4: back online.
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(40), time.UnixMilli(40),
			map[uint64]*int{1: intPtr(1)})
	})
	rows = listPersonaRows(t, d, 1)
	if len(rows) != 3 || rows[2].end.Valid || rows[2].personaState != 1 || rows[2].start != 40 {
		t.Fatalf("step 4: expected fresh open row, got %+v", rows)
	}

	// Step 5: disable closes everything.
	disableUser(t, d, 1, time.UnixMilli(50))
	if countOpen(listPersonaRows(t, d, 1)) != 0 {
		t.Fatalf("step 5: all rows should be closed after DisableUser")
	}

	// Step 6: SyncPersonaStateSessions on a disabled user still opens a row
	//         (storesteam, unlike storediscord, does not gate sync on enabled).
	//         We don't claim either way — we just snapshot that DisableUser fully
	//         closed prior state.
	rowsBefore := len(listPersonaRows(t, d, 1))
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(60), time.UnixMilli(60),
			map[uint64]*int{1: intPtr(1)})
	})
	rowsAfter := listPersonaRows(t, d, 1)
	if len(rowsAfter) != rowsBefore+1 {
		t.Fatalf("step 6: expected one new row added by sync, got before=%d after=%d", rowsBefore, len(rowsAfter))
	}
}

func TestFlow_AppSessionsLifecycle(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(10), time.UnixMilli(10),
			map[uint64]*uint64{1: uint64Ptr(100)})
	})
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(20), time.UnixMilli(20),
			map[uint64]*uint64{1: uint64Ptr(200)})
	})
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(30), time.UnixMilli(30),
			map[uint64]*uint64{1: uint64Ptr(100)})
	})
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(40), time.UnixMilli(40),
			map[uint64]*uint64{1: nil})
	})
	// Nil again — should not produce a new row.
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(50), time.UnixMilli(50),
			map[uint64]*uint64{1: nil})
	})

	rows := listAppRows(t, d, 1)
	if len(rows) != 3 {
		t.Fatalf("expected exactly 3 rows (A, B, A), got %+v", rows)
	}
	expect := []struct {
		appID uint64
		start int64
		end   int64
	}{
		{100, 10, 20},
		{200, 20, 30},
		{100, 30, 40},
	}
	for i, e := range expect {
		if rows[i].appID != e.appID || rows[i].start != e.start || !rows[i].end.Valid || rows[i].end.Int64 != e.end {
			t.Errorf("row %d: want app=%d start=%d end=%d, got %+v", i, e.appID, e.start, e.end, rows[i])
		}
	}
}

func TestFlow_ReconnectViaCloseAllSessions(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)
	enableUser(t, d, 2)

	// Seed open rows for both users.
	writeTx(t, d, func(tx *db.Tx) error {
		if err := storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(10), time.UnixMilli(10),
			map[uint64]*int{1: intPtr(1), 2: intPtr(1)}); err != nil {
			return err
		}
		return storesteam.SyncAppSessions(tx, time.UnixMilli(10), time.UnixMilli(10),
			map[uint64]*uint64{1: uint64Ptr(500), 2: uint64Ptr(600)})
	})

	// Simulated reconnect.
	writeTx(t, d, func(tx *db.Tx) error { return storesteam.CloseAllSessions(tx, time.UnixMilli(20)) })

	for _, uid := range []uint64{1, 2} {
		for _, r := range listPersonaRows(t, d, uid) {
			if !r.end.Valid || r.end.Int64 != 20 {
				t.Errorf("u%d persona should end at 20: %+v", uid, r)
			}
		}
		for _, r := range listAppRows(t, d, uid) {
			if !r.end.Valid || r.end.Int64 != 20 {
				t.Errorf("u%d app should end at 20: %+v", uid, r)
			}
		}
	}

	// New snapshot containing only U1.
	writeTx(t, d, func(tx *db.Tx) error {
		if err := storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(30), time.UnixMilli(30),
			map[uint64]*int{1: intPtr(1)}); err != nil {
			return err
		}
		return storesteam.SyncAppSessions(tx, time.UnixMilli(30), time.UnixMilli(30),
			map[uint64]*uint64{1: uint64Ptr(500)})
	})

	if countOpen(listPersonaRows(t, d, 1)) != 1 || countOpen(listAppRows(t, d, 1)) != 1 {
		t.Errorf("u1 should have fresh open rows after reconnect")
	}
	if countOpen(listPersonaRows(t, d, 2)) != 0 || countOpen(listAppRows(t, d, 2)) != 0 {
		t.Errorf("u2 (absent from new snapshot) should remain closed")
	}
}

func TestFlow_HeartbeatMonotonicOnAllClosePaths(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1)
	enableUser(t, d, 2)

	writeTx(t, d, func(tx *db.Tx) error {
		if err := storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(1000), time.UnixMilli(1000),
			map[uint64]*int{1: intPtr(1), 2: intPtr(1)}); err != nil {
			return err
		}
		return storesteam.SyncAppSessions(tx, time.UnixMilli(1000), time.UnixMilli(1000),
			map[uint64]*uint64{1: uint64Ptr(500), 2: uint64Ptr(600)})
	})

	// Force every close path with timestamps in the past.
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncPersonaStateSessions(tx, time.UnixMilli(500), time.UnixMilli(500),
			map[uint64]*int{1: intPtr(3)})
	})
	writeTx(t, d, func(tx *db.Tx) error {
		return storesteam.SyncAppSessions(tx, time.UnixMilli(400), time.UnixMilli(400),
			map[uint64]*uint64{1: uint64Ptr(0)})
	})
	writeTx(t, d, func(tx *db.Tx) error { return storesteam.CloseAllSessions(tx, time.UnixMilli(0)) })
	disableUser(t, d, 2, time.UnixMilli(0))

	check := func(table string, start int64, end sql.NullInt64) {
		if end.Valid && end.Int64 < start {
			t.Errorf("%s: end %d < start %d (MAX clamp missing)", table, end.Int64, start)
		}
	}
	for _, uid := range []uint64{1, 2} {
		for _, r := range listPersonaRows(t, d, uid) {
			check("persona", r.start, r.end)
		}
		for _, r := range listAppRows(t, d, uid) {
			check("app", r.start, r.end)
		}
	}
}
