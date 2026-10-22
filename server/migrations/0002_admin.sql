-- 固定主键和 CHECK 在所有进程之间共同保证唯一管理员。
CREATE TABLE administrator (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    username TEXT NOT NULL UNIQUE CHECK (length(CAST(username AS BLOB)) > 0),
    password_hash TEXT NOT NULL CHECK (length(password_hash) > 0),
    auth_generation INTEGER NOT NULL DEFAULT 1
        CHECK (typeof(auth_generation) = 'integer' AND auth_generation >= 1)
);
