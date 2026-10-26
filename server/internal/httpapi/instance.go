package httpapi

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"relaynest/server/internal/storage"
)

// 所有实例探测共享一个进程级令牌桶，不按客户端分配条目，保持内存恒定。
var instanceProbeBucket = probeBucket{tokens: 40, updated: time.Now()}

type probeBucket struct {
	mu      sync.Mutex
	tokens  float64
	updated time.Time
}

func (b *probeBucket) allow(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if elapsed := now.Sub(b.updated).Seconds(); elapsed > 0 {
		b.tokens = min(40, b.tokens+elapsed*20)
		b.updated = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

type InstanceHandler struct{ store *storage.Store }

func NewInstanceHandler(store *storage.Store) *InstanceHandler { return &InstanceHandler{store} }

func probeError(w http.ResponseWriter, status int, code, message string, retryable bool) {
	type detail struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
		Retryable bool   `json:"retryable"`
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error detail `json:"error"`
	}{detail{code, message, rand.Text(), retryable}})
}

func (h *InstanceHandler) GetInstance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	for _, header := range []string{"ETag", "Last-Modified", "Location"} {
		w.Header().Del(header)
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		probeError(w, 405, "INVALID_REQUEST", "请求无效", false)
		return
	}
	// HTTP/2 即使无正文也有 Body 对象；按传输长度拒绝正文及未知长度流。
	invalid := r.URL.RawQuery != "" || r.URL.ForceQuery || r.ContentLength != 0 || len(r.TransferEncoding) != 0
	// 只检查键的存在，不读取、解析或记录任何凭据值。
	for name := range r.Header {
		for _, forbidden := range []string{"Authorization", "Cookie", "X-CSRF-Token", "X-RelayNest-Instance-ID"} {
			if strings.EqualFold(name, forbidden) {
				invalid = true
			}
		}
	}
	if invalid {
		probeError(w, 400, "INVALID_REQUEST", "请求无效", false)
		return
	}
	if !instanceProbeBucket.allow(time.Now()) {
		w.Header().Set("Retry-After", "1")
		probeError(w, 429, "RATE_LIMITED", "请求过于频繁", true)
		return
	}
	var id string
	// 每次通过严格持久化读取；启动缓存不能成为成功响应的来源。
	if h == nil || h.store == nil {
		probeError(w, 503, "INSTANCE_UNAVAILABLE", "实例暂不可用", true)
		return
	}
	err := h.store.Read(r.Context(), func(tx *sql.Tx) error {
		var err error
		id, err = storage.ReadInstanceIDTx(r.Context(), tx)
		return err
	})
	if err != nil {
		probeError(w, 503, "INSTANCE_UNAVAILABLE", "实例暂不可用", true)
		return
	}
	_ = json.NewEncoder(w).Encode(struct {
		InstanceID string `json:"instance_id"`
	}{id})
}
