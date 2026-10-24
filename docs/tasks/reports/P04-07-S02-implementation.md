# P04-07-S02 Bot 删除确认与基础事务

- 角色 developer；结果 implemented，等待另一 agent 固定 SHA 独审与集成，不自标 done。
- 基线 `7bd18fbbaf8125850de36f2ae2d36cfb507e3ff4`；树 `/home/lijx/Projects/RelayNest-recovery-20261024/bot-delete-store`；分支 `codex/bot-delete-store-20261024`，开始干净。实现 SHA 提交后交接，不预写自身 SHA。
- 仅改 bots 内 models/lifecycle/delete_test/lifecycle_auth_test 四个 Go 文件及本报告，无 HTTP/router/main/auth/storage/changes/API/SQL/依赖/registry 修改。
- 已读 AGENTS/CONTRIBUTING/WORKER_PROTOCOL、S02/父卡、PLAN01/A01/I01、S01 实际 I01、架构 §1/PRD3.1/T4.3/只读 DeleteBotRequest/deleteBot 及局部实现；S01 已独审集成，不重规划。

## 接口与原子边界

`DeleteRequest{ConfirmName string, ExpectedVersion int64}`；`Delete(ctx,id,req)` / `DeleteWithWrite(ctx,id,req,write,hook)` 返回既有 `changes.Sequence`。调用方确认后才调用，取消不调用仓储。缺字段（零值）、空名/ID、非正版本、nil write 在进入事务前返回 ErrInvalid。Tx 内读取真实 Bot，先核对版本、逐字名称、版本耗尽；stale/耗尽为 ErrConflict，名称不符 ErrInvalid，unknown ErrNotFound。不新加 trim/case 策略；HTTP 错误映射仍属 S03 门槛。

顺序：当前快照/确认 → LifecycleDeleteBefore → DELETE（id/version/name CAS）→ LifecycleDeleteAfter → typed changes.BotDeleted → 调用方续期 → 外层 COMMIT。复用 S01 LifecycleFunc；两个删除阶段的 Before 均为当前公开 Bot，After 零值，后续适配使用 Before.ID，终态版本 Before.Version+1。扩展必须同步使用给定 Tx、不保留、不嵌套/网络等待/自行提交回滚。前置扩展后再次 CAS，避免删错目标版本。任何错误均返回零 Sequence。

实际基础删除仅 Bot 与两凭据（沿 0004 既有 FK cascade）。changes 无 Bot FK，既有 BotDeleted 元数据保留 entity_id/bot_id/revision（终态版本）/UTC created_at（供 deleted_at 投影），重开数据库仍可读。本卡不实现完整 SyncChange 投影或墓碑清理策略。无需持久新字段/表，未申请或预占迁移，0001–0004 不变。没有生产装配调用。

实际 delete hook 为前后两阶段同 Tx 接入位置，测试标记表仅证明回滚，不是 P06 队列、消息或附件实现；空 hook 不代表完整永久删除。

## 逐项验收

全部新增数据库测试使用真实临时 SQLite 文件及 storage.Store，无 mock DB。

| 验收 | 证据与结果 |
| --- | --- |
| 正确双确认删除、旧 Identity 拒绝、B 隔离 | PASS：TestDeleteIsolationAndMarker；Bot/两凭据不存在，两旧秘密 Authenticate 及两 Identity 的 Send/Claim/ACK/Renew/Download 拒绝。B 公开元数据、凭据 ID/代次/摘要逐字不变，B 两秘密可用、原 changes 保留。 |
| 删除标记及保密 | PASS：同用例重开 SQLite 读 BotDeleted/entity_id/bot_id/revision/UTC 时间；重复删除无额外日志。hook JSON 无两秘密及 digest；删除不调用故障随机源。 |
| 名称、stale、unknown、缺字段及耗尽 | PASS：TestDeleteRollbackMatrix/TestDeleteInputAndBoundary；含大小写、尾空格、缺名/版本、空 ID、nil write、非正版本、MaxInt64、日志序号耗尽。失败比较 Bot/全部凭据摘要/changes 数量与序号/B 快照；无效请求 write 调用数为零。 |
| 最后有效数值与名称策略 | PASS：TestDeleteInputAndBoundary；原样空白名称可确认，终态版本及 change_seq 到 MaxInt64 无回绕；当前版本已上限拒绝。 |
| 并发 edit/delete、重复、旧版本反序 | PASS：TestDeleteCommitOrdering；两个真实 Store，显式事务屏障固定 edit-first/delete-first/delete-delete，后续旧操作 Conflict/NotFound；新版本确认删除后更旧请求不复活/重复写墓碑。 |
| 旧 Identity 与删除提交顺序 | PASS：TestDeleteAuthorizationOrdering；delete-first 拒绝旧 Identity 最终 Tx 写；business-first 保留已经合法提交的测试标记，随后删除；不声称追回已返回数据。 |
| 前后扩展、凭据/实体写、changes、提交失败回滚 | PASS：TestDeleteRollbackMatrix；hook 写真实标记再失败，SQL trigger ABORT、DELETE IGNORE/CAS、前置 hook 改版 CAS、changes ABORT/序号耗尽，均无部分写。deferred FK 制造真实 COMMIT 失败，实体/凭据/日志整体恢复。 |
| actor 续期及撤销 | PASS：TestLifecycleAuthorizedSessionTransaction/delete；真实 RequestSecurity 签发 actor + WithAuthorizedWrite。成功删除/双 hook/续期同提交；changes、续期 trigger、COMMIT 失败均回滚，revoked actor 不进入 hook；另一会话 idle 时间不变。 |
| 取消确认 | 契约要求调用方取消不调用；本卡没有取消写方法，缺少确认在 write 前拒绝已有证据。真实 UI/HTTP 取消及其续期 NOT_RUN，后续调用方负责，不把手写空调用当证明。 |

## 实际检查

2026-10-25 Linux amd64；Go1.27.3 路径 `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go`，GOTOOLCHAIN=local。Go 命令在 server 执行。六包：`./internal/bots ./internal/changes ./internal/storage ./internal/auth ./internal/httpapi ./cmd/relaynest`。

| 命令 | 真实结果 |
| --- | --- |
| 首轮 go test -count=1 bots/changes/storage | FAIL：隔离树缺 internal/webui/dist，bots 引用 httpapi 导致 embed 失败；changes/storage 通过。保留环境失败记录。 |
| web 中 Node22.23.2/npm10.9.8，npm ci、npm run build | PASS；16 packages，typecheck+Vite 成功。仅 ignored node_modules/dist，无依赖源码修改，不提交产物。 |
| 最终 go test -count=1 六包 | PASS，非缓存：bots 0.407s、changes 0.056s、storage 0.556s、auth 13.448s、httpapi 0.113s、cmd 17.152s。 |
| go test -race -count=1 六包 | PASS：bots 5.522s、changes 1.435s、storage 2.469s、auth 43.548s、httpapi 1.126s、cmd 60.245s。 |
| go vet 六包 | PASS，退出0，无输出。 |
| go build -o /tmp/relaynest-p0407s02 ./cmd/relaynest | PASS，产物不入库。 |
| gofmt -l 四个改动 Go 文件 | PASS，无输出。 |
| git diff --check | PASS。 |

## 后续与边界

S01 停用/独立重置/旧代次最终 Tx 验证，与 S02 基础删除证据可支持父 P04-07 基础事务验收；仍需本卡独审 PASS 及精确集成后由调度收口，不自行标父卡 done。

完整消息/受保护队列/租约/附件引用/异步文件回收、完整永久 cascade、真实 DELETE/reset HTTP、真实取消 UI、手机/NAS/外部系统均 NOT_RUN。S03 仍等待 P06-07、设备 S03 最终 A/I 与公开错误映射契约；G1 C01 独立开发，本卡未引入新 instance header 或猜签名。未 push/部署/新聊天/现网删除/额外 agent。
