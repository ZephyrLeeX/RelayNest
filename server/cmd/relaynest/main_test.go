package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
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
