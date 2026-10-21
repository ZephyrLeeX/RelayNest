-- 基础状态只有一行；Bot 业务表由后续迁移创建。
CREATE TABLE system_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    sync_epoch TEXT NOT NULL CHECK (length(sync_epoch) = 36),
    change_floor INTEGER NOT NULL DEFAULT 0 CHECK (change_floor >= 0),
    config_version INTEGER NOT NULL DEFAULT 1 CHECK (config_version >= 1)
);
INSERT INTO system_state (id, sync_epoch)
VALUES (1, lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' ||
    substr(lower(hex(randomblob(2))), 2) || '-' ||
    substr('89ab', (random() & 3) + 1, 1) || substr(lower(hex(randomblob(2))), 2) || '-' ||
    lower(hex(randomblob(6))));

-- 不引用未来的 Bot 表，保证删除 Bot 后 tombstone 仍可追赶。
CREATE TABLE changes (
    change_seq INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK (length(type) > 0),
    entity_id TEXT NOT NULL CHECK (length(entity_id) > 0),
    revision INTEGER NOT NULL CHECK (revision >= 1),
    bot_id TEXT,
    created_at TEXT NOT NULL
);
