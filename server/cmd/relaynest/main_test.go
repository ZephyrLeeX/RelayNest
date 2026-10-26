package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"relaynest/server/internal/auth"
	"relaynest/server/internal/bots"
	"relaynest/server/internal/httpapi"
	"relaynest/server/internal/storage"
	"strings"
	"syscall"
	"testing"
	"time"

	"relaynest/server/internal/health"
)

func TestGracefulShutdownDrainsRequest(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	h := health.New(func(context.Context) error {
		close(entered)
		<-release
		return nil
	})
	done := make(chan error, 1)
	go func() { done <- serve(ctx, ln, h) }()
	response := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 3 * time.Second}
		res, err := client.Get("http://" + ln.Addr().String() + "/health/ready")
		if err == nil {
			_, err = io.Copy(io.Discard, res.Body)
			err = errors.Join(err, res.Body.Close())
		}
		response <- err
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("请求未到达")
	}
	cancel()
	select {
	case err := <-done:
		t.Fatalf("在途请求未完成服务已退出: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	if err := <-response; err != nil {
		t.Fatalf("请求未排空: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("服务未退出")
	}
	conn, err := net.DialTimeout("tcp", ln.Addr().String(), time.Second)
	if err == nil {
		conn.Close()
		t.Fatal("退出后仍接受连接")
	}
}

// 子进程直接执行生产 main，覆盖参数解析、admin 分派和信号处理。
func TestMainProcessHelper(t *testing.T) {
	if os.Getenv("RELAYNEST_MAIN_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"relaynest"}, os.Args[i+1:]...)
			main()
			os.Exit(0)
		}
	}
	os.Exit(2)
}

func mainProcess(args ...string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=^TestMainProcessHelper$", "--"}, args...)...)
	cmd.Env = append(os.Environ(), "RELAYNEST_MAIN_HELPER=1")
	return cmd
}

func TestMainProcessAuthentication(t *testing.T) {
	t.Run("explicit repeated proxies", func(t *testing.T) { testMainAuthentication(t, true) })
	t.Run("no implicit loopback trust", func(t *testing.T) { testMainAuthentication(t, false) })
}

func testMainAuthentication(t *testing.T, trusted bool) {
	path := filepath.Join(t.TempDir(), "main.sqlite")
	init := mainProcess("admin", "init", "-db", path, "-username", "Admin")
	init.Stdin = strings.NewReader("correct\n")
	if out, err := init.CombinedOutput(); err != nil {
		t.Fatalf("admin init: %v %s", err, out)
	}
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := reservation.Addr().String()
	reservation.Close()
	args := []string{"-listen", addr, "-db", path, "-external-origin", "https://relay.example"}
	if trusted {
		args = append(args, "-trusted-proxy", "127.0.0.1", "-trusted-proxy", "192.0.2.0/24")
	}
	cmd := mainProcess(args...)
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
	base := "http://" + addr
	deadline := time.Now().Add(10 * time.Second)
	for {
		res, err := client.Get(base + "/health/live")
		if err == nil {
			res.Body.Close()
			if res.StatusCode != 200 {
				t.Fatal(res.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("进程未监听")
		}
		time.Sleep(20 * time.Millisecond)
	}
	request := func(method, path, body string, trusted bool, cookie *http.Cookie) *http.Response {
		t.Helper()
		req, _ := http.NewRequest(method, base+path, strings.NewReader(body))
		req.Host = "relay.example"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://relay.example")
		if trusted {
			req.Header.Set("X-Forwarded-Proto", "https")
			req.Header.Set("X-Forwarded-Host", "relay.example")
			req.Header.Set("X-Forwarded-For", "192.0.2.10")
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { res.Body.Close() })
		return res
	}
	body := `{"username":"Admin","password":"correct","client_type":"web","device_name":"startup-test"}`
	if res := request("POST", "/api/v1/auth/login", body, false, nil); res.StatusCode != 403 {
		t.Fatalf("缺 HTTPS 来源: %d", res.StatusCode)
	}
	res := request("POST", "/api/v1/auth/login", body, true, nil)
	if !trusted {
		if res.StatusCode != 403 {
			t.Fatalf("隐式信任环回代理: %d", res.StatusCode)
		}
	} else {
		if res.StatusCode != 200 {
			raw, _ := io.ReadAll(res.Body)
			t.Fatalf("登录: %d %s", res.StatusCode, raw)
		}
		cookies := res.Cookies()
		if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly {
			t.Fatal("会话 Cookie")
		}
		if got := request("GET", "/api/v1/auth/session", "", true, cookies[0]); got.StatusCode != 200 {
			t.Fatalf("获取会话: %d", got.StatusCode)
		}
	}
	if got := request("GET", "/health/ready", "", false, nil); got.StatusCode != 503 {
		t.Fatalf("提前 ready: %d", got.StatusCode)
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		stopped = true
		if err != nil {
			t.Fatalf("关闭: %v %s", err, logs.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("未关闭")
	}
	if !strings.Contains(logs.String(), "shutdown_complete") {
		t.Fatal(logs.String())
	}
	if conn, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		conn.Close()
		t.Fatal("监听未释放")
	}
	store, err := storage.Open(context.Background(), storage.Config{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.EngineVersion(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestStartupConfigurationAndCleanup(t *testing.T) {
	dir := t.TempDir()
	valid := []string{"-db", filepath.Join(dir, "main.sqlite"), "-external-origin", "https://relay.example"}
	for _, args := range [][]string{nil, {"-db", filepath.Join(dir, "missing-origin.sqlite")}, {"-external-origin", "https://relay.example"}, append(append([]string{}, valid...), "-trusted-proxy", "invalid"), {"-db", filepath.Join(dir, "bad-origin.sqlite"), "-external-origin", "http://relay.example"}, append(append([]string{}, valid...), "unexpected")} {
		if err := runServer(context.Background(), args); err == nil {
			t.Fatalf("接受无效配置: %v", args)
		}
	}
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	if err := runServer(context.Background(), append(valid, "-listen", occupied.Addr().String())); err == nil || err.Error() != "listen_failed" {
		t.Fatalf("监听失败: %v", err)
	}
	assertNoDatabaseFDs(t, dir)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runServer(ctx, valid); err == nil {
		t.Fatal("取消的启动未拒绝")
	}
	assertNoDatabaseFDs(t, dir)
}

func assertNoDatabaseFDs(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		target, _ := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if strings.HasPrefix(target, dir+"/") {
			t.Fatalf("数据库资源未释放: %s", target)
		}
	}
}

func TestRunServerCancellationClosesDatabase(t *testing.T) {
	dir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runServer(ctx, []string{"-listen", addr, "-db", filepath.Join(dir, "cancel.sqlite"), "-external-origin", "https://relay.example"})
	}()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(5 * time.Second)
	for {
		res, err := client.Get("http://" + addr + "/health/live")
		if err == nil {
			res.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			cancel()
			<-done
			t.Fatal("未启动")
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("未关闭")
	}
	assertNoDatabaseFDs(t, dir)
	if conn, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		conn.Close()
		t.Fatal("监听未关闭")
	}
}

const botCreateBody = `{"name":"测试Bot","enabled":true,"capabilities":{"send":true,"receive":true}}`

type botHTTPFixture struct {
	t                *testing.T
	store            *storage.Store
	instanceID       string
	sessions         *auth.SessionStore
	server           *httptest.Server
	handler          *managementHandlers
	web, android     auth.SessionSecrets
	webID, androidID string
	authorize        httpapi.BotAuthorize
	afterAuthorize   func()
	beforeRead       func()
}

// 使用实际服务库的身份，并在故障注入前保存，避免夹具读库遮蔽原断言。
func fixtureInstanceID(t *testing.T, store *storage.Store) string {
	t.Helper()
	var id string
	if err := store.Read(context.Background(), func(tx *sql.Tx) error {
		var err error
		id, err = storage.ReadInstanceIDTx(context.Background(), tx)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

func botHTTP(t *testing.T, proxies []string) *botHTTPFixture {
	t.Helper()
	ts := httptest.NewTLSServer(http.NotFoundHandler())
	store, h, err := authenticationDependencies(context.Background(), filepath.Join(t.TempDir(), "bots.sqlite"), ts.URL, proxies)
	if err != nil {
		t.Fatal(err)
	}
	f := &botHTTPFixture{t: t, store: store, instanceID: fixtureInstanceID(t, store), server: ts, handler: h, sessions: auth.NewSessionStore(store)}
	t.Cleanup(func() { ts.Close(); store.Close() })
	if err := auth.InitAdministrator(context.Background(), store, "Admin", []byte("correct")); err != nil {
		t.Fatal(err)
	}
	a, err := auth.ReadAdministrator(context.Background(), store)
	if err != nil {
		t.Fatal(err)
	}
	for _, client := range []string{"web", "android"} {
		in := auth.SessionInput{ClientType: client, DeviceName: "测试设备"}
		if client == "android" {
			in.DeviceID = "00000000-0000-4000-8000-000000000001"
		}
		session, secrets, err := f.sessions.Create(context.Background(), a.AuthGeneration, in)
		if err != nil {
			t.Fatal(err)
		}
		if client == "web" {
			f.web = secrets
			f.webID = session.ID
		} else {
			f.android = secrets
			f.androidID = session.ID
		}
	}
	p, err := httpapi.NewOriginPolicy(ts.URL, proxies)
	if err != nil {
		t.Fatal(err)
	}
	f.authorize = botAuthorization(f.sessions, auth.NewRequestSecurity(f.sessions, p))
	// 请求入口和授权能力均为生产实现；只在明确边界插入真实 SQLite 状态变更。
	h.BotHandler = httpapi.NewBotHandler(bots.New(store), func(w http.ResponseWriter, r *http.Request, write bool) (httpapi.BotAccess, bool) {
		access, ok := f.authorize(w, r, write)
		if ok && f.afterAuthorize != nil {
			f.afterAuthorize()
		}
		if ok {
			read := access.Read
			access.Read = func(ctx context.Context, fn func(*sql.Tx) error) error {
				if f.beforeRead != nil {
					f.beforeRead()
				}
				return read(ctx, fn)
			}
		}
		return access, ok
	})
	ts.Config.Handler = httpapi.New(health.New(nil), h)
	return f
}
func (f *botHTTPFixture) request(method, path, body, kind, token string, headers map[string]string) (int, string, http.Header) {
	f.t.Helper()
	req, err := http.NewRequest(method, f.server.URL+path, strings.NewReader(body))
	if err != nil {
		f.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-RelayNest-Instance-ID", f.instanceID)
	if token != "" {
		if kind == "web" {
			req.AddCookie(&http.Cookie{Name: "relaynest_session", Value: token})
		} else {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	raw, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		f.t.Fatal(err)
	}
	return resp.StatusCode, string(raw), resp.Header
}
func (f *botHTTPFixture) admin(method, path, body string) (int, string, http.Header) {
	return f.request(method, path, body, "android", f.android.Token, nil)
}
func (f *botHTTPFixture) sql(q string, args ...any) {
	f.t.Helper()
	if err := f.store.Write(context.Background(), func(tx *sql.Tx) error { _, err := tx.Exec(q, args...); return err }); err != nil {
		f.t.Fatal(err)
	}
}
func (f *botHTTPFixture) snapshot() [32]byte {
	f.t.Helper()
	var all []any
	err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
		for _, table := range []string{"bots", "bot_credentials", "changes", "sessions", "administrator"} {
			rows, err := tx.Query("SELECT * FROM " + table + " ORDER BY 1")
			if err != nil {
				return err
			}
			cols, _ := rows.Columns()
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
				all = append(all, v)
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
		f.t.Fatal(err)
	}
	raw, _ := json.Marshal(all)
	return sha256.Sum256(raw)
}
func decodeBotCreated(t *testing.T, raw string) bots.Created {
	t.Helper()
	var c bots.Created
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatal(err)
	}
	return c
}
func assertBotResponse(t *testing.T, status, want int, raw string, h http.Header) {
	t.Helper()
	if status != want {
		t.Fatalf("status=%d want=%d", status, want)
	}
	if h.Get("Cache-Control") != "no-store" {
		t.Fatal("missing no-store")
	}
	var v map[string]any
	if json.Unmarshal([]byte(raw), &v) != nil {
		t.Fatal("invalid JSON")
	}
}
func TestBotHTTPProductionAssembly(t *testing.T) {
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(old)
	var issued []string
	f := botHTTP(t, nil)
	// 换回无测试边界的实际组装，确认真实 router 挂载两个 handler 能力。
	store, h, err := authenticationDependencies(context.Background(), filepath.Join(t.TempDir(), "assembled.sqlite"), f.server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := auth.InitAdministrator(context.Background(), store, "Admin", []byte("correct")); err != nil {
		t.Fatal(err)
	}
	// 生产装配更换了服务库，请求头必须对应这个库。
	f.instanceID = fixtureInstanceID(t, store)
	f.server.Config.Handler = httpapi.New(health.New(nil), h)
	for _, client := range []string{"web", "android"} {
		body := `{"username":"Admin","password":"correct","client_type":"` + client + `","device_name":"browser"`
		if client == "android" {
			body += `,"device_id":"00000000-0000-4000-8000-000000000001"`
		}
		body += `}`
		status, raw, header := f.request("POST", "/api/v1/auth/login", body, "", "", map[string]string{"Origin": f.server.URL})
		assertBotResponse(t, status, 200, raw, header)
		var login map[string]any
		json.Unmarshal([]byte(raw), &login)
		token := ""
		headers := map[string]string{}
		if client == "web" {
			resp := http.Response{Header: header}
			token = resp.Cookies()[0].Value
			headers["Origin"] = f.server.URL
			headers["X-CSRF-Token"] = login["csrf_token"].(string)
		} else {
			token = login["access_token"].(string)
		}
		status, raw, header = f.request("POST", "/api/v1/admin/bots", botCreateBody, client, token, headers)
		assertBotResponse(t, status, 201, raw, header)
		c := decodeBotCreated(t, raw)
		issued = append(issued, c.APIToken.Token, c.WebhookKey.Key)
		status, raw, header = f.request("GET", "/api/v1/admin/bots", "", client, token, nil)
		assertBotResponse(t, status, 200, raw, header)
		if strings.Contains(raw, c.APIToken.Token) || strings.Contains(raw, c.WebhookKey.Key) || strings.Contains(raw, "digest") {
			t.Fatal("list leaked credentials")
		}
		status, raw, header = f.request("PATCH", "/api/v1/admin/bots/"+c.Bot.ID, `{"expected_version":1,"enabled":false}`, client, token, headers)
		assertBotResponse(t, status, 200, raw, header)
		if strings.Contains(raw, c.APIToken.Token) || strings.Contains(raw, c.WebhookKey.Key) || strings.Contains(raw, "digest") {
			t.Fatal("patch leaked credentials")
		}
	}
	for _, secret := range issued {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("logs leaked issued credential")
		}
	}
}
func TestBotHTTPAuthorizationAndCredentialMatrix(t *testing.T) {
	f := botHTTP(t, nil)
	paths := []struct{ method, path, body string }{{"GET", "/api/v1/admin/bots", ""}, {"POST", "/api/v1/admin/bots", botCreateBody}, {"PATCH", "/api/v1/admin/bots/00000000-0000-4000-8000-000000000001", `{"expected_version":1}`}}
	secrets := []string{""}
	for _, cap := range []bots.Capabilities{{}, {Send: true}, {Receive: true}, {Send: true, Receive: true}} {
		c, _, err := bots.New(f.store).Create(context.Background(), bots.CreateRequest{Name: "matrix", Enabled: true, Capabilities: cap})
		if err != nil {
			t.Fatal(err)
		}
		secrets = append(secrets, c.APIToken.Token, c.WebhookKey.Key)
	}
	for _, token := range secrets {
		for _, path := range paths {
			for _, kind := range []string{"android", "web"} {
				before := f.snapshot()
				status, raw, h := f.request(path.method, path.path, path.body, kind, token, map[string]string{"Origin": f.server.URL, "X-CSRF-Token": f.web.CSRFToken})
				assertBotResponse(t, status, 401, raw, h)
				if before != f.snapshot() {
					t.Fatal("denied request changed database")
				}
				if token != "" && strings.Contains(raw, token) {
					t.Fatal("error leaked credential")
				}
			}
		}
	}
	for _, state := range []string{"revoked=1", "idle_expires_at=0", "absolute_expires_at=idle_expires_at-1, idle_expires_at=0"} {
		t.Run(state, func(t *testing.T) {
			g := botHTTP(t, nil)
			g.sql("UPDATE sessions SET " + state)
			for _, path := range paths {
				before := g.snapshot()
				for _, kind := range []string{"android", "web"} {
					token := g.android.Token
					if kind == "web" {
						token = g.web.Token
					}
					status, raw, h := g.request(path.method, path.path, path.body, kind, token, map[string]string{"Origin": g.server.URL, "X-CSRF-Token": g.web.CSRFToken})
					assertBotResponse(t, status, 401, raw, h)
					if before != g.snapshot() {
						t.Fatal("invalid session changed database")
					}
				}
			}
		})
	}
	// 尚不存在的业务入口仅证明确切 404，不计为发送业务授权通过。
	for _, path := range []string{"/api/v1/bots/unknown/messages", "/api/v1/webhooks/unknown", "/api/v1/admin/bots/unknown/messages"} {
		status, _, _ := f.admin("POST", path, `{}`)
		if status != 404 {
			t.Fatalf("unknown route status=%d", status)
		}
	}
}

func TestBotHTTPWebSecurity(t *testing.T) {
	for _, trusted := range []bool{false, true} {
		t.Run(map[bool]string{false: "untrusted", true: "trusted"}[trusted], func(t *testing.T) {
			var proxies []string
			if trusted {
				proxies = []string{"127.0.0.1"}
			}
			f := botHTTP(t, proxies)
			status, raw, h := f.admin("POST", "/api/v1/admin/bots", botCreateBody)
			assertBotResponse(t, status, 201, raw, h)
			id := decodeBotCreated(t, raw).Bot.ID
			for _, method := range []string{"POST", "PATCH"} {
				path, body := "/api/v1/admin/bots", botCreateBody
				if method == "PATCH" {
					path += "/" + id
					body = `{"expected_version":1}`
				}
				for _, tc := range []struct {
					name    string
					headers map[string]string
					want    int
				}{
					{"missing csrf", map[string]string{"Origin": f.server.URL}, 403},
					{"bad csrf", map[string]string{"Origin": f.server.URL, "X-CSRF-Token": "bad"}, 403},
					{"missing source", map[string]string{"X-CSRF-Token": f.web.CSRFToken}, 403},
					{"cross origin", map[string]string{"Origin": "https://evil.example", "X-CSRF-Token": f.web.CSRFToken}, 403},
					{"wrong port", map[string]string{"Origin": "https://127.0.0.1:1", "X-CSRF-Token": f.web.CSRFToken}, 403},
					{"wrong scheme", map[string]string{"Origin": strings.Replace(f.server.URL, "https:", "http:", 1), "X-CSRF-Token": f.web.CSRFToken}, 403},
					{"bad origin no fallback", map[string]string{"Origin": "null", "Referer": f.server.URL + "/", "X-CSRF-Token": f.web.CSRFToken}, 403},
					{"bad referer", map[string]string{"Referer": "https://evil.example/path", "X-CSRF-Token": f.web.CSRFToken}, 403},
				} {
					before := f.snapshot()
					status, raw, h := f.request(method, path, body, "web", f.web.Token, tc.headers)
					assertBotResponse(t, status, tc.want, raw, h)
					if before != f.snapshot() {
						t.Fatal(tc.name + " changed database")
					}
				}
			}
			// 有效 Referer 回退成功；可信代理的外部端口和 scheme 必须匹配。
			headers := map[string]string{"Referer": f.server.URL + "/management", "X-CSRF-Token": f.web.CSRFToken}
			status, raw, h = f.request("POST", "/api/v1/admin/bots", botCreateBody, "web", f.web.Token, headers)
			assertBotResponse(t, status, 201, raw, h)
			headers = map[string]string{"Origin": f.server.URL, "X-CSRF-Token": f.web.CSRFToken, "X-Forwarded-Host": "evil.example", "X-Forwarded-Proto": "http"}
			want := 201
			if trusted {
				want = 403
			}
			status, raw, h = f.request("POST", "/api/v1/admin/bots", botCreateBody, "web", f.web.Token, headers)
			assertBotResponse(t, status, want, raw, h)
			if trusted {
				headers["X-Forwarded-Host"] = strings.TrimPrefix(f.server.URL, "https://")
				headers["X-Forwarded-Proto"] = "https"
				headers["X-Forwarded-For"] = "203.0.113.8"
				status, raw, h = f.request("POST", "/api/v1/admin/bots", botCreateBody, "web", f.web.Token, headers)
				assertBotResponse(t, status, 201, raw, h)
			}
		})
	}
}
func TestBotHTTPActorRecheckAndRollback(t *testing.T) {
	for _, method := range []string{"GET", "POST", "PATCH"} {
		for _, mutation := range []string{"revoke", "idle", "absolute", "generation", "session generation"} {
			t.Run(method+"/"+mutation, func(t *testing.T) {
				f := botHTTP(t, nil)
				status, raw, h := f.admin("POST", "/api/v1/admin/bots", botCreateBody)
				assertBotResponse(t, status, 201, raw, h)
				c := decodeBotCreated(t, raw)
				path, body := "/api/v1/admin/bots", ""
				if method == "POST" {
					body = botCreateBody
				}
				if method == "PATCH" {
					path += "/" + c.Bot.ID
					body = `{"expected_version":1,"name":"changed"}`
				}
				var before [32]byte
				mutate := func() {
					switch mutation {
					case "revoke":
						f.sql("UPDATE sessions SET revoked=1 WHERE id=?", f.androidID)
					case "idle":
						f.sql("UPDATE sessions SET idle_expires_at=0 WHERE id=?", f.androidID)
					case "absolute":
						f.sql("UPDATE sessions SET absolute_expires_at=1,idle_expires_at=0 WHERE id=?", f.androidID)
					case "generation":
						f.sql("UPDATE administrator SET auth_generation=auth_generation+1")
					case "session generation":
						f.sql("UPDATE sessions SET auth_generation=auth_generation+1 WHERE id=?", f.androidID)
					}
					before = f.snapshot()
				}
				if method == "GET" {
					f.beforeRead = mutate
				} else {
					f.afterAuthorize = mutate
				}
				status, raw, h = f.admin(method, path, body)
				assertBotResponse(t, status, 401, raw, h)
				if before != f.snapshot() {
					t.Fatal("actor recheck left side effects")
				}
				if strings.Contains(raw, c.APIToken.Token) || strings.Contains(raw, c.WebhookKey.Key) || strings.Contains(raw, "items") || strings.Contains(raw, "\"bot\"") {
					t.Fatal("failed authorization returned result")
				}
			})
		}
	}
	for _, method := range []string{"POST", "PATCH"} {
		for _, failure := range []string{"business", "credentials", "changes", "renewal", "final authorization", "commit"} {
			t.Run(method+"/"+failure, func(t *testing.T) {
				f := botHTTP(t, nil)
				status, raw, h := f.admin("POST", "/api/v1/admin/bots", botCreateBody)
				assertBotResponse(t, status, 201, raw, h)
				c := decodeBotCreated(t, raw)
				path, body := "/api/v1/admin/bots", botCreateBody
				verb := "INSERT"
				if method == "PATCH" {
					path += "/" + c.Bot.ID
					body = `{"expected_version":1,"enabled":false}`
					verb = "UPDATE"
				}
				switch failure {
				case "business":
					f.sql("CREATE TRIGGER fail BEFORE " + verb + " ON bots BEGIN SELECT RAISE(ABORT,'private business error'); END")
				case "credentials":
					if method == "PATCH" {
						return
					}
					f.sql("CREATE TRIGGER fail BEFORE INSERT ON bot_credentials BEGIN SELECT RAISE(ABORT,'private credential error'); END")
				case "changes":
					f.sql("CREATE TRIGGER fail BEFORE INSERT ON changes BEGIN SELECT RAISE(ABORT,'private changes error'); END")
				case "renewal":
					f.sql("CREATE TRIGGER fail BEFORE UPDATE OF idle_expires_at ON sessions BEGIN SELECT RAISE(ABORT,'private renewal error'); END")
				case "final authorization":
					f.sql("CREATE TRIGGER fail AFTER " + verb + " ON bots BEGIN UPDATE sessions SET revoked=1; END")
				case "commit":
					f.sql("CREATE TABLE deferred_check(id INTEGER REFERENCES bots(id) DEFERRABLE INITIALLY DEFERRED)")
					f.sql("CREATE TRIGGER fail AFTER " + verb + " ON bots BEGIN INSERT INTO deferred_check VALUES('missing'); END")
				}
				before := f.snapshot()
				status, raw, h = f.admin(method, path, body)
				want := 503
				if failure == "final authorization" {
					want = 401
				}
				assertBotResponse(t, status, want, raw, h)
				if before != f.snapshot() {
					t.Fatal("failed transaction left side effects")
				}
				if strings.Contains(raw, "private") || strings.Contains(raw, "token") || strings.Contains(raw, "digest") || strings.Contains(raw, "webhook_key") {
					t.Fatal("failure leaked internals")
				}
			})
		}
	}
}
func TestBotHTTPContractAndPagination(t *testing.T) {
	f := botHTTP(t, nil)
	var created []bots.Created
	for i := 0; i < 3; i++ {
		status, raw, h := f.admin("POST", "/api/v1/admin/bots", botCreateBody)
		assertBotResponse(t, status, 201, raw, h)
		created = append(created, decodeBotCreated(t, raw))
		var top map[string]any
		json.Unmarshal([]byte(raw), &top)
		if len(top) != 3 {
			t.Fatal("create response keys")
		}
	}
	seen := map[string]bool{}
	path := "/api/v1/admin/bots?limit=1"
	for {
		status, raw, h := f.admin("GET", path, "")
		assertBotResponse(t, status, 200, raw, h)
		var page struct {
			Items []bots.Bot `json:"items"`
			Next  *string    `json:"next_cursor"`
			More  bool       `json:"has_more"`
		}
		if err := json.Unmarshal([]byte(raw), &page); err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 1 || seen[page.Items[0].ID] {
			t.Fatal("invalid page")
		}
		seen[page.Items[0].ID] = true
		if !page.More {
			if page.Next != nil {
				t.Fatal("terminal cursor")
			}
			break
		}
		if page.Next == nil {
			t.Fatal("missing cursor")
		}
		path = "/api/v1/admin/bots?limit=1&cursor=" + *page.Next
	}
	if len(seen) != 3 {
		t.Fatal("missing bots")
	}
	c := created[0]
	for _, tc := range []struct {
		method, path, body string
		status             int
		code               string
	}{
		{"PATCH", "/api/v1/admin/bots/" + c.Bot.ID, `{"expected_version":1,"name":"updated"}`, 200, ""},
		{"PATCH", "/api/v1/admin/bots/" + c.Bot.ID, `{"expected_version":1,"name":"stale"}`, 409, "VERSION_CONFLICT"},
		{"PATCH", "/api/v1/admin/bots/00000000-0000-4000-8000-000000000001", `{"expected_version":1}`, 404, "RESOURCE_NOT_FOUND"},
		{"PATCH", "/api/v1/admin/bots/" + c.Bot.ID, `{"expected_version":0}`, 422, "VALIDATION_FAILED"},
		{"PATCH", "/api/v1/admin/bots/" + c.Bot.ID, `{"expected_version":2,"name":null}`, 422, "VALIDATION_FAILED"},
		{"POST", "/api/v1/admin/bots", `{"name":"","enabled":true,"capabilities":{"send":true,"receive":false}}`, 422, "VALIDATION_FAILED"},
		{"POST", "/api/v1/admin/bots", `{"name":"missing"}`, 422, "VALIDATION_FAILED"},
		{"POST", "/api/v1/admin/bots", `{"name":"missing flag","enabled":true,"capabilities":{"send":true}}`, 422, "VALIDATION_FAILED"},
		{"POST", "/api/v1/admin/bots", botCreateBody + ` {}`, 400, "INVALID_REQUEST"},
		{"POST", "/api/v1/admin/bots", `{"unknown":1}`, 400, "INVALID_REQUEST"},
		{"POST", "/api/v1/admin/bots", `{`, 400, "INVALID_REQUEST"},
		{"POST", "/api/v1/admin/bots", `null`, 400, "INVALID_REQUEST"},
		{"POST", "/api/v1/admin/bots", `{"name":"` + strings.Repeat("a", 262144) + `"}`, 413, "PAYLOAD_TOO_LARGE"},
		{"GET", "/api/v1/admin/bots?limit=0", "", 400, "INVALID_REQUEST"},
		{"GET", "/api/v1/admin/bots?limit=bad", "", 400, "INVALID_REQUEST"},
		{"GET", "/api/v1/admin/bots?limit=1&limit=2", "", 400, "INVALID_REQUEST"},
		{"GET", "/api/v1/admin/bots?cursor=", "", 400, "INVALID_REQUEST"},
		{"GET", "/api/v1/admin/bots?cursor=bad", "", 400, "INVALID_REQUEST"},
	} {
		before := f.snapshot()
		status, raw, h := f.admin(tc.method, tc.path, tc.body)
		assertBotResponse(t, status, tc.status, raw, h)
		if tc.status >= 400 {
			if before != f.snapshot() {
				t.Fatal("invalid request changed database")
			}
			if !strings.Contains(raw, `"code":"`+tc.code+`"`) {
				t.Fatal("incorrect error code")
			}
		}
	}
	// 直接检查真实数据库中的所有字段，包括摘要 BLOB；输出只保留断言结果。
	err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
		for _, table := range []string{"bots", "bot_credentials", "changes", "sessions"} {
			rows, err := tx.Query("SELECT * FROM " + table)
			if err != nil {
				return err
			}
			cols, _ := rows.Columns()
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
				var fields []string
				for _, value := range v {
					switch value := value.(type) {
					case string:
						fields = append(fields, value)
					case []byte:
						fields = append(fields, string(value))
					}
				}
				b := []byte(strings.Join(fields, "\x00"))
				for _, c := range created {
					if strings.Contains(string(b), c.APIToken.Token) || strings.Contains(string(b), c.WebhookKey.Key) {
						t.Fatal("database contains plaintext")
					}
				}
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
}

func TestBotHTTPExactJSONKeys(t *testing.T) {
	for _, kind := range []string{"android", "web"} {
		for _, method := range []string{"POST", "PATCH"} {
			var cases []struct {
				key, alias string
				replace    bool
			}
			keys := []string{"name", "enabled", "capabilities", "send", "receive"}
			if method == "PATCH" {
				keys = append(keys, "expected_version")
			}
			keys = append(keys, "unknown", "nested_unknown")
			for _, key := range keys {
				aliases := []string{strings.ToUpper(key[:1]) + key[1:], strings.ToUpper(key)}
				if strings.Contains(key, "unknown") {
					aliases = []string{key}
				}
				for _, alias := range aliases {
					for _, replace := range []bool{false, true} {
						if strings.Contains(key, "unknown") && replace {
							continue
						}
						cases = append(cases, struct {
							key, alias string
							replace    bool
						}{key, alias, replace})
					}
				}
			}
			for _, tc := range cases {
				key, alias, replace := tc.key, tc.alias, tc.replace
				t.Run(fmt.Sprintf("%s/%s/%s/replace=%t", kind, method, alias, replace), func(t *testing.T) {
					f := botHTTP(t, nil)
					token, actorID := f.android.Token, f.androidID
					var headers map[string]string
					if kind == "web" {
						token, actorID = f.web.Token, f.webID
						headers = map[string]string{"Origin": f.server.URL, "X-CSRF-Token": f.web.CSRFToken}
					}
					path := "/api/v1/admin/bots"
					body := botCreateBody
					var created bots.Created
					if method == "PATCH" {
						status, raw, h := f.request("POST", path, body, kind, token, headers)
						assertBotResponse(t, status, 201, raw, h)
						created = decodeBotCreated(t, raw)
						path += "/" + created.Bot.ID
						body = `{"expected_version":1,"name":"updated","enabled":false,"capabilities":{"send":false,"receive":true}}`
					}
					var fields map[string]any
					if err := json.Unmarshal([]byte(body), &fields); err != nil {
						t.Fatal(err)
					}
					target := fields
					if key == "send" || key == "receive" || key == "nested_unknown" {
						target = fields["capabilities"].(map[string]any)
					}
					if strings.Contains(key, "unknown") {
						target[alias] = true
					} else {
						target[alias] = target[key]
						if replace {
							delete(target, key)
						}
					}
					invalid, err := json.Marshal(fields)
					if err != nil {
						t.Fatal(err)
					}
					// 缩短 actor 空闲期限，确保错误路径若续期会被五表快照捕获。
					f.sql("UPDATE sessions SET idle_expires_at=? WHERE id=?", time.Now().Add(time.Hour).UnixNano(), actorID)
					before := f.snapshot()
					status, raw, h := f.request(method, path, string(invalid), kind, token, headers)
					assertBotResponse(t, status, 400, raw, h)
					var envelope map[string]json.RawMessage
					if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
						t.Fatal(err)
					}
					if len(envelope) != 1 || envelope["error"] == nil || !strings.Contains(raw, `"code":"INVALID_REQUEST"`) {
						t.Fatal("incorrect error envelope")
					}
					for _, secret := range []string{f.android.Token, f.web.Token, f.web.CSRFToken, created.APIToken.Token, created.WebhookKey.Key} {
						if secret != "" && strings.Contains(raw, secret) {
							t.Fatal("error leaked credential")
						}
					}
					if before != f.snapshot() {
						t.Fatal("invalid key changed database or renewed session")
					}
					// 原始合法请求立即成功；PATCH 仍使用拒绝前的 expected_version=1。
					status, raw, h = f.request(method, path, body, kind, token, headers)
					want := 201
					if method == "PATCH" {
						want = 200
						var updated bots.Bot
						if err := json.Unmarshal([]byte(raw), &updated); err != nil {
							t.Fatal(err)
						}
						if updated.Version != 2 || updated.Name != "updated" || updated.Enabled || updated.Capabilities.Send || !updated.Capabilities.Receive {
							t.Fatal("legal patch did not apply exact fields")
						}
					} else {
						c := decodeBotCreated(t, raw)
						if c.Bot.Version != 1 || c.Bot.Name != "测试Bot" || !c.Bot.Enabled || !c.Bot.Capabilities.Send || !c.Bot.Capabilities.Receive || c.APIToken.Token == "" || c.WebhookKey.Key == "" {
							t.Fatal("legal create did not apply exact fields")
						}
					}
					assertBotResponse(t, status, want, raw, h)
				})
			}
		}
	}
}

func TestBotHTTPExactJSONKeyOccurrences(t *testing.T) {
	for _, method := range []string{"POST", "PATCH"} {
		t.Run(method, func(t *testing.T) {
			f := botHTTP(t, nil)
			path := "/api/v1/admin/bots"
			prefix := ""
			if method == "PATCH" {
				status, raw, h := f.admin("POST", path, botCreateBody)
				assertBotResponse(t, status, 201, raw, h)
				path += "/" + decodeBotCreated(t, raw).Bot.ID
				prefix = `"expected_version":1,`
			}
			for _, fields := range []string{
				`"capabilities":{"Send":false},"name":"valid","enabled":true,"capabilities":{"send":true,"receive":true}`,
				`"N\u0061me":"alias","name":"valid","enabled":true,"capabilities":{"send":true,"receive":true}`,
				`"name":"valid","enabled":true,"capabilities":{"S\u0065nd":true,"send":true,"receive":true}`,
			} {
				f.sql("UPDATE sessions SET idle_expires_at=? WHERE id=?", time.Now().Add(time.Hour).UnixNano(), f.androidID)
				before := f.snapshot()
				status, raw, h := f.admin(method, path, "{"+prefix+fields+"}")
				assertBotResponse(t, status, 400, raw, h)
				if !strings.Contains(raw, `"code":"INVALID_REQUEST"`) || before != f.snapshot() {
					t.Fatal("undeclared key occurrence was not rejected without side effects")
				}
			}
			// 精确重复键和转义后的精确合法键仍按原解码语义成功，不新增重复键政策。
			body := "{" + prefix + `"name":"first","na\u006de":"last","enabled":false,"enabled":true,"capabilities":{"send":false},"capabilities":{"receive":true,"send":true}}`
			status, raw, h := f.admin(method, path, body)
			want := 201
			var result bots.Bot
			if method == "POST" {
				result = decodeBotCreated(t, raw).Bot
			} else {
				want = 200
				if err := json.Unmarshal([]byte(raw), &result); err != nil {
					t.Fatal(err)
				}
			}
			assertBotResponse(t, status, want, raw, h)
			if result.Name != "last" || !result.Enabled || !result.Capabilities.Send || !result.Capabilities.Receive {
				t.Fatal("legal duplicate field semantics changed")
			}
		})
	}
}

func TestBotHTTPSuccessRenewsOnlyActor(t *testing.T) {
	for _, kind := range []string{"android", "web"} {
		t.Run(kind, func(t *testing.T) {
			f := botHTTP(t, nil)
			status, raw, h := f.admin("POST", "/api/v1/admin/bots", botCreateBody)
			assertBotResponse(t, status, 201, raw, h)
			c := decodeBotCreated(t, raw)
			readIdle := func(id string) int64 {
				t.Helper()
				var n int64
				if err := f.store.Read(context.Background(), func(tx *sql.Tx) error {
					return tx.QueryRow("SELECT idle_expires_at FROM sessions WHERE id=?", id).Scan(&n)
				}); err != nil {
					t.Fatal(err)
				}
				return n
			}
			actorID, otherID, token := f.androidID, f.webID, f.android.Token
			headers := map[string]string(nil)
			if kind == "web" {
				actorID, otherID, token = f.webID, f.androidID, f.web.Token
				headers = map[string]string{"Origin": f.server.URL, "X-CSRF-Token": f.web.CSRFToken}
			}
			for _, method := range []string{"GET", "POST", "PATCH"} {
				deadline := time.Now().Add(time.Hour).UnixNano()
				f.sql("UPDATE sessions SET idle_expires_at=? WHERE id=?", deadline, actorID)
				other := readIdle(otherID)
				path, body, want := "/api/v1/admin/bots", "", 200
				if method == "POST" {
					body = botCreateBody
					want = 201
				}
				if method == "PATCH" {
					path += "/" + c.Bot.ID
					body = `{"expected_version":1,"enabled":false}`
				}
				status, raw, h = f.request(method, path, body, kind, token, headers)
				assertBotResponse(t, status, want, raw, h)
				if readIdle(actorID) <= deadline || readIdle(otherID) != other {
					t.Fatal("success renewal target incorrect")
				}
			}
		})
	}
}

// 实际启动装配与真实登录验证同安装会话并存、精确撤销和重新登录代次。
func TestDeviceHTTPProductionAssembly(t *testing.T) {
	f := botHTTP(t, nil)
	dbPath := filepath.Join(t.TempDir(), "devices.sqlite")
	store, h, err := authenticationDependencies(context.Background(), dbPath, f.server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := auth.InitAdministrator(context.Background(), store, "Admin", []byte("correct")); err != nil {
		t.Fatal(err)
	}
	// 生产装配更换了服务库，请求头必须对应这个库。
	f.instanceID = fixtureInstanceID(t, store)
	f.server.Config.Handler = httpapi.New(health.New(nil), h)
	type identity struct{ id, token, csrf, client string }
	login := func(client, install string) identity {
		t.Helper()
		body := map[string]string{"username": "Admin", "password": "correct", "client_type": client, "device_name": "设备"}
		if install != "" {
			body["device_id"] = install
		}
		data, _ := json.Marshal(body)
		status, raw, header := f.request("POST", "/api/v1/auth/login", string(data), "", "", map[string]string{"Origin": f.server.URL})
		assertBotResponse(t, status, 200, raw, header)
		var p struct {
			Session struct {
				ID string `json:"id"`
			} `json:"session"`
			Token string `json:"access_token"`
			CSRF  string `json:"csrf_token"`
		}
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			t.Fatal(err)
		}
		token := p.Token
		if client == "web" {
			response := http.Response{Header: header}
			token = response.Cookies()[0].Value
		}
		if p.Session.ID == "" || token == "" {
			t.Fatal("登录缺少身份")
		}
		return identity{p.Session.ID, token, p.CSRF, client}
	}
	request := func(want int, method, path string, who identity) (string, http.Header) {
		t.Helper()
		headers := map[string]string{}
		if who.client == "web" {
			headers["Origin"] = f.server.URL
			headers["X-CSRF-Token"] = who.csrf
		}
		status, raw, header := f.request(method, path, "", who.client, who.token, headers)
		if want == http.StatusNoContent {
			if status != want || raw != "" || header.Get("Cache-Control") != "no-store" {
				t.Fatal("204 响应必须为空且禁止缓存")
			}
		} else {
			assertBotResponse(t, status, want, raw, header)
		}
		if want == 401 {
			code := "AUTHENTICATION_FAILED"
			if method == "DELETE" && strings.HasPrefix(path, "/api/v1/devices/") {
				code = "AUTHENTICATION_REQUIRED"
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if json.Unmarshal([]byte(raw), &body) != nil || body.Error.Code != code || header.Get("Set-Cookie") != "" {
				t.Fatalf("401机器码或Cookie错误: %s", raw)
			}
		}
		return raw, header
	}
	const install = "00000000-0000-4000-8000-000000000001"
	a := login("web", "")
	s1 := login("android", install)
	s2 := login("android", install)
	b := login("android", "00000000-0000-4000-8000-000000000002")
	for _, who := range []identity{{}, {client: "android", token: "invalid"}} {
		request(401, "DELETE", "/api/v1/devices/"+s1.id+"/session", who)
		request(401, "GET", "/api/v1/devices", who)
		request(401, "GET", "/api/v1/auth/session", who)
		request(401, "POST", "/api/v1/auth/logout", who)
		request(401, "GET", "/api/v1/admin/bots", who)
	}
	status, botRaw, botHeader := f.request("POST", "/api/v1/admin/bots", botCreateBody, a.client, a.token, map[string]string{"Origin": f.server.URL, "X-CSRF-Token": a.csrf})
	assertBotResponse(t, status, 201, botRaw, botHeader)
	bot := decodeBotCreated(t, botRaw)
	for _, credential := range []string{bot.APIToken.Token, bot.WebhookKey.Key} {
		before := func() string {
			var state string
			if err := store.Read(context.Background(), func(tx *sql.Tx) error {
				return tx.QueryRow("SELECT group_concat(id || ':' || revoked || ':' || idle_expires_at) FROM sessions ORDER BY id").Scan(&state)
			}); err != nil {
				t.Fatal(err)
			}
			return state
		}
		original := before()
		request(401, "GET", "/api/v1/devices", identity{token: credential, client: "android"})
		request(401, "DELETE", "/api/v1/devices/"+s1.id+"/session", identity{token: credential, client: "android"})
		for _, route := range []struct{ method, path string }{{"GET", "/api/v1/auth/session"}, {"POST", "/api/v1/auth/logout"}, {"GET", "/api/v1/admin/bots"}} {
			request(401, route.method, route.path, identity{token: credential, client: "android"})
		}
		if before() != original {
			t.Fatal("Bot凭据拒绝后改变管理员会话")
		}
	}
	raw, _ := request(200, "GET", "/api/v1/devices", a)
	var page struct {
		Items []map[string]any `json:"items"`
		Next  *string          `json:"next_cursor"`
		More  bool             `json:"has_more"`
	}
	if err := json.Unmarshal([]byte(raw), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 4 || page.More || page.Next != nil {
		t.Fatal("列表丢失会话")
	}
	for _, item := range page.Items {
		if len(item) != 7 {
			t.Fatal("非冻结Session字段")
		}
		admin, ok := item["administrator"].(map[string]any)
		if !ok || len(admin) != 1 || admin["username"] != "Admin" {
			t.Fatal("管理员投影错误")
		}
	}
	for _, secret := range []string{a.token, a.csrf, s1.token, s2.token, b.token, bot.APIToken.Token, bot.WebhookKey.Key} {
		if secret != "" && strings.Contains(raw, secret) {
			t.Fatal("泄露真实凭据")
		}
	}
	if err := store.Read(context.Background(), func(tx *sql.Tx) error {
		rows, err := tx.Query("SELECT hex(token_hash) FROM sessions")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var hash string
			if err := rows.Scan(&hash); err != nil {
				return err
			}
			if strings.Contains(strings.ToLower(raw), strings.ToLower(hash)) {
				t.Error("泄露持久化凭据摘要")
			}
		}
		return rows.Err()
	}); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	path := "/api/v1/devices?limit=1"
	for i := 0; i < 4; i++ {
		raw, _ = request(200, "GET", path, a)
		if err := json.Unmarshal([]byte(raw), &page); err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 1 {
			t.Fatal("分页项数错误")
		}
		id := page.Items[0]["id"].(string)
		if seen[id] {
			t.Fatal("重复会话")
		}
		seen[id] = true
		if !page.More {
			if i != 3 || page.Next != nil {
				t.Fatal("分页提前结束")
			}
			break
		}
		if page.Next == nil {
			t.Fatal("缺cursor")
		}
		path = "/api/v1/devices?limit=1&cursor=" + *page.Next
	}
	for _, who := range []identity{a, s1, s2, b} {
		if !seen[who.id] {
			t.Fatal("分页缺会话")
		}
	}
	raw, header := request(204, "DELETE", "/api/v1/devices/"+s1.id+"/session", a)
	if raw != "" || header.Get("Set-Cookie") != "" {
		t.Fatal("远程撤销响应错误")
	}
	request(401, "GET", "/api/v1/devices", s1)
	for _, who := range []identity{a, s2, b} {
		request(200, "GET", "/api/v1/devices", who)
	}
	replacement := login("android", install)
	if replacement.id == s1.id || replacement.id == s2.id {
		t.Fatal("新登录复用身份")
	}
	request(404, "DELETE", "/api/v1/devices/"+s1.id+"/session", a)
	request(200, "GET", "/api/v1/devices", replacement)
	_, header = request(204, "DELETE", "/api/v1/devices/"+s2.id+"/session", s2)
	if header.Get("Set-Cookie") != "" {
		t.Fatal("Android清Cookie")
	}
	request(401, "GET", "/api/v1/devices", s2)
	_, header = request(204, "DELETE", "/api/v1/devices/"+a.id+"/session", a)
	response := http.Response{Header: header}
	cookies := response.Cookies()
	if len(cookies) != 1 || cookies[0].Value != "" || cookies[0].MaxAge != -1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatal("自撤Cookie属性错误")
	}
	request(401, "GET", "/api/v1/devices", a)
	for _, who := range []identity{b, replacement} {
		request(200, "GET", "/api/v1/devices", who)
	}
	// 关闭并从同一文件重新装配，旧撤销状态和独立有效会话必须持久保留。
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, handlers, err := authenticationDependencies(context.Background(), dbPath, f.server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	f.server.Config.Handler = httpapi.New(health.New(nil), handlers)
	request(401, "GET", "/api/v1/devices", s1)
	request(401, "GET", "/api/v1/devices", a)
	for _, who := range []identity{b, replacement} {
		request(200, "GET", "/api/v1/devices", who)
	}
}

// 使用生产装配与真实路由，防止仅直接调用 handler 的验证遗漏接线。
func TestAssembledBotMessageHTTP(t *testing.T) {
	store, handlers, err := authenticationDependencies(context.Background(), filepath.Join(t.TempDir(), "bot-message.sqlite"), "https://relay.example", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	created, _, err := bots.New(store).Create(context.Background(), bots.CreateRequest{Name: "HTTP bot", Enabled: true, Capabilities: bots.Capabilities{Send: true}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(httpapi.New(health.New(nil), handlers))
	defer server.Close()
	request, err := http.NewRequest("POST", server.URL+"/api/v1/bot/messages", strings.NewReader(`{"format":"plain","text":"production wiring"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+created.APIToken.Token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "abcdefff-1234-4567-89ab-123456789abc")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 201 || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("%d %s", response.StatusCode, data)
	}
	var out map[string]any
	if json.Unmarshal(data, &out) != nil || out["bot_id"] != created.Bot.ID || out["bot_seq"] != "1" || out["sender"] != "application" {
		t.Fatalf("invalid receipt %s", data)
	}
}

// 八条真实生产路由逐项核对实例门禁；无效凭据不能盖过实例错误。
func TestInstanceBindingEightProductionRoutes(t *testing.T) {
	f := botHTTP(t, nil)
	routes := []struct{ method, path, body string }{
		{"POST", "/api/v1/auth/login", `{"username":"Admin","password":"wrong","client_type":"android","device_name":"门禁","device_id":"00000000-0000-4000-8000-000000000009"}`},
		{"GET", "/api/v1/auth/session", ""},
		{"POST", "/api/v1/auth/logout", ""},
		{"GET", "/api/v1/devices", ""},
		{"DELETE", "/api/v1/devices/" + f.androidID + "/session", ""},
		{"GET", "/api/v1/admin/bots", ""},
		{"POST", "/api/v1/admin/bots", botCreateBody},
		{"PATCH", "/api/v1/admin/bots/00000000-0000-4000-8000-000000000001", `{"expected_version":1,"enabled":false}`},
	}
	for _, route := range routes {
		t.Run(route.method+route.path, func(t *testing.T) {
			for _, tc := range []struct {
				id, code string
				status   int
			}{
				{"", "INSTANCE_REQUIRED", 400},
				{"invalid", "INVALID_REQUEST", 400},
				{"11111111-1111-4111-8111-111111111111", "INSTANCE_MISMATCH", 409},
				{f.instanceID, "", 401},
			} {
				req, err := http.NewRequest(route.method, f.server.URL+route.path, strings.NewReader(route.body))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer invalid")
				if tc.id != "" {
					req.Header.Set("X-RelayNest-Instance-ID", tc.id)
				}
				res, err := f.server.Client().Do(req)
				if err != nil {
					t.Fatal(err)
				}
				raw, err := io.ReadAll(res.Body)
				res.Body.Close()
				if err != nil {
					t.Fatal(err)
				}
				if res.StatusCode != tc.status || res.Header.Get("Cache-Control") != "no-store" || (tc.code != "" && !strings.Contains(string(raw), `"code":"`+tc.code+`"`)) {
					t.Fatalf("%s: %d %s", tc.code, res.StatusCode, raw)
				}
				if strings.Contains(string(raw), f.instanceID) || strings.Contains(string(raw), "invalid\"") {
					t.Fatal("门禁泄漏请求值")
				}
			}
		})
	}
	// 真实库故障必须在读取 Bearer 前失败关闭；逐路由验证而不是替身中间件。
	if err := f.store.Close(); err != nil {
		t.Fatal(err)
	}
	for _, route := range routes {
		status, raw, _ := f.request(route.method, route.path, route.body, "android", "invalid", nil)
		if status != 503 || !strings.Contains(raw, `"code":"INSTANCE_UNAVAILABLE"`) {
			t.Fatalf("%s %s: %d %s", route.method, route.path, status, raw)
		}
	}
}

func TestInstanceProbeAndHonestDatabaseSwitch(t *testing.T) {
	f := botHTTP(t, nil)
	// 公开 probe 走实际装配，不携带夹具默认的实例头或 Bearer。
	probe := func() string {
		t.Helper()
		res, err := f.server.Client().Get(f.server.URL + "/api/v1/instance")
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out map[string]string
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != 200 || len(out) != 1 || res.Header.Get("Cache-Control") != "no-store" || res.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("probe assembly failed")
		}
		return out["instance_id"]
	}
	for _, check := range []struct {
		method, path string
		status       int
	}{
		{"HEAD", "/api/v1/instance", 405}, {"POST", "/api/v1/instance", 405},
		{"GET", "/api/v1/instance/", 404}, {"GET", "/api/v1/instance/extra", 404},
	} {
		req, _ := http.NewRequest(check.method, f.server.URL+check.path, nil)
		res, err := f.server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != check.status || res.Header.Get("Location") != "" || res.Request.URL.Path != check.path {
			t.Fatalf("probe route redirected or fell back: %s %d", check.path, res.StatusCode)
		}
	}
	idA := probe()
	if idA != f.instanceID {
		t.Fatal("probe did not identify A")
	}
	b, handlers, err := authenticationDependencies(context.Background(), filepath.Join(t.TempDir(), "b.sqlite"), f.server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if err := auth.InitAdministrator(context.Background(), b, "Admin", []byte("correct")); err != nil {
		t.Fatal(err)
	}
	idB := fixtureInstanceID(t, b)
	if idB == idA {
		t.Fatal("independent database identity reused")
	}
	f.server.Config.Handler = httpapi.New(health.New(nil), handlers)
	if probe() != idB {
		t.Fatal("probe remained on A")
	}
	for _, route := range []struct{ method, path, body string }{
		{"GET", "/api/v1/auth/session", ""}, {"POST", "/api/v1/auth/logout", ""}, {"GET", "/api/v1/devices", ""},
		{"DELETE", "/api/v1/devices/" + f.androidID + "/session", ""}, {"GET", "/api/v1/admin/bots", ""},
		{"POST", "/api/v1/admin/bots", botCreateBody}, {"PATCH", "/api/v1/admin/bots/00000000-0000-4000-8000-000000000001", `{"expected_version":1,"enabled":false}`},
		{"POST", "/api/v1/auth/login", `{"username":"Admin","password":"correct","client_type":"android","device_name":"switch","device_id":"00000000-0000-4000-8000-000000000009"}`},
	} {
		status, raw, _ := f.admin(route.method, route.path, route.body)
		if status != 409 || !strings.Contains(raw, `"code":"INSTANCE_MISMATCH"`) {
			t.Fatalf("switch %s %s: %d %s", route.method, route.path, status, raw)
		}
	}
	// 换成 B 身份后旧 A 会话仍不能获权，显式新登录才获得 B 的 Android 投影。
	f.instanceID = idB
	status, raw, _ := f.admin("GET", "/api/v1/auth/session", "")
	if status != 401 {
		t.Fatalf("A session accepted in B: %d %s", status, raw)
	}
	status, raw, _ = f.request("POST", "/api/v1/auth/login", `{"username":"Admin","password":"correct","client_type":"android","device_name":"switch","device_id":"00000000-0000-4000-8000-000000000009"}`, "", "", nil)
	if status != 200 || !strings.Contains(raw, `"instance_id":"`+idB+`"`) {
		t.Fatalf("explicit B login: %d %s", status, raw)
	}
}

func TestBotHTTPReadProjectionRollback(t *testing.T) {
	for _, mutation := range []struct {
		name, query, code string
		status            int
	}{
		{"identity", "UPDATE instance_identity SET instance_id='22222222-2222-4222-8222-222222222222'", "INSTANCE_MISMATCH", 409},
		{"generation", "UPDATE administrator SET auth_generation=auth_generation+1", "AUTHENTICATION_FAILED", 401},
		{"revoke", "UPDATE sessions SET revoked=1", "AUTHENTICATION_FAILED", 401},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			f := botHTTP(t, nil)
			status, raw, h := f.admin("POST", "/api/v1/admin/bots", botCreateBody)
			assertBotResponse(t, status, 201, raw, h)
			before := f.snapshot()
			real := f.authorize
			f.authorize = func(w http.ResponseWriter, r *http.Request, write bool) (httpapi.BotAccess, bool) {
				access, ok := real(w, r, write)
				read := access.Read
				access.Read = func(ctx context.Context, fn func(*sql.Tx) error) error {
					return read(ctx, func(tx *sql.Tx) error {
						if err := fn(tx); err != nil {
							return err
						}
						_, err := tx.ExecContext(ctx, mutation.query)
						return err
					})
				}
				return access, ok
			}
			status, raw, h = f.admin("GET", "/api/v1/admin/bots", "")
			assertBotResponse(t, status, mutation.status, raw, h)
			if !strings.Contains(raw, `"code":"`+mutation.code+`"`) || strings.Contains(raw, "items") || f.snapshot() != before || fixtureInstanceID(t, f.store) != f.instanceID {
				t.Fatalf("projection or partial transaction escaped: %s", raw)
			}
		})
	}
}

func TestBotHTTPReadSnapshotConcurrentReset(t *testing.T) {
	f := botHTTP(t, nil)
	status, raw, h := f.admin("POST", "/api/v1/admin/bots", botCreateBody)
	assertBotResponse(t, status, 201, raw, h)
	created := decodeBotCreated(t, raw)
	entered, release, started := make(chan struct{}), make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	real := f.authorize
	f.authorize = func(w http.ResponseWriter, r *http.Request, write bool) (httpapi.BotAccess, bool) {
		access, ok := real(w, r, write)
		read := access.Read
		access.Read = func(ctx context.Context, fn func(*sql.Tx) error) error {
			return read(ctx, func(tx *sql.Tx) error {
				close(entered)
				select {
				case <-release:
				case <-ctx.Done():
					return ctx.Err()
				}
				return fn(tx)
			})
		}
		return access, ok
	}
	type response struct {
		status int
		raw    string
	}
	responseDone := make(chan response, 1)
	go func() {
		status, raw, _ := f.admin("GET", "/api/v1/admin/bots", "")
		responseDone <- response{status, raw}
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	writeDone := make(chan error, 1)
	go func() {
		close(started)
		writeDone <- f.store.Write(ctx, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, "UPDATE bots SET name='new projection' WHERE id=?", created.Bot.ID); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, "UPDATE administrator SET auth_generation=auth_generation+1")
			return err
		})
	}()
	<-started
	select {
	case err := <-writeDone:
		t.Fatalf("writer crossed authorized snapshot: %v", err)
	default:
	}
	close(release)
	var result response
	select {
	case result = <-responseDone:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if result.status != 200 || !strings.Contains(result.raw, `"name":"测试Bot"`) || strings.Contains(result.raw, "new projection") {
		t.Fatalf("stitched projection: %d %s", result.status, result.raw)
	}
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
	// 并发事务提交后旧 actor 不能再次获得任何投影。
	status, raw, _ = f.admin("GET", "/api/v1/admin/bots", "")
	if status != 401 || strings.Contains(raw, "items") {
		t.Fatalf("stale actor accepted: %d %s", status, raw)
	}
}

func TestInstanceBindingWebAndGlobalRevocation(t *testing.T) {
	f := botHTTP(t, nil)
	headers := map[string]string{"Origin": f.server.URL, "X-CSRF-Token": f.web.CSRFToken, "X-RelayNest-Instance-ID": "invalid"}
	for _, method := range []string{"GET", "POST"} {
		body, want := "", 200
		if method == "POST" {
			body, want = botCreateBody, 201
		}
		status, raw, h := f.request(method, "/api/v1/admin/bots", body, "web", f.web.Token, headers)
		assertBotResponse(t, status, want, raw, h)
	}
	delete(headers, "X-CSRF-Token")
	status, raw, h := f.request("POST", "/api/v1/admin/bots", botCreateBody, "web", f.web.Token, headers)
	assertBotResponse(t, status, 403, raw, h)
	if err := auth.ResetPassword(context.Background(), f.store, []byte("replacement")); err != nil {
		t.Fatal(err)
	}
	for _, client := range []string{"android", "web"} {
		token := f.android.Token
		if client == "web" {
			token = f.web.Token
		}
		for _, route := range []struct{ method, path string }{{"GET", "/api/v1/auth/session"}, {"GET", "/api/v1/devices"}, {"GET", "/api/v1/admin/bots"}, {"POST", "/api/v1/auth/logout"}} {
			status, raw, h = f.request(route.method, route.path, "", client, token, map[string]string{"Origin": f.server.URL, "X-CSRF-Token": f.web.CSRFToken})
			assertBotResponse(t, status, 401, raw, h)
		}
	}
}

func TestBotHTTPWriteBindingMutationRollback(t *testing.T) {
	for _, method := range []string{"POST", "PATCH"} {
		for _, mutation := range []struct {
			name, sql, code string
			status          int
		}{
			{"identity", "UPDATE instance_identity SET instance_id='22222222-2222-4222-8222-222222222222'", "INSTANCE_MISMATCH", 409},
			{"generation", "UPDATE administrator SET auth_generation=auth_generation+1", "AUTHENTICATION_FAILED", 401},
		} {
			t.Run(method+"/"+mutation.name, func(t *testing.T) {
				f := botHTTP(t, nil)
				status, raw, h := f.admin("POST", "/api/v1/admin/bots", botCreateBody)
				assertBotResponse(t, status, 201, raw, h)
				created := decodeBotCreated(t, raw)
				verb, path, body := "INSERT", "/api/v1/admin/bots", botCreateBody
				if method == "PATCH" {
					verb = "UPDATE"
					path += "/" + created.Bot.ID
					body = `{"expected_version":1,"name":"transaction mutation"}`
				}
				f.sql("CREATE TRIGGER binding_change AFTER " + verb + " ON bots BEGIN " + mutation.sql + "; END")
				before := f.snapshot()
				status, raw, h = f.admin(method, path, body)
				assertBotResponse(t, status, mutation.status, raw, h)
				if !strings.Contains(raw, `"code":"`+mutation.code+`"`) || strings.Contains(raw, "api_token") || f.snapshot() != before || fixtureInstanceID(t, f.store) != f.instanceID {
					t.Fatalf("partial write escaped binding transaction: %s", raw)
				}
			})
		}
	}
}
