-- 注册屏障属于不可复用的 Session.id；墓碑保留版本且绝不保留 token。
CREATE TABLE device_push_registrations (
 session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
 provider TEXT NOT NULL CHECK(provider IN ('fcm','opush')),
 registration_version INTEGER NOT NULL CHECK(typeof(registration_version)='integer' AND registration_version>=1),
 token TEXT CHECK(token IS NULL OR (length(CAST(token AS BLOB))>0 AND length(CAST(token AS BLOB))<=16384)),
 updated_at TEXT NOT NULL,
 PRIMARY KEY(session_id,provider)
);
-- 偏好属于安装设备，不能因某一 Session 撤销或删除而丢失。
CREATE TABLE device_notification_settings (
 device_id TEXT PRIMARY KEY NOT NULL,
 settings_version INTEGER NOT NULL CHECK(typeof(settings_version)='integer' AND settings_version>=1),
 updated_at TEXT NOT NULL,
 configuration TEXT NOT NULL
);
-- 安全撤销不推进屏障，直接持久清除所有秘密；即使版本耗尽仍可撤销。
CREATE TRIGGER device_push_session_invalidated AFTER UPDATE OF revoked,auth_generation,client_type,device_id,id ON sessions
WHEN NEW.revoked<>0 OR NEW.auth_generation<>OLD.auth_generation OR NEW.client_type<>OLD.client_type OR NEW.device_id<>OLD.device_id OR NEW.id<>OLD.id
BEGIN
 DELETE FROM device_push_registrations WHERE session_id=OLD.id OR session_id=NEW.id;
END;
CREATE TRIGGER device_push_generation_invalidated AFTER UPDATE OF auth_generation ON administrator
WHEN NEW.auth_generation<>OLD.auth_generation
BEGIN
 DELETE FROM device_push_registrations;
END;
CREATE TRIGGER device_push_administrator_deleted AFTER DELETE ON administrator
BEGIN
 DELETE FROM device_push_registrations;
END;
-- 不依赖调用者是否启用外键，离线受控删除也清除秘密。
CREATE TRIGGER device_push_session_deleted AFTER DELETE ON sessions
BEGIN
 DELETE FROM device_push_registrations WHERE session_id=OLD.id;
END;
