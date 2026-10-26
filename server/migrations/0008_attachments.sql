-- 附件上传幂等独立于强绑定 message_id 的通用幂等表。
CREATE TABLE attachments (
 id TEXT PRIMARY KEY NOT NULL,
 bot_id TEXT NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
 kind TEXT NOT NULL CHECK(kind IN ('image','file')),
 filename TEXT NOT NULL,
 media_type TEXT NOT NULL,
 storage_key TEXT UNIQUE,
 state TEXT NOT NULL CHECK(state IN ('staging','ready','deleting','missing')),
 size INTEGER CHECK(size IS NULL OR (typeof(size)='integer' AND size>=0)),
 sha256 BLOB CHECK(sha256 IS NULL OR (typeof(sha256)='blob' AND length(sha256)=32)),
 created_at TEXT NOT NULL,
 CHECK(state!='ready' OR (storage_key IS NOT NULL AND size IS NOT NULL AND sha256 IS NOT NULL))
);
CREATE INDEX attachments_state_created ON attachments(state,created_at);
CREATE TABLE attachment_uploads (
 identity TEXT NOT NULL,
 key TEXT NOT NULL,
 attachment_id TEXT NOT NULL UNIQUE REFERENCES attachments(id) ON DELETE CASCADE,
 generation INTEGER NOT NULL CHECK(typeof(generation)='integer' AND generation>=1),
 active_token TEXT NOT NULL,
 lease_until INTEGER NOT NULL CHECK(typeof(lease_until)='integer'),
 PRIMARY KEY(identity,key)
) WITHOUT ROWID;
-- 每次尝试的独立物理键不被重试覆盖；失败后供 BE-05-04 检查半文件/rename 证据。
CREATE TABLE attachment_attempts (
 token TEXT PRIMARY KEY NOT NULL,
 attachment_id TEXT NOT NULL REFERENCES attachments(id) ON DELETE CASCADE,
 generation INTEGER NOT NULL CHECK(typeof(generation)='integer' AND generation>=1),
 created_at TEXT NOT NULL
);
CREATE INDEX attachment_attempts_attachment ON attachment_attempts(attachment_id);
