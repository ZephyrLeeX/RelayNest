# P04-05-PLAN01 设备会话实施边界预备方案

## 对象与结论

规划基线 `153f6679dcdcaf169574ab9915e794ef5cfb8cbc`，分支 `codex/device-session-plan`，独占树 `/home/lijx/.codex/worktrees/device-session-plan/RelayNest`，开始干净。本次仅文档；未改实现、API、registry、依赖或迁移。P04-02 尚未完整 done，启动接线另树进行，本方案不提前放行实现。原 P04-05 前置和全部验收不删改。

一个实现轮次预计至少涉及公共安全提取、会话存储、设备 HTTP、router、startup 五类职责，超过默认3～5主要文件的局部阅读/验证边界，故拆三卡串行。仅技术接入范围补足，无产品、安全政策或外部契约变更，无需新增用户决策；出现实际契约冲突仍须报告。

## 固定证据及准确映射

| 来源 | 现有事实与实施约束 |
| --- | --- |
| PRD §3.1/AC-08、T3.2/T7.1 | 安装独立、单设备退出/远程指定会话、旧凭据失效、不影响其他设备；Web 写同源及会话 CSRF；256 bit 随机原值只存摘要 |
| `server/api/openapi.yaml` 的 `listDeviceSessions`、`DeviceSessionPage`、`Session` | 列有效 Session 的有界页；`Session.id` 不可复用，`device_id` 只是安装元数据；同安装多个 Session 并存。列表没有任何 Token/hash/CSRF/provider token |
| `revokeDeviceSession` | 路径 id 是 Session.id；事务内复核归属/有效性，撤一个代次；未知/失效 404、成功204、自撤 Web 清 Cookie；不能改成按 device_id 撤一组 |
| Android 登录、设备 push/notifications 相关操作 | Android device_id 每安装生成；provider 注册归属当前 Session+provider，撤销使该 Session 所有映射失效，不影响同安装其他 Session。当前 P04-05 不实施这些注册/偏好 HTTP 或多通道发送 |
| P03-07-S02-S01 最终集成报告及 F01 实施报告 | 原卡“单设备撤销”的歧义已冻结为 Session.id；E660～E687 覆盖同安装双 Session、旧 id/新代次/迟到回调隔离。只读输入，声明夹具不代表运行时已测 |
| P03-07-S02-S02/S03 精确集成报告 | 注册 CAS/条件删除/通知配置和 D1～D3 的后续边界已存在；本卡只能交接失效接口，不能复制一套简化注册或借设备登记提前实现 P09 |
| `auth/session.go`（已审 P04-02-S01，A02 对 `1da5e72aed508e01fb0d172b79fba0052af61a52` PASS） | 实际 SessionStore 是 Create、Authenticate（续期）、Inspect（不续期）、Revoke（按 token 当前会话）；没有 List/按 ID 撤销。access 通过 storage.Write 串行事务核数据库有效性/管理员代次。新管理撤销不可误用现有 Revoke(target id) |
| `0003_sessions.sql` | 现有 sessions 已保存 device_id/device_name/client_type 和会话关系，device_id 无唯一约束；当前模型可表达一个安装多会话，安装关联不必强制新增独立表/接口。新增必要索引才申请迁移 |
| `auth/http.go` 与 `httpapi/origin.go` | credential/current 私有；公开 Handler 仅 Login/GetSession/Logout。OriginPolicy 的 Resolve/ValidateWrite 只解决来源，不是可直接消费的完整管理员安全。不能复制私有逻辑给设备/Bot HTTP |
| `httpapi/router.go`、`cmd/relaynest/main.go` | 基线仅挂 auth 三路由，main 仍 `httpapi.New(h)`，无 devices 或 WS；最终 P04-02 startup 接口未冻结，后续派发重新按 main 的实际接口核对 |

API/共享会话契约/origin 只读，不新增身份、权限、批量撤销、安装注册/重命名或偏好功能；不会将设备 ID 当 Session ID。

## DAG、路径及消费交接

建议由调度登记如下依赖（本报告不写 registry）：

`P04-02完整 + P04-03完整 + PLAN01独审集成 → P04-05-S01 → P04-05-S02 → P04-05-S03 → P04-05父卡组合收口`。

P04-03 本身在完整 P04-02 后收口，S01 等完整安全审计后提取公共能力，不产生反向依赖。S02/S03 继承原 P04-02、完整 P04-03 前置。每个实现的审计任务依精确 subject SHA 触发，不要求被审实现先 done；PASS后精确集成，下一卡从实际 main 派发。

- S01：仅 auth 的 http/request_security 两实现及测试。先交出实际公共安全接口和事务复核约束，复用原认证端点，不承诺规划签名。
- S02：auth 的 session/device_sessions 两实现及测试，必要一个新增迁移须由调度串行分配；0001～0004 已占、0005 未分配，本规划不预占。持久化安装关联、分页、按 ID 撤销及 commit 后失效钩子；actor/target 同事务复核，钩子不能替代数据库鉴权。
- S03：devices/http、httpapi/router、cmd/relaynest/main 三实现及对应测试；这是原允许范围外必须的精确扩展，仅现有 HTTP 挂载/启动注入和真实夹具。公共安全/存储不足退前卡修复，不跨界造假。无需改 storage、API、依赖或全目录接线。

真实消费者影响：`P04-06-S02` 现卡依赖 full P04-02/full P04-03，要求复用公共安全且禁止改 auth；实际接口尚未导出。因此建议调度在本规划独审/集成后为 Bot HTTP 增加 **P04-05-S01 实际实现独审并集成** 的依赖，连同精确接口交接；只完成本 PLAN01 或看到 fullP04-03 报告不能替代。若 fullP04-03 最终确已导出等价能力，调度先取精确代码/审计证据重新核验、最小修订 S01，禁止凭未来假接口派发。此轮不改 Bot 卡或状态。

## 验证矩阵与限制

S01 验完整 Cookie/Bearer/CSRF/来源/代次和失败不续期；S02 用真实 SQLite 验多 Session、分页筛除、重开、精确撤销/重登/旧 ID、提交前有效性与失败无副作用；S03 经真实 mux/登录/SQLite 验 A 不影响 B、旧 token 后续请求失败、无敏感字段和 HTTP 状态/Cookie。各卡记录实际工具版本、命令和环境，另人固定 SHA 审计，组合集成后才收口原父卡。

连接关闭只能验证 future hook 的提交后时序/精确 Session.id，真实 WS 尚不存在，E2E 为 NOT_RUN；将来 WS 接入要处理订阅/撤销竞争和数据库再鉴权。P09 引入 provider/job 存储时须将失效事务及迟到结果的会话代次检查真实落地；当前没有映射表，不虚报“已注销推送”。偏好正式功能仍由后续卡负责。

本次只做文档和引用/差异检查，不运行 Go、HTTP、SQLite、API 校验或真机；不把未实现方案写成验证通过。提交前执行 `git diff --check`，核全部新卡本地链接、父原前置和验收保留。初始 shell 没有 `python3`，一次文档编辑命令退出127、没有写入；后用 apply_patch 完成，不安装依赖。

实际检查结果：`git diff --check` PASS；Perl 扫描本次五个文档的 Markdown 本地链接全部存在 PASS；从 `git show HEAD:docs/tasks/cards/P04-05.md` 提取“派发元数据”“测试与完成标准”“审计与交接”三节，和修改后逐字比较一致 PASS。源码没有修改，运行时测试 NOT_RUN。

## 独审/集成交接

本规划须另 agent 固定 doc SHA 审查范围、原验收保留、真实源证据、DAG及消费者影响，然后精确集成；调度再登记子卡/依赖/基线。当前不得派实现、预填 ready 或更改冻结 API。无新需求/安全未决项；遗留条件是原父前置未完成、公共接口未导出和未来真实 WS/provider 尚未实现。
