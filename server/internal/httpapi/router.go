package httpapi

import (
	"net/http"

	"relaynest/server/internal/health"
	"relaynest/server/internal/webui"
)

type AuthHandlers interface {
	Login(http.ResponseWriter, *http.Request)
	GetSession(http.ResponseWriter, *http.Request)
	Logout(http.ResponseWriter, *http.Request)
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
	if len(authentication) > 0 && authentication[0] != nil {
		a := authentication[0]
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
	mux.HandleFunc("GET /health/live", h.Live)
	mux.HandleFunc("GET /health/ready", h.Ready)
	// 精确挂载管理首页和构建资源，未知 API 不回落为 HTML。
	static := webui.New()
	mux.Handle("GET /{$}", static)
	mux.Handle("GET /assets/", static)
	return mux
}
