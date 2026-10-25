package messages

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"path/filepath"
	"testing"

	"relaynest/server/internal/storage"
)

const (
	messageA = "00000000-0000-4000-8000-000000000001"
	messageB = "00000000-0000-4000-8000-000000000002"
	messageC = "00000000-0000-4000-8000-000000000003"
)

func TestSchemaConstraintsAndCleanup(t *testing.T) {
	ctx := context.Background()
	cfg := storage.Config{Path: filepath.Join(t.TempDir(), "messages.sqlite")}
	s, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	exec := func(query string, args ...any) error {
		return s.Write(ctx, func(tx *sql.Tx) error { _, err := tx.Exec(query, args...); return err })
	}
	must := func(query string, args ...any) {
		t.Helper()
		if err := exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}
	reject := func(query string, args ...any) {
		t.Helper()
		if err := exec(query, args...); err == nil {
			t.Fatalf("accepted invalid SQL: %s", query)
		}
	}
	must("INSERT INTO bots VALUES('a','A',1,1,1,1,0),('b','B',1,1,1,1,0)")
	must("INSERT INTO sessions VALUES('s',zeroblob(32),1,'android','device','name',100,200,0)")
	insert := `INSERT INTO messages(id,bot_id,bot_seq,created_change_seq,sender,revision,format,text,buttons_json,created_at,updated_at) VALUES(?,?,?,?,?,?,'plain','body','[]','2026-10-25T00:00:00Z','2026-10-25T00:00:00Z')`
	must(insert, messageA, "a", int64(1), int64(1), Application, int64(1))
	// SQLite 正整数上界可保存，越界/小数落为 REAL 时被 typeof 拒绝。
	must(insert, messageB, "a", int64(math.MaxInt64), int64(math.MaxInt64), Administrator, int64(math.MaxInt64))
	for _, tc := range []struct {
		name                          string
		id, bot                       string
		seq, change, sender, revision any
	}{
		{"duplicate seq", messageC, "a", 1, 1, Application, 1},
		{"bad UUID", "bad", "a", 2, 1, Application, 1},
		{"missing bot", messageC, "absent", 2, 1, Application, 1},
		{"zero seq", messageC, "a", 0, 1, Application, 1},
		{"negative seq", messageC, "a", -1, 1, Application, 1},
		{"fraction seq", messageC, "a", 1.5, 1, Application, 1},
		{"overflow seq", messageC, "a", "9223372036854775808", 1, Application, 1},
		{"zero change", messageC, "a", 2, 0, Application, 1},
		{"fraction change", messageC, "a", 2, 1.5, Application, 1},
		{"overflow change", messageC, "a", 2, "9223372036854775808", Application, 1},
		{"bad sender", messageC, "a", 2, 1, "bot", 1},
		{"zero revision", messageC, "a", 2, 1, Application, 0},
		{"fraction revision", messageC, "a", 2, 1, Application, 1.5},
		{"overflow revision", messageC, "a", 2, 1, Application, "9223372036854775808"},
	} {
		t.Run(tc.name, func(t *testing.T) { reject(insert, tc.id, tc.bot, tc.seq, tc.change, tc.sender, tc.revision) })
	}
	must(insert, messageC, "b", 1, 1, Application, 1)
	reject("UPDATE messages SET reply_to_message_id=?,original_reply_to_message_id=? WHERE id=?", messageC, messageC, messageB)
	reject("UPDATE messages SET created_change_seq=2 WHERE id=?", messageA)
	reject("UPDATE messages SET format='html' WHERE id=?", messageA)
	reject("UPDATE messages SET buttons_json='{}' WHERE id=?", messageA)
	// 引用原值随插入确定，后续只允许清理当前可空外键。
	const reply = "00000000-0000-4000-8000-000000000004"
	reject(`INSERT INTO messages(id,bot_id,bot_seq,sender,revision,format,text,buttons_json,reply_to_message_id,original_reply_to_message_id,created_at,updated_at) VALUES(?,'a',2,'application',1,'plain','','[]',?,?,'now','now')`, reply, messageC, messageC)
	must(`INSERT INTO messages(id,bot_id,bot_seq,sender,revision,format,text,buttons_json,reply_to_message_id,original_reply_to_message_id,created_at,updated_at) VALUES(?,'a',2,'application',1,'plain','','[]',?,?,'now','now')`, reply, messageA, messageA)
	must("UPDATE messages SET created_change_seq=2 WHERE id=?", reply)
	reject("UPDATE messages SET created_change_seq=3 WHERE id=?", reply)
	reject("UPDATE messages SET original_reply_to_message_id=? WHERE id=?", messageC, reply)
	snapshot := fmt.Sprintf(`{"type":"message.created","message":{"id":%q,"bot_id":"a","bot_seq":"9223372036854775807","revision":1,"sender":"administrator","format":"plain","text":"snapshot","attachments":[],"reply_to_message_id":null,"buttons":[],"created_at":"2026-10-25T00:00:00Z","updated_at":"2026-10-25T00:00:00Z"}}`, messageB)
	delivery := "INSERT INTO deliveries(delivery_id,event_id,bot_id,payload_json,enqueue_seq,next_attempt_at) VALUES(?,?,?,?,?,?)"
	must(delivery, "d", "e", "a", snapshot, math.MaxInt64, "now")
	reject(delivery, "d2", "e", "a", snapshot, 1, "now")
	reject(delivery, "d2", "e2", "b", snapshot, 1, "now")
	reject(delivery, "d2", "e2", "missing", snapshot, 1, "now")
	reject(delivery, "d2", "e2", "a", "{}", 1, "now")
	reject(delivery, "d2", "e2", "a", snapshot, 0, "now")
	for _, column := range []string{"event_id", "payload_json", "bot_id", "delivery_id", "enqueue_seq"} {
		reject("UPDATE deliveries SET "+column+"=? WHERE delivery_id='d'", "changed")
	}
	for _, change := range []string{"state='leased'", "state_version=0", "attempt=-1", "retry_generation=-1", "state_version=1.5"} {
		reject("UPDATE deliveries SET " + change + " WHERE delivery_id='d'")
	}
	push := "INSERT INTO push_jobs(id,session_id,device_id,target_change_seq,next_attempt_at) VALUES(?,?,?,?,?)"
	must(push, "p", "s", "device", math.MaxInt64, "now")
	reject(push, "p2", "s", "wrong", 1, "now")
	reject(push, "p2", "missing", "device", 1, "now")
	reject(push, "p2", "s", "device", 0, "now")
	reject(push, "p2", "s", "device", "9223372036854775808", "now")
	must("UPDATE messages SET text='edited' WHERE id=?", messageB)
	must("DELETE FROM messages WHERE id=?", messageA)
	if err := s.Read(ctx, func(tx *sql.Tx) error {
		var current sql.NullString
		var original, payload, state string
		var version, attempt, generation int
		if err := tx.QueryRow("SELECT reply_to_message_id,original_reply_to_message_id FROM messages WHERE id=?", reply).Scan(&current, &original); err != nil {
			return err
		}
		if current.Valid || original != messageA {
			t.Fatal("reference cleanup lost original ID")
		}
		if err := tx.QueryRow("SELECT payload_json,state,state_version,attempt,retry_generation FROM deliveries WHERE delivery_id='d'").Scan(&payload, &state, &version, &attempt, &generation); err != nil {
			return err
		}
		if payload != snapshot || state != "pending" || version != 1 || attempt != 0 || generation != 0 {
			t.Fatal("snapshot/default state changed")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Read(ctx, func(tx *sql.Tx) error {
		for _, name := range []string{"messages_created_at", "messages_created_change_seq", "messages_reply_to", "deliveries_pending", "sessions_id_device", "push_jobs_pending", "push_jobs_session"} {
			var count int
			if err := tx.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='index' AND name=?", name).Scan(&count); err != nil {
				return err
			}
			if count != 1 {
				t.Fatalf("missing index %s", name)
			}
		}
		rows, err := tx.Query("PRAGMA foreign_key_check")
		if err != nil {
			return err
		}
		defer rows.Close()
		if rows.Next() {
			t.Fatal("foreign key violation")
		}
		return rows.Err()
	}); err != nil {
		t.Fatal(err)
	}
	must("DELETE FROM bots WHERE id='a'")
	if err := s.Read(ctx, func(tx *sql.Tx) error {
		for table, want := range map[string]int{"messages": 1, "deliveries": 0, "push_jobs": 1} {
			var count int
			if err := tx.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil {
				return err
			}
			if count != want {
				t.Fatalf("%s count=%d want=%d", table, count, want)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	must("DELETE FROM sessions WHERE id='s'")
	if err := s.Read(ctx, func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRow("SELECT count(*) FROM push_jobs").Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			t.Fatal("session delete left push job")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
