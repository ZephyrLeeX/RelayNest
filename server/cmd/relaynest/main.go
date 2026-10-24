package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"relaynest/server/internal/auth"
	"relaynest/server/internal/bots"
	"relaynest/server/internal/devices"
	"relaynest/server/internal/health"
	"relaynest/server/internal/httpapi"
	"relaynest/server/internal/storage"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "admin" {
		if err := runAdmin(context.Background(), os.Args[2:], os.Stdin, os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := runServer(ctx, os.Args[1:])
	stop()
	if err != nil {
		logger.Error("启动或服务退出失败", "code", err.Error())
		os.Exit(1)
	}
	logger.Info("服务已停止", "code", "shutdown_complete")
}

// 在返回 main 前释放资源，避免 os.Exit 跳过 defer。
func runServer(ctx context.Context, args []string) (result error) {
	flags := flag.NewFlagSet("relaynest", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	addr := flags.String("listen", "127.0.0.1:8080", "HTTP监听地址")
	db := flags.String("db", "", "数据库文件路径（必填）")
	externalOrigin := flags.String("external-origin", "", "外部 HTTPS Origin（必填）")
	var proxies proxyFlags
	flags.Var(&proxies, "trusted-proxy", "可信代理 IP/CIDR（可重复，默认不信任任何代理）")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			flags.SetOutput(os.Stderr)
			flags.PrintDefaults()
			return nil
		}
		return errors.New("configuration_failed")
	}
	if flags.NArg() != 0 {
		return errors.New("configuration_failed")
	}
	store, authentication, err := authenticationDependencies(ctx, *db, *externalOrigin, proxies)
	if err != nil {
		return errors.New("configuration_or_storage_failed")
	}
	defer func() {
		if err := store.Close(); err != nil {
			result = errors.Join(result, errors.New("storage_close_failed"))
		}
	}()
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return errors.New("listen_failed")
	}
	defer ln.Close()
	if err := serve(ctx, ln, health.New(nil), authentication); err != nil {
		return errors.New("http_failed")
	}
	return nil
}

func serve(ctx context.Context, ln net.Listener, h *health.Handler, authentication ...httpapi.AuthHandlers) error {
	srv := &http.Server{
		Handler:           httpapi.New(h, authentication...),
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

// 未显式声明的代理一律不可信，不由监听地址或环回地址推导。
type proxyFlags []string

func (p *proxyFlags) String() string         { return "" }
func (p *proxyFlags) Set(value string) error { *p = append(*p, value); return nil }

func authenticationDependencies(ctx context.Context, path, externalOrigin string, proxies []string) (*storage.Store, *managementHandlers, error) {
	policy, err := httpapi.NewOriginPolicy(externalOrigin, proxies)
	if err != nil {
		return nil, nil, err
	}
	store, err := storage.Open(ctx, storage.Config{Path: path})
	if err != nil {
		return nil, nil, err
	}
	sessions := auth.NewSessionStore(store)
	security := auth.NewRequestSecurity(sessions, policy)
	return store, &managementHandlers{
		Handler:       auth.NewHandler(store, sessions, policy),
		BotHandler:    httpapi.NewBotHandler(bots.New(store), botAuthorization(sessions, security)),
		DeviceHandler: devices.NewHandler(sessions, security),
	}, nil
}

// botAuthorization 只将经公共入口签发的身份绑定至受控事务能力。
func botAuthorization(sessions *auth.SessionStore, security *auth.RequestSecurity) httpapi.BotAuthorize {
	return func(w http.ResponseWriter, r *http.Request, write bool) (httpapi.BotAccess, bool) {
		mode := auth.RequestRead
		if write {
			mode = auth.RequestWrite
		}
		actor, ok := security.Authorize(w, r, mode)
		if !ok {
			return httpapi.BotAccess{}, false
		}
		// 闭包保留私有 actor；业务只能使用会话存储创建的 Tx，不能自行嵌套 Write。
		return httpapi.BotAccess{
			RenewRead: func(ctx context.Context) error { _, err := security.RenewRead(ctx, actor); return err },
			Write: func(ctx context.Context, fn func(*sql.Tx) error) error {
				return sessions.WithAuthorizedWrite(ctx, actor, fn)
			},
			SessionError: auth.WriteSessionError,
		}, true
	}
}

// 合并路由能力，认证与 Bot handler 使用同一会话仓储和来源政策。
type managementHandlers struct {
	*auth.Handler
	*httpapi.BotHandler
	DeviceHandler *devices.Handler
}

// 在装配层适配路由能力，避免 httpapi 反向导入依赖它的 auth/devices。
func (h *managementHandlers) ListDeviceSessions(w http.ResponseWriter, r *http.Request) {
	h.DeviceHandler.ListDeviceSessions(w, r)
}
func (h *managementHandlers) RevokeDeviceSession(w http.ResponseWriter, r *http.Request) {
	h.DeviceHandler.RevokeDeviceSession(w, r)
}
