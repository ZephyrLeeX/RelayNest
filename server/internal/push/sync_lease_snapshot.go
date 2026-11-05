package push

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"relaynest/server/internal/storage"
)

// 只比较原事件版本，不把当前注册/配置补回旧 job；不读取正文、DND、已读
// 或源消息存在性。静默 sync 的本层复核不能替代提醒策略或发送前最终检查。
func syncLeaseCurrentSnapshot(ctx context.Context, tx *sql.Tx, j JobSnapshot, now time.Time) (bool, error) {
	return leaseCurrentSnapshot(ctx, tx, j, now, syncLeaseScope)
}

func leaseCurrentSnapshot(ctx context.Context, tx *sql.Tx, j JobSnapshot, now time.Time, scope leaseScope) (bool, error) {
	// 身份查询只接受经用途验证的原提供方，不能用当前注册为旧 job 改换通道。
	provider := "fcm"
	if scope == alertLeaseScope {
		if !scope.accepts(j) {
			return false, nil
		}
		provider = j.Provider
	} else if scope != syncLeaseScope {
		return false, nil
	}
	deployment, err := storage.ReadInstanceIDTx(ctx, tx)
	if err != nil {
		return false, err
	}
	// 共享读取保留实例格式及唯一性语义；本叶还须拒绝可被 Scan 转成字符串的 BLOB，
	// 不能把损坏的部署元数据提升为有效 sync 候选。
	var deploymentType string
	if err := tx.QueryRowContext(ctx, `SELECT typeof(instance_id) FROM instance_identity WHERE id=1`).Scan(&deploymentType); err != nil {
		return false, err
	}
	if deploymentType != "text" {
		return false, ErrJobData
	}
	if deployment != j.DeploymentID {
		return false, nil
	}
	var device, client string
	var generation, revoked, idle, absolute int64
	var admin, registration, settings sql.NullInt64
	var registrationType, settingsType string
	var hasRegistration, hasSettings, tokenPresent int
	var currentIntegerStorage, currentClientText, currentIdentityStorage bool
	// token 仅在 SQLite 内判断存在和长度，不读取/返回其原始文本。
	err = tx.QueryRowContext(ctx, `SELECT s.device_id,s.client_type,s.auth_generation,s.revoked,s.idle_expires_at,s.absolute_expires_at,
 a.auth_generation,r.registration_version,n.settings_version,r.session_id IS NOT NULL,n.device_id IS NOT NULL,
 typeof(r.registration_version),typeof(n.settings_version),typeof(s.client_type)='text',
 typeof(s.auth_generation)='integer' AND typeof(s.revoked)='integer'
 AND typeof(s.idle_expires_at)='integer' AND typeof(s.absolute_expires_at)='integer'
 AND (a.id IS NULL OR typeof(a.auth_generation)='integer'),
 CASE WHEN r.token IS NULL THEN 0
      WHEN typeof(r.token)='text' AND length(CAST(r.token AS BLOB)) BETWEEN 1 AND 16384 THEN 1 ELSE -1 END,
 typeof(s.id)='text' AND typeof(s.device_id)='text'
 AND (a.id IS NULL OR typeof(a.id)='integer')
 AND (r.session_id IS NULL OR (typeof(r.session_id)='text' AND typeof(r.provider)='text'))
 AND (n.device_id IS NULL OR typeof(n.device_id)='text')
 FROM sessions s LEFT JOIN administrator a ON a.id=1
 LEFT JOIN device_push_registrations r ON r.session_id=s.id AND r.provider=?
 LEFT JOIN device_notification_settings n ON n.device_id=s.device_id
 WHERE s.id=?`, provider, j.SessionID).Scan(&device, &client, &generation, &revoked, &idle, &absolute,
		&admin, &registration, &settings, &hasRegistration, &hasSettings, &registrationType, &settingsType, &currentClientText, &currentIntegerStorage, &tokenPresent, &currentIdentityStorage)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// 数字 BLOB 也能被 database/sql 转成 int64，关键数值必须额外确认 SQLite
	// 原存储类型。LEFT JOIN 缺行与已存在的坏版本分开判断，保留缺设置的虚拟 0。
	if (scope == alertLeaseScope && !currentIdentityStorage) || !currentIntegerStorage || generation < 1 || (admin.Valid && admin.Int64 < 1) || idle > absolute ||
		(hasRegistration != 0 && (registrationType != "integer" || !registration.Valid || registration.Int64 < 1)) ||
		(hasSettings != 0 && (settingsType != "integer" || !settings.Valid || settings.Int64 < 1)) || tokenPresent < 0 {
		return false, ErrJobData
	}
	version := int64(0)
	if hasSettings != 0 {
		version = settings.Int64
	}
	// client_type 同值 BLOB 也能被 Scan 转成 android；只有原 TEXT 可获得 Android 资格。
	return device == j.DeviceID && currentClientText && client == "android" && revoked == 0 && admin.Valid && admin.Int64 == generation && generation == j.SessionGeneration &&
		idle > now.UnixNano() && absolute > now.UnixNano() && registration.Valid && registration.Int64 == j.RegistrationVersion &&
		tokenPresent == 1 && version == j.SettingsVersion, nil
}
