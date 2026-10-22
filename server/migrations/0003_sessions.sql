-- 会话凭据仅保存摘要；代次与管理员记录在鉴权事务内核对。
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    token_hash BLOB NOT NULL UNIQUE CHECK(length(token_hash)=32),
    auth_generation INTEGER NOT NULL CHECK(auth_generation>=1),
    client_type TEXT NOT NULL CHECK(client_type IN ('web','android')),
    device_id TEXT NOT NULL,
    device_name TEXT NOT NULL CHECK(length(CAST(device_name AS BLOB))>0),
    idle_expires_at INTEGER NOT NULL,
    absolute_expires_at INTEGER NOT NULL,
    revoked INTEGER NOT NULL DEFAULT 0 CHECK(revoked IN (0,1)),
    CHECK(idle_expires_at<=absolute_expires_at)
);
