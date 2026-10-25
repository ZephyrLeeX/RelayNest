# P04-05-S02 开发 checkpoint

状态：未完成，达到派发的材料/工作预算退出点，不能据此验收或派审。基线 `571a4d1cf870602783426f9644fb8149cf759e61`，分支 `codex/device-session-store`，工作树 `/home/lijx/.codex/worktrees/device-session-store/RelayNest`。未创建实现提交。

已改文件：`server/internal/auth/session.go`，新增 `server/internal/auth/device_sessions.go`。现有 access 提取共享 `accessTx`，保持令牌鉴权 SQL 与原期限/代次校验；token Revoke 在成功提交后调用精确 ID hook。新增草稿接口：`SessionStore.AuthorizeTx(ctx, *sql.Tx, AdminRequest) (Session,error)`、`RenewTx` 同签名、`ListDeviceSessions(ctx,actor,cursor string,limit int) (DeviceSessionPage,error)`、`RevokeDeviceSession(ctx,actor,id string) error`、`SetRevocationHook(func(string))`。新文件已 gofmt，session.go 最后修正 err 赋值后尚应重新 gofmt。

AuthorizeTx 复用 S01 actor 私有 store/issuer/issued 绑定与原身份比较，只允许 idle 期限推进；RenewTx 再复核并沿 accessTx 续期。调用方必须用构造 SessionStore 的同一 storage.Store.Write 回调中的 Tx，错误全部传播。裸 sql.Tx 无公开所属 DB API，因此可信 Go 调用方负责接线；跨 SessionStore actor 可拒绝，但相同内容外库克隆加错误 Tx 接线不能强制辨认。该限制已向调度报告并获指示按实际契约记录，不越界改 storage。

列表当前按 Session.id 升序、查询 limit+1，游标 base64url(v1:UUID)，默认50/最大100，负数及非法游标拒绝。按当前 administrator generation、revoked 和 idle/absolute 过滤，成功在同一 Write 中复核续期。精确撤销原子 UPDATE 按 Session.id 和有效性约束，未知/失效返回 ErrSessionNotFound，其他错误沿 ErrInvalidSession/ErrStorage；非自撤末尾 RenewTx，自撤跳过续期。成功提交后通知一次 ID；hook 用互斥锁安全替换。以上仅草稿，尚未完成边界审查和新增测试。

已实际检查：Node `v22.23.2`、npm `10.9.8` 的 `npm ci`、`npm run build` PASS（仅生成忽略的 embed 文件）。Go 绝对路径 `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go`、`GOTOOLCHAIN=local` 执行 `go test ./internal/auth` PASS（9.114s）。前两次草稿命令路径错误/编译 err:= 错误已修正，不能算新增验收通过。尚无新测试；storage/migrations/race/vet/diff 尚未运行。

下一步：补 `device_sessions_test.go` 真实 SQLite，用已有 sessionFixture/createSession；actor 建议通过真实 RequestSecurity.Authorize 获取，复用 newHTTPFixture/securityFixture 或在同包夹具构造私有绑定须明确测试范围。必须覆盖同安装S1/S2+另安装B与关闭重开、分页边界/失效过滤/敏感字段、精确撤销和旧token鉴权、重登旧id/代次/actor提交前失效、并发和回滚不通知、logout同路、existing Tx业务写+续期成功共同commit与错误共同rollback、跨绑定拒绝无副作用。尤其自撤到期时点、所有失败不续期、异常 Rows/提交回滚应审查。跑卡内 auth/storage/migrations、race、vet、gofmt、diff，写完整 implementation 报告并仅stage本卡明确文件独立提交，再交不同 agent 固定 SHA 审计。真实 WS/provider E2E NOT_RUN。
