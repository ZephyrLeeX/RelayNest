package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"relaynest/server/internal/storage"
	"strings"
	"testing"
	"time"
)

func resetProbeBucket(t *testing.T, tokens float64) {
	t.Helper()
	instanceProbeBucket.mu.Lock()
	instanceProbeBucket.tokens, instanceProbeBucket.updated = tokens, time.Now().Add(time.Hour)
	instanceProbeBucket.mu.Unlock()
	t.Cleanup(func() {
		instanceProbeBucket.mu.Lock()
		instanceProbeBucket.tokens, instanceProbeBucket.updated = 40, time.Now()
		instanceProbeBucket.mu.Unlock()
	})
}
func probeStore(t *testing.T) *storage.Store {
	t.Helper()
	s, err := storage.Open(context.Background(), storage.Config{Path: filepath.Join(t.TempDir(), "probe.sqlite")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func probeHTTP(t *testing.T, server *httptest.Server, method, suffix, body string, headers map[string]string, want int, code string) string {
	t.Helper()
	r, err := http.NewRequest(method, server.URL+"/api/v1/instance"+suffix, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range headers {
		r.Header[name] = []string{value}
	}
	response, err := server.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != want || response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("X-Content-Type-Options") != "nosniff" || response.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("status/headers: %d %v %s", response.StatusCode, response.Header, raw)
	}
	for _, name := range []string{"ETag", "Last-Modified", "Location"} {
		if response.Header.Get(name) != "" {
			t.Fatal("unexpected header", name)
		}
	}
	if want == 429 && response.Header.Get("Retry-After") != "1" {
		t.Fatal("missing retry interval")
	}
	if code != "" && method != "HEAD" {
		var envelope map[string]map[string]any
		if json.Unmarshal(raw, &envelope) != nil || len(envelope) != 1 || len(envelope["error"]) != 4 {
			t.Fatalf("invalid error schema: %s", raw)
		}
		e := envelope["error"]
		message := map[int]string{400: "请求无效", 405: "请求无效", 429: "请求过于频繁", 503: "实例暂不可用"}[want]
		if e["code"] != code || e["message"] != message || e["retryable"] != (want == 429 || want == 503) || e["request_id"] == "" || e["request_id"] == nil {
			t.Fatalf("invalid error: %s", raw)
		}
	}
	return string(raw)
}
func TestInstanceProbePersistentHTTP(t *testing.T) {
	resetProbeBucket(t, 40)
	s := probeStore(t)
	server := httptest.NewServer(http.HandlerFunc(NewInstanceHandler(s).GetInstance))
	defer server.Close()
	raw := probeHTTP(t, server, "GET", "", "", map[string]string{"If-None-Match": "*", "If-Modified-Since": time.Now().Format(http.TimeFormat)}, 200, "")
	if !regexp.MustCompile(`^\{"instance_id":"[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}"\}\n$`).MatchString(raw) {
		t.Fatalf("invalid success: %s", raw)
	}
	var id string
	if err := s.Read(context.Background(), func(tx *sql.Tx) error {
		var err error
		id, err = storage.ReadInstanceIDTx(context.Background(), tx)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, id) {
		t.Fatal("not persisted identity")
	}
	if err := s.Write(context.Background(), func(tx *sql.Tx) error { _, err := tx.Exec("DELETE FROM instance_identity"); return err }); err != nil {
		t.Fatal(err)
	}
	raw = probeHTTP(t, server, "GET", "", "", nil, 503, "INSTANCE_UNAVAILABLE")
	if strings.Contains(raw, id) || strings.Contains(raw, "sqlite") {
		t.Fatal("leaked identity/storage failure")
	}
}
func TestInstanceProbeInvalidBeforeLimitBeforeStorage(t *testing.T) {
	resetProbeBucket(t, 0)
	s := probeStore(t)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(NewInstanceHandler(s).GetInstance))
	defer server.Close()
	for _, header := range []string{"Authorization", "authorization", "COOKIE", "x-CsRf-ToKeN", "X-RelayNest-Instance-ID"} {
		for _, value := range []string{"", "private-value"} {
			probeHTTP(t, server, "GET", "", "", map[string]string{header: value}, 400, "INVALID_REQUEST")
		}
	}
	for _, suffix := range []string{"?", "?account=private-value", "?%invalid"} {
		probeHTTP(t, server, "GET", suffix, "", nil, 400, "INVALID_REQUEST")
	}
	probeHTTP(t, server, "GET", "", "private-value", nil, 400, "INVALID_REQUEST")
	probeHTTP(t, server, "POST", "", "", nil, 405, "INVALID_REQUEST")
	probeHTTP(t, server, "HEAD", "", "", nil, 405, "")
	probeHTTP(t, server, "GET", "", "", nil, 429, "RATE_LIMITED")
	resetProbeBucket(t, 1)
	probeHTTP(t, server, "GET", "", "", nil, 503, "INSTANCE_UNAVAILABLE")
}
func TestInstanceProbeSharedLimiter(t *testing.T) {
	resetProbeBucket(t, 40)
	s := probeStore(t)
	one := httptest.NewServer(http.HandlerFunc(NewInstanceHandler(s).GetInstance))
	defer one.Close()
	two := httptest.NewServer(http.HandlerFunc(NewInstanceHandler(s).GetInstance))
	defer two.Close()
	for i := 0; i < 40; i++ {
		server := one
		if i%2 == 1 {
			server = two
		}
		probeHTTP(t, server, "GET", "", "", nil, 200, "")
	}
	probeHTTP(t, one, "GET", "", "", nil, 429, "RATE_LIMITED")
	probeHTTP(t, two, "GET", "", "", nil, 429, "RATE_LIMITED")
	now := time.Now()
	b := probeBucket{updated: now}
	if b.allow(now) || !b.allow(now.Add(50*time.Millisecond)) || b.allow(now.Add(50*time.Millisecond)) {
		t.Fatal("incorrect refill rate")
	}
	for i := 0; i < 40; i++ {
		if !b.allow(now.Add(time.Hour)) {
			t.Fatal("burst too small")
		}
	}
	if b.allow(now.Add(time.Hour)) {
		t.Fatal("burst exceeded 40")
	}
}
func TestInstanceProbeCorruptAndMissingHTTP(t *testing.T) {
	for _, query := range []string{"DELETE FROM instance_identity", "PRAGMA ignore_check_constraints=ON; UPDATE instance_identity SET instance_id='private-corrupt-id'", "DROP TABLE instance_identity"} {
		t.Run(query, func(t *testing.T) {
			resetProbeBucket(t, 40)
			s := probeStore(t)
			if err := s.Write(context.Background(), func(tx *sql.Tx) error { _, err := tx.Exec(query); return err }); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(NewInstanceHandler(s).GetInstance))
			defer server.Close()
			probeHTTP(t, server, "GET", "", "", nil, 503, "INSTANCE_UNAVAILABLE")
		})
	}
}

func TestInstanceProbeHTTP2WithoutBody(t *testing.T) {
	resetProbeBucket(t, 40)
	server := httptest.NewUnstartedServer(http.HandlerFunc(NewInstanceHandler(probeStore(t)).GetInstance))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	probeHTTP(t, server, "GET", "", "", nil, 200, "")
	response, err := server.Client().Get(server.URL + "/api/v1/instance")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.ProtoMajor != 2 || response.StatusCode != 200 {
		t.Fatal("HTTP/2 probe failed", response.Proto, response.StatusCode)
	}
}

func TestInstanceProbeChunkedBody(t *testing.T) {
	resetProbeBucket(t, 40)
	server := httptest.NewServer(http.HandlerFunc(NewInstanceHandler(probeStore(t)).GetInstance))
	defer server.Close()
	req, err := http.NewRequest("GET", server.URL+"/api/v1/instance", io.NopCloser(strings.NewReader("private-body")))
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 400 {
		t.Fatal("chunked body accepted", response.StatusCode)
	}
}
