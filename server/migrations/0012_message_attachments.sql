-- 当前消息关系独立于不可变投递保护；这里只保存公开引用及稳定显示顺序。
CREATE TABLE message_attachments (
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    attachment_id TEXT NOT NULL REFERENCES attachments(id),
    kind TEXT NOT NULL CHECK (kind IN ('image','file')),
    position INTEGER NOT NULL CHECK (typeof(position)='integer' AND position>=0 AND position<10),
    PRIMARY KEY (message_id,attachment_id),
    UNIQUE (message_id,position)
) WITHOUT ROWID;
CREATE INDEX message_attachments_attachment ON message_attachments(attachment_id);
