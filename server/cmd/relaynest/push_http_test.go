package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/devices"
	"relaynest/server/internal/health"
	"relaynest/server/internal/httpapi"
	"relaynest/server/internal/storage"
)

const assembledPushPath = "/api/v1/devices/current/push"
const assembledNotificationsPath = "/api/v1/devices/current/notifications"

func assembledRegistration(token string, version, expected int64) string {
	secret, _ := json.Marshal(token)
	return fmt.Sprintf(`{"provider":"fcm","token":%s,"registration_version":%d,"expected_version":%d}`, secret, version, expected)
}

func assembledNotifications(expected int64, channel string) string {
	provider := `"fcm"`
	if channel == "vendor" {
		provider = `"opush"`
	} else if channel == "none" {
		provider = "null"
	}
	return fmt.Sprintf(`{"expected_version":%d,"enabled":true,"bot_overrides":[],"dnd":{"enabled":false,"timezone":"UTC","start":"22:00","end":"07:00"},"preview":false,"notify_channel":%q,"notify_provider":%s}`, expected, channel, provider)
}

func assertAssembledPush(t *testing.T, status, want int, raw string, headers http.Header, code string) map[string]json.RawMessage {
	t.Helper()
	assertBotResponse(t, status, want, raw, headers)
	if headers.Get("Set-Cookie") != "" || headers.Get("Content-Type") != "application/json" {
		t.Fatal("push response changed cookie or content type")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &object); err != nil {
		t.Fatal(err)
	}
	if code == "" {
		if _, hasError := object["error"]; hasError {
			t.Fatal("success has error")
		}
		return object
	}
	var envelope map[string]json.RawMessage
	if len(object) != 1 || json.Unmarshal(object["error"], &envelope) != nil {
		t.Fatal("failure published success data")
	}
	var got string
	if json.Unmarshal(envelope["code"], &got) != nil || got != code {
		t.Fatalf("error code=%s, want %s", got, code)
	}
	wantFields := 4
	if code == "SETTINGS_CONFLICT" || code == "SETTINGS_VERSION_EXHAUSTED" {
		wantFields = 6
	}
	if len(envelope) != wantFields {
		t.Fatal("unexpected public error fields")
	}
	return envelope
}

// 包含秘密的原始行只参与内存哈希；失败日志不会打印它们。
func assembledPushState(store *storage.Store) ([32]byte, error) {
	var state []any
	err := store.Read(context.Background(), func(tx *sql.Tx) error {
		for _, table := range []string{"administrator", "sessions", "instance_identity", "device_push_registrations", "device_notification_settings", "changes", "sqlite_sequence", "system_state"} {
			rows, err := tx.Query("SELECT * FROM " + table + " ORDER BY 1,2")
			if err != nil {
				return err
			}
			columns, err := rows.Columns()
			if err != nil {
				rows.Close()
				return err
			}
			state = append(state, table)
			for rows.Next() {
				values := make([]any, len(columns))
				args := make([]any, len(columns))
				for i := range values {
					args[i] = &values[i]
				}
				if err := rows.Scan(args...); err != nil {
					rows.Close()
					return err
				}
				state = append(state, values)
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
		return [32]byte{}, err
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(raw), nil
}

func mustAssembledPushState(t *testing.T, store *storage.Store) [32]byte {
	t.Helper()
	state, err := assembledPushState(store)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestAssembledPushLifecycleAndPreferenceIsolation(t *testing.T) {
	f := botHTTP(t, nil)
	status, raw, headers := f.admin("GET", assembledPushPath, "")
	assertAssembledPush(t, status, 200, raw, headers, "")
	var initial devices.PushRegistrationState
	if json.Unmarshal([]byte(raw), &initial) != nil || initial.SessionID != f.androidID || len(initial.Registrations) != 2 || initial.Registrations[0].Registered || initial.Registrations[1].Registered {
		t.Fatal("production main did not expose current-session providers")
	}
	const firstToken = "synthetic-production-first-token"
	const largeVersion int64 = 9007199254740993
	status, raw, headers = f.admin("PUT", assembledPushPath, assembledRegistration(firstToken, largeVersion, 0))
	assertAssembledPush(t, status, 200, raw, headers, "")
	var first devices.PushRegistrationState
	if json.Unmarshal([]byte(raw), &first) != nil || first.Registrations[0].RegistrationVersion != largeVersion || strings.Contains(raw, firstToken) || strings.Contains(raw, `"token"`) {
		t.Fatal("production state lost int64 precision or leaked token")
	}
	repo := devices.NewPushRepository(f.store, f.sessions, nil)
	targets, err := repo.ReadPushTargets(context.Background())
	if err != nil || len(targets) != 1 {
		t.Fatal("missing private send snapshot", err)
	}
	oldTarget := targets[0]
	status, raw, headers = f.admin("PUT", assembledPushPath, `{"provider":"opush","token":"synthetic-production-opush","registration_version":1,"expected_version":0}`)
	assertAssembledPush(t, status, 200, raw, headers, "")
	status, raw, headers = f.admin("PUT", assembledPushPath, assembledRegistration(firstToken, largeVersion, 0))
	assertAssembledPush(t, status, 200, raw, headers, "")
	var replay devices.PushRegistrationState
	if json.Unmarshal([]byte(raw), &replay) != nil || !replay.Registrations[0].UpdatedAt.Equal(*first.Registrations[0].UpdatedAt) || !replay.Registrations[1].Registered {
		t.Fatal("real HTTP replay changed timestamp/other provider")
	}
	_, second, err := f.sessions.Create(context.Background(), 1, auth.SessionInput{ClientType: "android", DeviceID: "00000000-0000-4000-8000-000000000001", DeviceName: "second-session"})
	if err != nil {
		t.Fatal(err)
	}
	status, raw, headers = f.request("GET", assembledPushPath, "", "android", second.Token, nil)
	assertAssembledPush(t, status, 200, raw, headers, "")
	var isolated devices.PushRegistrationState
	if json.Unmarshal([]byte(raw), &isolated) != nil || isolated.SessionID == f.androidID || isolated.Registrations[0].Registered || isolated.Registrations[1].Registered {
		t.Fatal("same-device registration was not session-isolated")
	}
	status, raw, headers = f.admin("GET", assembledNotificationsPath, "")
	assertAssembledPush(t, status, 200, raw, headers, "")
	var defaults devices.NotificationSettingsState
	if json.Unmarshal([]byte(raw), &defaults) != nil || defaults.Settings.SettingsVersion != 0 || defaults.Settings.UpdatedAt != nil || !defaults.Settings.Enabled || defaults.Settings.Preview || defaults.Settings.NotifyChannel != "fcm" {
		t.Fatal("invalid virtual settings")
	}
	var count int
	if err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
		return tx.QueryRow("SELECT count(*) FROM device_notification_settings").Scan(&count)
	}); err != nil || count != 0 {
		t.Fatal("GET persisted virtual settings", err)
	}
	status, raw, headers = f.admin("PUT", assembledNotificationsPath, assembledNotifications(0, "fcm"))
	assertAssembledPush(t, status, 200, raw, headers, "")
	status, raw, headers = f.request("GET", assembledNotificationsPath, "", "android", second.Token, nil)
	assertAssembledPush(t, status, 200, raw, headers, "")
	var shared devices.NotificationSettingsState
	if json.Unmarshal([]byte(raw), &shared) != nil || shared.Settings.SettingsVersion != 1 || shared.Settings.DeviceID != defaults.Settings.DeviceID {
		t.Fatal("same-device sessions did not share preferences")
	}
	before := mustAssembledPushState(t, f.store)
	status, raw, headers = f.request("PUT", assembledNotificationsPath, assembledNotifications(0, "fcm"), "android", second.Token, nil)
	e := assertAssembledPush(t, status, 409, raw, headers, "SETTINGS_CONFLICT")
	if string(e["latest_settings_version"]) != "1" || string(e["recovery_path"]) != `"/devices/current/notifications"` || mustAssembledPushState(t, f.store) != before {
		t.Fatal("settings conflict lost recovery metadata or changed state")
	}
	status, raw, headers = f.request("PUT", assembledNotificationsPath, assembledNotifications(1, "fcm"), "android", second.Token, nil)
	assertAssembledPush(t, status, 200, raw, headers, "")
	before = mustAssembledPushState(t, f.store)
	status, raw, headers = f.admin("PUT", assembledNotificationsPath, assembledNotifications(2, "vendor"))
	assertAssembledPush(t, status, 422, raw, headers, "PROVIDER_NOT_READY")
	if mustAssembledPushState(t, f.store) != before {
		t.Fatal("nil production vendor readiness did not fail atomically")
	}
	status, raw, headers = f.admin("PUT", assembledNotificationsPath, assembledNotifications(2, "none"))
	assertAssembledPush(t, status, 200, raw, headers, "")
	status, raw, headers = f.admin("PUT", assembledPushPath, assembledRegistration("synthetic-production-new-token", largeVersion+1, largeVersion))
	assertAssembledPush(t, status, 200, raw, headers, "")
	if changed, err := repo.ApplyPushResult(context.Background(), oldTarget, devices.PushTargetInvalid); err != nil || changed {
		t.Fatal("old send result erased a new registration", err)
	}
	status, raw, headers = f.admin("DELETE", assembledPushPath+"/fcm", fmt.Sprintf(`{"expected_version":%d}`, largeVersion+1))
	assertAssembledPush(t, status, 200, raw, headers, "")
	var tombstone devices.PushRegistrationState
	if json.Unmarshal([]byte(raw), &tombstone) != nil || tombstone.Registrations[0].Registered || tombstone.Registrations[0].RegistrationVersion != largeVersion+2 || !tombstone.Registrations[1].Registered {
		t.Fatal("production delete did not retain isolated tombstone")
	}
	before = mustAssembledPushState(t, f.store)
	status, raw, headers = f.admin("PUT", assembledPushPath, assembledRegistration(firstToken, largeVersion, largeVersion+2))
	assertAssembledPush(t, status, 409, raw, headers, "REGISTRATION_CONFLICT")
	if mustAssembledPushState(t, f.store) != before {
		t.Fatal("late HTTP registration revived the tombstone")
	}
	status, raw, headers = f.admin("PUT", assembledPushPath, assembledRegistration(firstToken, math.MaxInt64, largeVersion+2))
	assertAssembledPush(t, status, 200, raw, headers, "")
	before = mustAssembledPushState(t, f.store)
	status, raw, headers = f.admin("DELETE", assembledPushPath+"/fcm", fmt.Sprintf(`{"expected_version":%d}`, int64(math.MaxInt64)))
	assertAssembledPush(t, status, 409, raw, headers, "REGISTRATION_VERSION_EXHAUSTED")
	if mustAssembledPushState(t, f.store) != before {
		t.Fatal("registration exhaustion partially committed")
	}
	f.sql("UPDATE device_notification_settings SET settings_version=?", int64(math.MaxInt64))
	before = mustAssembledPushState(t, f.store)
	status, raw, headers = f.admin("PUT", assembledNotificationsPath, assembledNotifications(math.MaxInt64, "none"))
	e = assertAssembledPush(t, status, 409, raw, headers, "SETTINGS_VERSION_EXHAUSTED")
	if string(e["latest_settings_version"]) != "9223372036854775807" || mustAssembledPushState(t, f.store) != before {
		t.Fatal("settings exhaustion lost precision or partially committed")
	}
	status, raw, headers = f.admin("DELETE", "/api/v1/devices/"+f.androidID+"/session", "")
	if status != 204 || raw != "" || headers.Get("Cache-Control") != "no-store" {
		t.Fatal("current-session revocation did not commit")
	}
	before = mustAssembledPushState(t, f.store)
	status, raw, headers = f.admin("PUT", assembledPushPath, assembledRegistration("synthetic-late", 1, 0))
	assertAssembledPush(t, status, 401, raw, headers, "AUTHENTICATION_REQUIRED")
	if mustAssembledPushState(t, f.store) != before {
		t.Fatal("revoked-session late request changed state")
	}
}

func TestAssembledPushStrictRequestsAndInstanceSwitch(t *testing.T) {
	f := botHTTP(t, nil)
	for _, operation := range []struct{ method, path, body string }{
		{"GET", assembledPushPath, ""}, {"PUT", assembledPushPath, assembledRegistration("synthetic", 1, 0)},
		{"DELETE", assembledPushPath + "/fcm", `{"expected_version":0}`},
		{"GET", assembledNotificationsPath, ""}, {"PUT", assembledNotificationsPath, assembledNotifications(0, "fcm")},
	} {
		for _, duplicate := range []bool{false, true} {
			before := mustAssembledPushState(t, f.store)
			req, err := http.NewRequest(operation.method, f.server.URL+operation.path, strings.NewReader(operation.body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+f.android.Token)
			wantCode := "INSTANCE_REQUIRED"
			if duplicate {
				req.Header.Add("X-RelayNest-Instance-ID", f.instanceID)
				req.Header.Add("X-RelayNest-Instance-ID", f.instanceID)
				wantCode = "INVALID_REQUEST"
			}
			response, err := f.server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			assertAssembledPush(t, response.StatusCode, 400, string(body), response.Header, wantCode)
			if mustAssembledPushState(t, f.store) != before {
				t.Fatal("missing or duplicate instance header changed state")
			}
		}
		for _, tc := range []struct {
			instance, token, kind, code string
			status                      int
		}{
			{"", f.android.Token, "android", "INVALID_REQUEST", 400},
			{"bad", f.android.Token, "android", "INVALID_REQUEST", 400},
			{"11111111-1111-4111-8111-111111111111", "synthetic-bogus-token", "android", "INSTANCE_MISMATCH", 409},
			{f.instanceID, "synthetic-bogus-token", "android", "AUTHENTICATION_REQUIRED", 401},
			{f.instanceID, f.web.Token, "web", "FORBIDDEN", 403},
		} {
			before := mustAssembledPushState(t, f.store)
			status, raw, h := f.request(operation.method, operation.path, operation.body, tc.kind, tc.token, map[string]string{"X-RelayNest-Instance-ID": tc.instance, "Origin": f.server.URL, "X-CSRF-Token": f.web.CSRFToken})
			assertAssembledPush(t, status, tc.status, raw, h, tc.code)
			if mustAssembledPushState(t, f.store) != before || strings.Contains(raw, f.instanceID) {
				t.Fatal("rejected instance/client changed state or exposed identity")
			}
		}
	}
	for _, tc := range []struct{ path, body string }{
		{assembledPushPath, `{"provider":"fcm","provider":"opush","token":"synthetic","registration_version":1,"expected_version":0}`},
		{assembledPushPath, assembledRegistration("synthetic", 1, 0) + " {}"},
		{assembledPushPath, strings.Replace(assembledRegistration("synthetic", 1, 0), "synthetic", string([]byte{0xff}), 1)},
		{assembledPushPath + "?device_id=other", assembledRegistration("synthetic", 1, 0)},
		{assembledNotificationsPath, strings.Replace(assembledNotifications(0, "fcm"), `"timezone":"UTC"`, `"timezone":"UTC","observed_at":1`, 1)},
	} {
		before := mustAssembledPushState(t, f.store)
		status, raw, h := f.admin("PUT", tc.path, tc.body)
		assertAssembledPush(t, status, 400, raw, h, "INVALID_REQUEST")
		if mustAssembledPushState(t, f.store) != before {
			t.Fatal("strict production request failure changed state")
		}
	}
	before := mustAssembledPushState(t, f.store)
	status, raw, h := f.admin("PUT", assembledPushPath, assembledRegistration(strings.Repeat("x", 262144), 1, 0))
	assertAssembledPush(t, status, 413, raw, h, "PAYLOAD_TOO_LARGE")
	if mustAssembledPushState(t, f.store) != before {
		t.Fatal("oversized production body changed state")
	}
	before = mustAssembledPushState(t, f.store)
	missingBot := strings.Replace(assembledNotifications(0, "fcm"), `"bot_overrides":[]`, `"bot_overrides":[{"bot_id":"00000000-0000-4000-8000-000000000099","enabled":true}]`, 1)
	status, raw, h = f.admin("PUT", assembledNotificationsPath, missingBot)
	assertAssembledPush(t, status, 404, raw, h, "RESOURCE_NOT_FOUND")
	if mustAssembledPushState(t, f.store) != before {
		t.Fatal("missing Bot changed settings or renewed the session")
	}
	// 该库尚无注册；真实生产装配仍允许保存FCM偏好，不等待平台Adapter。
	status, raw, h = f.admin("PUT", assembledNotificationsPath, assembledNotifications(0, "fcm"))
	assertAssembledPush(t, status, 200, raw, h, "")
	// probe取得A后，切换实际生产依赖到B；旧A身份不得认证或执行B上的业务。
	probe, err := f.server.Client().Get(f.server.URL + "/api/v1/instance")
	if err != nil {
		t.Fatal(err)
	}
	var identity struct {
		InstanceID string `json:"instance_id"`
	}
	if json.NewDecoder(probe.Body).Decode(&identity) != nil || identity.InstanceID != f.instanceID {
		probe.Body.Close()
		t.Fatal("probe A identity")
	}
	probe.Body.Close()
	b, hB, err := authenticationDependencies(context.Background(), t.TempDir()+"/b.sqlite", f.server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	f.server.Config.Handler = httpapi.New(health.New(nil), hB)
	before = mustAssembledPushState(t, b)
	status, raw, h = f.admin("PUT", assembledPushPath, assembledRegistration("synthetic", 1, 0))
	assertAssembledPush(t, status, 409, raw, h, "INSTANCE_MISMATCH")
	if mustAssembledPushState(t, b) != before {
		t.Fatal("old probe identity executed on the replacement deployment")
	}
}

func TestMainProcessPushRouteAndSecretLogging(t *testing.T) {
	f := botHTTP(t, nil)
	// main以独立真实进程重新打开同一库；先关闭夹具连接与HTTP服务。
	f.server.Close()
	if err := f.store.Close(); err != nil {
		t.Fatal(err)
	}
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := reservation.Addr().String()
	reservation.Close()
	cmd := mainProcess("-listen", address, "-db", f.databasePath, "-external-origin", "https://relay.example", "-trusted-proxy", "127.0.0.1")
	var logs bytes.Buffer
	cmd.Stdout, cmd.Stderr = &logs, &logs
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	stopped := false
	defer func() {
		if !stopped {
			cmd.Process.Kill()
			<-done
		}
	}()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)
	for {
		response, err := client.Get("http://" + address + "/health/live")
		if err == nil {
			response.Body.Close()
			if response.StatusCode != 200 {
				t.Fatal("main process health route")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("main process did not listen")
		}
		time.Sleep(20 * time.Millisecond)
	}
	const secret = "synthetic-main-push-token-not-for-logs"
	for _, payload := range []struct {
		body string
		want int
		code string
	}{
		{assembledRegistration(secret, 1, 0), 200, ""},
		{strings.Replace(assembledRegistration(secret, 1, 0), `"provider":`, `"provider":"fcm","provider":`, 1), 400, "INVALID_REQUEST"},
	} {
		req, err := http.NewRequest("PUT", "http://"+address+assembledPushPath, strings.NewReader(payload.body))
		if err != nil {
			t.Fatal(err)
		}
		req.Host = "relay.example"
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", "relay.example")
		req.Header.Set("X-Forwarded-For", "192.0.2.10")
		req.Header.Set("X-RelayNest-Instance-ID", f.instanceID)
		req.Header.Set("Authorization", "Bearer "+f.android.Token)
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		assertAssembledPush(t, response.StatusCode, payload.want, string(raw), response.Header, payload.code)
		if strings.Contains(string(raw), secret) {
			t.Fatal("main process returned registration secret")
		}
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		stopped = true
		if err != nil {
			t.Fatal("main process shutdown", err)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("main process did not stop")
	}
	for _, value := range []string{secret, f.android.Token, f.databasePath} {
		if strings.Contains(logs.String(), value) {
			t.Fatal("main process logged sensitive value")
		}
	}
}

func TestAssembledPushTransactionsAndBinding(t *testing.T) {
	for _, tc := range []struct {
		name, statement, path, body, code string
		status                            int
	}{
		{"registration", `CREATE TRIGGER push_http_fault BEFORE INSERT ON device_push_registrations BEGIN SELECT RAISE(ABORT,'synthetic-private-diagnostic'); END`, assembledPushPath, assembledRegistration("synthetic", 1, 0), "SERVICE_UNAVAILABLE", 503},
		{"changes", `CREATE TRIGGER push_http_fault BEFORE INSERT ON changes BEGIN SELECT RAISE(ABORT,'synthetic-private-diagnostic'); END`, assembledNotificationsPath, assembledNotifications(0, "fcm"), "SERVICE_UNAVAILABLE", 503},
		{"commit", `CREATE TABLE deferred_http_fault(id TEXT REFERENCES sessions(id) DEFERRABLE INITIALLY DEFERRED)`, assembledNotificationsPath, assembledNotifications(0, "fcm"), "SERVICE_UNAVAILABLE", 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := botHTTP(t, nil)
			f.sql("UPDATE sessions SET idle_expires_at=? WHERE id=?", time.Now().Add(time.Hour).UnixNano(), f.androidID)
			f.sql(tc.statement)
			if tc.name == "commit" {
				f.sql(`CREATE TRIGGER push_http_fault AFTER INSERT ON device_notification_settings BEGIN INSERT INTO deferred_http_fault VALUES('missing-session'); END`)
			}
			before := mustAssembledPushState(t, f.store)
			status, raw, h := f.admin("PUT", tc.path, tc.body)
			assertAssembledPush(t, status, tc.status, raw, h, tc.code)
			if strings.Contains(raw, "synthetic-private") || mustAssembledPushState(t, f.store) != before {
				t.Fatal("failed transaction exposed internals or committed partial state")
			}
			if tc.name == "commit" {
				var count int
				if err := f.store.Read(context.Background(), func(tx *sql.Tx) error { return tx.QueryRow("SELECT count(*) FROM deferred_http_fault").Scan(&count) }); err != nil || count != 0 {
					t.Fatal("failed COMMIT retained side effects", err)
				}
			}
		})
	}
	for _, operation := range []struct{ method, path, body string }{
		{"GET", assembledPushPath, ""}, {"PUT", assembledPushPath, assembledRegistration("synthetic-new", 2, 1)},
		{"DELETE", assembledPushPath + "/fcm", `{"expected_version":1}`},
		{"GET", assembledNotificationsPath, ""}, {"PUT", assembledNotificationsPath, assembledNotifications(0, "fcm")},
	} {
		for _, mutation := range []struct {
			name, sql, code string
			status          int
		}{
			{"generation", "UPDATE administrator SET auth_generation=auth_generation+1", "AUTHENTICATION_REQUIRED", 401},
			{"instance", "UPDATE instance_identity SET instance_id='22222222-2222-4222-8222-222222222222'", "INSTANCE_MISMATCH", 409},
		} {
			t.Run(operation.method+operation.path+mutation.name, func(t *testing.T) {
				f := botHTTP(t, nil)
				status, raw, h := f.admin("PUT", assembledPushPath, assembledRegistration("synthetic-existing", 1, 0))
				assertAssembledPush(t, status, 200, raw, h, "")
				f.sql("CREATE TRIGGER push_http_fault AFTER UPDATE OF idle_expires_at ON sessions BEGIN " + mutation.sql + "; END")
				before := mustAssembledPushState(t, f.store)
				status, raw, h = f.admin(operation.method, operation.path, operation.body)
				assertAssembledPush(t, status, mutation.status, raw, h, mutation.code)
				if mustAssembledPushState(t, f.store) != before {
					t.Fatal("late binding failure committed business data or renewal")
				}
			})
		}
	}
}

type assembledChangingBody struct {
	io.ReadCloser
	before func() error
	done   bool
}

func (b *assembledChangingBody) Read(p []byte) (int, error) {
	if !b.done {
		b.done = true
		if err := b.before(); err != nil {
			return 0, err
		}
	}
	return b.ReadCloser.Read(p)
}

func TestAssembledPushAuthorizationBecomesStale(t *testing.T) {
	for _, tc := range []struct {
		name, statement, code string
		status                int
	}{
		{"session", "UPDATE sessions SET revoked=1 WHERE client_type='android'", "AUTHENTICATION_REQUIRED", 401},
		{"idle", "UPDATE sessions SET idle_expires_at=1 WHERE client_type='android'", "AUTHENTICATION_REQUIRED", 401},
		{"absolute", "UPDATE sessions SET idle_expires_at=1,absolute_expires_at=1 WHERE client_type='android'", "AUTHENTICATION_REQUIRED", 401},
		{"generation", "UPDATE administrator SET auth_generation=auth_generation+1", "AUTHENTICATION_REQUIRED", 401},
		{"instance", "UPDATE instance_identity SET instance_id='22222222-2222-4222-8222-222222222222'", "INSTANCE_UNAVAILABLE", 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := botHTTP(t, nil)
			production := f.server.Config.Handler
			type observed struct {
				store *storage.Store
				state [32]byte
				err   error
			}
			observations := make(chan observed, 1)
			f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.Body = &assembledChangingBody{ReadCloser: r.Body, before: func() error {
					err := f.store.Write(r.Context(), func(tx *sql.Tx) error { _, err := tx.Exec(tc.statement); return err })
					observer := f.store
					if err == nil && tc.name == "instance" {
						// 新Store只观测外部提交后的状态，生产装配及旧身份绑定不变。
						observer, err = storage.Open(r.Context(), storage.Config{Path: f.databasePath})
					}
					var state [32]byte
					if err == nil {
						state, err = assembledPushState(observer)
					}
					observations <- observed{observer, state, err}
					return err
				}}
				production.ServeHTTP(w, r)
			})
			status, raw, h := f.admin("PUT", assembledPushPath, assembledRegistration("synthetic-no-commit", 1, 0))
			assertAssembledPush(t, status, tc.status, raw, h, tc.code)
			select {
			case observation := <-observations:
				if observation.store != f.store {
					defer observation.store.Close()
				}
				if observation.err != nil || observation.state != mustAssembledPushState(t, observation.store) {
					t.Fatal("stale actor wrote after the real request authorization boundary", observation.err)
				}
			default:
				t.Fatal("request did not reach the body boundary after authorization")
			}
		})
	}
}

func TestAssembledPushRejectsValidBotToken(t *testing.T) {
	f := botHTTP(t, nil)
	// 从真实管理员管理入口取得新签发的有效凭据，不能用任意字符串冒称Bot身份。
	status, raw, headers := f.admin("POST", "/api/v1/admin/bots", botCreateBody)
	assertBotResponse(t, status, 201, raw, headers)
	created := decodeBotCreated(t, raw)
	if created.APIToken.Token == "" {
		t.Fatal("production Bot create did not issue an API credential")
	}
	// 先经正式Bot消息路由完成正对照，证明凭据当前有效而非未知/失效Token。
	status, raw, headers = f.request("POST", "/api/v1/bot/messages", `{"format":"plain","text":"Bot credential positive control"}`, "bot", created.APIToken.Token, map[string]string{"Idempotency-Key": "00000000-0000-4000-8000-000000000137"})
	assertBotResponse(t, status, 201, raw, headers)
	var receipt struct {
		BotID  string `json:"bot_id"`
		Sender string `json:"sender"`
	}
	if json.Unmarshal([]byte(raw), &receipt) != nil || receipt.BotID != created.Bot.ID || receipt.Sender != "application" || strings.Contains(raw, created.APIToken.Token) {
		t.Fatal("valid Bot credential positive control did not commit the expected message")
	}
	var messages, deliveries, idempotency int
	if err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
		return tx.QueryRow("SELECT (SELECT count(*) FROM messages),(SELECT count(*) FROM deliveries),(SELECT count(*) FROM idempotency)").Scan(&messages, &deliveries, &idempotency)
	}); err != nil || messages != 1 || deliveries != 0 || idempotency != 1 {
		t.Fatal("positive control did not finish its legitimate persistent effects", err)
	}
	// application消息不排管理员发往Bot的delivery；随后必须连这个空队列也保持不变。
	// 正对照完成后才取基线，覆盖消息/投递/待办/幂等、Bot凭据及会话和设备偏好。
	state := func() [3][32]byte {
		return [3][32]byte{mustAssembledPushState(t, f.store), clientMessageState(t, f.store), f.snapshot()}
	}
	for _, operation := range []struct{ method, path, body string }{
		{"GET", assembledPushPath, ""},
		{"PUT", assembledPushPath, assembledRegistration("synthetic-device-registration", 1, 0)},
		{"DELETE", assembledPushPath + "/fcm", `{"expected_version":0}`},
		{"GET", assembledNotificationsPath, ""},
		{"PUT", assembledNotificationsPath, assembledNotifications(0, "fcm")},
	} {
		t.Run(operation.method+operation.path, func(t *testing.T) {
			before := state()
			status, raw, headers := f.request(operation.method, operation.path, operation.body, "bot", created.APIToken.Token, nil)
			assertAssembledPush(t, status, 401, raw, headers, "AUTHENTICATION_REQUIRED")
			if strings.Contains(raw, created.APIToken.Token) || state() != before {
				t.Fatal("valid Bot token crossed into current-device authorization or changed persistent state")
			}
		})
	}
}
