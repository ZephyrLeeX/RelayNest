package main

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// syncMergeRows 保留数据库全部26列，防止只检查公开 Job 投影遗漏平台/租约字段。
func syncMergeRows(t *testing.T, f *botHTTPFixture) map[string][]any {
	t.Helper()
	out := map[string][]any{}
	if err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
		r, e := tx.Query(`SELECT * FROM push_jobs ORDER BY id`)
		if e != nil {
			return e
		}
		defer r.Close()
		cols, e := r.Columns()
		if e != nil {
			return e
		}
		if len(cols) != 26 {
			t.Fatal("job schema changed", len(cols))
		}
		for r.Next() {
			v := make([]any, len(cols))
			p := make([]any, len(cols))
			for i := range v {
				p[i] = &v[i]
			}
			if e = r.Scan(p...); e != nil {
				return e
			}
			out[v[0].(string)] = v
		}
		return r.Err()
	}); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestAssembledSyncMergeProduction(t *testing.T) {
	for _, admin := range []bool{false, true} {
		t.Run(fmt.Sprint(admin), func(t *testing.T) {
			f := botHTTP(t, nil)
			bot := clientMessageBot(t, f)
			otherBot := clientMessageBot(t, f)
			syncCreateRegister(t, f, "synthetic-merge-secret", 7, 0)
			status, raw, h := syncCreateRequest(f, bot, admin, clientMessageKey)
			syncCreateReceipt(t, status, raw, h, admin)
			first := syncCreateJobs(t, f)
			if len(first) != 1 {
				t.Fatal("first create")
			}
			id := first[0].Snapshot.ID
			// 已经消耗过预算且处于退避；新事件仅推进水位。
			f.sql(`UPDATE push_jobs SET attempt=5,next_attempt_at=? WHERE id=?`, time.Now().Add(time.Hour).UTC().Format("2006-01-02T15:04:05.000000000Z"), id)
			original := syncMergeRows(t, f)
			for i, b := range []struct{ cross bool }{{false}, {true}} {
				chosen := bot
				if b.cross {
					chosen = otherBot
				}
				key := fmt.Sprintf("00000000-0000-4000-8000-%012d", 701+i)
				status, receipt, h := syncCreateRequest(f, chosen, admin, key)
				m := syncCreateReceipt(t, status, receipt, h, admin)
				var highest, created int64
				if err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
					if e := tx.QueryRow(`SELECT max(change_seq) FROM changes`).Scan(&highest); e != nil {
						return e
					}
					return tx.QueryRow(`SELECT created_change_seq FROM messages WHERE id=?`, m.ID).Scan(&created)
				}); err != nil {
					t.Fatal(err)
				}
				if admin && highest <= created {
					t.Fatal("administrator delivery watermark omitted")
				}
				want := append([]any(nil), original[id]...)
				want[3] = highest
				after := syncMergeRows(t, f)
				if len(after) != 1 || !reflect.DeepEqual(want, after[id]) {
					t.Fatal("same/cross bot merge changed original 26 columns", after, want)
				}
				state := clientMessageState(t, f.store)
				status, replay, h := syncCreateRequest(f, chosen, admin, key)
				syncCreateReceipt(t, status, replay, h, admin)
				if receipt != replay || state != clientMessageState(t, f.store) || !reflect.DeepEqual(after, syncMergeRows(t, f)) {
					t.Fatal("merged event replay mutated state")
				}
				// 同键异内容仍409，不可触发hook或推进target。
				path := "/api/v1/bot/messages"
				token := chosen.APIToken.Token
				if admin {
					path = "/api/v1/client/bots/" + chosen.Bot.ID + "/messages"
					token = f.android.Token
				}
				status, raw, h = f.request("POST", path, strings.Replace(clientMessageBody, "markdown", "plain", 1), "android", token, map[string]string{"Idempotency-Key": key})
				assertBotResponse(t, status, 409, raw, h)
				if state != clientMessageState(t, f.store) || !reflect.DeepEqual(after, syncMergeRows(t, f)) {
					t.Fatal("conflict mutated merge")
				}
			}
			// UPDATE之后receipt失败、真实deferred FK COMMIT失败均回滚消息/delivery/changes及旧target。
			for i, fault := range []string{
				`CREATE TRIGGER merge_fault BEFORE UPDATE OF target_change_seq ON push_jobs BEGIN SELECT RAISE(IGNORE);END`,
				`CREATE TRIGGER merge_fault BEFORE UPDATE OF target_change_seq ON push_jobs BEGIN SELECT RAISE(ABORT,'synthetic');END`,
				`CREATE TRIGGER merge_fault AFTER UPDATE OF target_change_seq ON push_jobs BEGIN UPDATE push_jobs SET attempt=attempt+1 WHERE id=NEW.id;END`,
				`CREATE TRIGGER merge_fault BEFORE INSERT ON idempotency BEGIN SELECT RAISE(ABORT,'synthetic');END`,
				`CREATE TABLE merge_deferred(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED);CREATE TRIGGER merge_fault AFTER UPDATE OF target_change_seq ON push_jobs BEGIN INSERT INTO merge_deferred VALUES('absent');END`,
			} {
				f.sql(fault)
				before := syncMergeRows(t, f)
				state := clientMessageState(t, f.store)
				status, raw, h := syncCreateRequest(f, bot, admin, fmt.Sprintf("00000000-0000-4000-8000-%012d", 801+i))
				assertBotResponse(t, status, 503, raw, h)
				if state != clientMessageState(t, f.store) || !reflect.DeepEqual(before, syncMergeRows(t, f)) {
					t.Fatal("merge fault committed partial transaction")
				}
				f.sql(`DROP TRIGGER merge_fault`)
			}
		})
	}
}
