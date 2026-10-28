-- 共享已读从未读开始；旧 Bot 仅在升级时初始化，重开不得重置时间或版本。
CREATE TABLE bot_read_states (
    bot_id TEXT PRIMARY KEY NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
    read_seq INTEGER NOT NULL DEFAULT 0 CHECK (typeof(read_seq)='integer' AND read_seq>=0),
    state_version INTEGER NOT NULL DEFAULT 1 CHECK (typeof(state_version)='integer' AND state_version>=1),
    updated_at TEXT NOT NULL CHECK (length(updated_at)>0)
);
INSERT INTO bot_read_states(bot_id,read_seq,state_version,updated_at)
SELECT id,0,1,strftime('%Y-%m-%dT%H:%M:%fZ','now') FROM bots;
