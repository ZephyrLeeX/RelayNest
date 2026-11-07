package alertoriginstorage_test

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"relaynest/server/internal/bots"
	"relaynest/server/internal/httpapi"
	"relaynest/server/internal/messages"
	"relaynest/server/internal/push"
)

// 此处只在测试中装配真实 Handler/repo first-create hook 和本地 TLS，
// 不代表当前 main、Webhook 或任何 provider 生产接线已完成。
func TestLocallyAssembledTLSIdempotencyAndCommitFailure(t *testing.T) {
	for _, fault := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "deferred-commit-fault"}[fault], func(t *testing.T) {
			f := setup(t)
			called := 0
			repo := f.repository(func(c context.Context, tx *sql.Tx, m messages.Created) error {
				called++
				return push.EnqueueAlertTx(c, tx, event(m))
			})
			handler := httpapi.NewMessageHandler(bots.New(f.store), repo)
			mux := http.NewServeMux()
			mux.HandleFunc("POST /api/v1/bot/messages", handler.CreateBotMessage)
			server := httptest.NewTLSServer(mux)
			defer server.Close()
			request := func(key string) (int, string) {
				t.Helper()
				req, err := http.NewRequest("POST", server.URL+"/api/v1/bot/messages", strings.NewReader(`{"format":"plain","text":"local TLS"}`))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Authorization", "Bearer "+f.bot.APIToken.Token)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Idempotency-Key", key)
				res, err := server.Client().Do(req)
				if err != nil {
					t.Fatal(err)
				}
				defer res.Body.Close()
				body, err := io.ReadAll(res.Body)
				if err != nil {
					t.Fatal(err)
				}
				return res.StatusCode, string(body)
			}
			before, prior := rows(t, f.store), state(t, f.store)
			if fault {
				exec(t, f.store, `CREATE TABLE deferred_fault(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED);CREATE TRIGGER fault AFTER INSERT ON push_jobs BEGIN INSERT INTO deferred_fault VALUES('absent');END`)
			}
			status, body := request("00000000-0000-4000-8000-000000000960")
			if fault {
				if status != 503 || strings.Contains(body, `"id"`) {
					t.Fatal("failed commit published", status, body)
				}
				unchanged(t, f, before, prior)
				return
			}
			if status != 201 || called != 1 || len(rows(t, f.store)) != 1 {
				t.Fatal("TLS creation failed", status, called, body)
			}
			before, prior = rows(t, f.store), state(t, f.store)
			exec(t, f.store, `UPDATE device_push_registrations SET registration_version=2;UPDATE device_notification_settings SET settings_version=2`)
			status, replay := request("00000000-0000-4000-8000-000000000960")
			if status != 201 || body != replay || called != 1 {
				t.Fatal("HTTP idem reentered hook", status, called)
			}
			unchanged(t, f, before, prior)
			if !reflect.DeepEqual(before, rows(t, f.store)) {
				t.Fatal("snapshot changed")
			}
		})
	}
}
