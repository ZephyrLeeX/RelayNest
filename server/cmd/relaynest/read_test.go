package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/httpapi"
	"relaynest/server/internal/storage"
)

type assembledReadReceipt struct {
	Read struct {
		BotID   string `json:"bot_id"`
		Seq     string `json:"read_seq"`
		Version int64  `json:"version"`
		Updated string `json:"updated_at"`
	} `json:"read"`
	ServerTime string `json:"server_time"`
}

func readPath(bot string) string { return "/api/v1/client/bots/" + bot + "/read" }
func readPut(f *botHTTPFixture, bot string, seq int64) (int, string, http.Header) {
	return f.admin("PUT", readPath(bot), fmt.Sprintf(`{"through_seq":"%d"}`, seq))
}
func assertReadReceipt(t *testing.T, bot string, status int, raw string, h http.Header) assembledReadReceipt {
	t.Helper()
	if status != 200 || h.Get("Cache-Control") != "no-store" || h.Get("Content-Type") != "application/json" {
		t.Fatalf("read response %d %s", status, raw)
	}
	var out assembledReadReceipt
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(raw), &fields); err != nil || len(fields) != 2 || fields["read"] == nil || fields["server_time"] == nil {
		t.Fatal(raw)
	}
	var read map[string]json.RawMessage
	if err := json.Unmarshal(fields["read"], &read); err != nil || len(read) != 4 || read["bot_id"] == nil || read["read_seq"] == nil || read["version"] == nil || read["updated_at"] == nil {
		t.Fatal(raw)
	}
	n, err := strconv.ParseInt(out.Read.Seq, 10, 64)
	if err != nil || n < 0 || strconv.FormatInt(n, 10) != out.Read.Seq || out.Read.BotID != bot || out.Read.Version < 1 || len(read["version"]) == 0 || read["version"][0] == '"' {
		t.Fatal(raw)
	}
	for _, stamp := range []string{out.Read.Updated, out.ServerTime} {
		if _, err := time.Parse(time.RFC3339Nano, stamp); err != nil || !strings.HasSuffix(stamp, "Z") {
			t.Fatal(raw)
		}
	}
	return out
}

// 另开合法 Store 观察完整状态，包含共享水位、事件、会话续期及故障表，避免生产绑定掩盖回滚。
func readSnapshot(t *testing.T, f *botHTTPFixture) [32]byte {
	t.Helper()
	observer, err := storage.Open(context.Background(), storage.Config{Path: f.databasePath})
	if err != nil {
		t.Fatal(err)
	}
	defer observer.Close()
	var all []any
	err = observer.Read(context.Background(), func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
		if err != nil {
			return err
		}
		var names []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				rows.Close()
				return err
			}
			names = append(names, name)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, name := range names {
			rows, err := tx.Query(`SELECT * FROM "` + strings.ReplaceAll(name, `"`, `""`) + `" ORDER BY 1`)
			if err != nil {
				return err
			}
			cols, err := rows.Columns()
			if err != nil {
				rows.Close()
				return err
			}
			for rows.Next() {
				v := make([]any, len(cols))
				p := make([]any, len(cols))
				for i := range v {
					p[i] = &v[i]
				}
				if err := rows.Scan(p...); err != nil {
					rows.Close()
					return err
				}
				all = append(all, []any{name, v})
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(all)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(data)
}
func readFailure(t *testing.T, f *botHTTPFixture, bot, body string, want int, code string) {
	t.Helper()
	before := readSnapshot(t, f)
	status, raw, h := f.admin("PUT", readPath(bot), body)
	if status != want || !strings.Contains(raw, code) || h.Get("Cache-Control") != "no-store" || strings.Contains(raw, `"read"`) || strings.Contains(raw, "SELECT") || readSnapshot(t, f) != before {
		t.Fatalf("failure %d %s", status, raw)
	}
}
func TestAssembledReadSharedMax(t *testing.T) {
	f := botHTTP(t, nil)
	bot := clientMessageBot(t, f).Bot.ID
	f.sql("UPDATE bots SET bot_seq=12 WHERE id=?", bot)
	status, raw, h := readPut(f, bot, 0)
	initial := assertReadReceipt(t, bot, status, raw, h)
	if initial.Read.Seq != "0" || initial.Read.Version != 1 {
		t.Fatal(initial)
	}
	a, err := auth.ReadAdministrator(context.Background(), f.store)
	if err != nil {
		t.Fatal(err)
	}
	_, second, err := f.sessions.Create(context.Background(), a.AuthGeneration, auth.SessionInput{ClientType: "android", DeviceID: "00000000-0000-4000-8000-000000000002", DeviceName: "另一设备"})
	if err != nil {
		t.Fatal(err)
	}
	status, raw, h = f.request("PUT", readPath(bot), `{"through_seq":"4"}`, "android", second.Token, nil)
	advanced := assertReadReceipt(t, bot, status, raw, h)
	status, raw, h = readPut(f, bot, 2)
	old := assertReadReceipt(t, bot, status, raw, h)
	if old.Read != advanced.Read {
		t.Fatal("旧设备回放改变水位", old, advanced)
	}
	// 两个真实设备并发合并；每次成功响应不能落后于本请求水位。
	var wg sync.WaitGroup
	start := make(chan struct{})
	for n := int64(1); n <= 12; n++ {
		wg.Add(1)
		go func(n int64) {
			defer wg.Done()
			<-start
			token := f.android.Token
			if n%2 == 0 {
				token = second.Token
			}
			status, raw, h := f.request("PUT", readPath(bot), fmt.Sprintf(`{"through_seq":"%d"}`, n), "android", token, nil)
			got := assertReadReceipt(t, bot, status, raw, h)
			seq, _ := strconv.ParseInt(got.Read.Seq, 10, 64)
			if seq < n {
				t.Errorf("max failed %s", raw)
			}
		}(n)
	}
	close(start)
	wg.Wait()
	status, raw, h = readPut(f, bot, 0)
	final := assertReadReceipt(t, bot, status, raw, h)
	if final.Read.Seq != "12" || final.Read.Version < 3 {
		t.Fatal(final)
	}
	var events, version int64
	if err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
		if err := tx.QueryRow("SELECT count(*) FROM changes WHERE type='read.updated' AND entity_id=?", bot).Scan(&events); err != nil {
			return err
		}
		return tx.QueryRow("SELECT state_version FROM bot_read_states WHERE bot_id=?", bot).Scan(&version)
	}); err != nil {
		t.Fatal(err)
	}
	if events != version-1 {
		t.Fatalf("events=%d version=%d", events, version)
	}
	status, raw, h = readPut(f, bot, 12)
	same := assertReadReceipt(t, bot, status, raw, h)
	if same.Read != final.Read {
		t.Fatal("same changed state")
	}
}
func TestAssembledReadBoundaries(t *testing.T) {
	f := botHTTP(t, nil)
	bot := clientMessageBot(t, f).Bot.ID
	m := historyCreate(t, f, bot, "read-deleted-high", "removed", nil)
	f.sql("DELETE FROM messages WHERE id=?", m.ID)
	f.sql("UPDATE bots SET enabled=0 WHERE id=?", bot)
	status, raw, h := readPut(f, bot, 1)
	assertReadReceipt(t, bot, status, raw, h)
	readFailure(t, f, bot, `{"through_seq":"2"}`, 422, "READ_SEQ_OUT_OF_RANGE")
	readFailure(t, f, "00000000-0000-4000-8000-000000000099", `{"through_seq":"0"}`, 404, "RESOURCE_NOT_FOUND")
	f.sql("UPDATE bots SET bot_seq=9223372036854775807 WHERE id=?", bot)
	status, raw, h = readPut(f, bot, 9223372036854775807)
	out := assertReadReceipt(t, bot, status, raw, h)
	if out.Read.Seq != "9223372036854775807" {
		t.Fatal(raw)
	}
	f.sql("UPDATE bot_read_states SET state_version=9223372036854775807 WHERE bot_id=?", bot)
	status, raw, h = readPut(f, bot, 0)
	old := assertReadReceipt(t, bot, status, raw, h)
	status, raw, h = readPut(f, bot, 9223372036854775807)
	same := assertReadReceipt(t, bot, status, raw, h)
	if old.Read != same.Read || same.Read.Version != 9223372036854775807 {
		t.Fatal(raw)
	}
}
func TestAssembledReadInvalidInputAndCredentials(t *testing.T) {
	f := botHTTP(t, nil)
	created := clientMessageBot(t, f)
	bot := created.Bot.ID
	for _, tc := range []struct {
		body   string
		status int
		code   string
	}{
		{`{}`, 422, "VALIDATION_FAILED"}, {`{"through_seq":null}`, 422, "VALIDATION_FAILED"}, {`{"through_seq":0}`, 400, "INVALID_REQUEST"},
		{`{"through_seq":"-1"}`, 422, "VALIDATION_FAILED"}, {`{"through_seq":"01"}`, 422, "VALIDATION_FAILED"}, {`{"through_seq":"1e0"}`, 422, "VALIDATION_FAILED"}, {`{"through_seq":"9223372036854775808"}`, 422, "VALIDATION_FAILED"},
		{`{"through_seq":"0","through_seq":"0"}`, 400, "INVALID_REQUEST"}, {`{"through_seq":"0","extra":1}`, 400, "INVALID_REQUEST"}, {`{"through_seq":"0"}null`, 400, "INVALID_REQUEST"}, {`[]`, 400, "INVALID_REQUEST"},
		{strings.Repeat(" ", 262144) + `{"through_seq":"0"}`, 413, "PAYLOAD_TOO_LARGE"},
	} {
		readFailure(t, f, bot, tc.body, tc.status, tc.code)
	}
	for _, identity := range []struct{ kind, token string }{{"web", f.web.Token}, {"android", f.web.Token}, {"android", created.APIToken.Token}, {"android", created.WebhookKey.Key}, {"android", ""}} {
		before := readSnapshot(t, f)
		status, raw, h := f.request("PUT", readPath(bot), `{"through_seq":"0"}`, identity.kind, identity.token, nil)
		if status != 401 || !strings.Contains(raw, "AUTHENTICATION_REQUIRED") || h.Get("Cache-Control") != "no-store" || before != readSnapshot(t, f) {
			t.Fatalf("credentials %d %s", status, raw)
		}
	}
	for _, headers := range []map[string]string{{"X-RelayNest-Instance-ID": ""}, {"X-RelayNest-Instance-ID": "00000000-0000-4000-8000-000000000099"}, {"Content-Type": "text/plain"}, {"Cookie": "relaynest_session=" + f.web.Token}} {
		before := readSnapshot(t, f)
		status, raw, _ := f.request("PUT", readPath(bot), `{"through_seq":"0"}`, "android", f.android.Token, headers)
		if status == 200 || before != readSnapshot(t, f) {
			t.Fatalf("headers %d %s", status, raw)
		}
	}
}

// 保留生产授权和真实事务，仅在明确的前置／最终复核边界注入状态变化。
func readIntercept(f *botHTTPFixture, before func(), after func(*sql.Tx) error) {
	policy, err := httpapi.NewOriginPolicy(f.server.URL, nil)
	if err != nil {
		f.t.Fatal(err)
	}
	authorize := clientMessageAuthorization(f.sessions, auth.NewRequestSecurity(f.sessions, policy))
	f.handler.ReadHandler = httpapi.NewReadHandler(func(w http.ResponseWriter, r *http.Request, write bool) (httpapi.BotAccess, bool) {
		if !write {
			f.t.Error("已读未使用写授权")
		}
		access, ok := authorize(w, r, write)
		if !ok {
			return access, false
		}
		doWrite := access.Write
		access.Write = func(ctx context.Context, fn func(*sql.Tx) error) error {
			if before != nil {
				before()
			}
			return doWrite(ctx, func(tx *sql.Tx) error {
				if err := fn(tx); err != nil {
					return err
				}
				if after != nil {
					return after(tx)
				}
				return nil
			})
		}
		return access, true
	})
}

// 全表内容仍由独立 observer 读取；另核 schema，防止共享夹具残留故障 trigger。
func readRollbackBaseline(t *testing.T, f *botHTTPFixture) [32]byte {
	t.Helper()
	data := readSnapshot(t, f)
	var schema []string
	if err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT type, name, tbl_name, COALESCE(sql,'') FROM sqlite_master ORDER BY type,name`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var kind, name, table, statement string
			if err := rows.Scan(&kind, &name, &table, &statement); err != nil {
				return err
			}
			schema = append(schema, kind, name, table, statement)
		}
		return rows.Err()
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(struct {
		Data   [32]byte
		Schema []string
	}{data, schema})
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(raw)
}

func TestAssembledReadAuthorizationRollback(t *testing.T) {
	// before 会提交身份变化，必须各用独立 Store；其余故障必须随被测事务回滚。
	shared := botHTTP(t, nil)
	sharedBot := clientMessageBot(t, shared).Bot.ID
	shared.sql("UPDATE bots SET bot_seq=1 WHERE id=?", sharedBot)
	shared.sql("UPDATE sessions SET idle_expires_at=? WHERE id=?", time.Now().Add(time.Hour).UnixNano(), shared.androidID)
	baseline := readRollbackBaseline(t, shared)
	originalReadHandler := shared.handler.ReadHandler
	for _, mutation := range []struct {
		name, query string
		status      int
		code        string
	}{
		{"revoked", "UPDATE sessions SET revoked=1 WHERE id=?", 401, "AUTHENTICATION_REQUIRED"},
		{"expired", "UPDATE sessions SET idle_expires_at=0 WHERE id=?", 401, "AUTHENTICATION_REQUIRED"},
		{"absolute-expired", "UPDATE sessions SET idle_expires_at=0,absolute_expires_at=0 WHERE id=?", 401, "AUTHENTICATION_REQUIRED"},
		{"generation", "UPDATE administrator SET auth_generation=auth_generation+1", 401, "AUTHENTICATION_REQUIRED"},
		{"session-generation", "UPDATE sessions SET auth_generation=auth_generation+1 WHERE id=?", 401, "AUTHENTICATION_REQUIRED"},
		{"device", "UPDATE sessions SET device_id='00000000-0000-4000-8000-000000000099' WHERE id=?", 401, "AUTHENTICATION_REQUIRED"},
		{"instance", "UPDATE instance_identity SET instance_id='00000000-0000-4000-8000-000000000099'", 409, "INSTANCE_MISMATCH"},
	} {
		for _, phase := range []string{"before", "final", "renewal"} {
			if !t.Run(mutation.name+"/"+phase, func(t *testing.T) {
				var f *botHTTPFixture
				bot := sharedBot
				if phase == "before" {
					f = botHTTP(t, nil)
					bot = clientMessageBot(t, f).Bot.ID
					f.sql("UPDATE bots SET bot_seq=1 WHERE id=?", bot)
					f.sql("UPDATE sessions SET idle_expires_at=? WHERE id=?", time.Now().Add(time.Hour).UnixNano(), f.androidID)
				} else {
					local := *shared
					local.t = t
					f = &local
					// 清理在任何 Fatal 路径均执行；失败后父循环立即返回，不复用污染状态。
					t.Cleanup(func() {
						f.handler.ReadHandler = originalReadHandler
						f.afterAuthorize, f.beforeRead = nil, nil
						f.sessions.SetRevocationHook(nil)
						f.sql("DROP TRIGGER IF EXISTS read_renewal_fault")
						if readRollbackBaseline(t, f) != baseline {
							t.Fatal("共享已读夹具未恢复全表及 schema 基线")
						}
					})
					if readRollbackBaseline(t, f) != baseline {
						t.Fatal("共享已读夹具起始基线改变")
					}
					if session, _, err := f.sessions.Inspect(context.Background(), f.android.Token, "android"); err != nil || session.ID != f.androidID {
						t.Fatalf("共享会话无效: %v", err)
					}
				}
				args := []any{}
				if strings.Contains(mutation.query, "?") {
					args = append(args, f.androidID)
				}
				status, code := mutation.status, mutation.code
				var before [32]byte
				var reached atomic.Int32
				if phase == "before" {
					readIntercept(f, func() { f.sql(mutation.query, args...); before = readSnapshot(t, f) }, nil)
					if mutation.name == "instance" {
						status, code = 503, "INSTANCE_UNAVAILABLE"
					}
				} else {
					// 每例必须真正推进 0→1 并写事件；仅返回原错误不足以证明共享状态可达。
					readIntercept(f, nil, func(tx *sql.Tx) error {
						var seq, version, events int64
						if err := tx.QueryRow("SELECT read_seq,state_version FROM bot_read_states WHERE bot_id=?", bot).Scan(&seq, &version); err != nil {
							return err
						}
						if err := tx.QueryRow("SELECT count(*) FROM changes WHERE type='read.updated' AND entity_id=?", bot).Scan(&events); err != nil {
							return err
						}
						if seq != 1 || version != 2 || events != 1 {
							return fmt.Errorf("已读事务未推进: seq=%d version=%d events=%d", seq, version, events)
						}
						reached.Add(1)
						if phase == "final" {
							_, err := tx.Exec(mutation.query, args...)
							return err
						}
						return nil
					})
					if phase == "renewal" {
						// 真实续期 UPDATE 后再失效，验证最末次授权也会回滚全部写入。
						statement := strings.ReplaceAll(mutation.query, "?", "'"+f.androidID+"'")
						f.sql("CREATE TRIGGER read_renewal_fault AFTER UPDATE OF idle_expires_at ON sessions BEGIN " + statement + "; END")
					}
					before = readSnapshot(t, f)
				}
				got, raw, h := readPut(f, bot, 1)
				if phase != "before" && reached.Load() != 1 {
					t.Fatal("故障请求未到达真实已读写入后的复核边界")
				}
				if got != status || !strings.Contains(raw, code) || strings.Contains(raw, `"read"`) || h.Get("Cache-Control") != "no-store" || readSnapshot(t, f) != before {
					t.Fatalf("rollback %d %s", got, raw)
				}
			}) {
				return
			}
		}
	}
}
func TestAssembledReadStorageFailuresRollback(t *testing.T) {
	for _, fault := range []struct{ name, sql string }{
		{"missing", `DELETE FROM bot_read_states`},
		{"corrupt-time", `UPDATE bot_read_states SET updated_at='invalid'`},
		{"corrupt-seq", `PRAGMA ignore_check_constraints=ON; UPDATE bot_read_states SET read_seq=0.5; PRAGMA ignore_check_constraints=OFF`},
		{"corrupt-version", `PRAGMA ignore_check_constraints=ON; UPDATE bot_read_states SET state_version=1.5; PRAGMA ignore_check_constraints=OFF`},
		{"corrupt-upper", `PRAGMA ignore_check_constraints=ON; UPDATE bots SET bot_seq=1.5; PRAGMA ignore_check_constraints=OFF`},
		{"overflow", `UPDATE bot_read_states SET state_version=9223372036854775807`},
		{"read-sql", `CREATE TRIGGER read_fault BEFORE UPDATE ON bot_read_states BEGIN SELECT RAISE(ABORT,'private-read-fault');END`},
		{"changes", `CREATE TRIGGER read_fault BEFORE INSERT ON changes WHEN NEW.type='read.updated' BEGIN SELECT RAISE(ABORT,'private-read-fault');END`},
		{"commit", `CREATE TABLE read_deferred(id TEXT REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED);CREATE TRIGGER read_fault AFTER UPDATE ON bot_read_states BEGIN INSERT INTO read_deferred VALUES('absent');END`},
		{"renewal-sql", `CREATE TRIGGER read_fault BEFORE UPDATE OF idle_expires_at ON sessions BEGIN SELECT RAISE(ABORT,'private-read-fault');END`},
	} {
		t.Run(fault.name, func(t *testing.T) {
			f := botHTTP(t, nil)
			bot := clientMessageBot(t, f).Bot.ID
			f.sql("UPDATE bots SET bot_seq=1 WHERE id=?", bot)
			f.sql("UPDATE sessions SET idle_expires_at=? WHERE id=?", time.Now().Add(time.Hour).UnixNano(), f.androidID)
			f.sql(fault.sql)
			readFailure(t, f, bot, `{"through_seq":"1"}`, 503, "SERVICE_UNAVAILABLE")
		})
	}
}
