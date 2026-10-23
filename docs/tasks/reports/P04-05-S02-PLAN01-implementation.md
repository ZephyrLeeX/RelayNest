# P04-05-S02-PLAN01 限定续作规划交接

## 结论与状态

仅新增一张 [P04-05-S02-S01](../cards/P04-05-S02-S01.md)，覆盖真实 SQLite 验证与现有实现收敛；原 S02 添加预算退出后的组合验收说明，未删原验收或强前置。本规划未执行源码工作、未冻结草稿接口，未标父 S02 done。待不同 agent 独立规划审计、精确集成、调度登记 registry 后派发，不能以本文直接开始开发。

基线 `571a4d1cf870602783426f9644fb8149cf759e61`，树 `/home/lijx/.codex/worktrees/device-session-store/RelayNest`，分支 `codex/device-session-store`。原作者已停止写入。规划只允许本报告、新子卡和原 S02 最小技术说明；未改 API、origin、storage、旧 SQL、依赖、registry 或其它卡。主树与 WebI 无接触。

## 局部事实与拆卡判断

读根 AGENTS、贡献规范、WORKER_PROTOCOL 的预算/退出/角色/交接规则，原 S02 全文、checkpoint、父 P04-05 和先前 PLAN01/S01 的相关前置与边界；先索引后看 `session.go`、`device_sessions.go` 的 Tx/access/list/revoke 局部、`session_test.go` SQLite 夹具、S01 actor/Authorize 与 storage.Write 定位。没有整批灌入任务或测试。

已有 tracked diff 为 `session.go` +51/-37，216 行；新 `device_sessions.go` 166 行未计入 git diff --stat。两实现文件合计382行，现有 `session_test.go` 344行。新增测试可放在父卡已允许 `device_sessions_test.go`，共4个主要实现/测试文件及1份报告。它们共同验证 actor/业务/changes/续期事务和精确撤销/hook，职责紧密；无需拆成依赖未提交源码的“Tx基础”或仅测试卡。一次新上下文的精简输入约5–7k、工作3–5k、证据1–2k，总计9–14k估算，遵守模型实际窗口≤60%和≥25%交付余量；不宣称精确消耗。若再次触及预算，须带剩余真实职责和证据重新评估，不能无界续同卡。

现稿包含 `AuthorizeTx/RenewTx(ctx,*sql.Tx,AdminRequest)`、`ListDeviceSessions`、`RevokeDeviceSession`、`SetRevocationHook`；这些仅候选签名。现有 auth test PASS 为原作者 checkpoint 报告，未获新增 SQLite 验收证据；race/vet/storage/migrations 等未完成，原作者材料/工作预算退出，不能派源码审计。

## 路径、依赖和验收组合

S02-S01 唯一源码范围：`server/internal/auth/session.go`、`server/internal/auth/device_sessions.go`、`server/internal/auth/session_test.go`、`server/internal/auth/device_sessions_test.go`，报告 `docs/tasks/reports/P04-05-S02-S01-implementation.md`。开发接管原草稿后一次完整实现提交，不遗留半成品依赖；原 checkpoint 只读不提交。

前置继承完整 P04-02、完整 P04-03、P04-05-PLAN01、P04-05-S01 的独审与集成，不以局部 helper 替代。最终父 S02 组合是这些强前置 + 新子卡完整实现提交（包含原草稿）+ 另人固定 SHA 独审/必要修复/精确集成验证，再导出实际接口交 S03。规划文档的集成与源码实现的集成是两个门槛。

新增卡验收1–3覆盖父 S02 安装关联、多 Session 持久化、有界列表/失效筛除/无敏感字段、精确撤销及旧凭据失效；验收4–5补足真实私有 actor binding、代次/期限/撤销复核、业务+changes+成功续期同 Tx、失败全回滚及并发边界；验收6覆盖提交后精确 ID hook 与 logout 同路。必跑 gofmt、auth/storage/migrations、race、vet、diff，并逐条建立测试证据。真实 WS/provider E2E 仍为 NOT_RUN；P09 的实际持久化映射失效与发送/回调前复核义务不变。

本规划不要求公共 Tx 专用文件：草稿已将共享复核和续期放进父卡允许的新 `device_sessions.go`，accessTx 留在允许的 `session.go`；没有证据需要扩路径。若后续确需新文件/迁移，先报告最小扩展证据，不能暗改父 scope；0005 未分配，仅确需后 root 串行分配。

裸 `*sql.Tx` 的所属 DB 无法强制检查，草稿只能约定同一 SessionStore 所用 `Store.Write` 的可信 Go 调用与错误传播。root 要求如实记录不是安全 PASS、技术豁免或用户批准；独立审计必须判断，若无法满足原要求则阻断并上报。限制不豁免真实 actor绑定、代次/期限/撤销、成功续期及业务/changes/失败回滚测试。

## 保全与验证

原有三项未提交内容的接管前 SHA-256（本规划结束复核保持一致）：

| 路径 | SHA-256 |
| --- | --- |
| `server/internal/auth/session.go` | `eef6148f6cd0d6bb72949e7e150851bd81478573b48d77296a9a42eaa2231ab4` |
| `server/internal/auth/device_sessions.go` | `884cf761e428121245a541e7cdca4c104ddd2ad529f7a1f8bdc8bf3d53204b15` |
| `docs/tasks/reports/P04-05-S02-checkpoint.md` | `53fdb287635edd538b35ef65eb7e0703744ba26e242ba1de2391292500f3294f` |

本轮仅文档，核对链接/准确路径、原验收保留、`git diff --check` 与 staged 文件清单；不重跑源码测试，也不将原作者已报 PASS 升级为本轮验证。提交仅上述三份规划文档；提交 SHA 由最终交接给出。遗留工作全部在新子卡，父 S02 未完成。
