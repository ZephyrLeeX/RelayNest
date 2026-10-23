-- Bot 计数、实体版本和全局 change_seq 分离；未发生轮询时不记录连接状态。
CREATE TABLE bots (
 id TEXT PRIMARY KEY NOT NULL CHECK (length(id) > 0),
 name TEXT NOT NULL CHECK (length(name) > 0),
 enabled INTEGER NOT NULL CHECK (enabled IN (0,1)),
 can_send INTEGER NOT NULL CHECK (can_send IN (0,1)),
 can_receive INTEGER NOT NULL CHECK (can_receive IN (0,1)),
 version INTEGER NOT NULL CHECK (typeof(version) = 'integer' AND version >= 1),
 bot_seq INTEGER NOT NULL DEFAULT 0 CHECK (typeof(bot_seq) = 'integer' AND bot_seq >= 0)
);
CREATE TABLE bot_credentials (
 bot_id TEXT NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
 kind TEXT NOT NULL CHECK (kind IN ('api_token','webhook_key')),
 credential_id TEXT NOT NULL UNIQUE CHECK (length(credential_id) > 0),
 generation INTEGER NOT NULL CHECK (typeof(generation) = 'integer' AND generation >= 1),
 digest BLOB NOT NULL CHECK (typeof(digest) = 'blob' AND length(digest) = 32),
 PRIMARY KEY (bot_id, kind)
);
