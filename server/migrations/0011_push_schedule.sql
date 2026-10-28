-- 原表原子重建：旧 writer 仍可五列 INSERT，旧 hint 不补造历史事件资格。
CREATE TABLE push_jobs_schedule (
    id TEXT PRIMARY KEY NOT NULL CHECK(length(id)>0),
    session_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    target_change_seq INTEGER NOT NULL CHECK(typeof(target_change_seq)='integer' AND target_change_seq>=1),
    state TEXT NOT NULL DEFAULT 'pending',
    attempt INTEGER NOT NULL DEFAULT 0 CHECK(typeof(attempt)='integer' AND attempt>=0),
    next_attempt_at TEXT NOT NULL,
    purpose TEXT,
    provider TEXT,
    registration_version INTEGER,
    settings_version INTEGER,
    deployment_id TEXT,
    message_id TEXT,
    bot_id TEXT,
    bot_seq INTEGER,
    session_generation INTEGER,
    created_at_ns INTEGER,
    expires_at_ns INTEGER,
    alert_eligible INTEGER,
    lease_hash BLOB,
    lease_until_ns INTEGER,
    FOREIGN KEY(session_id,device_id) REFERENCES sessions(id,device_id) ON DELETE CASCADE,
    CHECK (
      (state='pending' AND purpose IS NULL AND provider IS NULL
       AND registration_version IS NULL AND settings_version IS NULL
       AND deployment_id IS NULL AND message_id IS NULL AND bot_id IS NULL AND bot_seq IS NULL
       AND session_generation IS NULL AND created_at_ns IS NULL AND expires_at_ns IS NULL
       AND alert_eligible IS NULL AND lease_hash IS NULL AND lease_until_ns IS NULL)
      OR
      (state IN ('queued','leased')
       AND purpose IS NOT NULL AND purpose IN ('sync','alert')
       AND provider IS NOT NULL AND provider IN ('fcm','opush')
       AND typeof(registration_version)='integer' AND registration_version>=1
       AND typeof(settings_version)='integer' AND settings_version>=0
       AND deployment_id IS NOT NULL AND length(deployment_id)>0
       AND message_id IS NOT NULL AND length(message_id)>0
       AND bot_id IS NOT NULL AND length(bot_id)>0
       AND typeof(bot_seq)='integer' AND bot_seq>=1
       AND typeof(session_generation)='integer' AND session_generation>=1
       AND typeof(created_at_ns)='integer' AND created_at_ns>=1
       AND typeof(expires_at_ns)='integer' AND expires_at_ns>created_at_ns
       AND typeof(alert_eligible)='integer' AND alert_eligible IN (0,1)
       AND (purpose='alert' OR (provider='fcm' AND alert_eligible=0))
       -- 完整 job 使用统一九位小数 UTC，字符串排序保留纳秒边界。
       AND length(next_attempt_at)=30 AND substr(next_attempt_at,20,1)='.'
       AND substr(next_attempt_at,21,9) NOT GLOB '*[^0-9]*' AND substr(next_attempt_at,30,1)='Z'
       AND strftime('%Y-%m-%dT%H:%M:%S',next_attempt_at) IS NOT NULL
       AND ((state='queued' AND lease_hash IS NULL AND lease_until_ns IS NULL)
            OR (state='leased' AND typeof(lease_hash)='blob' AND length(lease_hash)=32
                AND typeof(lease_until_ns)='integer' AND lease_until_ns>0)))
    )
);
INSERT INTO push_jobs_schedule(id,session_id,device_id,target_change_seq,state,attempt,next_attempt_at)
SELECT id,session_id,device_id,target_change_seq,state,attempt,next_attempt_at FROM push_jobs;
DROP TABLE push_jobs;
ALTER TABLE push_jobs_schedule RENAME TO push_jobs;
CREATE INDEX push_jobs_pending ON push_jobs(state,next_attempt_at);
CREATE INDEX push_jobs_session ON push_jobs(session_id,device_id);
CREATE INDEX push_jobs_device_lease ON push_jobs(device_id,state,lease_until_ns);
-- 部分唯一索引把同设备串行约束落在数据库；过期租约先在同事务释放。
CREATE UNIQUE INDEX push_jobs_one_lease ON push_jobs(device_id) WHERE state='leased';
-- legacy 永不升级为完整事件；完整事件也不能退回 pending 进入旧同步投影。
CREATE TRIGGER push_jobs_snapshot_immutable BEFORE UPDATE ON push_jobs
WHEN NEW.id IS NOT OLD.id OR NEW.session_id IS NOT OLD.session_id OR NEW.device_id IS NOT OLD.device_id
 OR NEW.purpose IS NOT OLD.purpose OR NEW.provider IS NOT OLD.provider
 OR NEW.registration_version IS NOT OLD.registration_version OR NEW.settings_version IS NOT OLD.settings_version
 OR NEW.deployment_id IS NOT OLD.deployment_id OR NEW.message_id IS NOT OLD.message_id
 OR NEW.bot_id IS NOT OLD.bot_id OR NEW.bot_seq IS NOT OLD.bot_seq
 OR NEW.session_generation IS NOT OLD.session_generation OR NEW.created_at_ns IS NOT OLD.created_at_ns
 OR NEW.expires_at_ns IS NOT OLD.expires_at_ns OR NEW.alert_eligible IS NOT OLD.alert_eligible
 OR (NEW.state='pending')<>(OLD.state='pending')
 OR NEW.target_change_seq<OLD.target_change_seq
 OR (OLD.purpose='alert' AND NEW.target_change_seq IS NOT OLD.target_change_seq)
BEGIN SELECT RAISE(ABORT,'immutable push job snapshot'); END;
