package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"relaynest/server/internal/health"
	"relaynest/server/internal/httpapi"
)

func main() {
	addr := flag.String("listen", "127.0.0.1:8080", "HTTP监听地址")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		logger.Error("启动失败", "code", "listen_failed")
		os.Exit(1)
	}
	if err := serve(ctx, ln, health.New(nil)); err != nil {
		logger.Error("服务退出失败", "code", "http_failed")
		os.Exit(1)
	}
	logger.Info("服务已停止", "code", "shutdown_complete")
}

func serve(ctx context.Context, ln net.Listener, h *health.Handler) error {
	srv := &http.Server{
		Handler:           httpapi.New(h),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	result := make(chan error, 1)
	go func() { result <- srv.Serve(ln) }()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
	}
	h.Stop()
	// 信号上下文已取消；单独给在途请求留出有界排空时间。
	drain, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(drain); err != nil {
		// 超时后强制关闭连接，不能让残留连接阻塞进程退出。
		closeErr := srv.Close()
		<-result
		return errors.Join(err, closeErr)
	}
	err := <-result
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
