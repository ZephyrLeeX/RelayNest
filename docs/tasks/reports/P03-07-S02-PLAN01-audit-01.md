# P03-07-S02-PLAN01-A01 独立规划审计

```yaml
task_id: P03-07-S02-PLAN01-A01
subject_id: P03-07-S02-PLAN01
role: auditor
reviewed_base_sha: 33cc7585e873a5debaddc5b93c8657b60d5a73a8
reviewed_implementation_sha: 1122c739a692835657807e9c3cd2d3f59810d8a5
verdict: PASS
findings: []
report: docs/tasks/reports/P03-07-S02-PLAN01-audit-01.md
```

## 范围与结论

在独占工作树 `/home/lijx/.codex/worktrees/api-device-plan/RelayNest`、分支 `codex/api-device-plan` 审计上述精确提交范围。起始 HEAD 与被审 SHA 一致，工作区干净；审计者独立于规划实现者。仅写本报告，不修改卡片、API、registry 或旧报告，不派子 agent。

结论为 **PASS，仅限文档预规划**。变更确为父卡、三张串行子卡及规划实施报告五份文档。原父卡八条验收逐字保留；拆分没有遗失职责或把注册版本屏障拆成跨卡补洞。没有发现需要编号修复的问题，不代表 S02 已实施、S01 已完成或任何运行时门槛通过。

对照根 AGENTS、WORKER_PROTOCOL §1～7 相关约定、CONTRIBUTING §1～4，局部读取 PRD §4.2～4.4、ARCHITECTURE §1/§5、技术设计 T3.1～3.2/T6.3/T7.1/T9.1～9.3，以及 PUSH-DECISION 集成报告“批准与保留门槛”。API 仅定位已有 keys，未整灌设计、API 或示例。

## 原验收与职责覆盖

以下编号依父卡原八条顺序，原文未改。

| 父验收 | 规划落点与审计判断 |
| --- | --- |
| 1 会话列表/撤销 | S02-S01 工作验收覆盖两个端点、管理员 Bearer/Web Cookie、Cookie CSRF/同源、敏感字段、未知/无权设备、全会话 provider/WS 失效及 logout 范围；交付身份/代次不变量，不提前塞注册占位 Schema。 |
| 2 注册及版本屏障 | S02-S02 同卡定义写入、恢复读、条件删除、provider 独立版本/CAS、幂等与冲突、初始/缺失 CAS、删除后迟到写入、重登隔离及持久屏障；没有按端点或文件割裂事务。 |
| 3 条件解绑 | S02-S02 区分客户端删除与内部发送失败解绑；快照限定 provider/注册代次，轮换迟到结果、网络/未知结果不能删除新 token、其他通道、会话或偏好；不向客户端开放 job/厂商错误接口。 |
| 4 通知配置 | S02-S03 同卡定义写入和当前配置恢复、全局/Bot/DND 时区跨午夜/预览、channel/provider 分离、单调配置版本与 CAS/冲突恢复、vendor 前置及非法组合。注册不自动激活模式。 |
| 5 设置变更/时间/耗尽 | 注册耗尽在 S02-S02 闭合，配置耗尽及设备设置 union/恢复在 S02-S03 闭合；版本不回绕、不部分提交；跨设备偏好隔离和服务端时间保留，不修改 S01 envelope/高水位/floor/epoch。 |
| 6 D1～D3 | S02-S03 保留固定通用文案、显式可选 vendor、首次 FCM/补偿、只对未来新事件切换、旧 job 失效/外发残余/超时不跨通道补响和收敛漏提醒。仅 vendor 接受离线展示/撤回限制，FCM T9.2 原验收不减。 |
| 7 夹具与发送快照 | 三卡局部正反夹具和最后联合复核保留多通道/局部更新/幂等/越权/轮换/撤销重登/恢复。S02-S03 完整列会话代次、用途、水位、provider、注册/配置版本、稳定键/过期，以及发送前检查、发送后同快照条件更新、租约/同设备串行、旧结果 CAS 不改新状态、合并不复活提醒。 |
| 8 枚举/非回归/后交接 | 各卡均要求精确 SHA、路径/权限/状态/Schema、检查及 NOT_RUN；最终卡须按原八验收汇总三卡实际证据并枚举全部恢复读/条件删除和剩余全局设置分支。原 S03 等父 S02 收口。 |

## 依赖、上下文及权限边界

依赖方向为 S01 父最终审计集成与最终摘要、本 PLAN01 审计集成/登记 → S02-S01 审计集成 → S02-S02 审计集成 → S02-S03 审计集成 → 调度汇总 S02 → 原 S03。子卡不依赖父 S02 done，集成以审计 PASS 为前提，没有父子死锁。父卡保留原 S01 前置并加强为完整父收口，未抢冻 bootstrap/WS 字段。

每卡具备目标、依赖、输入、路径、验收、检查及交接；指定目标输入≤8k、硬≤12k、局部 keys/行段、直接引用与前卡≤800中文字符摘要。共用分配为约定/摘要≤2k、设计局部≤2k、API 局部≤3k、索引/环境≤1k，且先索引后读、超限 checkpoint，不要求整份 API 或旧报告。三张卡原始文件体积分别为 3452/4139/5378 bytes；结合明确的局部读取策略，当前职责划分具备可执行性。体积不等于 token 实测，最终 S01 keys 尚未确定，派发时仍必须重估；当前审计不保证未来任何材料组合天然落入预算。

共同四份 API 文件串行独占、两 requirements 锁只读；禁止运行时、迁移、registry、技术设计、其他卡/旧报告、RelayShelf/MP、真实推送/发布/部署及额外 agent。新恢复读/条件删除来自原父验收，普通命名留给实施，没有新增管理角色或业务能力。Android 当前会话限制继续由父卡冻结输入与 T3.2 权限表约束，不授权 Web/Bot 注册设备。

25操作/73 Schema/446例在父卡与报告明确仅为本基线的历史核验点，并与既有 sync/read 集成摘要一致。首卡必须采用 S01 父完整收口后的最终 main 精确 SHA，重新核验全部基数、字段、权限、错误、时间及剩余分支；后两卡包含前卡集成增量，不能以本规划树或中间 bootstrap 计数替代派发基线。

D1～D3 与 T9.3/批准摘要一致：OPPO 账号未准备仅阻厂商实测/激活，不阻核心契约；资格、签名分类、SDK/REST 与厂商错误未伪造，也未授权代注册。保留厂商平台前置、FCM 原验收、发送前策略过滤、DND 结束不补响及数据补偿，不放行 G-API/G-OPPO 或 P1/P2。

## 实际检查与限制

| 检查 | 方法与结果 |
| --- | --- |
| 起始归属/基线 | `pwd`、`git status --short`、`git rev-parse HEAD`、`git branch --show-current`：PASS，路径/分支/HEAD 匹配且干净。 |
| 精确变更范围 | `git diff --stat`、`git diff --name-only` 与完整五文档差异，均指定上述两 SHA：PASS，仅五份规划文档。 |
| 父八验收逐字保留 | Perl 从 `git show <base>:docs/tasks/cards/P03-07-S02.md` 与当前文件提取 checkbox；数量均为8且逐字节相同：PASS。 |
| 相对链接 | Perl 按各文件目录解析五文档全部 Markdown 相对链接并检查存在：PASS，分别8/2/3/3/4个。首轮脚本因 Perl `map` 列表括号遗漏，把报告路径错误拼入子卡名而退出2；修正检查脚本后五文件完整通过，未改被审文件。 |
| 六份 API 不变 | `git diff --exit-code <base> <subject> -- server/api/openapi.yaml server/api/examples.json server/api/validate.py server/api/README.md server/api/requirements.in server/api/requirements.txt`：PASS，退出0、无差异，包含两锁。 |
| 差异格式 | `git diff --check <base> <subject>`：PASS，退出0。 |
| keys 导航 | `rg -n` 局部核对 `getSession`、`Sequence`、`Version`、`SyncChange`、`SyncPage`：PASS，规划基线导航行可定位；未来按 keys 重定位。 |

纯文档审计未运行 OpenAPI 校验、业务矩阵、破坏探针、HTTP/SQLite 并发、多 worker、真实 WS、Android、SDK、平台展示、真机 DND 或厂商资格验证。这些均 NOT_RUN；实现卡仍需执行其已锁环境正式检查。只核验规划实施报告的现存结果与可复核声明，没有追认其历史临时脚本、解释器失败记录或未知外部环境。没有联网核实厂商接口，因为本次没有引入或冻结任何此类接口事实。

交付后仍须由调度执行本规划精确集成/registry 登记及 S01 最终门槛复核，再派发 S02 实施。报告 PASS 不替代这些步骤。
