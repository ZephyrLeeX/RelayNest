package httpapi

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func policy(t *testing.T, origin string, proxies ...string) *OriginPolicy {
	t.Helper()
	p, err := NewOriginPolicy(origin, proxies)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func directRequest() *http.Request {
	return &http.Request{Host: "hub.example", RemoteAddr: "192.0.2.20:12345", TLS: &tls.ConnectionState{}, Header: make(http.Header)}
}

func forwardedHeaders() http.Header {
	return http.Header{"X-Forwarded-Host": {"hub.example"}, "X-Forwarded-Proto": {"https"}, "X-Forwarded-For": {"198.51.100.9"}}
}

func assertSourceError(t *testing.T, got RequestSource, err, want error) {
	t.Helper()
	if !errors.Is(err, want) || got != (RequestSource{}) {
		t.Fatalf("失败必须返回固定错误和零结果: result=%+v err=%v", got, err)
	}
}

func TestOriginConfiguration(t *testing.T) {
	valid := map[string]string{
		"HTTPS://HUB.Example:00443":      "https://hub.example",
		"https://hub.example:8443":       "https://hub.example:8443",
		"https://127.0.0.1":              "https://127.0.0.1",
		"https://[2001:0DB8:0:0::1]:443": "https://[2001:db8::1]",
		"https://[2001:db8::1]:8443":     "https://[2001:db8::1]:8443",
		"https://[::ffff:192.0.2.1]":     "https://192.0.2.1",
		"https://xn--fiqs8s.example.":    "https://xn--fiqs8s.example.",
	}
	for raw, want := range valid {
		t.Run(raw, func(t *testing.T) {
			p := policy(t, raw)
			if p.origin != want || len(p.proxies) != 0 {
				t.Fatalf("origin=%q want=%q", p.origin, want)
			}
		})
	}
	invalid := []string{"", "null", "http://hub.example", "https://", "https://*.example", "https://a b", "https://例.example", "https://user@hub.example", "https://hub.example/", "https://hub.example?", "https://hub.example?q", "https://hub.example#", "https://hub.example#x", "https://hub.example:", "https://hub.example:0", "https://hub.example:65536", "https://hub.example:-1", "https://hub.example:+443", "https://hub.example:4x", "https://::1", "https://[::1", "https://[127.0.0.1]", "https://[fe80::1%25eth0]", "https://a..b", "https://-a.example", "https://a_.example", "https://a%2eexample", "https://hub.example\n", "https://hub.example,https://evil.example", "https://hub.example https://evil.example", "https://hub.example\\@evil.example", "https://" + strings.Repeat("a", 64) + ".example"}
	for _, raw := range invalid {
		p, err := NewOriginPolicy(raw, nil)
		if p != nil || !errors.Is(err, ErrOriginConfig) || err.Error() != "invalid origin configuration" {
			t.Errorf("非法配置应拒绝: %q", raw)
		}
	}
	for _, raw := range []string{"", "localhost", "127.0.0.1:1234", "0.0.0.0/0", "192.0.2.1/0", "::/0", "::ffff:0:0/96", "::ffff:192.0.2.0/80", "192.0.2.1/33", "fe80::1%eth0", " 127.0.0.1"} {
		p, err := NewOriginPolicy("https://hub.example", []string{raw})
		if p != nil || err != ErrOriginConfig {
			t.Errorf("非法代理配置应拒绝: %q", raw)
		}
	}
	input := []string{"127.0.0.1", "10.1.0.0/16", "::1", "2001:db8::/32", "::ffff:192.0.2.0/120"}
	p := policy(t, "https://hub.example", input...)
	input[0] = "0.0.0.0/0"
	if len(p.proxies) != 5 || p.proxies[4].String() != "192.0.2.0/24" || p.proxies[0].String() != "127.0.0.1/32" {
		t.Fatal("代理配置须复制并规范 mapped IPv6")
	}
}

func TestDirectSource(t *testing.T) {
	p := policy(t, "https://hub.example")
	for _, remote := range []string{"192.0.2.20:12345", "127.0.0.1:12345", "10.1.2.3:12345", "[::1]:12345", "[::ffff:192.0.2.20]:12345"} {
		r := directRequest()
		r.RemoteAddr = remote
		before, err := p.Resolve(r)
		if err != nil || !before.HTTPS || !before.Secure || before.Forwarded || before.ExternalOrigin != "https://hub.example" || before.ConfiguredOrigin != before.ExternalOrigin || before.ClientIP != before.DirectPeer {
			t.Fatalf("合法直连失败: %+v %v", before, err)
		}
		r.Header = forwardedHeaders()
		r.Header["Forwarded"] = []string{"for=attacker"}
		r.Header["X-Forwarded-Host"] = []string{"evil.example", "bad"}
		r.Header["X-Forwarded-For"] = []string{"127.0.0.1, 10.0.0.1"}
		after, err := p.Resolve(r)
		if err != nil || before != after {
			t.Fatal("非可信头不得改变合法直连结果")
		}
	}
	for _, mutate := range []func(*http.Request){
		func(r *http.Request) { r.TLS = nil; r.Header = forwardedHeaders() },
		func(r *http.Request) { r.Host = "evil.example"; r.Header = forwardedHeaders() },
		func(r *http.Request) { r.Host = "hub.example:8443" },
		func(r *http.Request) { r.Host = "hub.example." },
		func(r *http.Request) { r.Host = "hub.example/path" },
		func(r *http.Request) { r.Host = "hub.example:443@evil.example" },
		func(r *http.Request) { r.RemoteAddr = "not-an-ip:1234" },
		func(r *http.Request) { r.RemoteAddr = "192.0.2.20" },
		func(r *http.Request) { r.RemoteAddr = "192.0.2.20:bad" },
		func(r *http.Request) { r.RemoteAddr = "[fe80::1%eth0]:1234" },
	} {
		r := directRequest()
		mutate(r)
		got, err := p.Resolve(r)
		assertSourceError(t, got, err, ErrRequestSource)
	}
	r := directRequest()
	r.Host = "HUB.example:443"
	if _, err := p.Resolve(r); err != nil {
		t.Fatal("缺省与显式443应等价")
	}
	for _, p := range []*OriginPolicy{nil, {}} {
		got, err := p.Resolve(r)
		assertSourceError(t, got, err, ErrRequestSource)
	}
	got, err := policy(t, "https://hub.example").Resolve(nil)
	assertSourceError(t, got, err, ErrRequestSource)
}

func TestTrustedForwarding(t *testing.T) {
	for _, tc := range []struct{ proxy, remote string }{
		{"127.0.0.1", "127.0.0.1:1234"}, {"10.1.0.0/16", "10.1.2.3:1234"},
		{"192.0.2.20", "[::ffff:192.0.2.20]:1234"}, {"::ffff:192.0.2.0/120", "192.0.2.20:1234"},
		{"::1", "[::1]:1234"}, {"2001:db8::/32", "[2001:db8::1]:1234"},
	} {
		p := policy(t, "https://hub.example", tc.proxy)
		r := directRequest()
		r.RemoteAddr, r.TLS, r.Host, r.Header = tc.remote, nil, "internal:8080", forwardedHeaders()
		got, err := p.Resolve(r)
		if err != nil || !got.Forwarded || !got.HTTPS || !got.Secure || got.ClientIP.String() != "198.51.100.9" {
			t.Fatalf("显式代理解析失败: %+v %v", got, err)
		}
		r.TLS, r.Host, r.Header = &tls.ConnectionState{}, "hub.example", make(http.Header)
		got, err = p.Resolve(r)
		if err != nil || got.Forwarded || got.DirectPeer != got.ClientIP {
			t.Fatal("可信peer无转发头应验证直连")
		}
	}
	p := policy(t, "https://hub.example", "192.0.2.0/24")
	cases := map[string]func(http.Header){
		"缺Host":       func(h http.Header) { delete(h, "X-Forwarded-Host") },
		"缺Proto":      func(h http.Header) { delete(h, "X-Forwarded-Proto") },
		"缺For":        func(h http.Header) { delete(h, "X-Forwarded-For") },
		"空Host":       func(h http.Header) { h["X-Forwarded-Host"] = []string{""} },
		"nil值":        func(h http.Header) { h["X-Forwarded-For"] = nil },
		"重复":          func(h http.Header) { h.Add("X-Forwarded-Host", "hub.example") },
		"大小写重复":       func(h http.Header) { h["x-forwarded-proto"] = []string{"https"} },
		"host链":       func(h http.Header) { h.Set("X-Forwarded-Host", "hub.example, hub.example") },
		"proto链":      func(h http.Header) { h.Set("X-Forwarded-Proto", "https,https") },
		"for链":        func(h http.Header) { h.Set("X-Forwarded-For", "198.51.100.9,192.0.2.1") },
		"Forwarded混用": func(h http.Header) { h.Set("Forwarded", "for=198.51.100.9;proto=https") },
		"空Forwarded":  func(h http.Header) { h["forwarded"] = nil },
		"HTTP":        func(h http.Header) { h.Set("X-Forwarded-Proto", "http") },
		"大写proto":     func(h http.Header) { h.Set("X-Forwarded-Proto", "HTTPS") },
		"Host冲突":      func(h http.Header) { h.Set("X-Forwarded-Host", "evil.example") },
		"端口冲突":        func(h http.Header) { h.Set("X-Forwarded-Host", "hub.example:8443") },
		"Host畸形":      func(h http.Header) { h.Set("X-Forwarded-Host", "hub.example/path") },
		"IP畸形":        func(h http.Header) { h.Set("X-Forwarded-For", "client.example") },
		"IP端口":        func(h http.Header) { h.Set("X-Forwarded-For", "198.51.100.9:12") },
		"IPzone":      func(h http.Header) { h.Set("X-Forwarded-For", "fe80::1%eth0") },
		"空白":          func(h http.Header) { h.Set("X-Forwarded-For", " 198.51.100.9") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := directRequest() // 合法TLS/Host也不得挽救坏转发协议。
			r.Header = forwardedHeaders()
			mutate(r.Header)
			got, err := p.Resolve(r)
			assertSourceError(t, got, err, ErrRequestSource)
		})
	}
	r := directRequest()
	r.Header.Set("Forwarded", "for=198.51.100.9")
	got, err := p.Resolve(r)
	assertSourceError(t, got, err, ErrRequestSource)
	r.Header = forwardedHeaders()
	r.Header.Set("X-Forwarded-For", "::ffff:198.51.100.9")
	got, err = p.Resolve(r)
	if err != nil || got.ClientIP != netip.MustParseAddr("198.51.100.9") {
		t.Fatal("客户端mapped IP应规范化")
	}
	r.TLS, r.Header = nil, make(http.Header)
	r.Header.Set("X-Forwarded-Port", "443")
	r.URL = httptest.NewRequest("GET", "https://hub.example", nil).URL
	got, err = p.Resolve(r)
	assertSourceError(t, got, err, ErrRequestSource)
}

func TestBrowserOrigins(t *testing.T) {
	p := policy(t, "https://hub.example")
	for _, tc := range []struct {
		name, origin, referer string
		login, write          bool
	}{
		{"origin", "https://hub.example", "", true, true},
		{"大小写默认端口", "HTTPS://HUB.example:443", "", true, true},
		{"回退", "", "https://hub.example/path?secret=synthetic", false, true},
		{"路径逗号", "", "https://hub.example/a,b?x=1,2", false, true},
		{"双头同源", "https://hub.example", "https://hub.example:443/path", true, true},
		{"均缺", "", "", false, false},
		{"null", "null", "https://hub.example/", false, false},
		{"跨源", "https://evil.example", "https://hub.example/", false, false},
		{"端口", "https://hub.example:8443", "", false, false},
		{"HTTP", "http://hub.example", "", false, false},
		{"多origin", "https://hub.example https://hub.example", "", false, false},
		{"逗号origin", "https://hub.example,https://hub.example", "", false, false},
		{"origin路径", "https://hub.example/", "https://hub.example/", false, false},
		{"冲突", "https://hub.example", "https://evil.example/path", false, false},
		{"referer端口", "", "https://hub.example:8443/path", false, false},
		{"userinfo", "", "https://user@hub.example/path", false, false},
		{"fragment", "", "https://hub.example/path#fragment", false, false},
		{"空fragment", "", "https://hub.example/path#", false, false},
		{"相对referer", "", "/path", false, false},
		{"referer转义", "https://hub.example", "https://hub.example/%GG", false, false},
		{"query转义", "https://hub.example", "https://hub.example/?x=%GG", false, false},
		{"referer空格", "https://hub.example", "https://hub.example/bad path", false, false},
		{"前缀欺骗", "https://hub.example.evil", "", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := directRequest()
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			if tc.referer != "" {
				r.Header.Set("Referer", tc.referer)
			}
			for _, check := range []struct {
				fn   func(*http.Request) (RequestSource, error)
				want bool
			}{{p.ValidateLogin, tc.login}, {p.ValidateWrite, tc.write}} {
				got, err := check.fn(r)
				if check.want {
					if err != nil || !got.Secure {
						t.Fatalf("合法浏览器来源失败: %v", err)
					}
				} else {
					assertSourceError(t, got, err, ErrBrowserOrigin)
				}
			}
		})
	}
	for _, h := range []http.Header{
		{"Origin": {"https://hub.example", "https://hub.example"}},
		{"Origin": {"https://hub.example"}, "origin": {"https://hub.example"}},
		{"Origin": {""}, "Referer": {"https://hub.example/"}},
		{"Origin": nil, "Referer": {"https://hub.example/"}},
		{"Origin": {"https://hub.example"}, "Referer": {""}},
		{"Referer": {"https://hub.example/", "https://hub.example/"}},
	} {
		r := directRequest()
		r.Header = h
		got, err := p.ValidateWrite(r)
		assertSourceError(t, got, err, ErrBrowserOrigin)
	}
	r := directRequest()
	r.TLS = nil
	r.Header.Set("Origin", "https://hub.example")
	got, err := p.ValidateLogin(r)
	assertSourceError(t, got, err, ErrRequestSource)
	// 同一三元组规则也覆盖非缺省端口和IPv6浏览器来源。
	for _, host := range []string{"hub.example:8443", "[2001:db8::1]:8443", "[2001:db8::1]"} {
		p := policy(t, "https://"+host)
		r := directRequest()
		r.Host = host
		r.Header.Set("Origin", "https://"+host)
		if _, err := p.ValidateLogin(r); err != nil {
			t.Fatal(err)
		}
	}
}

// 本地真实net/http入站验证；不代表真实NAS或代理覆盖配置已验证。
func TestOriginRealHTTPAndTLS(t *testing.T) {
	for _, tc := range []struct {
		name         string
		tls, trusted bool
		header       http.Header
		status       int
	}{
		{"代理HTTP", false, true, forwardedHeaders(), 200},
		{"非可信HTTP伪造", false, false, forwardedHeaders(), 403},
		{"TLS直连", true, false, nil, 200},
		{"TLS伪造忽略", true, false, http.Header{"Forwarded": {"bad"}, "X-Forwarded-Host": {"evil", "evil"}}, 200},
		{"可信重复头", true, true, http.Header{"X-Forwarded-Host": {"hub.example", "hub.example"}, "X-Forwarded-Proto": {"https"}, "X-Forwarded-For": {"198.51.100.9"}}, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var proxies []string
			if tc.trusted {
				proxies = []string{"127.0.0.1", "::1"}
			}
			p := policy(t, "https://hub.example", proxies...)
			s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got, err := p.Resolve(r)
				if err != nil {
					if got != (RequestSource{}) {
						t.Error("失败非零结果")
					}
					w.WriteHeader(403)
					return
				}
				if !got.Secure || !got.DirectPeer.IsLoopback() || got.Forwarded != (tc.trusted && tc.header != nil) {
					t.Error("入站来源结果不符")
				}
				w.WriteHeader(200)
			}))
			if tc.tls {
				s.StartTLS()
			} else {
				s.Start()
			}
			defer s.Close()
			r, _ := http.NewRequest("GET", s.URL, nil)
			r.Host = "hub.example"
			r.Header = tc.header.Clone()
			resp, err := s.Client().Do(r)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.status {
				t.Fatalf("status=%d want=%d", resp.StatusCode, tc.status)
			}
		})
	}
}

func TestOriginWireParserBoundaries(t *testing.T) {
	// ReadRequest保留重复头，模块须拒绝；非法控制字符由标准库先拒绝。
	p := policy(t, "https://hub.example", "127.0.0.1")
	for _, field := range []string{"Origin", "Referer", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Forwarded-For"} {
		headers := forwardedHeaders()
		headers.Set("Origin", "https://hub.example")
		headers.Set("Referer", "https://hub.example/path")
		headers.Add(field, headers.Get(field))
		var wire strings.Builder
		wire.WriteString("POST / HTTP/1.1\r\nHost: internal\r\n")
		if err := headers.Write(&wire); err != nil {
			t.Fatal(err)
		}
		wire.WriteString("\r\n")
		r, err := http.ReadRequest(bufio.NewReader(strings.NewReader(wire.String())))
		if err != nil {
			t.Fatal(err)
		}
		r.RemoteAddr = "127.0.0.1:12345"
		got, err := p.ValidateWrite(r)
		want := ErrRequestSource
		if field == "Origin" || field == "Referer" {
			want = ErrBrowserOrigin
		}
		assertSourceError(t, got, err, want)
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("非法头不应到达handler") }))
	defer s.Close()
	conn, err := net.Dial("tcp", strings.TrimPrefix(s.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: hub.example\r\nOrigin: https://hub.example\x00\r\nConnection: close\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatal(fmt.Sprintf("非法控制字符status=%d", resp.StatusCode))
	}
}
