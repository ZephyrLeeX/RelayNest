# P00-03 MoviePilot 环境登记与交接

日期：2026-10-22。角色：researcher。基线：`ab6530f93c300ca870d6aeb8cb4bdd79c226b951`。工作树：`/home/lijx/.codex/worktrees/moviepilot-ci/RelayNest`，分支：`codex/moviepilot-ci-run3`。

## 结论与证据时点

本卡交付既有环境证据登记及 P10/P11 的必要输入清单，未重跑 PoC。**P1：PASS；G-MP：pass**，沿用用户调整最小 PoC 范围后的[验收结论](P01-06-user-acceptance.md)。方案 A 显式助手补丁已获批准；完整上下文恢复、自动编辑为可选项，不重新请求批准。原报告中的历史 BLOCKED 不能覆盖后来用户放行，原 NOT_RUN 也不改为 PASS。

所有实例观察均来自 **2026-10-21** 的已有报告；本报告日期不代表实例于 2026-10-22 仍健康、可访问或指纹不变。本轮只读取仓库文档和助手固定常量，没有连接现网、读取 SSH 配置/私钥或 token 正文、运行探针、修改实例/Compose/数据库。当前现场状态与凭据可用性均 **NOT_RUN**。

P00-01 前置已存在[集成报告](P00-01-integration-01.md)：原实现 `522dbeb3cb96c645a1351e05452a01df54100093`、原审计 `52635d825cbe00e0eb025ab1875de045b9211409`，目标实现 `56f10516fa53e5b963a72e56c4d3c26bae777a7d`、目标审计 `4dc245f35ebfa26035dda4307bc78176a2f5d7f7`，结论 PASS。本卡不改台账或前置状态。

## 固定版本与复现边界

| 对象 | 既有记录 | 证明范围 |
| --- | --- | --- |
| 最低适配目标 | MP v3.0.0，`773d8490645e5b7dd3c07a397e6e6d5ac58ef48e` | 仅源码研究；运行 NOT_RUN |
| 拟议市场声明 | `>=3.0.0` | 不是已批准发布声明或全版本兼容结果 |
| 实际研究/运行基线 | MP v3.1.5，`99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3` | 原容器关键文件比对及限定主进程运行证据；非整个容器文件层等同上游源码 |
| 配套前端 | MoviePilot-Frontend v3.1.5，`21166c45fc1596af42978659ddf253b207b939cf` | 固定源码研究；原容器版本文件声明前后端 v3.1.5，未证明部署前端全量构建逐字节等同该 commit，真实 UI 点击/配置保存 NOT_RUN |
| 原镜像/Compose 声明 | `jxxghp/moviepilot-v3:3.1.5` | 标签可变，不是不可变镜像定位 |
| 原本地镜像 ID | `sha256:d9511187e90725fa8c408ad296eb5f0dbfa8b262cadd6a06b0a49d1b40175d61` | 本地 image ID，不是 registry RepoDigest |
| 恢复后实际本地镜像 | `relaynest-local/moviepilot-recovery:20261021-callback` | 本地恢复快照，未发布 |
| 恢复后本地镜像 ID | `sha256:2d41a349e3a8b72f02a6b82a29d3c74b1b63e5f1d21a8fdda0a568c45d1e94d1` | 2026-10-21 记录，不保证现仍保留 |

来源：[P01-01 固定研究基线](P01-01-implementation.md)、[P01-05 基线及环境](P01-05-implementation.md)、[callback-live 重启故障、影响与恢复](P01-06-callback-live.md)。本轮核对的是仓库记录一致性，没有重新取得上游源码或拉取镜像。**registry RepoDigest 未见已有证据，仍缺失**；本地 image ID 不能填入该字段，亦不能由标签推算 digest。

方案 A 助手只接受 APP_VERSION=`v3.1.5` 及精确两文件指纹；同版本未知源码也拒绝，不外推至 v3.0.0 或所有后续版本。下表原始值与基线 `integrations/moviepilot/poc/relaynestpatch/core_patch.py` 常量一致；补丁值来自 callback-live 的既有现场核对。

| 文件 | 原始 SHA-256 | 已记录补丁 SHA-256 |
| --- | --- | --- |
| `app/schemas/types.py` | `47c5a9f4a10d9350ac61ee64e31871880ba8571326b8df31237d4a748f543419` | `b536fb434f26e45a43a6c173ee7648c19c9af70e0083717a8783812a57e4292c` |
| `app/schemas/notification.py` | `a83d9e1123fea976e30dad3bd258e63813157d1d2373ac9074ed55335983b31d` | `4503e3f57d355d329439be8253bdfc74662a22bcfd1c989d784cbca844b48286` |

助手仅新增 RelayNest 枚举及 INLINE_BUTTONS/CALLBACK_QUERIES，无 MESSAGE_EDITING。已有 runtime_callback_active=true、runtime_editing_active=false；这两个值也是历史现场结果。[助手原型报告](P01-06-callback-patch-prototype.md) 的未部署描述属于较早时点，后由 callback-live 的真实应用证据补充。

## 实例与访问方式（无凭据）

- 实例为用户既有 fnOS / Docker Compose 服务，容器名 `moviepilot-v2`。用户明确授权在该实例最小验证，覆盖原卡独立测试实例选择；**没有独立隔离实例证据，不能标记“与现网隔离 PASS”**。合成身份、loopback 桩及不执行有效业务选择限制了当时测试范围，不等于基础设施隔离。
- 历史入口为 SSH 显式使用 `-F /home/lijx/.ssh/config fnos`，由用户已有安全配置加载认证。本报告只登记别名及配置路径，不读取配置或私钥，也不新增公开主机地址。
- 历史 MP API 通过容器内 `127.0.0.1` 访问；既有 API 凭据仅在当时进程内存与请求头使用。当前应恢复安全加载方式，不能把凭据复制进报告。没有证据可登记正式 Hub/Android 公网 Origin、端口或当前可达性。
- 助手已通过官方本地仓库机制安装，持久仓库 `/config/relaynest-local-plugins`，来源 `local://RelayNestPatch?version=v3`，版本 0.0.1、system_version=`==3.1.5`。备份/manifest 使用插件持久数据目录下 `relaynest-callback-v1/v3.1.5/`；这些是既有记录，不是本轮目录检查结果。

## 恢复快照与重建风险

callback-live 记录旧容器因 `/netdisk` 的绑定源不存在而停止后无法启动；用户随后明确授权移除该绑定。Compose 文件 `/vol1/1000/docker/media-stack/compose.yaml` 当时仅删除 `/vol02/1000-1-12ecd637/media:/netdisk`。不能把这次操作描述为“未改部署”。

恢复保留了旧可写层中的插件、依赖及补丁，使用上列本地恢复镜像；报告记录镜像配置运行 Env 值与标签值已清空、入口 `/bin/false`，不作为携带凭据元数据的发布备份。bind 卷数据没有复制入镜像。新容器 ID 为 `08a6046e4c274048a0f0bb0ac8048be46e2d0d291c387f73fd46a54612313be8`，StartedAt=`2026-10-21T08:40:14.136058624Z`，当时 running/healthy；保留 `/config`、`/moviepilot/.cloakbrowser`、`/media`，`/netdisk` 不再可用，依赖功能需有效路径。

**实际容器与 Compose 声明镜像不一致**。后续重建/升级可能覆盖补丁与旧可写层变化；保留持久备份并不保证新镜像自动带补丁。正式安装登记后再启动、由声明镜像重建、升级后重新应用、线上 restore 后重启仍为 NOT_RUN。P10/P11 操作前需核对实际镜像/挂载/助手状态与版本指纹，并安排可恢复的变更窗口；本轮不执行这些动作。

## 身份与业务边界

P01-05 的 24 次创建、2 次手动编辑、1 次显式重放，以及 callback-live 的媒体/种子导航 28 项，是固定合成用例与临时 HTTP 桩证据。后者只覆盖 next/previous、错误身份、失效/边界请求；未执行有效选择、搜索、订阅、下载，不是正式 Hub/Android 端到端或持久恢复证明。

跨 Bot 隔离依赖不同命名空间 userid；P01-04/05 已发现相同 userid 跨 source 可覆盖宿主会话，不能宣称 MP 按 source 原生隔离。正式插件仍需绑定可信 Bot/user/request 身份及消息映射；命名空间 userid 不代表 MP 管理员，不能对所有输入硬编码 `is_channel_admin=true`。MP 原生流程负责业务权限、危险确认和执行防重；Hub 仅传输、存储、路由，不执行命令或 MCP。此界限不受完整上下文/自动编辑列为可选影响。

## 卡片验收与最小补齐条件

| 卡片项/缺项 | 本报告判定 | 最小后续输入及影响 |
| --- | --- | --- |
| 核对 commit、镜像、配套前端 | 既有记录登记 PASS；当前现场复核 NOT_RUN | 已锁定源码与本地 ID；若需镜像远程复现，需安全读取实际 RepoDigests/受控镜像获取方式，不能用 tag 或 image ID 冒充 |
| 版本可复现 | 部分支持；完整可重建门槛 blocked | 恢复快照可用性、原镜像 RepoDigest、必要持久卷/插件来源及配套前端部署标识未重新确认；需在后续授权环境核实，不要求本轮重跑 P1 |
| 测试实例与现网隔离 | 未实现该物理隔离；用户已批准 P1 环境例外 | 不重新阻断 P1；P10/P11 应明确可用隔离环境或既有实例的授权测试窗口、业务范围和回退条件，不能沿用旧最小测试授权扩大为真实业务操作 |
| 访问/权限 | 历史方式已登记；当前 NOT_RUN | fnOS SSH 别名和认证的安全加载方式、容器/API 管理权限；无需在仓库提供秘密值 |
| 正式 P10/P11 组合验证 | NOT_RUN；对应未来运行验收尚无输入 | 明确受测 Hub/MP/Android 版本及地址、Bot 凭据安全注入、可用目标手机、真实用户/权限测试身份与必要业务夹具；需要媒体路径时补有效挂载。手机型号/系统与当前可用性本卡无证据 |
| 助手恢复/重建与其它版本 | NOT_RUN | 仅在正式功能涉及范围补版本/指纹、备份卷、重启/重建恢复及拒绝未知版本证据；v3.0.0 运行需其独立环境与适配，不能宣称当前助手支持 |
| 缺失条件逐项列出 | PASS（文档交付） | 如上；不把文档 PASS 当运行门槛通过 |

以上缺项明确阻塞其对应的完整环境复现或未来 P10/P11 运行验收，不推翻已获用户放行的 P1/G-MP，也不自动要求机械重测全部 PoC。本任务可交付研究报告，但不自标 P00-03 done；由独立审计及调度按真实缺项处理。

## 来源固定与本轮检查

以下为本基线 Git blob ID（不是文件 SHA-256），供独立审计在不可变对象上读取；源报告不改写：

| 相对 `docs/tasks/reports/` 路径 | 基线 blob ID |
| --- | --- |
| `P01-06-user-acceptance.md` | `172a56fb289a854c396f756f2b1ff8e5bb08e6e5` |
| `P01-06-callback-live.md` | `e42bae783dd8f13582fc68a0a53acdae14d2a1be` |
| `P01-01-implementation.md` | `30c72f5f0a16900b05f185a93896c4e53e5b8879` |
| `P01-05-implementation.md` | `c85db99eac255869cdde550a3215b7db953047d3` |
| `P01-06-implementation.md` | `2ca54e14cf408b38416ebba63eb054600577e43b` |

助手 `integrations/moviepilot/poc/relaynestpatch/core_patch.py` 基线 blob：`0311357f8a19d363f17d975e952084588e51169c`。P1 验收源最后修改提交 `b058474ce4e7e0b4321fe8dec174c23601ce30a7`，callback-live 为 `5d31b6fcebd0a02f28a9b9792ebdd6b54696abc4`；这些是仓库证据来源，不是本卡实现 SHA。

本轮环境 Linux/zsh/Git，本地文档检查：

- `git rev-parse HEAD`、`git branch --show-current`、`git status --porcelain`：起始基线/分支正确且干净。
- `git rev-parse HEAD:<source-path>`、`git log -1 --format='%H' -- <source-path>`：核对上述 blob 与来源提交。
- 首次尝试 `python`、`python3` 均因命令不存在未执行检查；改用 `perl` 核对报告 blob 表、版本/ref/镜像及文件指纹均可追溯至固定源报告/助手，核对 Markdown 本地链接与唯一变更路径，通过；不加载助手、不执行 PoC。
- `git diff --check`、提交前 diff/status：通过；仅新增本报告。无业务代码变更，未运行无关代码测试或任何现场检查。

本报告不预写自身提交 SHA；提交后向调度交付精确 SHA，由不同 agent 对该 SHA 独立审计。未推送、部署或发布。
