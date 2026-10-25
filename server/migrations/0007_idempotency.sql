-- 身份与操作类别共同隔离键，不引用凭据/会话代次；活记录无时间驱动删除。
CREATE TABLE idempotency (
    identity TEXT NOT NULL CHECK(length(identity)>0),
    operation TEXT NOT NULL CHECK(length(operation)>0),
    key TEXT NOT NULL CHECK(length(key)=36),
    digest BLOB NOT NULL CHECK(length(digest)=32),
    message_id TEXT NOT NULL,
    delivery_id TEXT,
    result_json TEXT,
    expired_at TEXT,
    PRIMARY KEY(identity,operation,key),
    CHECK ((expired_at IS NULL AND result_json IS NOT NULL AND json_valid(result_json)) OR
           (expired_at IS NOT NULL AND result_json IS NULL))
) WITHOUT ROWID;

-- 永久删除 Bot 时，级联删除消息/投递前仍能定位其创建快照。
-- 墓碑与删除共用原事务；删除失败时一起回滚，不另存正文或缩短存活记录期限。
CREATE TRIGGER idempotency_bot_delete BEFORE DELETE ON bots
BEGIN
    UPDATE idempotency SET result_json=NULL,expired_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')
    WHERE expired_at IS NULL AND (
        message_id IN (SELECT id FROM messages WHERE bot_id=OLD.id) OR
        delivery_id IN (SELECT delivery_id FROM deliveries WHERE bot_id=OLD.id)
    );
END;
