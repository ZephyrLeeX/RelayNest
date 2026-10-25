-- 当前内容与创建事件水位分离；NULL 仅供同事务插入后回填真实 changes.Append 结果。
-- 不引用 changes：变更日志可按保留窗口清理，不得连带删除仍保留的消息。
CREATE TABLE messages (
    id TEXT PRIMARY KEY NOT NULL CHECK (
        length(id) = 36 AND substr(id,9,1) = '-' AND substr(id,14,1) = '-'
        AND substr(id,19,1) = '-' AND substr(id,24,1) = '-'
        AND length(replace(id,'-','')) = 32 AND replace(id,'-','') NOT GLOB '*[^0-9a-f]*'
        AND substr(id,15,1) = '4' AND substr(id,20,1) IN ('8','9','a','b')
    ),
    bot_id TEXT NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
    bot_seq INTEGER NOT NULL CHECK (typeof(bot_seq) = 'integer' AND bot_seq >= 1),
    created_change_seq INTEGER CHECK (created_change_seq IS NULL OR (typeof(created_change_seq) = 'integer' AND created_change_seq >= 1)),
    sender TEXT NOT NULL CHECK (sender IN ('application','administrator')),
    revision INTEGER NOT NULL CHECK (typeof(revision) = 'integer' AND revision >= 1),
    format TEXT NOT NULL CHECK (format IN ('plain','markdown')),
    text TEXT NOT NULL,
    buttons_json TEXT NOT NULL CHECK (json_valid(buttons_json) AND json_type(buttons_json) = 'array'),
    reply_to_message_id TEXT REFERENCES messages(id) ON DELETE SET NULL,
    original_reply_to_message_id TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(bot_id, bot_seq),
    CHECK (reply_to_message_id IS NULL OR (original_reply_to_message_id IS NOT NULL AND reply_to_message_id = original_reply_to_message_id))
);
CREATE INDEX messages_created_at ON messages(created_at);
CREATE INDEX messages_created_change_seq ON messages(created_change_seq);
CREATE INDEX messages_reply_to ON messages(reply_to_message_id);
-- 创建水位仅允许从临时 NULL 回填一次，编辑不能改写 bootstrap 创建高水位。
CREATE TRIGGER messages_creation_seq_immutable BEFORE UPDATE OF created_change_seq ON messages
WHEN OLD.created_change_seq IS NOT NULL AND NEW.created_change_seq IS NOT OLD.created_change_seq
BEGIN SELECT RAISE(ABORT, 'immutable message creation sequence'); END;
-- 普通引用仅允许本 Bot；清理源消息时 SET NULL，不形成永久保护。
CREATE TRIGGER messages_reply_insert BEFORE INSERT ON messages
WHEN NEW.reply_to_message_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM messages WHERE id = NEW.reply_to_message_id AND bot_id = NEW.bot_id
)
BEGIN SELECT RAISE(ABORT, 'invalid message reference'); END;
CREATE TRIGGER messages_reply_update BEFORE UPDATE OF reply_to_message_id, bot_id ON messages
WHEN NEW.reply_to_message_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM messages WHERE id = NEW.reply_to_message_id AND bot_id = NEW.bot_id
)
BEGIN SELECT RAISE(ABORT, 'invalid message reference'); END;
CREATE TRIGGER messages_original_reply_immutable BEFORE UPDATE OF original_reply_to_message_id ON messages
WHEN NEW.original_reply_to_message_id IS NOT OLD.original_reply_to_message_id
BEGIN SELECT RAISE(ABORT, 'immutable original reference'); END;

-- 只承接创建事务的 pending；领取/租约/完整状态机由 P06 后续迁移扩展。
-- 快照保存完整 MessageDeliveryEvent，后续重试不得从当前正文重新拼装。
CREATE TABLE deliveries (
    delivery_id TEXT PRIMARY KEY NOT NULL CHECK (length(delivery_id) > 0),
    event_id TEXT NOT NULL UNIQUE CHECK (length(event_id) > 0),
    bot_id TEXT NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
    payload_json TEXT NOT NULL CHECK (
        json_valid(payload_json) AND json_type(payload_json) = 'object'
        AND json_extract(payload_json,'$.type') IS 'message.created'
        AND json_type(payload_json,'$.message') IS 'object'
        AND json_extract(payload_json,'$.message.bot_id') IS bot_id
        AND json_extract(payload_json,'$.message.sender') IS 'administrator'
    ),
    state TEXT NOT NULL DEFAULT 'pending' CHECK (state = 'pending'),
    state_version INTEGER NOT NULL DEFAULT 1 CHECK (typeof(state_version) = 'integer' AND state_version >= 1),
    attempt INTEGER NOT NULL DEFAULT 0 CHECK (typeof(attempt) = 'integer' AND attempt >= 0),
    retry_generation INTEGER NOT NULL DEFAULT 0 CHECK (typeof(retry_generation) = 'integer' AND retry_generation >= 0),
    enqueue_seq INTEGER NOT NULL CHECK (typeof(enqueue_seq) = 'integer' AND enqueue_seq >= 1),
    next_attempt_at TEXT NOT NULL
);
CREATE INDEX deliveries_pending ON deliveries(bot_id, state, next_attempt_at, enqueue_seq);
CREATE TRIGGER deliveries_snapshot_immutable BEFORE UPDATE OF delivery_id, event_id, bot_id, payload_json, enqueue_seq ON deliveries
WHEN NEW.delivery_id IS NOT OLD.delivery_id OR NEW.event_id IS NOT OLD.event_id
    OR NEW.bot_id IS NOT OLD.bot_id OR NEW.payload_json IS NOT OLD.payload_json OR NEW.enqueue_seq IS NOT OLD.enqueue_seq
BEGIN SELECT RAISE(ABORT, 'immutable delivery snapshot'); END;

-- 当前 sessions 是唯一设备绑定来源；复合外键拒绝把合法会话绑到另一设备。
CREATE UNIQUE INDEX sessions_id_device ON sessions(id, device_id);
CREATE TABLE push_jobs (
    id TEXT PRIMARY KEY NOT NULL CHECK (length(id) > 0),
    session_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    target_change_seq INTEGER NOT NULL CHECK (typeof(target_change_seq) = 'integer' AND target_change_seq >= 1),
    state TEXT NOT NULL DEFAULT 'pending' CHECK (state = 'pending'),
    attempt INTEGER NOT NULL DEFAULT 0 CHECK (typeof(attempt) = 'integer' AND attempt >= 0),
    next_attempt_at TEXT NOT NULL,
    FOREIGN KEY(session_id, device_id) REFERENCES sessions(id, device_id) ON DELETE CASCADE
);
-- 全局水位提示不引用消息/Bot，不持有正文、URL、凭据或消息保留保护。
CREATE INDEX push_jobs_pending ON push_jobs(state, next_attempt_at);
CREATE INDEX push_jobs_session ON push_jobs(session_id, device_id);
