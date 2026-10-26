package httpapi

import (
	"net/http"
	"net/url"
	"path"
	"strings"

	"relaynest/server/internal/health"
	"relaynest/server/internal/webui"
)

type AuthHandlers interface {
	Login(http.ResponseWriter, *http.Request)
	GetSession(http.ResponseWriter, *http.Request)
	Logout(http.ResponseWriter, *http.Request)
}

type InstanceHandlers interface {
	GetInstance(http.ResponseWriter, *http.Request)
}

type BotHandlers interface {
	ListBots(http.ResponseWriter, *http.Request)
	CreateBot(http.ResponseWriter, *http.Request)
	PatchBot(http.ResponseWriter, *http.Request)
}

type MessageHandlers interface {
	CreateBotMessage(http.ResponseWriter, *http.Request)
}

type DeviceHandlers interface {
	ListDeviceSessions(http.ResponseWriter, *http.Request)
	RevokeDeviceSession(http.ResponseWriter, *http.Request)
}

func New(h *health.Handler, authentication ...AuthHandlers) http.Handler {
	mux := http.NewServeMux()
	// 探测不依赖登录能力；未装配持久化存储时也保持失败关闭。
	probe := NewInstanceHandler(nil).GetInstance
	if len(authentication) > 0 && authentication[0] != nil {
		a := authentication[0]
		if instance, ok := a.(InstanceHandlers); ok {
			probe = instance.GetInstance
		}
		if m, ok := a.(MessageHandlers); ok {
			mux.HandleFunc("POST /api/v1/bot/messages", m.CreateBotMessage)
		}
		mux.HandleFunc("POST /api/v1/auth/login", a.Login)
		mux.HandleFunc("GET /api/v1/auth/session", a.GetSession)
		mux.HandleFunc("POST /api/v1/auth/logout", a.Logout)
		if d, ok := a.(DeviceHandlers); ok {
			mux.HandleFunc("GET /api/v1/devices", d.ListDeviceSessions)
			mux.HandleFunc("DELETE /api/v1/devices/{id}/session", d.RevokeDeviceSession)
		}
		if b, ok := a.(BotHandlers); ok {
			mux.HandleFunc("GET /api/v1/admin/bots", b.ListBots)
			mux.HandleFunc("POST /api/v1/admin/bots", b.CreateBot)
			mux.HandleFunc("PATCH /api/v1/admin/bots/{id}", b.PatchBot)
		}
	}
	mux.HandleFunc("/api/v1/instance", probe)
	mux.HandleFunc("GET /health/live", h.Live)
	mux.HandleFunc("GET /health/ready", h.Ready)
	// 精确挂载管理首页和构建资源，未知 API 不回落为 HTML。
	static := webui.New()
	mux.Handle("GET /{$}", static)
	mux.Handle("GET /assets/", static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 在 ServeMux 清理路径前拒绝探测别名，避免重定向复制 query 到响应。
		if redirectsToInstanceProbe(r.URL.EscapedPath()) {
			probeHeaders(w)
			probeError(w, 400, "INVALID_REQUEST", "请求无效", false)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func redirectsToInstanceProbe(escapedPath string) bool {
	cleaned := path.Clean(escapedPath)
	// ServeMux 保留尾斜杠，且按段解码匹配；编码斜杠不能成为路由分隔符。
	if strings.HasSuffix(escapedPath, "/") && cleaned != "/" {
		cleaned += "/"
	}
	if cleaned == escapedPath {
		return false
	}
	segments := strings.Split(cleaned, "/")
	if len(segments) != 4 || segments[0] != "" {
		return false
	}
	for i, want := range []string{"api", "v1", "instance"} {
		segment, err := url.PathUnescape(segments[i+1])
		if err != nil || segment != want {
			return false
		}
	}
	return true
}
