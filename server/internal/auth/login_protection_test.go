package auth

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"relaynest/server/internal/httpapi"
)

func TestLoginFailureWindowAndBounds(t *testing.T) {
	p := newLoginProtection()
	now := time.Now()
	peer := netip.MustParseAddr("192.0.2.1")
	var releases []func(bool)
	for i := 0; i < loginFailureLimit; i++ {
		f, _ := p.begin(peer, now)
		if f == nil {
			t.Fatal("early rejection")
		}
		releases = append(releases, f)
	}
	if f, wait := p.begin(peer, now); f != nil || wait != loginWindow {
		t.Fatal("in-flight limit bypassed")
	}
	for _, f := range releases {
		f(true)
	}
	if f, _ := p.begin(peer, now.Add(loginWindow-time.Nanosecond)); f != nil {
		t.Fatal("failure limit bypassed")
	}
	f, _ := p.begin(peer, now.Add(loginWindow))
	if f == nil {
		t.Fatal("window did not expire")
	}
	f(false)
	for i := 0; i < loginPeerLimit; i++ {
		f, _ := p.begin(netip.AddrFrom4([4]byte{10, byte(i >> 8), byte(i), 1}), now.Add(loginWindow))
		if f != nil {
			f(true)
		}
	}
	if len(p.peers) != loginPeerLimit {
		t.Fatal("unbounded peers")
	}
	if f, _ := p.begin(netip.MustParseAddr("203.0.113.1"), now.Add(loginWindow)); f != nil {
		t.Fatal("capacity bypass")
	}
	if f, _ := p.begin(peer, now.Add(2*loginWindow)); f == nil {
		t.Fatal("stale peers not cleaned")
	} else {
		f(false)
	}
}

func TestLoginConcurrentFailureReservations(t *testing.T) {
	p := newLoginProtection()
	now := time.Now()
	peer := netip.MustParseAddr("192.0.2.1")
	var admitted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if f, _ := p.begin(peer, now); f != nil {
				admitted.Add(1)
				f(true)
			}
		}()
	}
	wg.Wait()
	if admitted.Load() != loginFailureLimit {
		t.Fatalf("admitted %d", admitted.Load())
	}
}

func TestLoginHTTPProtectionAndEnumeration(t *testing.T) {
	db := testStore(t, filepath.Join(t.TempDir(), "limit.sqlite"))
	if err := InitAdministrator(context.Background(), db, "Admin", []byte("correct")); err != nil {
		t.Fatal(err)
	}
	policy, err := httpapi.NewOriginPolicy("https://nas.example", []string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(previous)
	h := NewHandler(db, NewSessionStore(db), policy)
	request := func(user, password, peer, forwarded string, trusted bool) *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"username":%q,"password":%q,"client_type":"web","device_name":"browser"}`, user, password)
		r := httptest.NewRequest("POST", "https://nas.example/api/v1/auth/login", strings.NewReader(body))
		r.RemoteAddr = peer
		r.TLS = &tls.ConnectionState{}
		r.Header.Set("Origin", "https://nas.example")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Forwarded-For", forwarded)
		if trusted {
			r.Header.Set("X-Forwarded-Host", "nas.example")
			r.Header.Set("X-Forwarded-Proto", "https")
		}
		w := httptest.NewRecorder()
		h.Login(w, r)
		return w
	}
	for _, tc := range []struct{ user, password, peer string }{{"Admin", "wrong", "192.0.2.1:123"}, {"unknown", "correct", "192.0.2.2:123"}} {
		for i := 0; i < 6; i++ {
			w := request(tc.user, tc.password, tc.peer, fmt.Sprintf("198.51.100.%d", i+1), false)
			expected := 401
			if i == 5 {
				expected = 429
			}
			if w.Code != expected || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Set-Cookie") != "" {
				t.Fatalf("response %d %s", w.Code, w.Body.String())
			}
			code := "AUTHENTICATION_FAILED"
			if i == 5 {
				code = "RATE_LIMITED"
				if w.Header().Get("Retry-After") == "" {
					t.Fatal("missing retry")
				}
			}
			if !strings.Contains(w.Body.String(), `"code":"`+code+`"`) {
				t.Fatal(w.Body.String())
			}
		}
	}
	// 可信代理可以区分客户端；相同客户端换源端口仍受限。
	for i := 0; i < 5; i++ {
		if w := request("unknown", "wrong", "127.0.0.1:123", "203.0.113.1", true); w.Code != 401 {
			t.Fatal(w.Code)
		}
	}
	if w := request("Admin", "correct", "127.0.0.1:456", "203.0.113.1", true); w.Code != 429 {
		t.Fatal(w.Code)
	}
	if w := request("Admin", "correct", "127.0.0.1:123", "203.0.113.2", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	// 登录实际日志入口注入凭据、头、正文中的哨兵，不允许进入底层日志。
	secret := "login-secret-sentinel"
	request(secret, secret, "192.0.2.6:123", secret, false)
	if strings.Contains(logs.String(), secret) || !strings.Contains(logs.String(), "duration") {
		t.Fatal("login log boundary failed")
	}
	// 直接占用摘要层原有两槽，真实入口不能排队或再获取同一把锁。
	verificationSlots <- struct{}{}
	verificationSlots <- struct{}{}
	w := request("Admin", "correct", "192.0.2.3:123", "", false)
	<-verificationSlots
	<-verificationSlots
	if w.Code != 429 || w.Header().Get("Retry-After") != "1" {
		t.Fatal(w.Code)
	}
	// 并行进入真实登录入口，验证失败预算与摘要槽同时生效。
	h.logger = slog.New(httpapi.NewSafeLogHandler(slog.NewJSONHandler(io.Discard, nil)))
	var concurrentWG sync.WaitGroup
	var failures, limited, unexpected atomic.Int32
	for i := 0; i < 24; i++ {
		concurrentWG.Add(1)
		go func() {
			defer concurrentWG.Done()
			w := request("unknown", "wrong", "192.0.2.5:123", "", false)
			switch w.Code {
			case 401:
				failures.Add(1)
			case 429:
				limited.Add(1)
			default:
				unexpected.Add(1)
			}
		}()
	}
	concurrentWG.Wait()
	if failures.Load() > loginFailureLimit || failures.Load() == 0 || limited.Load() == 0 || unexpected.Load() != 0 {
		t.Fatalf("concurrent failures=%d limited=%d unexpected=%d", failures.Load(), limited.Load(), unexpected.Load())
	}
	// 取消请求不消耗失败预算，也不创建会话。
	r := httptest.NewRequest("POST", "https://nas.example/api/v1/auth/login", strings.NewReader(`{"username":"Admin","password":"correct","client_type":"web","device_name":"browser"}`))
	r.RemoteAddr = "192.0.2.4:123"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://nas.example")
	ctx, cancel := context.WithCancel(r.Context())
	cancel()
	canceled := httptest.NewRecorder()
	h.Login(canceled, r.WithContext(ctx))
	if canceled.Code == 200 || canceled.Header().Get("Set-Cookie") != "" {
		t.Fatal("canceled login returned success")
	}
	if e := h.protection.peers[netip.MustParseAddr("192.0.2.4")]; e.active != 0 || e.failures != 0 {
		t.Fatal("cancel leaked reservation")
	}
}
