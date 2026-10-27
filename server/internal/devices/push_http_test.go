package devices

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/httpapi"
	"relaynest/server/internal/storage"
)

// 阶段1只验证真实身份与仓储的HTTP投影；正式router/main装配由阶段2另验。
type pushHTTPFixture struct {
	*pushFixture
	mux http.Handler
}

func newPushHTTPFixture(t *testing.T) *pushHTTPFixture {
	t.Helper()
	f := newPushFixture(t)
	policy, err := httpapi.NewOriginPolicy("https://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlerWithPush(f.sessions, auth.NewRequestSecurity(f.sessions, policy), f.repo)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/devices/current/push", h.GetCurrentPush)
	mux.HandleFunc("PUT /api/v1/devices/current/push", h.PutCurrentPush)
	mux.HandleFunc("DELETE /api/v1/devices/current/push/{provider}", h.DeleteCurrentPush)
	mux.HandleFunc("GET /api/v1/devices/current/notifications", h.GetCurrentNotifications)
	mux.HandleFunc("PUT /api/v1/devices/current/notifications", h.PutCurrentNotifications)
	return &pushHTTPFixture{f, mux}
}

func (f *pushHTTPFixture) request(method, path, body string, session int, customize func(*http.Request)) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "https://example.com/api/v1/devices/current/"+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-RelayNest-Instance-ID", f.instance)
	if session >= 0 {
		r.Header.Set("Authorization", "Bearer "+f.tokens[session].Token)
	}
	if customize != nil {
		customize(r)
	}
	w := httptest.NewRecorder()
	f.mux.ServeHTTP(w, r)
	return w
}

func registrationBody(provider, token string, version, expected int64) string {
	// 输入类型有主动脱敏的MarshalJSON，测试不得误用它来生成写入凭据。
	p, _ := json.Marshal(provider)
	secret, _ := json.Marshal(token)
	return fmt.Sprintf(`{"provider":%s,"token":%s,"registration_version":%d,"expected_version":%d}`, p, secret, version, expected)
}

func notificationBody(t *testing.T, in NotificationSettingsUpdate) string {
	t.Helper()
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func assertPushHTTP(t *testing.T, w *httptest.ResponseRecorder, status int, code string) map[string]json.RawMessage {
	t.Helper()
	if w.Code != status || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Content-Type") != "application/json" || w.Header().Get("Set-Cookie") != "" {
		t.Fatalf("HTTP status/headers=%d/%v, want status %d", w.Code, w.Header(), status)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &object); err != nil {
		t.Fatal("response is not JSON", err)
	}
	if code == "" {
		if _, exists := object["error"]; exists {
			t.Fatal("success contains error")
		}
		return object
	}
	if len(object) != 1 {
		t.Fatal("error response contains success fields")
	}
	var e map[string]json.RawMessage
	if err := json.Unmarshal(object["error"], &e); err != nil {
		t.Fatal(err)
	}
	var actualCode, message, requestID string
	var retryable bool
	if json.Unmarshal(e["code"], &actualCode) != nil || actualCode != code || json.Unmarshal(e["message"], &message) != nil || message == "" || json.Unmarshal(e["request_id"], &requestID) != nil || requestID == "" || json.Unmarshal(e["retryable"], &retryable) != nil || retryable != (status == 503) {
		t.Fatal("public error envelope mismatch")
	}
	wantFields := 4
	if code == "SETTINGS_CONFLICT" || code == "SETTINGS_VERSION_EXHAUSTED" {
		wantFields = 6
	}
	if len(e) != wantFields {
		t.Fatalf("public error fields=%d, want %d", len(e), wantFields)
	}
	return e
}

// 比较失败前后完整安全和业务状态，包括会话续期、变更序号及注册秘密。
// 快照只在内存比较，错误消息不输出其内容。
func pushHTTPSnapshot(t *testing.T, store *storage.Store) string {
	t.Helper()
	queries := []string{
		"SELECT * FROM administrator ORDER BY id",
		"SELECT * FROM sessions ORDER BY id",
		"SELECT * FROM instance_identity ORDER BY id",
		"SELECT * FROM device_push_registrations ORDER BY session_id,provider",
		"SELECT * FROM device_notification_settings ORDER BY device_id",
		"SELECT * FROM changes ORDER BY change_seq",
		"SELECT * FROM sqlite_sequence ORDER BY name",
		"SELECT * FROM system_state ORDER BY id",
	}
	var state [][][]any
	err := store.Read(t.Context(), func(tx *sql.Tx) error {
		for _, query := range queries {
			rows, err := tx.Query(query)
			if err != nil {
				return err
			}
			columns, err := rows.Columns()
			if err != nil {
				rows.Close()
				return err
			}
			var contents [][]any
			for rows.Next() {
				values := make([]any, len(columns))
				pointers := make([]any, len(columns))
				for i := range values {
					pointers[i] = &values[i]
				}
				if err := rows.Scan(pointers...); err != nil {
					rows.Close()
					return err
				}
				contents = append(contents, values)
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return err
			}
			state = append(state, contents)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestPushHTTPRegistrationLifecycle(t *testing.T) {
	f := newPushHTTPFixture(t)
	initial := f.request("GET", "push", "ignored", 0, nil)
	assertPushHTTP(t, initial, 200, "")
	var state PushRegistrationState
	if json.Unmarshal(initial.Body.Bytes(), &state) != nil || state.SessionID != f.actors[0].Session.ID || len(state.Registrations) != 2 {
		t.Fatal("missing current session registration state")
	}
	for _, registration := range state.Registrations {
		if registration.Registered || registration.RegistrationVersion != 0 || registration.UpdatedAt != nil {
			t.Fatal("missing provider is not version zero")
		}
	}
	if pushCount(t, f.pushFixture, "SELECT count(*) FROM device_push_registrations") != 0 {
		t.Fatal("GET created registration rows")
	}
	const token = "synthetic-private-registration"
	const largeVersion int64 = 9007199254740993
	first := f.request("PUT", "push", registrationBody("fcm", token, largeVersion, 0), 0, nil)
	assertPushHTTP(t, first, 200, "")
	if json.Unmarshal(first.Body.Bytes(), &state) != nil || state.Registrations[0].RegistrationVersion != largeVersion || !state.Registrations[0].Registered {
		t.Fatal("int64 registration version lost precision")
	}
	put := f.request("PUT", "push", registrationBody("opush", "synthetic-opush", 7, 0), 0, nil)
	assertPushHTTP(t, put, 200, "")
	replay := f.request("PUT", "push", registrationBody("fcm", token, largeVersion, 0), 0, nil)
	assertPushHTTP(t, replay, 200, "")
	var replayState PushRegistrationState
	if json.Unmarshal(replay.Body.Bytes(), &replayState) != nil || !reflect.DeepEqual(state.Registrations[0], replayState.Registrations[0]) || !replayState.Registrations[1].Registered {
		t.Fatal("replay changed time or another provider")
	}
	for _, response := range []*httptest.ResponseRecorder{first, put, replay} {
		if strings.Contains(response.Body.String(), token) || strings.Contains(response.Body.String(), `"token"`) {
			t.Fatal("registration secret in response")
		}
	}
	other := f.request("GET", "push", "", 1, nil)
	assertPushHTTP(t, other, 200, "")
	if json.Unmarshal(other.Body.Bytes(), &state) != nil || state.SessionID != f.actors[1].Session.ID || state.Registrations[0].Registered || state.Registrations[1].Registered {
		t.Fatal("registration crossed sessions sharing a device")
	}
	before := pushHTTPSnapshot(t, f.store)
	conflict := f.request("PUT", "push", registrationBody("fcm", "different", largeVersion, largeVersion), 0, nil)
	assertPushHTTP(t, conflict, 409, "REGISTRATION_CONFLICT")
	if pushHTTPSnapshot(t, f.store) != before {
		t.Fatal("conflict mutated registration or renewed session")
	}
	deleted := f.request("DELETE", "push/fcm", fmt.Sprintf(`{"expected_version":%d}`, largeVersion), 0, nil)
	assertPushHTTP(t, deleted, 200, "")
	if json.Unmarshal(deleted.Body.Bytes(), &state) != nil || state.Registrations[0].Registered || state.Registrations[0].RegistrationVersion != largeVersion+1 || !state.Registrations[1].Registered {
		t.Fatal("delete did not retain isolated tombstone")
	}
	before = pushHTTPSnapshot(t, f.store)
	assertPushHTTP(t, f.request("PUT", "push", registrationBody("fcm", token, largeVersion, largeVersion+1), 0, nil), 409, "REGISTRATION_CONFLICT")
	if pushHTTPSnapshot(t, f.store) != before {
		t.Fatal("late registration revived tombstone")
	}
	assertPushHTTP(t, f.request("PUT", "push", registrationBody("fcm", token, math.MaxInt64, largeVersion+1), 0, nil), 200, "")
	before = pushHTTPSnapshot(t, f.store)
	assertPushHTTP(t, f.request("DELETE", "push/fcm", fmt.Sprintf(`{"expected_version":%d}`, int64(math.MaxInt64)), 0, nil), 409, "REGISTRATION_VERSION_EXHAUSTED")
	if pushHTTPSnapshot(t, f.store) != before {
		t.Fatal("version exhaustion mutated state")
	}
}

func TestPushHTTPNotificationCASAndModes(t *testing.T) {
	f := newPushHTTPFixture(t)
	defaults := f.request("GET", "notifications", "", 0, nil)
	assertPushHTTP(t, defaults, 200, "")
	var initial NotificationSettingsState
	if json.Unmarshal(defaults.Body.Bytes(), &initial) != nil || !reflect.DeepEqual(initial.Settings, defaultNotifications(pushDevice)) || initial.ServerTime.IsZero() {
		t.Fatal("virtual notification defaults")
	}
	if pushCount(t, f.pushFixture, "SELECT count(*) FROM device_notification_settings") != 0 || pushCount(t, f.pushFixture, "SELECT count(*) FROM changes") != 0 {
		t.Fatal("default GET persisted configuration or changes")
	}
	in := notificationInput()
	in.Preview = true
	first := f.request("PUT", "notifications", notificationBody(t, in), 0, nil)
	assertPushHTTP(t, first, 200, "")
	var saved, shared NotificationSettingsState
	if json.Unmarshal(first.Body.Bytes(), &saved) != nil || saved.Settings.SettingsVersion != 1 || saved.Settings.UpdatedAt == nil || !saved.ServerTime.Equal(*saved.Settings.UpdatedAt) {
		t.Fatal("successful configuration did not publish committed version/time")
	}
	other := f.request("GET", "notifications", "", 1, nil)
	assertPushHTTP(t, other, 200, "")
	if json.Unmarshal(other.Body.Bytes(), &shared) != nil || !reflect.DeepEqual(saved.Settings, shared.Settings) {
		t.Fatal("same device sessions did not share settings")
	}
	before := pushHTTPSnapshot(t, f.store)
	e := assertPushHTTP(t, f.request("PUT", "notifications", notificationBody(t, in), 1, nil), 409, "SETTINGS_CONFLICT")
	var latest int64
	var recovery string
	if json.Unmarshal(e["latest_settings_version"], &latest) != nil || latest != 1 || json.Unmarshal(e["recovery_path"], &recovery) != nil || recovery != "/devices/current/notifications" || pushHTTPSnapshot(t, f.store) != before {
		t.Fatal("settings conflict projection or atomicity")
	}
	in.ExpectedVersion = 1
	assertPushHTTP(t, f.request("PUT", "notifications", notificationBody(t, in), 1, nil), 200, "")
	if pushCount(t, f.pushFixture, "SELECT settings_version FROM device_notification_settings") != 2 || pushCount(t, f.pushFixture, "SELECT count(*) FROM changes WHERE type='device.notifications.updated'") != 2 {
		t.Fatal("same-value replacement did not increment version/change")
	}
	// 保存FCM偏好不需要注册；登记opush也不能自动启用vendor模式。
	assertPushHTTP(t, f.request("PUT", "push", registrationBody("opush", "synthetic-opush", 1, 0), 0, nil), 200, "")
	provider := PushOPush
	in.ExpectedVersion, in.NotifyChannel, in.NotifyProvider = 2, "vendor", &provider
	before = pushHTTPSnapshot(t, f.store)
	assertPushHTTP(t, f.request("PUT", "notifications", notificationBody(t, in), 0, nil), 422, "PROVIDER_NOT_READY")
	if pushHTTPSnapshot(t, f.store) != before {
		t.Fatal("vendor readiness failure changed mode/registration")
	}
	in.NotifyChannel, in.NotifyProvider = "none", nil
	assertPushHTTP(t, f.request("PUT", "notifications", notificationBody(t, in), 0, nil), 200, "")
	if pushCount(t, f.pushFixture, "SELECT count(*) FROM device_push_registrations WHERE token IS NOT NULL") != 1 {
		t.Fatal("mode switch cleared independent registration")
	}
	mustPushSQL(t, f.pushFixture, "UPDATE device_notification_settings SET settings_version=?", int64(math.MaxInt64))
	in.ExpectedVersion = math.MaxInt64
	before = pushHTTPSnapshot(t, f.store)
	e = assertPushHTTP(t, f.request("PUT", "notifications", notificationBody(t, in), 0, nil), 409, "SETTINGS_VERSION_EXHAUSTED")
	if json.Unmarshal(e["latest_settings_version"], &latest) != nil || latest != math.MaxInt64 || pushHTTPSnapshot(t, f.store) != before {
		t.Fatal("exhaustion lost int64 or committed partial state")
	}
}

func TestPushHTTPStrictBodyAndSemanticErrors(t *testing.T) {
	f := newPushHTTPFixture(t)
	validPush := registrationBody("fcm", "synthetic-secret", 1, 0)
	validSettings := notificationBody(t, notificationInput())
	for _, tc := range []struct {
		name, method, path, body string
		status                   int
		code                     string
	}{
		{"empty", "PUT", "push", "", 400, "INVALID_REQUEST"},
		{"object-required", "PUT", "push", "[]", 400, "INVALID_REQUEST"},
		{"null-root", "PUT", "push", "null", 400, "INVALID_REQUEST"},
		{"missing", "PUT", "push", `{"provider":"fcm","token":"x","registration_version":1}`, 400, "INVALID_REQUEST"},
		{"duplicate", "PUT", "push", strings.Replace(validPush, `"provider":`, `"provider":"fcm","provider":`, 1), 400, "INVALID_REQUEST"},
		{"escaped-duplicate", "PUT", "push", strings.Replace(validPush, `"provider":`, `"pro\u0076ider":"fcm","provider":`, 1), 400, "INVALID_REQUEST"},
		{"case-alias", "PUT", "push", strings.Replace(validPush, "provider", "Provider", 1), 400, "INVALID_REQUEST"},
		{"selector-field", "PUT", "push", strings.Replace(validPush, "{", `{"session_id":"other",`, 1), 400, "INVALID_REQUEST"},
		{"null-token", "PUT", "push", strings.Replace(validPush, `"synthetic-secret"`, "null", 1), 400, "INVALID_REQUEST"},
		{"token-type", "PUT", "push", strings.Replace(validPush, `"synthetic-secret"`, "42", 1), 400, "INVALID_REQUEST"},
		{"string-version", "PUT", "push", strings.Replace(validPush, `"registration_version":1`, `"registration_version":"1"`, 1), 400, "INVALID_REQUEST"},
		{"fraction-version", "PUT", "push", strings.Replace(validPush, `"registration_version":1`, `"registration_version":1.0`, 1), 400, "INVALID_REQUEST"},
		{"exponent-version", "PUT", "push", strings.Replace(validPush, `"registration_version":1`, `"registration_version":1e0`, 1), 400, "INVALID_REQUEST"},
		{"overflow-version", "PUT", "push", strings.Replace(validPush, `"registration_version":1`, `"registration_version":9223372036854775808`, 1), 400, "INVALID_REQUEST"},
		{"trailing-document", "PUT", "push", validPush + " {}", 400, "INVALID_REQUEST"},
		{"raw-utf8", "PUT", "push", strings.Replace(validPush, "synthetic-secret", string([]byte{0xff}), 1), 400, "INVALID_REQUEST"},
		{"provider-semantic", "PUT", "push", registrationBody("other", "x", 1, 0), 422, "VALIDATION_FAILED"},
		{"version-semantic", "PUT", "push", registrationBody("fcm", "x", 0, 0), 422, "VALIDATION_FAILED"},
		{"negative-barrier", "PUT", "push", registrationBody("fcm", "x", 1, -1), 422, "VALIDATION_FAILED"},
		{"empty-token", "PUT", "push", registrationBody("fcm", "", 1, 0), 422, "VALIDATION_FAILED"},
		{"long-token", "PUT", "push", registrationBody("fcm", strings.Repeat("字", 4097), 1, 0), 422, "VALIDATION_FAILED"},
		{"delete-extra", "DELETE", "push/fcm", `{"expected_version":0,"device_id":"other"}`, 400, "INVALID_REQUEST"},
		{"delete-null", "DELETE", "push/fcm", `{"expected_version":null}`, 400, "INVALID_REQUEST"},
		{"delete-provider", "DELETE", "push/other", `{"expected_version":0}`, 422, "VALIDATION_FAILED"},
		{"settings-missing", "PUT", "notifications", strings.Replace(validSettings, `"preview":false,`, "", 1), 400, "INVALID_REQUEST"},
		{"settings-null-bool", "PUT", "notifications", strings.Replace(validSettings, `"enabled":true`, `"enabled":null`, 1), 400, "INVALID_REQUEST"},
		{"settings-null-array", "PUT", "notifications", strings.Replace(validSettings, `"bot_overrides":[]`, `"bot_overrides":null`, 1), 400, "INVALID_REQUEST"},
		{"settings-array-type", "PUT", "notifications", strings.Replace(validSettings, `"bot_overrides":[]`, `"bot_overrides":{}`, 1), 400, "INVALID_REQUEST"},
		{"settings-override-missing", "PUT", "notifications", strings.Replace(validSettings, `"bot_overrides":[]`, `"bot_overrides":[{"bot_id":"`+pushDevice+`"}]`, 1), 400, "INVALID_REQUEST"},
		{"settings-nested-duplicate", "PUT", "notifications", strings.Replace(validSettings, `"timezone":"UTC"`, `"timezone":"UTC","timezone":"UTC"`, 1), 400, "INVALID_REQUEST"},
		{"settings-nested-extra", "PUT", "notifications", strings.Replace(validSettings, `"timezone":"UTC"`, `"timezone":"UTC","observed_at":1`, 1), 400, "INVALID_REQUEST"},
		{"settings-nested-null", "PUT", "notifications", strings.Replace(validSettings, `"timezone":"UTC"`, `"timezone":null`, 1), 400, "INVALID_REQUEST"},
		{"settings-zone-semantic", "PUT", "notifications", strings.Replace(validSettings, `"timezone":"UTC"`, `"timezone":"Mars/Test"`, 1), 422, "VALIDATION_FAILED"},
		{"settings-provider-combination", "PUT", "notifications", strings.Replace(validSettings, `"notify_provider":"fcm"`, `"notify_provider":null`, 1), 422, "VALIDATION_FAILED"},
		{"settings-missing-bot", "PUT", "notifications", strings.Replace(validSettings, `"bot_overrides":[]`, `"bot_overrides":[{"bot_id":"`+pushDevice+`","enabled":true}]`, 1), 404, "RESOURCE_NOT_FOUND"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := pushHTTPSnapshot(t, f.store)
			w := f.request(tc.method, tc.path, tc.body, 0, nil)
			assertPushHTTP(t, w, tc.status, tc.code)
			if strings.Contains(w.Body.String(), "synthetic-secret") || pushHTTPSnapshot(t, f.store) != before {
				t.Fatal("rejected payload leaked secret or changed persistent state")
			}
		})
	}
	for _, customize := range []func(*http.Request){
		func(r *http.Request) { r.Header.Del("Content-Type") },
		func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") },
		func(r *http.Request) { r.Header.Add("Content-Type", "application/json") },
	} {
		before := pushHTTPSnapshot(t, f.store)
		assertPushHTTP(t, f.request("PUT", "push", validPush, 0, customize), 400, "INVALID_REQUEST")
		if pushHTTPSnapshot(t, f.store) != before {
			t.Fatal("content type rejection renewed a session")
		}
	}
	// 使用实际字节数而非Content-Length，边界内的空白仍是有效JSON文档。
	within := validPush + strings.Repeat(" ", pushBodyLimit-len(validPush))
	before := pushHTTPSnapshot(t, f.store)
	assertPushHTTP(t, f.request("PUT", "push", within+" ", 0, func(r *http.Request) { r.ContentLength = -1 }), 413, "PAYLOAD_TOO_LARGE")
	if pushHTTPSnapshot(t, f.store) != before {
		t.Fatal("oversized request changed state")
	}
	assertPushHTTP(t, f.request("PUT", "push", within, 0, nil), 200, "")
	assertPushHTTP(t, f.request("PUT", "push", registrationBody("fcm", "\x00opaque α", 2, 1), 0, nil), 200, "")
}

type unreadPushBody struct{ reads int }

func (b *unreadPushBody) Read([]byte) (int, error) { b.reads++; return 0, io.ErrUnexpectedEOF }
func (*unreadPushBody) Close() error               { return nil }

func TestPushHTTPAuthorizationOrderAndBodyIsolation(t *testing.T) {
	f := newPushHTTPFixture(t)
	for _, operation := range []struct{ method, path string }{{"GET", "push"}, {"PUT", "push"}, {"DELETE", "push/fcm"}, {"GET", "notifications"}, {"PUT", "notifications"}} {
		for _, tc := range []struct {
			name      string
			customize func(*http.Request)
			status    int
			code      string
		}{
			{"missing-instance", func(r *http.Request) { r.Header.Del("X-RelayNest-Instance-ID") }, 400, "INSTANCE_REQUIRED"},
			{"duplicate-instance", func(r *http.Request) { r.Header.Add("X-RelayNest-Instance-ID", f.instance) }, 400, "INVALID_REQUEST"},
			{"malformed-instance", func(r *http.Request) { r.Header.Set("X-RelayNest-Instance-ID", "bad") }, 400, "INVALID_REQUEST"},
			{"wrong-instance-before-bearer", func(r *http.Request) {
				r.Header.Set("X-RelayNest-Instance-ID", "00000000-0000-4000-8000-000000000099")
				r.Header.Set("Authorization", "Bearer synthetic-not-an-admin-session")
			}, 409, "INSTANCE_MISMATCH"},
			{"duplicate-auth-before-instance", func(r *http.Request) {
				r.Header.Add("Authorization", "Bearer another")
				r.Header.Del("X-RelayNest-Instance-ID")
			}, 401, "AUTHENTICATION_REQUIRED"},
			{"missing-auth", func(r *http.Request) { r.Header.Del("Authorization") }, 401, "AUTHENTICATION_REQUIRED"},
			{"bot-token", func(r *http.Request) { r.Header.Set("Authorization", "Bearer synthetic-bot-credential") }, 401, "AUTHENTICATION_REQUIRED"},
			{"selector", func(r *http.Request) { r.URL.RawQuery = "session_id=other" }, 400, "INVALID_REQUEST"},
		} {
			t.Run(operation.method+operation.path+tc.name, func(t *testing.T) {
				before := pushHTTPSnapshot(t, f.store)
				body := &unreadPushBody{}
				w := f.request(operation.method, operation.path, "", 0, func(r *http.Request) {
					r.Body = body
					tc.customize(r)
				})
				assertPushHTTP(t, w, tc.status, tc.code)
				if body.reads != 0 || pushHTTPSnapshot(t, f.store) != before || strings.Contains(w.Body.String(), f.instance) {
					t.Fatal("authorization rejection read body, mutated state or leaked instance identity")
				}
			})
		}
	}
	for _, path := range []string{"push", "notifications"} {
		body := &unreadPushBody{}
		assertPushHTTP(t, f.request("GET", path, "", 0, func(r *http.Request) { r.Body = body }), 200, "")
		if body.reads != 0 {
			t.Fatal("GET read client body")
		}
	}
	_, web, err := f.sessions.Create(f.ctx, 1, auth.SessionInput{ClientType: "web", DeviceName: "web"})
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"GET", "PUT"} {
		before := pushHTTPSnapshot(t, f.store)
		body := &unreadPushBody{}
		w := f.request(method, "notifications", "", -1, func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: "relaynest_session", Value: web.Token})
			r.Header.Set("Origin", "https://example.com")
			r.Header.Set("X-CSRF-Token", web.CSRFToken)
			r.Body = body
		})
		assertPushHTTP(t, w, 403, "FORBIDDEN")
		if body.reads != 0 || pushHTTPSnapshot(t, f.store) != before {
			t.Fatal("Web request entered Android operation")
		}
	}
}

func TestPushHTTPStaleSessionAndInstanceFailClosed(t *testing.T) {
	for _, mutation := range []string{"revoke", "idle", "absolute", "generation", "instance", "closed-store"} {
		t.Run(mutation, func(t *testing.T) {
			f := newPushHTTPFixture(t)
			var query string
			switch mutation {
			case "revoke":
				query = "UPDATE sessions SET revoked=1"
			case "idle":
				query = "UPDATE sessions SET idle_expires_at=1"
			case "absolute":
				query = "UPDATE sessions SET idle_expires_at=1,absolute_expires_at=1"
			case "generation":
				query = "UPDATE administrator SET auth_generation=auth_generation+1"
			case "instance":
				query = "UPDATE instance_identity SET instance_id='00000000-0000-4000-8000-000000000099'"
			case "closed-store":
				if err := f.store.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if query != "" {
				mustPushSQL(t, f.pushFixture, query)
			}
			// 只用新Store观测故障后的完整状态，不能替换生产handler的旧绑定。
			observer, err := storage.Open(f.ctx, storage.Config{Path: f.path})
			if err != nil {
				t.Fatal(err)
			}
			defer observer.Close()
			status, code := 401, "AUTHENTICATION_REQUIRED"
			if mutation == "instance" || mutation == "closed-store" {
				status, code = 503, "INSTANCE_UNAVAILABLE"
			}
			for _, operation := range []struct{ method, path, body string }{{"GET", "push", ""}, {"PUT", "push", registrationBody("fcm", "synthetic-secret", 1, 0)}, {"GET", "notifications", ""}, {"PUT", "notifications", notificationBody(t, notificationInput())}} {
				before := pushHTTPSnapshot(t, observer)
				assertPushHTTP(t, f.request(operation.method, operation.path, operation.body, 0, nil), status, code)
				if pushHTTPSnapshot(t, observer) != before {
					t.Fatal("stale actor/instance rejection changed state")
				}
			}
		})
	}
}

func TestPushHTTPFailedTransactionsPublishNoReceipt(t *testing.T) {
	for _, failure := range []string{"registration", "deletion", "settings", "changes", "commit", "generation-at-renew", "instance-at-renew"} {
		t.Run(failure, func(t *testing.T) {
			f := newPushHTTPFixture(t)
			// 成功续期会改变此短期限，失败应连续期和SQLite序号一同回滚。
			mustPushSQL(t, f.pushFixture, "UPDATE sessions SET idle_expires_at=?", time.Now().Add(time.Hour).UnixNano())
			method, path, body := "PUT", "push", registrationBody("fcm", "synthetic-secret", 1, 0)
			switch failure {
			case "registration":
				mustPushSQL(t, f.pushFixture, `CREATE TRIGGER request_fault BEFORE INSERT ON device_push_registrations BEGIN SELECT RAISE(ABORT,'synthetic-private-diagnostic'); END`)
			case "deletion":
				putPush(t, f.pushFixture, 0, PushFCM, 1, 0, "existing-synthetic-secret")
				method, path, body = "DELETE", "push/fcm", `{"expected_version":1}`
				mustPushSQL(t, f.pushFixture, `CREATE TRIGGER request_fault BEFORE UPDATE ON device_push_registrations BEGIN SELECT RAISE(ABORT,'synthetic-private-diagnostic'); END`)
			case "settings":
				path, body = "notifications", notificationBody(t, notificationInput())
				mustPushSQL(t, f.pushFixture, `CREATE TRIGGER request_fault BEFORE INSERT ON device_notification_settings BEGIN SELECT RAISE(ABORT,'synthetic-private-diagnostic'); END`)
			case "changes":
				path, body = "notifications", notificationBody(t, notificationInput())
				mustPushSQL(t, f.pushFixture, `CREATE TRIGGER request_fault BEFORE INSERT ON changes BEGIN SELECT RAISE(ABORT,'synthetic-private-diagnostic'); END`)
			case "commit":
				path, body = "notifications", notificationBody(t, notificationInput())
				mustPushSQL(t, f.pushFixture, `CREATE TABLE deferred_http_fault(id TEXT REFERENCES sessions(id) DEFERRABLE INITIALLY DEFERRED)`)
				mustPushSQL(t, f.pushFixture, `CREATE TRIGGER request_fault AFTER INSERT ON device_notification_settings BEGIN INSERT INTO deferred_http_fault VALUES('missing-session'); END`)
			case "generation-at-renew":
				mustPushSQL(t, f.pushFixture, `CREATE TRIGGER request_fault AFTER UPDATE OF idle_expires_at ON sessions BEGIN UPDATE administrator SET auth_generation=auth_generation+1 WHERE id=1; END`)
			case "instance-at-renew":
				mustPushSQL(t, f.pushFixture, `CREATE TRIGGER request_fault AFTER UPDATE OF idle_expires_at ON sessions BEGIN UPDATE instance_identity SET instance_id='00000000-0000-4000-8000-000000000099'; END`)
			}
			before := pushHTTPSnapshot(t, f.store)
			status, code := 503, "SERVICE_UNAVAILABLE"
			if failure == "generation-at-renew" {
				status, code = 401, "AUTHENTICATION_REQUIRED"
			} else if failure == "instance-at-renew" {
				status, code = 409, "INSTANCE_MISMATCH"
			}
			w := f.request(method, path, body, 0, nil)
			assertPushHTTP(t, w, status, code)
			if strings.Contains(w.Body.String(), "synthetic-private") || strings.Contains(w.Body.String(), f.path) || pushHTTPSnapshot(t, f.store) != before {
				t.Fatal("failed transaction leaked internals or partially committed")
			}
			if failure == "commit" && pushCount(t, f.pushFixture, "SELECT count(*) FROM deferred_http_fault") != 0 {
				t.Fatal("COMMIT failure retained deferred mutation")
			}
		})
	}
}
