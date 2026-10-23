package auth

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"relaynest/server/internal/bots"
	"relaynest/server/internal/httpapi"
)

func securityFixture(t *testing.T, f *httpFixture) *RequestSecurity {
	t.Helper()
	p, err := httpapi.NewOriginPolicy(f.server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	return NewRequestSecurity(f.sessions, p)
}

func securityRequest(f *httpFixture, headers http.Header) *http.Request {
	r := httptest.NewRequest("POST", f.server.URL+"/admin/resource", nil)
	r.RemoteAddr = "192.0.2.10:1234"
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
			before := f.snapshot()
			w := httptest.NewRecorder()
			actor, ok := guard.Authorize(w, securityRequest(f, h), mode)
			if !ok || actor.Session.ID == "" || actor.Session.Username != "Admin" || actor.Session.ClientType != client || !actor.Source.Secure {
				t.Fatalf("identity %+v %v", actor.Session, ok)
			}
			if f.snapshot() != before || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Set-Cookie") != "" || w.Body.Len() != 0 {
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
			before := f.snapshot()
			w := httptest.NewRecorder()
			actor, ok := guard.Authorize(w, securityRequest(f, tt.h), tt.mode)
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
