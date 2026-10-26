package devices

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"math"
	"time"
	"unicode/utf8"

	"relaynest/server/internal/auth"
)

type registrationSecret struct {
	version int64
	token   sql.NullString
	updated *time.Time
}

func readRegistration(ctx context.Context, tx *sql.Tx, session string, provider PushProvider) (registrationSecret, error) {
	var row registrationSecret
	var stamp string
	err := tx.QueryRowContext(ctx, `SELECT registration_version,token,updated_at FROM device_push_registrations WHERE session_id=? AND provider=?`, session, provider).Scan(&row.version, &row.token, &stamp)
	if err == sql.ErrNoRows {
		return row, nil
	}
	if err != nil {
		return registrationSecret{}, auth.ErrStorage
	}
	t, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return registrationSecret{}, auth.ErrStorage
	}
	row.updated = &t
	return row, nil
}
func readPushState(ctx context.Context, tx *sql.Tx, session string) (PushRegistrationState, error) {
	state := PushRegistrationState{SessionID: session, Registrations: make([]PushRegistration, 0, 2)}
	for _, provider := range []PushProvider{PushFCM, PushOPush} {
		row, err := readRegistration(ctx, tx, session, provider)
		if err != nil {
			return PushRegistrationState{}, err
		}
		state.Registrations = append(state.Registrations, PushRegistration{provider, row.version, row.token.Valid, row.updated})
	}
	return state, nil
}
func (r *PushRepository) GetCurrentPush(ctx context.Context, actor auth.AdminRequest) (PushRegistrationState, error) {
	var result PushRegistrationState
	err := r.authorized(ctx, actor, true, func(tx *sql.Tx, s auth.Session) error {
		var err error
		result, err = readPushState(ctx, tx, s.ID)
		return err
	})
	if err != nil {
		return PushRegistrationState{}, err
	}
	return result, nil
}
func (r *PushRepository) PutCurrentPush(ctx context.Context, actor auth.AdminRequest, in PushRegistrationUpdate) (PushRegistrationState, error) {
	var result PushRegistrationState
	err := r.authorized(ctx, actor, false, func(tx *sql.Tx, s auth.Session) error {
		if !in.Provider.valid() || in.ExpectedVersion < 0 || in.RegistrationVersion < 1 || !utf8.ValidString(in.Token) || utf8.RuneCountInString(in.Token) < 1 || utf8.RuneCountInString(in.Token) > 4096 {
			return ErrPushInput
		}
		row, err := readRegistration(ctx, tx, s.ID, in.Provider)
		if err != nil {
			return err
		}
		// 当前有效的相同版本/token 重放先于 CAS，保留原时间；墓碑永远不可重放复活。
		if !(row.token.Valid && row.version == in.RegistrationVersion && row.token.String == in.Token) {
			if row.version != in.ExpectedVersion {
				return pushVersionError(ErrPushConflict, row.version)
			}
			if in.RegistrationVersion == row.version {
				return pushVersionError(ErrPushConflict, row.version)
			}
			if row.version == math.MaxInt64 {
				return pushVersionError(ErrPushExhausted, row.version)
			}
			if in.RegistrationVersion < row.version {
				return pushVersionError(ErrPushConflict, row.version)
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO device_push_registrations(session_id,provider,registration_version,token,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(session_id,provider) DO UPDATE SET registration_version=excluded.registration_version,token=excluded.token,updated_at=excluded.updated_at`, s.ID, in.Provider, in.RegistrationVersion, in.Token, r.now().UTC().Format(time.RFC3339Nano))
			if err != nil {
				return auth.ErrStorage
			}
		}
		result, err = readPushState(ctx, tx, s.ID)
		return err
	})
	if err != nil {
		return PushRegistrationState{}, err
	}
	return result, nil
}
func (r *PushRepository) DeleteCurrentPush(ctx context.Context, actor auth.AdminRequest, provider PushProvider, expected int64) (PushRegistrationState, error) {
	var result PushRegistrationState
	err := r.authorized(ctx, actor, false, func(tx *sql.Tx, s auth.Session) error {
		if !provider.valid() || expected < 0 {
			return ErrPushInput
		}
		row, err := readRegistration(ctx, tx, s.ID, provider)
		if err != nil {
			return err
		}
		if row.version != expected {
			return pushVersionError(ErrPushConflict, row.version)
		}
		if row.token.Valid {
			if row.version == math.MaxInt64 {
				return pushVersionError(ErrPushExhausted, row.version)
			}
			if _, err = tx.ExecContext(ctx, `UPDATE device_push_registrations SET registration_version=registration_version+1,token=NULL,updated_at=? WHERE session_id=? AND provider=?`, r.now().UTC().Format(time.RFC3339Nano), s.ID, provider); err != nil {
				return auth.ErrStorage
			}
		}
		result, err = readPushState(ctx, tx, s.ID)
		return err
	})
	if err != nil {
		return PushRegistrationState{}, err
	}
	return result, nil
}

// PushTarget 是仅供可信发送器使用的秘密快照，不能由外部设备/Session 选择器构造。
// JSON 和格式化均不输出 token；发送器仅通过显式 Token 方法获取凭据。
type PushTarget struct {
	device, session string
	provider        PushProvider
	version         int64
	token           string
}

func (PushTarget) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "PushTarget{redacted}") }
func (PushTarget) MarshalJSON() ([]byte, error) { return []byte(`{}`), nil }
func (t PushTarget) DeviceID() string           { return t.device }
func (t PushTarget) SessionID() string          { return t.session }
func (t PushTarget) Provider() PushProvider     { return t.provider }
func (t PushTarget) RegistrationVersion() int64 { return t.version }
func (t PushTarget) Token() string              { return t.token }

// ReadPushTargets 只为内部调度提供当前有效的 Android 注册；最终发送仍须后包复核完整 job 策略。
func (r *PushRepository) ReadPushTargets(ctx context.Context) ([]PushTarget, error) {
	targets := make([]PushTarget, 0)
	if r == nil || r.store == nil {
		return nil, auth.ErrStorage
	}
	err := r.store.Read(ctx, func(tx *sql.Tx) error {
		now := r.now().UTC().UnixNano()
		rows, err := tx.QueryContext(ctx, `SELECT s.device_id,s.id,p.provider,p.registration_version,p.token FROM device_push_registrations p JOIN sessions s ON s.id=p.session_id JOIN administrator a ON a.id=1 AND a.auth_generation=s.auth_generation WHERE p.token IS NOT NULL AND s.revoked=0 AND s.client_type='android' AND s.idle_expires_at>? AND s.absolute_expires_at>? ORDER BY s.id,p.provider`, now, now)
		if err != nil {
			return auth.ErrStorage
		}
		defer rows.Close()
		for rows.Next() {
			var t PushTarget
			if err := rows.Scan(&t.device, &t.session, &t.provider, &t.version, &t.token); err != nil {
				return auth.ErrStorage
			}
			targets = append(targets, t)
		}
		if rows.Err() != nil {
			return auth.ErrStorage
		}
		return nil
	})
	if err != nil {
		return nil, auth.ErrStorage
	}
	return targets, nil
}

// PushDeliveryResult 只有已确认 target_invalid 可以条件清理，未知/超时等结果保持注册。
type PushDeliveryResult string

const (
	PushAccepted           PushDeliveryResult = "accepted"
	PushTargetInvalid      PushDeliveryResult = "target_invalid"
	PushTransient          PushDeliveryResult = "transient"
	PushRateLimited        PushDeliveryResult = "rate_limited"
	PushConfigurationError PushDeliveryResult = "configuration_error"
	PushUnknownDelivery    PushDeliveryResult = "unknown_delivery"
)

func (r *PushRepository) ApplyPushResult(ctx context.Context, target PushTarget, result PushDeliveryResult) (bool, error) {
	if result != PushTargetInvalid {
		return false, nil
	}
	if r == nil || r.store == nil {
		return false, auth.ErrStorage
	}
	changed := false
	err := r.store.Write(ctx, func(tx *sql.Tx) error {
		now := r.now().UTC().UnixNano()
		var version int64
		err := tx.QueryRowContext(ctx, `SELECT p.registration_version FROM device_push_registrations p JOIN sessions s ON s.id=p.session_id JOIN administrator a ON a.id=1 AND a.auth_generation=s.auth_generation WHERE s.id=? AND s.device_id=? AND p.provider=? AND p.registration_version=? AND p.token=? AND s.revoked=0 AND s.client_type='android' AND s.idle_expires_at>? AND s.absolute_expires_at>?`, target.session, target.device, target.provider, target.version, target.token, now, now).Scan(&version)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return auth.ErrStorage
		}
		if version == math.MaxInt64 {
			return pushVersionError(ErrPushExhausted, version)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE device_push_registrations SET registration_version=registration_version+1,token=NULL,updated_at=? WHERE session_id=? AND provider=?`, r.now().UTC().Format(time.RFC3339Nano), target.session, target.provider); err != nil {
			return auth.ErrStorage
		}
		changed = true
		return nil
	})
	if err != nil {
		if _, ok := err.(*VersionError); ok {
			return false, err
		}
		return false, auth.ErrStorage
	}
	return changed, nil
}
