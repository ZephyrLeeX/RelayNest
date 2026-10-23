# P04-05-S01-F01 修复实施报告

基线 `3e7221d53f92cfc5a8d783c226928d0004fda49f`；工作树 `/home/lijx/.codex/worktrees/admin-request-security/RelayNest`，分支 `codex/admin-request-security`，开始干净。仅处理 `P04-05-S01-A01-F01`（P2），修改本报告及两份 request_security Go 文件。

## 实现与边界

`AdminRequest` 增加私有 issuer（精确 RequestSecurity 指针）、store（签发 SessionStore 指针）及 issued（签发 Session 值副本）。公开字段及构造/Authorize/RenewRead/ClearCookie 签名不变，无新公开 API 或 Tx 方法。外部消费者只能由 Authorize 获得可续期 actor；零值、仅复制公开元数据、跨 guard（含同 store）、跨 store、guard 改接 store、公开 Session 任意字段篡改均先返回 ErrInvalidSession。

RenewRead 先核验签发域及完整公开 Session 与私有副本一致，再以私有 ClientType/凭据 Inspect。Inspect 不续期，必须匹配原 Session.ID、Username、ClientType、DeviceID、DeviceName、AbsoluteExpiresAt；idle 允许被其他成功读取推进。匹配后才调用原 Authenticate，由其再次核对当前撤销、期限、管理员代次并续期。Inspect/Authenticate 原错误直接传播；取消 context 继承 SessionStore 的 ErrStorage 映射，不增加新的错误政策。

未改 HTTP 政策、session/storage/SQL 定义、来源、路由、依赖。Inspect 与 Authenticate 仍是独立事务，不提供业务事务授权或任意数据库外部改写下的原子身份保证；S02 必须真实实现同一业务事务的 actor/target 复核及成功续期，Bot 消费者仍须自身事务接入。本卡不标消费者 ready，不关闭 finding；固定修复 SHA 待另人 A02 重审和精确集成。

## 反例与回归证据

`TestRequestSecurityForeignStoreActor` 使用双真实 SQLite/HTTP 夹具和仅测试确定性随机源，实际断言 token 相同、Session.ID 不同。源 active/revoked 两分支，推进 B 时钟后向 B 传 A actor；同时比较两库 administrator/sessions 全列快照及 B idle 期限，快照仅内存比较，不打印凭据。

修复前执行 `go test -count=1 ./internal/auth -run '^TestRequestSecurityForeignStoreActor$'`：实际 FAIL（0.457s），两分支均报 foreign actor accepted、sessions/idle expiry 改变、destination idle expiry 改变。修复后该探针随下面普通及 race 检查实际 PASS。触发前提是重复凭据加错误接线，不能视作当前单 store HTTP 远程绕过或生产随机碰撞。

`TestRequestSecurityActorBinding` 使用真实库验证同 store 跨 guard、零值/伪造、七类 Session 篡改、guard 改接 store，拒绝均比较全列快照；验证取消无写入、同实例正常及重复续期，并模拟凭据对应数据库 Session.ID 替换，确认续期之前拒绝且数据库不变。保留原撤销/idle/absolute/管理员代次、Cookie/Bearer/歧义、真实 Bot、来源/代理/Secure Cookie、CSRF、失败不续期及 HTTP/P04-04/strict JSON 断言；原正常入口无副作用断言加强为全列快照。

## 实际检查

Go 路径 `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go`，版本 `go1.27.3 linux/amd64`；以下 Go 命令均在 server 下以该绝对路径并设置 `GOTOOLCHAIN=local` 执行。

- `go test -count=1 ./internal/auth ./internal/httpapi ./cmd/relaynest` PASS：9.461s / 0.122s / 1.223s。
- `go test -race -count=1 ./internal/auth ./internal/httpapi ./cmd/relaynest` PASS：30.804s / 1.124s / 7.948s。
- `go vet ./internal/auth ./internal/httpapi ./cmd/relaynest` PASS。
- 修复后单独执行 `go test -count=1 ./internal/auth -run '^TestRequestSecurity(ForeignStoreActor|ActorBinding)$'` PASS（0.724s）。
- 同目录绝对路径 gofmt 对两份修改 Go 文件执行 `-w`，随后 `-l` 无输出；根目录 `git diff --check` PASS。
- 未执行无关 Web 构建；已有 web dist 满足测试条件。无其他要求检查处于 NOT_RUN。
