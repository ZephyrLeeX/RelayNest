package devices

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"unicode/utf8"

	"relaynest/server/internal/auth"
)

const pushBodyLimit = 262144

// pushActor 只进行身份快照检查，不续期；仓储负责同事务重验、变更和续期。
func (h *Handler) pushActor(w http.ResponseWriter, r *http.Request, mode auth.RequestMode) (auth.AdminRequest, bool) {
	if h == nil || h.security == nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE")
		return auth.AdminRequest{}, false
	}
	actor, ok := h.security.AuthorizeRequired(w, r, mode)
	if !ok {
		return auth.AdminRequest{}, false
	}
	if actor.Session.ClientType != "android" {
		writeError(w, 403, "FORBIDDEN")
		return auth.AdminRequest{}, false
	}
	// 当前身份来自真实 Session，任何 query 都不能成为目标或客户端时间选择器。
	if r.URL.RawQuery != "" {
		writeError(w, 400, "INVALID_REQUEST")
		return auth.AdminRequest{}, false
	}
	if h.push == nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE")
		return auth.AdminRequest{}, false
	}
	return actor, true
}

func (h *Handler) GetCurrentPush(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.pushActor(w, r, auth.RequestRead)
	if !ok {
		return
	}
	// GET 的请求体不参与身份或资源选择，不能读取或据此修改状态。
	state, err := h.push.GetCurrentPush(r.Context(), actor)
	if err != nil {
		writePushError(w, err)
		return
	}
	writeJSON(w, 200, state)
}

func (h *Handler) PutCurrentPush(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.pushActor(w, r, auth.RequestWrite)
	if !ok {
		return
	}
	raw, ok := readPushBody(w, r)
	if !ok {
		return
	}
	_, valid := pushObject(raw, []string{"provider", "token", "registration_version", "expected_version"})
	var in PushRegistrationUpdate
	if !valid || json.Unmarshal(raw, &in) != nil {
		writeError(w, 400, "INVALID_REQUEST")
		return
	}
	state, err := h.push.PutCurrentPush(r.Context(), actor, in)
	if err != nil {
		writePushError(w, err)
		return
	}
	writeJSON(w, 200, state)
}

func (h *Handler) DeleteCurrentPush(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.pushActor(w, r, auth.RequestWrite)
	if !ok {
		return
	}
	raw, ok := readPushBody(w, r)
	if !ok {
		return
	}
	_, valid := pushObject(raw, []string{"expected_version"})
	var in struct {
		ExpectedVersion int64 `json:"expected_version"`
	}
	if !valid || json.Unmarshal(raw, &in) != nil {
		writeError(w, 400, "INVALID_REQUEST")
		return
	}
	state, err := h.push.DeleteCurrentPush(r.Context(), actor, PushProvider(r.PathValue("provider")), in.ExpectedVersion)
	if err != nil {
		writePushError(w, err)
		return
	}
	writeJSON(w, 200, state)
}

func (h *Handler) GetCurrentNotifications(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.pushActor(w, r, auth.RequestRead)
	if !ok {
		return
	}
	state, err := h.push.GetCurrentNotifications(r.Context(), actor)
	if err != nil {
		writePushError(w, err)
		return
	}
	writeJSON(w, 200, state)
}

func (h *Handler) PutCurrentNotifications(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.pushActor(w, r, auth.RequestWrite)
	if !ok {
		return
	}
	raw, ok := readPushBody(w, r)
	if !ok {
		return
	}
	var in NotificationSettingsUpdate
	if !notificationObject(raw) || json.Unmarshal(raw, &in) != nil {
		writeError(w, 400, "INVALID_REQUEST")
		return
	}
	state, err := h.push.PutCurrentNotifications(r.Context(), actor, in)
	if err != nil {
		writePushError(w, err)
		return
	}
	writeJSON(w, 200, state)
}

func readPushBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if len(r.Header.Values("Content-Type")) != 1 || err != nil || media != "application/json" || r.Body == nil {
		writeError(w, 400, "INVALID_REQUEST")
		return nil, false
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, pushBodyLimit))
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			writeError(w, 413, "PAYLOAD_TOO_LARGE")
		} else {
			writeError(w, 400, "INVALID_REQUEST")
		}
		return nil, false
	}
	// encoding/json 会替换无效 UTF-8，必须在进入 DTO 前拒绝原始非法字节。
	if !utf8.Valid(raw) {
		writeError(w, 400, "INVALID_REQUEST")
		return nil, false
	}
	return raw, true
}

// pushObject 逐键验证精确 schema，拒绝缺失、重复、大小写别名和非声明的 null。
// 保留 RawMessage 后再解码有类型 DTO，版本从始至终使用 int64，不经过 float64。
func pushObject(raw []byte, names []string, nullable ...string) (map[string]json.RawMessage, bool) {
	d := json.NewDecoder(bytes.NewReader(raw))
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return nil, false
	}
	fields := make(map[string]json.RawMessage, len(names))
	for d.More() {
		token, err := d.Token()
		name, isString := token.(string)
		if err != nil || !isString {
			return nil, false
		}
		allowed := false
		for _, candidate := range names {
			allowed = allowed || candidate == name
		}
		if _, duplicate := fields[name]; !allowed || duplicate {
			return nil, false
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return nil, false
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			allowed = false
			for _, candidate := range nullable {
				allowed = allowed || candidate == name
			}
			if !allowed {
				return nil, false
			}
		}
		fields[name] = value
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') || len(fields) != len(names) || d.Decode(new(any)) != io.EOF {
		return nil, false
	}
	return fields, true
}

func notificationObject(raw []byte) bool {
	fields, ok := pushObject(raw, []string{"expected_version", "enabled", "bot_overrides", "dnd", "preview", "notify_channel", "notify_provider"}, "notify_provider")
	if !ok {
		return false
	}
	if _, ok = pushObject(fields["dnd"], []string{"enabled", "timezone", "start", "end"}); !ok {
		return false
	}
	var overrides []json.RawMessage
	if json.Unmarshal(fields["bot_overrides"], &overrides) != nil {
		return false
	}
	for _, override := range overrides {
		if _, ok = pushObject(override, []string{"bot_id", "enabled"}); !ok {
			return false
		}
	}
	return true
}

func writePushError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrAndroidRequired):
		writeError(w, 403, "FORBIDDEN")
	case errors.Is(err, ErrPushInput), errors.Is(err, ErrSettingsInput):
		writeError(w, 422, "VALIDATION_FAILED")
	case errors.Is(err, ErrNotificationBotNotFound):
		writeError(w, 404, "RESOURCE_NOT_FOUND")
	case errors.Is(err, ErrProviderNotReady):
		writeError(w, 422, "PROVIDER_NOT_READY")
	case errors.Is(err, ErrPushConflict):
		writeError(w, 409, "REGISTRATION_CONFLICT")
	case errors.Is(err, ErrPushExhausted):
		writeError(w, 409, "REGISTRATION_VERSION_EXHAUSTED")
	case errors.Is(err, ErrSettingsConflict), errors.Is(err, ErrSettingsExhausted):
		var version *VersionError
		if !errors.As(err, &version) || version.LatestVersion < 0 {
			auth.WriteSessionErrorRequired(w, auth.ErrStorage)
			return
		}
		code := "SETTINGS_CONFLICT"
		if errors.Is(err, ErrSettingsExhausted) {
			code = "SETTINGS_VERSION_EXHAUSTED"
		}
		writeJSON(w, 409, map[string]any{"error": map[string]any{
			"code": code, "message": http.StatusText(409), "request_id": rand.Text(), "retryable": false,
			"latest_settings_version": version.LatestVersion, "recovery_path": "/devices/current/notifications",
		}})
	default:
		// 实例门禁与会话错误沿原 REQUIRED 映射，绝不伪装成配置 CAS 冲突。
		auth.WriteSessionErrorRequired(w, err)
	}
}
