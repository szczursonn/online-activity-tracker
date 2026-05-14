package storediscord_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storediscord"
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

func enableUser(t *testing.T, database *db.DB, userID, presenceGuildID uint64) {
	t.Helper()
	writeTx(t, database, func(tx *db.Tx) error {
		return storediscord.InsertOrEnableUser(tx, userID, presenceGuildID, time.UnixMilli(0))
	})
}

func disableUser(t *testing.T, database *db.DB, userID uint64, ts time.Time) {
	t.Helper()
	writeTx(t, database, func(tx *db.Tx) error {
		return storediscord.DisableUser(tx, userID, ts)
	})
}

// ---------- row inspection ----------

type statusRow struct {
	id      int64
	userID  uint64
	guildID uint64
	desktop int
	mobile  int
	web     int
	start   int64
	end     sql.NullInt64
}

func listStatusRows(t *testing.T, database *db.DB, userID uint64) []statusRow {
	t.Helper()
	var rows []statusRow
	readTx(t, database, func(tx *db.Tx) error {
		r, err := tx.Query(`
			SELECT id, user_id, guild_id, status_desktop, status_mobile, status_web, start_observed_at, end_observed_at
			FROM discord_session_status WHERE user_id = ? ORDER BY id;
		`, int64(userID))
		if err != nil {
			return err
		}
		defer r.Close()
		for r.Next() {
			var s statusRow
			var uid, gid int64
			if err := r.Scan(&s.id, &uid, &gid, &s.desktop, &s.mobile, &s.web, &s.start, &s.end); err != nil {
				return err
			}
			s.userID, s.guildID = uint64(uid), uint64(gid)
			rows = append(rows, s)
		}
		return r.Err()
	})
	return rows
}

type activityRow struct {
	id      int64
	userID  uint64
	guildID uint64
	name    string
	details string
	state   string
	start   int64
	end     sql.NullInt64
}

func listActivityRows(t *testing.T, database *db.DB, userID uint64) []activityRow {
	t.Helper()
	var rows []activityRow
	readTx(t, database, func(tx *db.Tx) error {
		r, err := tx.Query(`
			SELECT dsa.id, dsa.user_id, dsa.guild_id, dan.name, COALESCE(dad.details, ''), COALESCE(das.state, ''), dsa.start_observed_at, dsa.end_observed_at
			FROM discord_session_activity dsa
			LEFT JOIN discord_activity_name dan ON dan.id = dsa.name_id
			LEFT JOIN discord_activity_details dad ON dad.id = dsa.details_id
			LEFT JOIN discord_activity_state das ON das.id = dsa.state_id
			WHERE dsa.user_id = ? ORDER BY dsa.id;
		`, int64(userID))
		if err != nil {
			return err
		}
		defer r.Close()
		for r.Next() {
			var a activityRow
			var uid, gid int64
			if err := r.Scan(&a.id, &uid, &gid, &a.name, &a.details, &a.state, &a.start, &a.end); err != nil {
				return err
			}
			a.userID, a.guildID = uint64(uid), uint64(gid)
			rows = append(rows, a)
		}
		return r.Err()
	})
	return rows
}

type voiceRow struct {
	id        int64
	userID    uint64
	channelID uint64
	start     int64
	end       sql.NullInt64
}

func listVoiceRows(t *testing.T, database *db.DB, userID uint64) []voiceRow {
	t.Helper()
	var rows []voiceRow
	readTx(t, database, func(tx *db.Tx) error {
		r, err := tx.Query(`
			SELECT id, user_id, channel_id, start_observed_at, end_observed_at
			FROM discord_session_voice WHERE user_id = ? ORDER BY id;
		`, int64(userID))
		if err != nil {
			return err
		}
		defer r.Close()
		for r.Next() {
			var v voiceRow
			var uid, cid int64
			if err := r.Scan(&v.id, &uid, &cid, &v.start, &v.end); err != nil {
				return err
			}
			v.userID, v.channelID = uint64(uid), uint64(cid)
			rows = append(rows, v)
		}
		return r.Err()
	})
	return rows
}

func countOpen[T statusRow | activityRow | voiceRow](rows []T) int {
	open := 0
	for _, r := range rows {
		switch v := any(r).(type) {
		case statusRow:
			if !v.end.Valid {
				open++
			}
		case activityRow:
			if !v.end.Valid {
				open++
			}
		case voiceRow:
			if !v.end.Valid {
				open++
			}
		}
	}
	return open
}

// ---------- builders ----------

func buildPresence(userID uint64, guildID uint64, desktop int, mobile int, web int, acts ...storediscord.SyncPresenceParamActivity) *storediscord.SyncPresenceParam {
	return &storediscord.SyncPresenceParam{
		UserID:        userID,
		GuildID:       guildID,
		DesktopStatus: desktop,
		MobileStatus:  mobile,
		WebStatus:     web,
		Activities:    acts,
	}
}

func buildActivity(name string, details string, state string) storediscord.SyncPresenceParamActivity {
	return storediscord.SyncPresenceParamActivity{Name: name, Details: details, State: state}
}

func buildVoice(userID uint64, guildID uint64, channelID uint64) *storediscord.SyncVoiceParam {
	return &storediscord.SyncVoiceParam{UserID: userID, GuildID: guildID, ChannelID: channelID}
}

// ---------- SyncPresence ----------

func TestSyncPresence_DisabledUserIsNoOp(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)
	disableUser(t, d, 1, time.UnixMilli(0))

	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(1000), buildPresence(1, 100, 0, 0, 0, buildActivity("Game", "", "")))
	})

	if got := listStatusRows(t, d, 1); len(got) != 0 {
		t.Errorf("expected no status rows for disabled user, got %d", len(got))
	}
	if got := listActivityRows(t, d, 1); len(got) != 0 {
		t.Errorf("expected no activity rows for disabled user, got %d", len(got))
	}
}

func TestSyncPresence_WrongPresenceGuildIsNoOp(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)

	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(1000), buildPresence(1, 200, 0, 0, 0, buildActivity("Game", "", "")))
	})

	if got := listStatusRows(t, d, 1); len(got) != 0 {
		t.Errorf("expected no status rows when presence guild does not match, got %d", len(got))
	}
	if got := listActivityRows(t, d, 1); len(got) != 0 {
		t.Errorf("expected no activity rows when presence guild does not match, got %d", len(got))
	}
}

func TestSyncPresence_FirstSyncOpensSessions(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)

	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(1000), buildPresence(1, 100, 0, 3, 3, buildActivity("A", "", ""), buildActivity("B", "deets", "stat")))
	})

	statusRows := listStatusRows(t, d, 1)
	if len(statusRows) != 1 || statusRows[0].end.Valid {
		t.Fatalf("expected 1 open status row, got %+v", statusRows)
	}
	if statusRows[0].desktop != 0 || statusRows[0].mobile != 3 || statusRows[0].web != 3 || statusRows[0].start != 1000 {
		t.Errorf("unexpected status row contents: %+v", statusRows[0])
	}

	activityRows := listActivityRows(t, d, 1)
	if len(activityRows) != 2 {
		t.Fatalf("expected 2 activity rows, got %+v", activityRows)
	}
	for _, a := range activityRows {
		if a.end.Valid {
			t.Errorf("activity row should be open, got %+v", a)
		}
	}
}

func TestSyncPresence_IdempotentResync(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)
	p := buildPresence(1, 100, 0, 3, 3, buildActivity("A", "", ""), buildActivity("B", "", ""))

	writeTx(t, d, func(tx *db.Tx) error { return storediscord.SyncPresence(tx, time.UnixMilli(1000), p) })
	statusBefore := listStatusRows(t, d, 1)
	activityBefore := listActivityRows(t, d, 1)

	writeTx(t, d, func(tx *db.Tx) error { return storediscord.SyncPresence(tx, time.UnixMilli(2000), p) })
	statusAfter := listStatusRows(t, d, 1)
	activityAfter := listActivityRows(t, d, 1)

	if len(statusAfter) != len(statusBefore) || statusAfter[0].id != statusBefore[0].id || statusAfter[0].end.Valid {
		t.Errorf("re-sync changed status rows: before=%+v after=%+v", statusBefore, statusAfter)
	}
	if len(activityAfter) != len(activityBefore) {
		t.Errorf("re-sync changed activity row count: before=%d after=%d", len(activityBefore), len(activityAfter))
	}
	for i, a := range activityAfter {
		if a.id != activityBefore[i].id || a.end.Valid {
			t.Errorf("re-sync changed activity row %d: before=%+v after=%+v", i, activityBefore[i], a)
		}
	}
}

func TestSyncPresence_StatusChangeClosesAndOpens(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)

	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(1000), buildPresence(1, 100, 0, 3, 3))
	})
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(2000), buildPresence(1, 100, 2, 3, 3))
	})

	rows := listStatusRows(t, d, 1)
	if len(rows) != 2 {
		t.Fatalf("expected 2 status rows, got %+v", rows)
	}
	if !rows[0].end.Valid || rows[0].end.Int64 != 2000 {
		t.Errorf("first status row should be closed at 2000: %+v", rows[0])
	}
	if rows[1].end.Valid || rows[1].desktop != 2 || rows[1].start != 2000 {
		t.Errorf("second status row should be open with desktop=2 start=2000: %+v", rows[1])
	}
}

func TestSyncPresence_ActivityDiff(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)

	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(1000), buildPresence(1, 100, 0, 3, 3, buildActivity("A", "", ""), buildActivity("B", "", "")))
	})
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(2000), buildPresence(1, 100, 0, 3, 3, buildActivity("B", "", ""), buildActivity("C", "", "")))
	})

	rows := listActivityRows(t, d, 1)
	byName := map[string]activityRow{}
	for _, r := range rows {
		byName[r.name] = r
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 activity rows (A closed, B unchanged, C new), got %+v", rows)
	}
	if a, ok := byName["A"]; !ok || !a.end.Valid || a.end.Int64 != 2000 {
		t.Errorf("A should be closed at 2000: %+v", a)
	}
	if b, ok := byName["B"]; !ok || b.end.Valid || b.start != 1000 {
		t.Errorf("B should be open and unchanged (start=1000): %+v", b)
	}
	if c, ok := byName["C"]; !ok || c.end.Valid || c.start != 2000 {
		t.Errorf("C should be newly opened at 2000: %+v", c)
	}
}

func TestSyncPresence_AllActivitiesRemovedClosesAll(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)

	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(1000), buildPresence(1, 100, 0, 3, 3, buildActivity("A", "", "")))
	})
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(2000), buildPresence(1, 100, 0, 3, 3))
	})

	rows := listActivityRows(t, d, 1)
	if len(rows) != 1 || !rows[0].end.Valid || rows[0].end.Int64 != 2000 {
		t.Errorf("activity A should be closed at 2000, no new rows: %+v", rows)
	}
}

func TestSyncPresence_MaxClampOnLateTimestamp(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)

	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(1000), buildPresence(1, 100, 0, 3, 3, buildActivity("A", "", "")))
	})
	// Earlier timestamp than start: close path must clamp to start_observed_at.
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(500), buildPresence(1, 100, 1, 3, 3))
	})

	statusRows := listStatusRows(t, d, 1)
	if len(statusRows) != 2 {
		t.Fatalf("expected 2 status rows, got %+v", statusRows)
	}
	if !statusRows[0].end.Valid || statusRows[0].end.Int64 != statusRows[0].start {
		t.Errorf("first status row end should be clamped to start (%d), got %+v", statusRows[0].start, statusRows[0])
	}

	activityRows := listActivityRows(t, d, 1)
	if len(activityRows) != 1 || !activityRows[0].end.Valid || activityRows[0].end.Int64 != activityRows[0].start {
		t.Errorf("activity A end should be clamped to start, got %+v", activityRows)
	}
}

// ---------- SyncVoice ----------

func TestSyncVoice_DisabledUserIsNoOp(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)
	disableUser(t, d, 1, time.UnixMilli(0))

	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncVoice(tx, time.UnixMilli(1000), buildVoice(1, 100, 10))
	})
	if got := listVoiceRows(t, d, 1); len(got) != 0 {
		t.Errorf("expected no voice rows for disabled user, got %+v", got)
	}
}

func TestSyncVoice_FirstConnectOpensSession(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)

	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncVoice(tx, time.UnixMilli(1000), buildVoice(1, 100, 10))
	})

	rows := listVoiceRows(t, d, 1)
	if len(rows) != 1 || rows[0].end.Valid || rows[0].channelID != 10 || rows[0].start != 1000 {
		t.Errorf("expected one open voice row in channel 10 at 1000, got %+v", rows)
	}
}

func TestSyncVoice_ChannelSwitchClosesOldOpensNew(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)

	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncVoice(tx, time.UnixMilli(1000), buildVoice(1, 100, 10))
	})
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncVoice(tx, time.UnixMilli(2000), buildVoice(1, 100, 20))
	})

	rows := listVoiceRows(t, d, 1)
	if len(rows) != 2 {
		t.Fatalf("expected 2 voice rows after switch, got %+v", rows)
	}
	if rows[0].channelID != 10 || !rows[0].end.Valid || rows[0].end.Int64 != 2000 {
		t.Errorf("first row should be channel 10 closed at 2000, got %+v", rows[0])
	}
	if rows[1].channelID != 20 || rows[1].end.Valid || rows[1].start != 2000 {
		t.Errorf("second row should be channel 20 open at 2000, got %+v", rows[1])
	}
}

func TestSyncVoice_DisconnectClosesAndDoesNotReopen(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)

	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncVoice(tx, time.UnixMilli(1000), buildVoice(1, 100, 10))
	})
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncVoice(tx, time.UnixMilli(2000), buildVoice(1, 100, 0))
	})

	rows := listVoiceRows(t, d, 1)
	if len(rows) != 1 {
		t.Fatalf("expected 1 voice row after disconnect, got %+v", rows)
	}
	if !rows[0].end.Valid || rows[0].end.Int64 != 2000 {
		t.Errorf("row should be closed at 2000, got %+v", rows[0])
	}
}

func TestSyncVoice_SameChannelResyncIsIdempotent(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)

	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncVoice(tx, time.UnixMilli(1000), buildVoice(1, 100, 10))
	})
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncVoice(tx, time.UnixMilli(2000), buildVoice(1, 100, 10))
	})

	rows := listVoiceRows(t, d, 1)
	if len(rows) != 1 || rows[0].end.Valid || rows[0].start != 1000 {
		t.Errorf("expected single open row unchanged, got %+v", rows)
	}
}

func TestSyncVoice_OtherGuildSessionUntouched(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)

	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncVoice(tx, time.UnixMilli(1000), buildVoice(1, 200, 20))
	})
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncVoice(tx, time.UnixMilli(2000), buildVoice(1, 100, 10))
	})

	rows := listVoiceRows(t, d, 1)
	if len(rows) != 2 {
		t.Fatalf("expected 2 open voice rows (one per guild), got %+v", rows)
	}
	for _, r := range rows {
		if r.end.Valid {
			t.Errorf("no row should be closed; SyncVoice is guild-scoped: %+v", r)
		}
	}
}

// ---------- ResetGuildState ----------

func TestResetGuildState_ClosesExistingGuildSessions(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)

	writeTx(t, d, func(tx *db.Tx) error {
		if err := storediscord.SyncPresence(tx, time.UnixMilli(1000), buildPresence(1, 100, 0, 3, 3, buildActivity("A", "", ""))); err != nil {
			return err
		}
		return storediscord.SyncVoice(tx, time.UnixMilli(1000), buildVoice(1, 100, 10))
	})

	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.ResetGuildState(tx, time.UnixMilli(2000), &storediscord.ResetGuildStateParam{ID: 100})
	})

	for _, r := range listStatusRows(t, d, 1) {
		if !r.end.Valid || r.end.Int64 != 2000 {
			t.Errorf("status row should be closed at 2000: %+v", r)
		}
	}
	for _, r := range listActivityRows(t, d, 1) {
		if !r.end.Valid || r.end.Int64 != 2000 {
			t.Errorf("activity row should be closed at 2000: %+v", r)
		}
	}
	for _, r := range listVoiceRows(t, d, 1) {
		if !r.end.Valid || r.end.Int64 != 2000 {
			t.Errorf("voice row should be closed at 2000: %+v", r)
		}
	}
}

func TestResetGuildState_OtherGuildSessionsUntouched(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)
	enableUser(t, d, 2, 200)

	writeTx(t, d, func(tx *db.Tx) error {
		if err := storediscord.SyncPresence(tx, time.UnixMilli(1000), buildPresence(1, 100, 0, 3, 3)); err != nil {
			return err
		}
		return storediscord.SyncPresence(tx, time.UnixMilli(1000), buildPresence(2, 200, 0, 3, 3))
	})

	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.ResetGuildState(tx, time.UnixMilli(2000), &storediscord.ResetGuildStateParam{ID: 100})
	})

	u1 := listStatusRows(t, d, 1)
	if len(u1) != 1 || !u1[0].end.Valid || u1[0].end.Int64 != 2000 || u1[0].guildID != 100 {
		t.Errorf("u1 status row in reset guild should be closed at 2000: %+v", u1)
	}

	u2 := listStatusRows(t, d, 2)
	if len(u2) != 1 || u2[0].end.Valid || u2[0].guildID != 200 {
		t.Errorf("u2 status row in other guild should remain open: %+v", u2)
	}
}

func TestResetGuildState_OpensSessionsFromSnapshot(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)
	enableUser(t, d, 2, 100)
	enableUser(t, d, 3, 200) // user in other presence guild

	param := &storediscord.ResetGuildStateParam{
		ID:                100,
		Name:              "Guild",
		VoiceChannelsByID: map[uint64]storediscord.ResetGuildStateVoiceChannel{10: {Name: "general"}},
		MembersByUserID:   map[uint64]storediscord.ResetGuildStateUser{1: {Name: "u1"}, 2: {Name: "u2"}, 3: {Name: "u3"}},
		PresencesByUserID: map[uint64]storediscord.ResetGuildStatePresence{
			1: {DesktopStatus: 0, MobileStatus: 3, WebStatus: 3, Activities: []storediscord.ResetGuildStateActivity{{Name: "A"}}},
			3: {DesktopStatus: 0, MobileStatus: 3, WebStatus: 3, Activities: []storediscord.ResetGuildStateActivity{{Name: "Z"}}},
		},
		VoiceStatesByUserID: map[uint64]storediscord.ResetGuildStateVoiceState{2: {ChannelID: 10}},
	}

	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.ResetGuildState(tx, time.UnixMilli(2000), param)
	})

	// User 1: presence guild matches → status + activity opened.
	u1s := listStatusRows(t, d, 1)
	if len(u1s) != 1 || u1s[0].end.Valid || u1s[0].start != 2000 {
		t.Errorf("u1 expected 1 open status at 2000: %+v", u1s)
	}
	u1a := listActivityRows(t, d, 1)
	if len(u1a) != 1 || u1a[0].name != "A" || u1a[0].end.Valid {
		t.Errorf("u1 expected 1 open activity A: %+v", u1a)
	}

	// User 2: no presence in snapshot → no status/activity. Voice opened.
	if got := listStatusRows(t, d, 2); len(got) != 0 {
		t.Errorf("u2 expected 0 status rows: %+v", got)
	}
	u2v := listVoiceRows(t, d, 2)
	if len(u2v) != 1 || u2v[0].channelID != 10 || u2v[0].end.Valid {
		t.Errorf("u2 expected 1 open voice in channel 10: %+v", u2v)
	}

	// User 3: presence guild does not match → no presence rows even though snapshot has activity.
	if got := listStatusRows(t, d, 3); len(got) != 0 {
		t.Errorf("u3 expected 0 status rows (wrong presence guild): %+v", got)
	}
	if got := listActivityRows(t, d, 3); len(got) != 0 {
		t.Errorf("u3 expected 0 activity rows (wrong presence guild): %+v", got)
	}
}

func TestResetGuildState_DisabledUsersIgnored(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)
	disableUser(t, d, 1, time.UnixMilli(0))

	param := &storediscord.ResetGuildStateParam{
		ID: 100,
		PresencesByUserID: map[uint64]storediscord.ResetGuildStatePresence{
			1: {Activities: []storediscord.ResetGuildStateActivity{{Name: "A"}}},
		},
	}
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.ResetGuildState(tx, time.UnixMilli(2000), param)
	})

	if got := listStatusRows(t, d, 1); len(got) != 0 {
		t.Errorf("disabled user should not get status rows: %+v", got)
	}
	if got := listActivityRows(t, d, 1); len(got) != 0 {
		t.Errorf("disabled user should not get activity rows: %+v", got)
	}
}

// ---------- multi-operation flows ----------

func TestFlow_UserPresenceLifecycle(t *testing.T) {
	d := newTestDB(t)

	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.InsertOrEnableUser(tx, 1, 100, time.UnixMilli(10))
	})

	// Step 2: first presence with activity A.
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(20), buildPresence(1, 100, 0, 3, 3, buildActivity("A", "", "")))
	})
	if s, a := listStatusRows(t, d, 1), listActivityRows(t, d, 1); countOpen(s) != 1 || countOpen(a) != 1 {
		t.Fatalf("step 2: want 1 open status + 1 open activity, got status=%+v activity=%+v", s, a)
	}

	// Step 3: status changes (desktop online → idle), activity unchanged.
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(30), buildPresence(1, 100, 2, 3, 3, buildActivity("A", "", "")))
	})
	s := listStatusRows(t, d, 1)
	if len(s) != 2 || !s[0].end.Valid || s[0].end.Int64 != 30 || s[1].end.Valid || s[1].desktop != 2 {
		t.Fatalf("step 3: status rows wrong: %+v", s)
	}
	a := listActivityRows(t, d, 1)
	if len(a) != 1 || a[0].end.Valid {
		t.Fatalf("step 3: activity A should still be open, got %+v", a)
	}

	// Step 4: add activity B.
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(40), buildPresence(1, 100, 2, 3, 3, buildActivity("A", "", ""), buildActivity("B", "", "")))
	})
	a = listActivityRows(t, d, 1)
	if len(a) != 2 || a[0].end.Valid || a[1].end.Valid || a[1].start != 40 {
		t.Fatalf("step 4: expected A still open + B opened at 40, got %+v", a)
	}

	// Step 5: drop A, keep B.
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(50), buildPresence(1, 100, 2, 3, 3, buildActivity("B", "", "")))
	})
	a = listActivityRows(t, d, 1)
	byName := map[string]activityRow{}
	for _, r := range a {
		byName[r.name] = r
	}
	if !byName["A"].end.Valid || byName["A"].end.Int64 != 50 {
		t.Fatalf("step 5: A should be closed at 50, got %+v", byName["A"])
	}
	if byName["B"].end.Valid || byName["B"].start != 40 {
		t.Fatalf("step 5: B should remain open from 40, got %+v", byName["B"])
	}

	// Step 6: offline + no activities.
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(60), buildPresence(1, 100, 3, 3, 3))
	})
	s = listStatusRows(t, d, 1)
	if len(s) != 3 || s[1].end.Int64 != 60 || s[2].end.Valid || s[2].desktop != 3 {
		t.Fatalf("step 6: status rows wrong: %+v", s)
	}
	a = listActivityRows(t, d, 1)
	for _, r := range a {
		if !r.end.Valid {
			t.Fatalf("step 6: all activity rows should be closed, got %+v", a)
		}
	}

	// Step 7: disable user.
	writeTx(t, d, func(tx *db.Tx) error { return storediscord.DisableUser(tx, 1, time.UnixMilli(70)) })
	if countOpen(listStatusRows(t, d, 1)) != 0 || countOpen(listActivityRows(t, d, 1)) != 0 {
		t.Fatalf("step 7: all rows should be closed after DisableUser")
	}

	// Step 8: SyncPresence on disabled user — no new rows.
	statusBefore := len(listStatusRows(t, d, 1))
	activityBefore := len(listActivityRows(t, d, 1))
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(80), buildPresence(1, 100, 0, 3, 3, buildActivity("A", "", "")))
	})
	if len(listStatusRows(t, d, 1)) != statusBefore || len(listActivityRows(t, d, 1)) != activityBefore {
		t.Fatalf("step 8: SyncPresence on disabled user must not create rows")
	}
}

func TestFlow_VoiceChannelHopping(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)

	writeTx(t, d, func(tx *db.Tx) error { return storediscord.SyncVoice(tx, time.UnixMilli(10), buildVoice(1, 100, 10)) })
	writeTx(t, d, func(tx *db.Tx) error { return storediscord.SyncVoice(tx, time.UnixMilli(20), buildVoice(1, 100, 20)) })
	writeTx(t, d, func(tx *db.Tx) error { return storediscord.SyncVoice(tx, time.UnixMilli(30), buildVoice(1, 100, 10)) })
	writeTx(t, d, func(tx *db.Tx) error { return storediscord.SyncVoice(tx, time.UnixMilli(40), buildVoice(1, 100, 0)) })
	writeTx(t, d, func(tx *db.Tx) error { return storediscord.SyncVoice(tx, time.UnixMilli(50), buildVoice(1, 100, 0)) })

	rows := listVoiceRows(t, d, 1)
	if len(rows) != 3 {
		t.Fatalf("expected exactly 3 voice rows (C1, C2, C1 again), got %+v", rows)
	}
	expect := []struct {
		channel uint64
		start   int64
		end     int64
	}{
		{10, 10, 20},
		{20, 20, 30},
		{10, 30, 40},
	}
	for i, e := range expect {
		if rows[i].channelID != e.channel || rows[i].start != e.start || !rows[i].end.Valid || rows[i].end.Int64 != e.end {
			t.Errorf("row %d: want channel=%d start=%d end=%d, got %+v", i, e.channel, e.start, e.end, rows[i])
		}
	}
}

func TestFlow_ReconnectHandoff(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)
	enableUser(t, d, 2, 100)

	// Seed pre-disconnect sessions for both users.
	writeTx(t, d, func(tx *db.Tx) error {
		if err := storediscord.SyncPresence(tx, time.UnixMilli(10), buildPresence(1, 100, 0, 3, 3, buildActivity("A", "", ""))); err != nil {
			return err
		}
		if err := storediscord.SyncPresence(tx, time.UnixMilli(10), buildPresence(2, 100, 0, 3, 3, buildActivity("X", "", ""))); err != nil {
			return err
		}
		if err := storediscord.SyncVoice(tx, time.UnixMilli(10), buildVoice(1, 100, 10)); err != nil {
			return err
		}
		return storediscord.SyncVoice(tx, time.UnixMilli(10), buildVoice(2, 100, 10))
	})

	// Simulated reconnect: CloseAllSessions at last heartbeat.
	writeTx(t, d, func(tx *db.Tx) error { return storediscord.CloseAllSessions(tx, time.UnixMilli(20)) })

	// Verify every pre-existing row is now closed at 20.
	for _, uid := range []uint64{1, 2} {
		for _, r := range listStatusRows(t, d, uid) {
			if !r.end.Valid || r.end.Int64 != 20 {
				t.Errorf("after CloseAllSessions: status row for u%d should end at 20: %+v", uid, r)
			}
		}
		for _, r := range listActivityRows(t, d, uid) {
			if !r.end.Valid || r.end.Int64 != 20 {
				t.Errorf("after CloseAllSessions: activity row for u%d should end at 20: %+v", uid, r)
			}
		}
		for _, r := range listVoiceRows(t, d, uid) {
			if !r.end.Valid || r.end.Int64 != 20 {
				t.Errorf("after CloseAllSessions: voice row for u%d should end at 20: %+v", uid, r)
			}
		}
	}

	// ResetGuildState with snapshot containing only U1 (not U2).
	param := &storediscord.ResetGuildStateParam{
		ID:                100,
		VoiceChannelsByID: map[uint64]storediscord.ResetGuildStateVoiceChannel{10: {Name: "general"}},
		MembersByUserID:   map[uint64]storediscord.ResetGuildStateUser{1: {}},
		PresencesByUserID: map[uint64]storediscord.ResetGuildStatePresence{
			1: {DesktopStatus: 0, MobileStatus: 3, WebStatus: 3, Activities: []storediscord.ResetGuildStateActivity{{Name: "A"}}},
		},
		VoiceStatesByUserID: map[uint64]storediscord.ResetGuildStateVoiceState{1: {ChannelID: 10}},
	}
	writeTx(t, d, func(tx *db.Tx) error { return storediscord.ResetGuildState(tx, time.UnixMilli(30), param) })

	// U1 should have fresh open rows at 30.
	if cs := listStatusRows(t, d, 1); countOpen(cs) != 1 || cs[len(cs)-1].start != 30 {
		t.Errorf("u1: expected 1 open status row at 30, got %+v", cs)
	}
	if ca := listActivityRows(t, d, 1); countOpen(ca) != 1 || ca[len(ca)-1].start != 30 || ca[len(ca)-1].name != "A" {
		t.Errorf("u1: expected 1 open activity A at 30, got %+v", ca)
	}
	if cv := listVoiceRows(t, d, 1); countOpen(cv) != 1 || cv[len(cv)-1].start != 30 || cv[len(cv)-1].channelID != 10 {
		t.Errorf("u1: expected 1 open voice in channel 10 at 30, got %+v", cv)
	}

	// U2 should have no open rows.
	if countOpen(listStatusRows(t, d, 2)) != 0 || countOpen(listActivityRows(t, d, 2)) != 0 || countOpen(listVoiceRows(t, d, 2)) != 0 {
		t.Errorf("u2 should have no open rows after handoff")
	}
}

func TestFlow_GuildJoinThenLeave(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)
	enableUser(t, d, 2, 100)
	enableUser(t, d, 3, 200) // unrelated user in another guild

	// Open a presence row for user 3 in guild 200 (should be unaffected by anything below).
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(5), buildPresence(3, 200, 0, 3, 3, buildActivity("Z", "", "")))
	})

	// Step 1: ResetGuildState on join.
	param := &storediscord.ResetGuildStateParam{
		ID:                100,
		VoiceChannelsByID: map[uint64]storediscord.ResetGuildStateVoiceChannel{10: {}},
		MembersByUserID:   map[uint64]storediscord.ResetGuildStateUser{1: {}, 2: {}},
		PresencesByUserID: map[uint64]storediscord.ResetGuildStatePresence{
			1: {Activities: []storediscord.ResetGuildStateActivity{{Name: "A"}}},
			2: {Activities: []storediscord.ResetGuildStateActivity{{Name: "B"}}},
		},
		VoiceStatesByUserID: map[uint64]storediscord.ResetGuildStateVoiceState{1: {ChannelID: 10}},
	}
	writeTx(t, d, func(tx *db.Tx) error { return storediscord.ResetGuildState(tx, time.UnixMilli(10), param) })

	// Step 2: a couple of presence updates for U1.
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(20), buildPresence(1, 100, 0, 3, 3, buildActivity("A", "", ""), buildActivity("A2", "", "")))
	})
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(30), buildPresence(1, 100, 0, 3, 3, buildActivity("A2", "", "")))
	})

	// Step 3: guild leave.
	writeTx(t, d, func(tx *db.Tx) error { return storediscord.CloseAllSessionsForGuild(tx, 100, time.UnixMilli(40)) })

	for _, uid := range []uint64{1, 2} {
		if countOpen(listStatusRows(t, d, uid)) != 0 {
			t.Errorf("u%d: status rows should all be closed after guild leave", uid)
		}
		if countOpen(listActivityRows(t, d, uid)) != 0 {
			t.Errorf("u%d: activity rows should all be closed after guild leave", uid)
		}
		if countOpen(listVoiceRows(t, d, uid)) != 0 {
			t.Errorf("u%d: voice rows should all be closed after guild leave", uid)
		}
	}
	// User 3 in guild 200 must be untouched.
	if countOpen(listStatusRows(t, d, 3)) != 1 || countOpen(listActivityRows(t, d, 3)) != 1 {
		t.Errorf("u3 in guild 200 should still have open presence rows")
	}

	// Step 4: a fresh SyncPresence reopens cleanly.
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(50), buildPresence(1, 100, 0, 3, 3, buildActivity("A", "", "")))
	})
	if countOpen(listStatusRows(t, d, 1)) != 1 || countOpen(listActivityRows(t, d, 1)) != 1 {
		t.Errorf("u1: after re-joining, expected fresh open rows")
	}
}

func TestFlow_PresenceGuildMigration(t *testing.T) {
	d := newTestDB(t)
	writeTx(t, d, func(tx *db.Tx) error { return storediscord.InsertOrEnableUser(tx, 1, 100, time.UnixMilli(0)) })

	// Open presence in G1.
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(10), buildPresence(1, 100, 0, 3, 3, buildActivity("A", "", "")))
	})

	// Migrate to G2.
	writeTx(t, d, func(tx *db.Tx) error { return storediscord.InsertOrEnableUser(tx, 1, 200, time.UnixMilli(20)) })

	// All G1 rows for this user should now be closed at 20.
	for _, r := range listStatusRows(t, d, 1) {
		if r.guildID == 100 && (!r.end.Valid || r.end.Int64 != 20) {
			t.Errorf("G1 status row should be closed at 20: %+v", r)
		}
	}
	for _, r := range listActivityRows(t, d, 1) {
		if r.guildID == 100 && (!r.end.Valid || r.end.Int64 != 20) {
			t.Errorf("G1 activity row should be closed at 20: %+v", r)
		}
	}

	// SyncPresence for G1 (wrong presence guild) must be a no-op.
	beforeStatus := len(listStatusRows(t, d, 1))
	beforeActivity := len(listActivityRows(t, d, 1))
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(30), buildPresence(1, 100, 0, 3, 3, buildActivity("A", "", "")))
	})
	if len(listStatusRows(t, d, 1)) != beforeStatus || len(listActivityRows(t, d, 1)) != beforeActivity {
		t.Errorf("SyncPresence in G1 after migration should be a no-op")
	}

	// SyncPresence for G2 opens fresh rows.
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(40), buildPresence(1, 200, 0, 3, 3, buildActivity("A", "", "")))
	})
	var openInG2 int
	for _, r := range listStatusRows(t, d, 1) {
		if r.guildID == 200 && !r.end.Valid {
			openInG2++
		}
	}
	if openInG2 != 1 {
		t.Errorf("expected exactly 1 open status row in G2, got %d", openInG2)
	}
}

func TestFlow_HeartbeatMonotonic(t *testing.T) {
	d := newTestDB(t)
	enableUser(t, d, 1, 100)

	writeTx(t, d, func(tx *db.Tx) error {
		if err := storediscord.SyncPresence(tx, time.UnixMilli(1000), buildPresence(1, 100, 0, 3, 3, buildActivity("A", "", ""))); err != nil {
			return err
		}
		return storediscord.SyncVoice(tx, time.UnixMilli(1000), buildVoice(1, 100, 10))
	})

	// Force every close path with timestamps in the past.
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncPresence(tx, time.UnixMilli(500), buildPresence(1, 100, 2, 3, 3))
	})
	writeTx(t, d, func(tx *db.Tx) error {
		return storediscord.SyncVoice(tx, time.UnixMilli(400), buildVoice(1, 100, 0))
	})
	writeTx(t, d, func(tx *db.Tx) error { return storediscord.CloseAllSessions(tx, time.UnixMilli(0)) })

	check := func(table string, start int64, end sql.NullInt64) {
		if end.Valid && end.Int64 < start {
			t.Errorf("%s: end_observed_at %d < start_observed_at %d (MAX clamp missing)", table, end.Int64, start)
		}
	}
	for _, r := range listStatusRows(t, d, 1) {
		check("status", r.start, r.end)
	}
	for _, r := range listActivityRows(t, d, 1) {
		check("activity", r.start, r.end)
	}
	for _, r := range listVoiceRows(t, d, 1) {
		check("voice", r.start, r.end)
	}
}
