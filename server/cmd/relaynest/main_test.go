package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
