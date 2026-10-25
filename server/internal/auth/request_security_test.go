package auth

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"relaynest/server/internal/bots"
	"relaynest/server/internal/httpapi"
)

// 固定随机源模拟分叉库共享凭据，不代表生产随机碰撞或远程绕过。
func TestRequestSecurityForeignStoreActor(t *testing.T) {
	for _, state := range []string{"active", "revoked"} {
		t.Run(state, func(t *testing.T) {
			a, b := newHTTPFixture(t, true), newHTTPFixture(t, true)
			b.clock.Store(a.clock.Load())
			guards := []*RequestSecurity{securityFixture(t, a), securityFixture(t, b)}
			var sessions [2]Session
			var secrets [2]SessionSecrets
			for i, f := range []*httpFixture{a, b} {
				raw := bytes.Repeat([]byte{42}, 48)
				raw[0] = byte(i + 1)
				f.sessions.random = bytes.NewReader(raw)
				var err error
				sessions[i], secrets[i], err = f.sessions.Create(context.Background(), 1, SessionInput{ClientType: "android", DeviceID: "11111111-1111-4111-8111-111111111111", DeviceName: "fixture"})
				if err != nil {
					t.Fatal(err)
				}
			}
			if sessions[0].ID == sessions[1].ID || secrets[0].Token != secrets[1].Token {
				t.Fatal("fixture requires distinct IDs and shared credentials")
			}
			actor, ok := guards[0].Authorize(httptest.NewRecorder(), securityRequest(a, http.Header{"Authorization": {"Bearer " + secrets[0].Token}}), RequestRead)
			if !ok {
				t.Fatal("initial authorization")
			}
			if state == "revoked" {
				if err := a.sessions.Revoke(context.Background(), secrets[0].Token, "android"); err != nil {
					t.Fatal(err)
				}
			}
			b.clock.Add(int64(time.Hour))
			beforeA, beforeB := securityDatabaseSnapshot(t, a), securityDatabaseSnapshot(t, b)
			_, renewalErr := guards[1].RenewRead(context.Background(), actor)
			if !errors.Is(renewalErr, ErrInvalidSession) {
				t.Errorf("foreign actor accepted: %v", renewalErr)
			}
			if securityDatabaseSnapshot(t, a) != beforeA || securityDatabaseSnapshot(t, b) != beforeB {
				t.Error("foreign actor changed sessions/idle expiry")
			}
			got, _, err := b.sessions.Inspect(context.Background(), secrets[1].Token, "android")
			if err != nil || !got.IdleExpiresAt.Equal(sessions[1].IdleExpiresAt) {
				t.Error("destination idle expiry changed", err)
			}
		})
	}
}

func securityFixture(t *testing.T, f *httpFixture) *RequestSecurity {
	t.Helper()
	p, err := httpapi.NewOriginPolicy(f.server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	return NewRequestSecurity(f.sessions, p)
}

// 比较两张认证表的全部列，快照只在内存比较，避免输出任何凭据摘要。
func securityDatabaseSnapshot(t *testing.T, f *httpFixture) string {
	t.Helper()
	var tables [][][]any
	err := f.db.Read(context.Background(), func(tx *sql.Tx) error {
		for _, query := range []string{"SELECT * FROM administrator ORDER BY id", "SELECT * FROM sessions ORDER BY id"} {
			rows, err := tx.Query(query)
			if err != nil {
				return err
			}
			columns, err := rows.Columns()
			if err != nil {
				rows.Close()
				return err
			}
			var table [][]any
			for rows.Next() {
				values, targets := make([]any, len(columns)), make([]any, len(columns))
				for i := range values {
					targets[i] = &values[i]
				}
				if err := rows.Scan(targets...); err != nil {
					rows.Close()
					return err
				}
				table = append(table, values)
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return err
			}
			tables = append(tables, table)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(tables)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestRequestSecurityActorBinding(t *testing.T) {
	f := newHTTPFixture(t, true)
	guard := securityFixture(t, f)
	token, _ := f.login("android")
	actor, ok := guard.Authorize(httptest.NewRecorder(), securityRequest(f, http.Header{"Authorization": {"Bearer " + token}}), RequestRead)
	if !ok {
		t.Fatal("initial authorization")
	}
	f.clock.Add(int64(time.Hour))
	type invalidActor struct {
		name  string
		guard *RequestSecurity
		actor AdminRequest
	}
	cases := []invalidActor{
		{"same-store-other-guard", securityFixture(t, f), actor},
		{"zero", guard, AdminRequest{}},
		{"forged", guard, AdminRequest{Session: actor.Session, Source: actor.Source, secrets: actor.secrets}},
	}
	for _, field := range []string{"id", "client", "username", "device", "device-name", "idle", "absolute"} {
		changed := actor
		switch field {
		case "id":
			changed.Session.ID = "other"
		case "client":
			changed.Session.ClientType = "web"
		case "username":
			changed.Session.Username = "other"
		case "device":
			changed.Session.DeviceID = "other"
		case "device-name":
			changed.Session.DeviceName = "other"
		case "idle":
			changed.Session.IdleExpiresAt = changed.Session.IdleExpiresAt.Add(time.Second)
		case "absolute":
			changed.Session.AbsoluteExpiresAt = changed.Session.AbsoluteExpiresAt.Add(time.Second)
		}
		cases = append(cases, invalidActor{field, guard, changed})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := securityDatabaseSnapshot(t, f)
			if _, err := tc.guard.RenewRead(context.Background(), tc.actor); !errors.Is(err, ErrInvalidSession) {
				t.Fatalf("invalid actor accepted: %v", err)
			}
			if securityDatabaseSnapshot(t, f) != before {
				t.Fatal("rejection changed database")
			}
		})
	}
	t.Run("guard-store-rewired", func(t *testing.T) {
		other := newHTTPFixture(t, true)
		before, otherBefore := securityDatabaseSnapshot(t, f), securityDatabaseSnapshot(t, other)
		guard.sessions = other.sessions
		defer func() { guard.sessions = f.sessions }()
		if _, err := guard.RenewRead(context.Background(), actor); !errors.Is(err, ErrInvalidSession) {
			t.Fatal("rewired guard accepted actor", err)
		}
		if securityDatabaseSnapshot(t, f) != before || securityDatabaseSnapshot(t, other) != otherBefore {
			t.Fatal("rewired guard changed database")
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before := securityDatabaseSnapshot(t, f)
	if _, err := guard.RenewRead(ctx, actor); !errors.Is(err, ErrStorage) {
		t.Fatal("cancellation not propagated", err)
	}
	if securityDatabaseSnapshot(t, f) != before {
		t.Fatal("cancellation changed database")
	}
	renewed, err := guard.RenewRead(context.Background(), actor)
	if err != nil || renewed.ID != actor.Session.ID || !renewed.IdleExpiresAt.After(actor.Session.IdleExpiresAt) {
		t.Fatal("valid renewal failed", err)
	}
	// 其他读取已续期后，合法身份仍可使用原签发快照。
	f.clock.Add(int64(time.Hour))
	if _, err := guard.RenewRead(context.Background(), actor); err != nil {
		t.Fatal(err)
	}
	// 模拟凭据指向的数据库身份替换，必须在任何续期写入之前拒绝。
	if err := f.db.Write(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE sessions SET id=? WHERE id=?", "22222222-2222-4222-8222-222222222222", actor.Session.ID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	f.clock.Add(int64(time.Hour))
	before = securityDatabaseSnapshot(t, f)
	if _, err := guard.RenewRead(context.Background(), actor); !errors.Is(err, ErrInvalidSession) {
		t.Fatal("changed database identity accepted", err)
	}
	if securityDatabaseSnapshot(t, f) != before {
		t.Fatal("identity rejection changed database")
	}
}

func securityRequest(f *httpFixture, headers http.Header) *http.Request {
	r := httptest.NewRequest("POST", f.server.URL+"/admin/resource", nil)
	r.RemoteAddr = "192.0.2.10:1234"
	r.Header.Set(instanceHeader, f.instanceID)
	for name, values := range headers {
		for _, value := range values {
			r.Header.Add(name, value)
		}
	}
	return r
}

func TestRequestSecuritySuccessfulReadWrite(t *testing.T) {
	f := newHTTPFixture(t, true)
	guard := securityFixture(t, f)
	for _, client := range []string{"web", "android"} {
		token, csrf := f.login(client)
		h := http.Header{}
		if client == "web" {
			h.Set("Cookie", sessionCookie+"="+token)
			h.Set("Origin", f.server.URL)
			h.Set("X-CSRF-Token", csrf)
		} else {
			h.Set("Authorization", "Bearer "+token)
		}
		f.clock.Add(int64(time.Hour))
		for _, mode := range []RequestMode{RequestRead, RequestWrite} {
			before := securityDatabaseSnapshot(t, f)
			w := httptest.NewRecorder()
			actor, ok := guard.Authorize(w, securityRequest(f, h), mode)
			if !ok || actor.Session.ID == "" || actor.Session.Username != "Admin" || actor.Session.ClientType != client || !actor.Source.Secure {
				t.Fatalf("identity %+v %v", actor.Session, ok)
			}
			if securityDatabaseSnapshot(t, f) != before || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Set-Cookie") != "" || w.Body.Len() != 0 {
				t.Fatal("authorization changed state/response")
			}
			raw, err := json.Marshal(actor)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]any
			if err = json.Unmarshal(raw, &fields); err != nil || len(fields) != 2 {
				t.Fatalf("private credentials serialized: %s", raw)
			}
			if mode == RequestRead {
				renewed, err := guard.RenewRead(context.Background(), actor)
				if err != nil || !renewed.IdleExpiresAt.After(actor.Session.IdleExpiresAt) {
					t.Fatal("read renewal", renewed, err)
				}
			}
			clear := httptest.NewRecorder()
			actor.ClearCookie(clear)
			cookies := clear.Result().Cookies()
			if client == "web" {
				if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].Path != "/api/v1" || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].MaxAge != -1 {
					t.Fatal("clear cookie", cookies)
				}
			} else if len(cookies) != 0 {
				t.Fatal("android cookie")
			}
		}
	}
}

func TestRequestSecurityRejectionsNoEffects(t *testing.T) {
	f := newHTTPFixture(t, true)
	guard := securityFixture(t, f)
	web, csrf := f.login("web")
	android, _ := f.login("android")
	cookie := sessionCookie + "=" + web
	cases := []struct {
		name   string
		h      http.Header
		mode   RequestMode
		status int
	}{
		{"missing", http.Header{}, RequestRead, 401},
		{"mixed", http.Header{"Cookie": {cookie}, "Authorization": {"Bearer " + android}}, RequestRead, 401},
		{"duplicate bearer", http.Header{"Authorization": {"Bearer " + android, "Bearer " + android}}, RequestRead, 401},
		{"duplicate cookie", http.Header{"Cookie": {cookie + "; " + cookie}}, RequestRead, 401},
		{"web bearer", http.Header{"Authorization": {"Bearer " + web}}, RequestRead, 401},
		{"android cookie", http.Header{"Cookie": {sessionCookie + "=" + android}}, RequestRead, 401},
		{"bot sized token", http.Header{"Authorization": {"Bearer " + base64.RawURLEncoding.EncodeToString(make([]byte, 32))}}, RequestRead, 401},
		{"missing csrf", http.Header{"Cookie": {cookie}, "Origin": {f.server.URL}}, RequestWrite, 403},
		{"wrong csrf", http.Header{"Cookie": {cookie}, "Origin": {f.server.URL}, "X-CSRF-Token": {"bad"}}, RequestWrite, 403},
		{"duplicate csrf", http.Header{"Cookie": {cookie}, "Origin": {f.server.URL}, "X-CSRF-Token": {csrf, csrf}}, RequestWrite, 403},
		{"cross origin", http.Header{"Cookie": {cookie}, "Origin": {"https://evil.invalid"}, "X-CSRF-Token": {csrf}}, RequestWrite, 403},
		{"no origin", http.Header{"Cookie": {cookie}, "X-CSRF-Token": {csrf}}, RequestWrite, 403},
		{"conflict referer", http.Header{"Cookie": {cookie}, "Origin": {f.server.URL}, "Referer": {"https://evil.invalid/path"}, "X-CSRF-Token": {csrf}}, RequestWrite, 403},
		{"invalid mode", http.Header{"Authorization": {"Bearer " + android}}, RequestMode(0), 503},
	}
	f.clock.Add(int64(time.Hour))
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			for _, required := range []bool{false, true} {
				authorize := guard.Authorize
				if required {
					authorize = guard.AuthorizeRequired
				}
				before := f.snapshot()
				w := httptest.NewRecorder()
				actor, ok := authorize(w, securityRequest(f, tt.h), tt.mode)
				if ok || actor.Session.ID != "" || w.Code != tt.status || f.snapshot() != before || w.Header().Get("Set-Cookie") != "" || w.Header().Get("Cache-Control") != "no-store" {
					t.Fatalf("rejection %d %v", w.Code, ok)
				}
				var body struct {
					Error struct {
						Code      string `json:"code"`
						RequestID string `json:"request_id"`
						Retryable bool   `json:"retryable"`
					} `json:"error"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Error.RequestID == "" || body.Error.Retryable != (tt.status == 503) {
					t.Fatal("error envelope", w.Body.String())
				}
				code := "AUTHENTICATION_FAILED"
				if required {
					code = "AUTHENTICATION_REQUIRED"
				}
				if tt.status == 403 {
					code = "FORBIDDEN"
				}
				if tt.status == 503 {
					code = "SERVICE_UNAVAILABLE"
				}
				assertSecurityErrorCode(t, w, tt.status, code)
			}
		})
	}
	// 合法 Referer 回退成功，非可信转发头不能替换实际 TLS 来源。
	h := http.Header{"Cookie": {cookie}, "Referer": {f.server.URL + "/settings"}, "X-CSRF-Token": {csrf}, "X-Forwarded-Proto": {"http"}, "X-Forwarded-Host": {"evil.invalid"}}
	actor, ok := guard.Authorize(httptest.NewRecorder(), securityRequest(f, h), RequestWrite)
	if !ok || !actor.Source.Secure || actor.Source.Forwarded {
		t.Fatal("source resolution")
	}
}

func TestRequestSecurityLifecycleAndRenewRecheck(t *testing.T) {
	for _, state := range []string{"revoked", "idle", "absolute", "generation"} {
		t.Run(state, func(t *testing.T) {
			f := newHTTPFixture(t, true)
			guard := securityFixture(t, f)
			token, _ := f.login("android")
			h := http.Header{"Authorization": {"Bearer " + token}}
			actor, ok := guard.Authorize(httptest.NewRecorder(), securityRequest(f, h), RequestRead)
			if !ok {
				t.Fatal("initial auth")
			}
			switch state {
			case "revoked":
				if err := f.sessions.Revoke(context.Background(), token, "android"); err != nil {
					t.Fatal(err)
				}
			case "idle":
				f.clock.Add(int64(31 * 24 * time.Hour))
			case "absolute":
				if err := f.db.Write(context.Background(), func(tx *sql.Tx) error {
					_, err := tx.Exec("UPDATE sessions SET idle_expires_at=?,absolute_expires_at=?", f.clock.Load(), f.clock.Load())
					return err
				}); err != nil {
					t.Fatal(err)
				}
			case "generation":
				if err := f.db.Write(context.Background(), func(tx *sql.Tx) error {
					_, err := tx.Exec("UPDATE administrator SET auth_generation=auth_generation+1")
					return err
				}); err != nil {
					t.Fatal(err)
				}
			}
			before := f.snapshot()
			for _, mode := range []RequestMode{RequestRead, RequestWrite} {
				w := httptest.NewRecorder()
				_, ok := guard.Authorize(w, securityRequest(f, h), mode)
				if ok || w.Code != 401 || f.snapshot() != before {
					t.Fatal("stale authorization", state)
				}
			}
			if _, err := guard.RenewRead(context.Background(), actor); err != ErrInvalidSession || f.snapshot() != before {
				t.Fatal("stale renewal", err)
			}
		})
	}
}

func TestRequestSecurityRealBotCredentialsDenied(t *testing.T) {
	f := newHTTPFixture(t, true)
	guard := securityFixture(t, f)
	created, _, err := bots.New(f.db).Create(context.Background(), bots.CreateRequest{Name: "transport", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	before := f.snapshot()
	for _, token := range []string{created.APIToken.Token, created.WebhookKey.Key} {
		for _, mode := range []RequestMode{RequestRead, RequestWrite} {
			w := httptest.NewRecorder()
			_, ok := guard.Authorize(w, securityRequest(f, http.Header{"Authorization": {"Bearer " + token}}), mode)
			if ok || w.Code != 401 || f.snapshot() != before || w.Header().Get("Set-Cookie") != "" {
				t.Fatal("Bot credential accepted or changed session")
			}
		}
	}
}

func TestRequestSecurityErrorSelection(t *testing.T) {
	f := newHTTPFixture(t, true)
	guard := securityFixture(t, f)
	token, _ := f.login("android")
	created, _, err := bots.New(f.db).Create(context.Background(), bots.CreateRequest{Name: "transport", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []bool{false, true} {
		code := "AUTHENTICATION_FAILED"
		if required {
			code = "AUTHENTICATION_REQUIRED"
		}
		authorize, write := guard.Authorize, WriteSessionError
		if required {
			authorize, write = guard.AuthorizeRequired, WriteSessionErrorRequired
		}
		for _, headers := range []http.Header{{}, {"Authorization": {"Bearer invalid"}}, {"Authorization": {"Bearer " + created.APIToken.Token}}, {"Authorization": {"Bearer " + created.WebhookKey.Key}}} {
			before := f.snapshot()
			w := httptest.NewRecorder()
			if _, ok := authorize(w, securityRequest(f, headers), RequestWrite); ok {
				t.Fatal("错误凭据获准")
			}
			assertSecurityErrorCode(t, w, 401, code)
			if before != f.snapshot() {
				t.Fatal("拒绝请求写入")
			}
		}
		w := httptest.NewRecorder()
		if _, ok := authorize(w, securityRequest(f, http.Header{"Authorization": {"Bearer " + token}}), RequestWrite); !ok {
			t.Fatal("有效管理员被拒绝")
		}
		for _, tc := range []struct {
			err    error
			status int
			code   string
		}{
			{ErrInvalidSession, 401, code}, {ErrNotInitialized, 401, code},
			{fmt.Errorf("wrapped: %w", ErrInvalidSession), 401, code},
			{errors.New("database failure"), 503, "SERVICE_UNAVAILABLE"},
		} {
			w := httptest.NewRecorder()
			write(w, tc.err)
			assertSecurityErrorCode(t, w, tc.status, tc.code)
		}
	}
}

func assertSecurityErrorCode(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || w.Code != status || body.Error.Code != code || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Set-Cookie") != "" {
		t.Fatalf("响应不匹配: %d %s", w.Code, w.Body.String())
	}
}
