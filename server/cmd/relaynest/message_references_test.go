package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/bots"
	"relaynest/server/internal/health"
	"relaynest/server/internal/httpapi"
	"relaynest/server/internal/messages"
	"relaynest/server/internal/storage"
)

const referenceKey = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
const referenceMissing = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"

func referenceBody(id string) string {
	return fmt.Sprintf(`{"format":"markdown","text":"引用正文 α","reply_to_message_id":%q}`, id)
}
func referenceRequest(f *botHTTPFixture, c bots.Created, admin bool, key, body string) (int, string, http.Header) {
	if admin {
		return clientMessageRequest(f, c.Bot.ID, key, body)
	}
	return f.request("POST", "/api/v1/bot/messages", body, "bot", c.APIToken.Token, map[string]string{"Idempotency-Key": key})
}
func referenceSource(t *testing.T, f *botHTTPFixture, c bots.Created) string {
	t.Helper()
	status, raw, h := referenceRequest(f, c, false, clientMessageKey, clientMessageBody)
	assertBotResponse(t, status, 201, raw, h)
	var m struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(raw), &m); err != nil || m.ID == "" {
		t.Fatal("source receipt", err)
	}
	return m.ID
}
func referenceCounters(t *testing.T, f *botHTTPFixture) []int64 {
	t.Helper()
	var out []int64
	if err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
		for _, q := range []string{"SELECT sum(bot_seq) FROM bots", "SELECT count(*) FROM messages", "SELECT count(*) FROM deliveries", "SELECT count(DISTINCT event_id) FROM deliveries", "SELECT count(*) FROM changes", "SELECT count(*) FROM push_jobs", "SELECT count(*) FROM idempotency", "SELECT coalesce(max(change_seq),0) FROM changes"} {
			var n int64
			if err := tx.QueryRow(q).Scan(&n); err != nil {
				return err
			}
			out = append(out, n)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}
func referenceAssertDelta(t *testing.T, before, after, want []int64) {
	t.Helper()
	for i := range want {
		if after[i]-before[i] != want[i] {
			t.Fatalf("counter %d delta %d want %d", i, after[i]-before[i], want[i])
		}
	}
}
func referenceMessage(t *testing.T, raw string, admin bool) struct{ ID, Reply string } {
	t.Helper()
	var m struct {
		ID    string `json:"id"`
		Reply string `json:"reply_to_message_id"`
	}
	if admin {
		var envelope struct {
			Message json.RawMessage `json:"message"`
		}
		if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
			t.Fatal(err)
		}
		raw = string(envelope.Message)
	}
	if err := json.Unmarshal([]byte(raw), &m); err != nil || m.ID == "" {
		t.Fatal("reply receipt", err)
	}
	return struct{ ID, Reply string }{m.ID, m.Reply}
}

func TestMessageReferencesProductionRecovery(t *testing.T) {
	for _, admin := range []bool{false, true} {
		t.Run(fmt.Sprint(admin), func(t *testing.T) {
			f := botHTTP(t, nil)
			c := clientMessageBot(t, f)
			source := referenceSource(t, f, c)
			body := referenceBody(strings.ToUpper(source))
			before := referenceCounters(t, f)
			status, receipt, h := referenceRequest(f, c, admin, referenceKey, body)
			assertBotResponse(t, status, 201, receipt, h)
			want := []int64{1, 1, 0, 0, 1, 0, 1, 1}
			if admin {
				want = []int64{1, 1, 1, 1, 2, 0, 1, 2}
			}
			referenceAssertDelta(t, before, referenceCounters(t, f), want)
			m := referenceMessage(t, receipt, admin)
			if m.Reply != source {
				t.Fatal("receipt missing original reference")
			}
			var eventID, payload string
			if err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
				var live, original string
				if err := tx.QueryRow("SELECT reply_to_message_id,original_reply_to_message_id FROM messages WHERE id=?", m.ID).Scan(&live, &original); err != nil {
					return err
				}
				if live != source || original != source {
					t.Fatal("dual reference not persisted")
				}
				if admin {
					if err := tx.QueryRow("SELECT event_id,payload_json FROM deliveries WHERE json_extract(payload_json,'$.message.id')=?", m.ID).Scan(&eventID, &payload); err != nil {
						return err
					}
					var event struct {
						Type    string `json:"type"`
						Message struct {
							Reply string `json:"reply_to_message_id"`
						} `json:"message"`
					}
					if err := json.Unmarshal([]byte(payload), &event); err != nil {
						return err
					}
					if event.Type != "message.created" || event.Message.Reply != source {
						t.Fatal("delivery reference mismatch")
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			state := clientMessageState(t, f.store)
			status, raw, h := referenceRequest(f, c, admin, referenceKey, body)
			assertBotResponse(t, status, 201, raw, h)
			if raw != receipt || state != clientMessageState(t, f.store) {
				t.Fatal("replay changed receipt/state")
			}
			f.sql("DELETE FROM messages WHERE id='" + source + "'")
			if err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
				var live sql.NullString
				var original string
				if err := tx.QueryRow("SELECT reply_to_message_id,original_reply_to_message_id FROM messages WHERE id=?", m.ID).Scan(&live, &original); err != nil {
					return err
				}
				if live.Valid || original != source {
					t.Fatal("source deletion lost original")
				}
				if admin {
					var id, p string
					if err := tx.QueryRow("SELECT event_id,payload_json FROM deliveries WHERE json_extract(payload_json,'$.message.id')=?", m.ID).Scan(&id, &p); err != nil {
						return err
					}
					if id != eventID || p != payload {
						t.Fatal("event changed on deletion")
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			state = clientMessageState(t, f.store)
			for _, tc := range []struct {
				key, body string
				status    int
				code      string
			}{{referenceKey, body, 201, ""}, {referenceMissing, body, 404, "RESOURCE_NOT_FOUND"}, {referenceKey, referenceBody(referenceMissing), 409, "IDEMPOTENCY_CONFLICT"}} {
				status, raw, h := referenceRequest(f, c, admin, tc.key, tc.body)
				assertBotResponse(t, status, tc.status, raw, h)
				if tc.status == 201 && raw != receipt {
					t.Fatal("deleted source replay changed bytes")
				}
				if tc.code != "" && !strings.Contains(raw, `"code":"`+tc.code+`"`) {
					t.Fatal("wrong error", raw)
				}
				if state != clientMessageState(t, f.store) {
					t.Fatal("replay/rejection wrote state")
				}
			}
			// 重建生产装配，不能依赖旧仓储缓存恢复成功回执。
			f.server.CloseClientConnections()
			if err := f.store.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, handlers, err := authenticationDependencies(context.Background(), f.databasePath, f.server.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			f.store = reopened
			f.server.Config.Handler = httpapi.New(health.New(nil), handlers)
			state = clientMessageState(t, reopened)
			status, raw, h = referenceRequest(f, c, admin, referenceKey, body)
			assertBotResponse(t, status, 201, raw, h)
			if raw != receipt || state != clientMessageState(t, reopened) {
				t.Fatal("reopen lost original receipt")
			}
		})
	}
}

func TestMessageReferencesProductionValidation(t *testing.T) {
	for _, admin := range []bool{false, true} {
		t.Run(fmt.Sprint(admin), func(t *testing.T) {
			f := botHTTP(t, nil)
			c := clientMessageBot(t, f)
			foreign := clientMessageBot(t, f)
			foreignID := referenceSource(t, f, foreign)
			for _, tc := range []struct {
				body   string
				status int
			}{{referenceBody(foreignID), 404}, {referenceBody(referenceMissing), 404}, {referenceBody(strings.ToUpper(referenceMissing)), 404}, {referenceBody("bad"), 422}, {referenceBody(""), 422}, {referenceBody("dddddddd-dddd-1ddd-8ddd-dddddddddddd"), 422}, {`{"format":"plain","text":"x","reply_to_message_id":12}`, 400}, {`{"format":"plain","text":"x","reply_to_message_id":{}}`, 400}, {`{"format":"plain","text":"x","reply_to_message_id":[]}`, 400}, {`{"format":"plain","text":"x","attachments":[{"id":"dddddddd-dddd-4ddd-8ddd-dddddddddddd","kind":"file"}]}`, 422}, {`{"format":"plain","text":"x","buttons":[[{"id":"b","label":"x","url":"https://example.invalid"}]]}`, 422}} {
				state := clientMessageState(t, f.store)
				status, raw, h := referenceRequest(f, c, admin, referenceKey, tc.body)
				assertBotResponse(t, status, tc.status, raw, h)
				if tc.status == 404 && !strings.Contains(raw, `"code":"RESOURCE_NOT_FOUND"`) {
					t.Fatal(raw)
				}
				if state != clientMessageState(t, f.store) {
					t.Fatal("invalid reply wrote state")
				}
			}
			body := `{"format":"plain","text":"regression"}`
			status, receipt, h := referenceRequest(f, c, admin, referenceKey, body)
			assertBotResponse(t, status, 201, receipt, h)
			state := clientMessageState(t, f.store)
			status, raw, h := referenceRequest(f, c, admin, referenceKey, `{"format":"plain","text":"regression","reply_to_message_id":null}`)
			assertBotResponse(t, status, 201, raw, h)
			if raw != receipt || state != clientMessageState(t, f.store) {
				t.Fatal("omitted/null normalization changed")
			}
		})
	}
}

func TestMessageReferencesProductionRollback(t *testing.T) {
	faults := map[string]string{
		"sequence":    "CREATE TRIGGER fail BEFORE UPDATE OF bot_seq ON bots BEGIN SELECT RAISE(ABORT,'private');END",
		"message":     "CREATE TRIGGER fail AFTER INSERT ON messages BEGIN SELECT RAISE(ABORT,'private');END",
		"changes":     "CREATE TRIGGER fail AFTER INSERT ON changes BEGIN SELECT RAISE(ABORT,'private');END",
		"backfill":    "CREATE TRIGGER fail BEFORE UPDATE OF created_change_seq ON messages BEGIN SELECT RAISE(IGNORE);END",
		"idempotency": "CREATE TRIGGER fail AFTER INSERT ON idempotency BEGIN SELECT RAISE(ABORT,'private');END",
		"commit":      "CREATE TABLE deferred_check(id REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED);CREATE TRIGGER fail AFTER INSERT ON idempotency BEGIN INSERT INTO deferred_check VALUES('missing');END",
	}
	for _, admin := range []bool{false, true} {
		for name, ddl := range faults {
			t.Run(fmt.Sprintf("%v/%s", admin, name), func(t *testing.T) {
				f := botHTTP(t, nil)
				c := clientMessageBot(t, f)
				source := referenceSource(t, f, c)
				f.sql(ddl)
				state := clientMessageState(t, f.store)
				sessions := f.snapshot()
				status, raw, h := referenceRequest(f, c, admin, referenceKey, referenceBody(source))
				assertBotResponse(t, status, 503, raw, h)
				if strings.Contains(raw, "private") || strings.Contains(raw, "delivery_id") || strings.Contains(raw, "引用正文") || state != clientMessageState(t, f.store) || sessions != f.snapshot() {
					t.Fatal("fault leaked success or kept partial transaction")
				}
			})
		}
	}
	t.Run("administrator-delivery", func(t *testing.T) {
		f := botHTTP(t, nil)
		c := clientMessageBot(t, f)
		source := referenceSource(t, f, c)
		f.sql("CREATE TRIGGER fail AFTER INSERT ON deliveries BEGIN SELECT RAISE(ABORT,'private');END")
		before := clientMessageState(t, f.store)
		status, raw, h := referenceRequest(f, c, true, referenceKey, referenceBody(source))
		assertBotResponse(t, status, 503, raw, h)
		if before != clientMessageState(t, f.store) {
			t.Fatal("delivery failure kept writes")
		}
	})
}

func TestMessageReferencesProductionReplayAuthorization(t *testing.T) {
	for _, admin := range []bool{false, true} {
		cases := map[string]string{"disabled": "UPDATE bots SET enabled=0", "send": "UPDATE bots SET can_send=0"}
		if admin {
			cases = map[string]string{"disabled": "UPDATE bots SET enabled=0", "receive": "UPDATE bots SET can_receive=0", "revoked": "UPDATE sessions SET revoked=1 WHERE client_type='android'", "expired": "UPDATE sessions SET idle_expires_at=1 WHERE client_type='android'", "generation": "UPDATE administrator SET auth_generation=auth_generation+1"}
		}
		for name, ddl := range cases {
			t.Run(fmt.Sprintf("%v/%s", admin, name), func(t *testing.T) {
				f := botHTTP(t, nil)
				c := clientMessageBot(t, f)
				source := referenceSource(t, f, c)
				body := referenceBody(source)
				status, raw, h := referenceRequest(f, c, admin, referenceKey, body)
				assertBotResponse(t, status, 201, raw, h)
				f.sql(ddl)
				before := clientMessageState(t, f.store)
				sessions := f.snapshot()
				want := 403
				if name == "revoked" || name == "expired" || name == "generation" {
					want = 401
				}
				status, raw, h = referenceRequest(f, c, admin, referenceKey, body)
				assertBotResponse(t, status, want, raw, h)
				if strings.Contains(raw, "reply_to_message_id") || before != clientMessageState(t, f.store) || sessions != f.snapshot() {
					t.Fatal("unauthorized replay exposed receipt or wrote")
				}
			})
		}
	}
	t.Run("bot-rotation", func(t *testing.T) {
		f := botHTTP(t, nil)
		c := clientMessageBot(t, f)
		source := referenceSource(t, f, c)
		body := referenceBody(source)
		status, receipt, h := referenceRequest(f, c, false, referenceKey, body)
		assertBotResponse(t, status, 201, receipt, h)
		fresh, _, err := bots.New(f.store).RotateAPIToken(context.Background(), c.Bot.ID)
		if err != nil {
			t.Fatal(err)
		}
		before := clientMessageState(t, f.store)
		status, raw, h := referenceRequest(f, c, false, referenceKey, body)
		assertBotResponse(t, status, 401, raw, h)
		c.APIToken = fresh
		status, raw, h = referenceRequest(f, c, false, referenceKey, body)
		assertBotResponse(t, status, 201, raw, h)
		if raw != receipt || before != clientMessageState(t, f.store) {
			t.Fatal("rotation changed receipt/scope")
		}
	})
	t.Run("administrator-instance", func(t *testing.T) {
		f := botHTTP(t, nil)
		c := clientMessageBot(t, f)
		source := referenceSource(t, f, c)
		body := referenceBody(source)
		status, raw, h := referenceRequest(f, c, true, referenceKey, body)
		assertBotResponse(t, status, 201, raw, h)
		f.instanceID = referenceMissing
		before := clientMessageState(t, f.store)
		status, raw, h = referenceRequest(f, c, true, referenceKey, body)
		assertBotResponse(t, status, 409, raw, h)
		if !strings.Contains(raw, `"code":"INSTANCE_MISMATCH"`) || before != clientMessageState(t, f.store) {
			t.Fatal("instance binding bypassed")
		}
	})
}

func TestMessageReferencesProductionLostResponse(t *testing.T) {
	for _, admin := range []bool{false, true} {
		t.Run(fmt.Sprint(admin), func(t *testing.T) {
			f := botHTTP(t, nil)
			c := clientMessageBot(t, f)
			source := referenceSource(t, f, c)
			body := referenceBody(source)
			before := referenceCounters(t, f)
			transport := f.server.Client().Transport.(*http.Transport)
			conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", strings.TrimPrefix(f.server.URL, "https://"), transport.TLSClientConfig.Clone())
			if err != nil {
				t.Fatal(err)
			}
			path, token := "/api/v1/bot/messages", c.APIToken.Token
			if admin {
				path = "/api/v1/client/bots/" + c.Bot.ID + "/messages"
				token = f.android.Token
			}
			if _, err = fmt.Fprintf(conn, "POST %s HTTP/1.1\r\nHost: %s\r\nContent-Type: application/json\r\nAuthorization: Bearer %s\r\nX-RelayNest-Instance-ID: %s\r\nIdempotency-Key: %s\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", path, strings.TrimPrefix(f.server.URL, "https://"), token, f.instanceID, referenceKey, len(body), body); err != nil {
				conn.Close()
				t.Fatal(err)
			}
			deadline := time.Now().Add(5 * time.Second)
			for referenceCounters(t, f)[6] == before[6] {
				if time.Now().After(deadline) {
					conn.Close()
					t.Fatal("request did not commit")
				}
				time.Sleep(time.Millisecond)
			}
			conn.Close()
			want := []int64{1, 1, 0, 0, 1, 0, 1, 1}
			if admin {
				want = []int64{1, 1, 1, 1, 2, 0, 1, 2}
			}
			referenceAssertDelta(t, before, referenceCounters(t, f), want)
			state := clientMessageState(t, f.store)
			status, receipt, h := referenceRequest(f, c, admin, referenceKey, body)
			assertBotResponse(t, status, 201, receipt, h)
			status, replay, h := referenceRequest(f, c, admin, referenceKey, body)
			assertBotResponse(t, status, 201, replay, h)
			if receipt != replay || !reflect.DeepEqual(state, clientMessageState(t, f.store)) {
				t.Fatal("lost response recovery duplicated")
			}
		})
	}
}

// 保留真实 B01 能力，只在授权后、受控事务前使数据库身份失效。
func TestMessageReferencesAdministratorWriteBinding(t *testing.T) {
	for _, tc := range []struct {
		name, sql, code string
		status          int
	}{
		{"revoked", "UPDATE sessions SET revoked=1 WHERE client_type='android'", "AUTHENTICATION_REQUIRED", 401},
		{"expired", "UPDATE sessions SET idle_expires_at=1 WHERE client_type='android'", "AUTHENTICATION_REQUIRED", 401},
		{"generation", "UPDATE administrator SET auth_generation=auth_generation+1", "AUTHENTICATION_REQUIRED", 401},
		{"instance", "UPDATE instance_identity SET instance_id='22222222-2222-4222-8222-222222222222'", "INSTANCE_UNAVAILABLE", 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := botHTTP(t, nil)
			c := clientMessageBot(t, f)
			source := referenceSource(t, f, c)
			body := referenceBody(source)
			status, raw, h := referenceRequest(f, c, true, referenceKey, body)
			assertBotResponse(t, status, 201, raw, h)
			policy, err := httpapi.NewOriginPolicy(f.server.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			authorize := clientMessageAuthorization(f.sessions, auth.NewRequestSecurity(f.sessions, policy))
			type observation struct {
				store *storage.Store
				state [32]byte
			}
			observed := make(chan observation, 1)
			f.handler.MessageHandler = httpapi.NewMessageHandler(bots.New(f.store), messages.New(f.store), func(w http.ResponseWriter, r *http.Request, write bool) (httpapi.BotAccess, bool) {
				access, ok := authorize(w, r, write)
				if ok {
					original := access.Write
					access.Write = func(ctx context.Context, fn func(*sql.Tx) error) error {
						f.sql(tc.sql)
						store := f.store
						if tc.name == "instance" {
							var err error
							store, err = storage.Open(context.Background(), storage.Config{Path: f.databasePath})
							if err != nil {
								t.Fatal(err)
							}
							t.Cleanup(func() { store.Close() })
						}
						observed <- observation{store, clientMessageState(t, store)}
						return original(ctx, fn)
					}
				}
				return access, ok
			})
			status, raw, h = referenceRequest(f, c, true, referenceKey, body)
			assertBotResponse(t, status, tc.status, raw, h)
			select {
			case state := <-observed:
				if !strings.Contains(raw, `"code":"`+tc.code+`"`) || strings.Contains(raw, "reply_to_message_id") || state.state != clientMessageState(t, state.store) {
					t.Fatal("stale B01 authorization replayed receipt or wrote")
				}
			default:
				t.Fatal("did not reach B01 write boundary")
			}
		})
	}
}

func TestMessageReferencesProductionConcurrentKeys(t *testing.T) {
	for _, admin := range []bool{false, true} {
		for _, same := range []bool{false, true} {
			t.Run(fmt.Sprintf("admin=%v/same=%v", admin, same), func(t *testing.T) {
				f := botHTTP(t, nil)
				c := clientMessageBot(t, f)
				source := referenceSource(t, f, c)
				body := referenceBody(source)
				before := referenceCounters(t, f)
				type result struct {
					status int
					body   string
					header http.Header
				}
				results := make(chan result, 12)
				var wg sync.WaitGroup
				for n := 0; n < 12; n++ {
					wg.Add(1)
					go func(n int) {
						defer wg.Done()
						key := referenceKey
						if !same {
							key = fmt.Sprintf("11111111-1111-4111-8111-%012d", n)
						}
						status, raw, h := referenceRequest(f, c, admin, key, body)
						results <- result{status, raw, h}
					}(n)
				}
				wg.Wait()
				close(results)
				ids := map[string]bool{}
				var first string
				for result := range results {
					assertBotResponse(t, result.status, 201, result.body, result.header)
					m := referenceMessage(t, result.body, admin)
					if m.Reply != source {
						t.Fatal("concurrent receipt lost reference")
					}
					ids[m.ID] = true
					if first == "" {
						first = result.body
					}
					if same && first != result.body {
						t.Fatal("same key changed receipt bytes")
					}
				}
				copies := int64(12)
				if same {
					copies = 1
				}
				if int64(len(ids)) != copies {
					t.Fatal("wrong distinct message count")
				}
				want := []int64{copies, copies, 0, 0, copies, 0, copies, copies}
				if admin {
					want = []int64{copies, copies, copies, copies, 2 * copies, 0, copies, 2 * copies}
				}
				referenceAssertDelta(t, before, referenceCounters(t, f), want)
			})
		}
	}
}
