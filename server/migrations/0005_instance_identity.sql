-- 部署身份独立于同步 epoch；安全随机值由 Go 在同一迁移事务内写入。
CREATE TABLE instance_identity (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    instance_id TEXT NOT NULL CHECK (
        length(instance_id) = 36
        AND substr(instance_id, 9, 1) = '-'
        AND substr(instance_id, 14, 1) = '-'
        AND substr(instance_id, 19, 1) = '-'
        AND substr(instance_id, 24, 1) = '-'
        AND length(replace(instance_id, '-', '')) = 32
        AND replace(instance_id, '-', '') NOT GLOB '*[^0-9a-f]*'
        AND substr(instance_id, 15, 1) = '4'
        AND substr(instance_id, 20, 1) IN ('8', '9', 'a', 'b')
    )
);
