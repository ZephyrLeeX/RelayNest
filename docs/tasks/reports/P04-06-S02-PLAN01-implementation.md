# P04-06-S02-PLAN01 Bot 管理写事务授权接入补充规划

## 对象与结论

- 角色：规划；结果：仅补充卡片，等待独立审计、精确集成及调度登记，不代表 Bot HTTP 或设备 S02 已实现。
- 基线：`32161b51abe1ea408d19a53823267e528e67c263`，`codex/bot-store`，工作树 `/home/lijx/.codex/worktrees/bot-store/RelayNest`；开始 HEAD 符合派发且干净。本次仅改 P04-06-S02 卡与本报告，不改 registry、源码、API、SQL、依赖或其他卡。
- 原卡仅依 Bot S01 与请求安全不足以封闭请求检查后的撤销/重置竞争。追加 P04-05-S01/S02 的实际独审集成依赖，以及 Bot 仓储最小事务授权适配；保留原前置、父卡五项验收和未来消息业务限制。
- 这是既有会话撤销、代次、失败不续期与同事务安全要求的接入落实；不引入新权限、策略或外部接口。无需额外需求确认。若实际开发需要改变这些规则、API、auth 所属实现或既定范围，交回调度，不以本规划授权扩张。

## 固定证据与缺口

| 固定对象 | 局部证据与含义 |
| --- | --- |
| 本基线 Bot S01；实现 `56fa387e91378a85f0fbaf6a34d51c228a57f196`；A01 PASS 报告在本基线 | `server/internal/bots/repository.go` 的 Create/Patch 无 actor 参数，各自行 `Store.Write`；创建 Bot/两种凭据、PATCH CAS/版本、公共 `changes.Append` 已原子提交。不是已接入管理员事务授权的证明。 |
| 本基线 S01 实施报告与 `authorization.go` | Bot Identity/AuthorizeTx 只承载 Bot 凭据、能力和隔离，Manage 拒绝；不能充当管理员授权。已有能力交集、旧代次、撤权竞争要求须保留。 |
| `3f39182da4cd67522a015a2ec675a3ad43878944` 的 P04-05-PLAN01 报告及 S01/S02 卡 | 公共安全入口先交真实接口；设备 S02 在真实写事务内复核 actor/target、落实成功续期和竞争测试。规划不承诺函数签名；卡路径在该固定提交读取，本树尚无这些子卡。 |
| 公共入口开发提交 `27c8df4f9d5a282e05d35146e28d65233a79311f` 的 `auth/request_security.go`、`auth/session.go` 及 P04-05-S01 实施报告 | `RequestSecurity.Authorize` 仅 Inspect；AdminRequest 凭据私有，无原值 getter。`RenewRead` 调 Authenticate 独立提交；只适用普通成功读取，不能代替业务写事务复核。SessionStore 当前 Inspect/Authenticate/Revoke 经私有 access 自行 Write，无公开接收已有 Tx 的复核方法。该提交仅开发证据，尚待独审集成，不是可直接消费基线。 |

上述异树材料均以 `git show <固定 SHA>:<路径>` 只读核对，未写其他工作树。依据为 T3.1 管理员路由/错误边界、T7.1 会话撤销/期限/auth_generation 与活动续期、T7.2 Bot 凭据独立授权，以及原 S01/S02 的原子性和安全前置；没有新增安全政策。

时间窗口是：请求安全 Inspect 成功 → 会话撤销、到期或管理员重置 → 原 Create/Patch 自开事务仍提交。预先 Authenticate 或 RenewRead 只能缩短窗口；复制私有 SQL 又会形成第二套认证。因此必须等公共 S02 提供真实同事务能力，再由 Bot 消费其精确已审 SHA。

## 最小实施边界

公共 P04-05-S02 开发及独审负责冻结实际事务接口，交接可供既有业务事务使用的 actor 复核与成功续期能力、输入身份来源、错误及回滚语义，并用真实 SQLite 证明。这里的“事务能力”是行为要求，不是规划 API 名称；不声称现有 S02 已实现，不用虚构签名或桩先行。若实际 S02 不能支持 Bot 接入，先交调度由其原归属补足，Bot 卡不得修改 auth、复制 SQL 或另造认证。

Bot S02 仅在 `server/internal/bots/**` 做必要的仓储接入及测试，沿用现有三 HTTP 路由适配范围；具体签名由实际已审接口决定。POST/PATCH 的 actor 复核、Bot 创建或目标/CAS 检查、版本/凭据写入、公共 changes、成功续期必须共享承载业务的同一真实 Tx；任一步失败全回滚，外层成功提交后才返回结果和创建明文。不新增嵌套 Write、独立认证事务或提交后补 changes，不重做 S01 仓储设计。

GET 走同一公共 RequestSecurity，业务读取成功后可用 RenewRead 再核会话并续期，必须先处理其错误再输出响应；失败读取不得先续期。消费者使用私有凭据封装，不提取、记录或输出 raw token。Bot API Token/Webhook Key 仍不能授予任何管理权限；保留能力收回、旧代次和跨 Bot 的真实 S01 授权证明。

## DAG、文件归属与集成建议

原 Bot S01、完整 P04-02/P04-03、P04-06-PLAN01 前置全部保留。建议新增边：

`P04-05-S01 独审集成 → P04-05-S02 实际事务能力独审集成 → Bot S02`；`本 PLAN01 独审集成 → Bot S02`。

P04-05-S03 设备 HTTP 不属于 Bot 前置，避免无谓串行；P04-05-S02 仍继承其原 S01/完整安全前置。调度先固定公共入口和事务能力的实际组合 SHA，再派发 Bot S02，从该 main 基线实施，最终把 Bot S01、公共安全与事务能力、本 PLAN01、Bot S02 的精确 SHA 及审计报告一起组合。不能只凭计划或 subject done 触发消费；合并后改业务代码须重审。

- 公共 S02：仅其已授权 auth/session/device_sessions 路径和测试；本规划不改该卡，不占迁移号，尤其不预占 0005。
- Bot S02：bots 最小接入、原 httpapi/router 与 cmd 启动注入及测试；auth/storage/迁移/API 只读，不增加其他管理功能。
- 本规划：仅本卡与报告；独立审计者只写自己的审计报告。registry 及依赖登记由调度处理，无需新实施子卡。

## 验证要求及本次验证

后续真实 SQLite/HTTP 测试必须在 Inspect 后、业务事务前可控插入 actor 撤销、期限跨越和管理员代次重置，断言拒绝且 Bot、凭据、版本、changes、会话续期无副作用；验证有效管理员成功、目标 CAS 冲突、业务/changes/续期失败全回滚。保留两个 Bot、两类凭据、能力收回/新增能力需重新认证、旧凭据代次与跨 Bot 拒绝组合证据。GET 复核失败不得先输出成功。未知未来消息路由、真实 WS/通知/NAS/手机与正式 AC 仍按原卡列 NOT_RUN，不能用存储测试冒充。

本次仅文档规划，`git diff --check` 通过，允许路径核对通过；以 `/home/lijx/.local/share/mise/installs/python/3.13.15/bin/python3` 检查本地 Markdown 链接、原卡全部验收勾选逐行保留及报告少于 100 行，均通过。初次裸 `python3` 因 PATH 不含解释器未运行，改用上述已安装路径完成。未执行 Go/HTTP 测试，未声称上述运行时验收通过。独立审计应复查固定源、依赖和范围确实支持该结论；具体提交 SHA 由提交后的交接给出，不在报告自填。
