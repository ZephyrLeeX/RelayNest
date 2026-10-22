# P00-05 fnOS / Nginx 测试环境登记

日期：2026-10-22；本轮本地取证时间 `2026-10-22T01:21:10Z`。角色：研究/登记。

**研究报告已交付，环境门槛 BLOCKED。** 本轮没有可安全复用的明确 NAS/OpenWrt 连接配置，未连接外部主机；CPU、卷、Nginx、Origin、端口、TLS 及独立测试隔离均未获当前实测证据。下文历史 P1 记录不能替代本卡当前现场核验，也不能据此放行 P11/P12 或部署。

## 基线、范围与前置证据

- 基线：`1cfc7516187f9a3b7c0c42f792f745d699576ec8`。
- 工作树：`/home/lijx/.codex/worktrees/p2-evidence/RelayNest`；分支 `codex/p2-evidence-run3`；开始时 `git status --porcelain=v1` 无输出。
- 只新增本报告；不修改源码、任务卡、registry 或现网。用户授权由总调度传达：允许必要 P0 收口和后续测试准备；真实部署、MP 重建升级、破坏性数据操作、公开发布最后一步另需确认。
- 已读根 AGENTS、CONTRIBUTING、WORKER_PROTOCOL、P00-05，架构 §1/§6、阶段计划 P0/P11/P12，以及 P01-06 callback-live/user-acceptance 的部署限制。
- P00-01 已有独立审计和集成报告；`56f10516fa53e5b963a72e56c4d3c26bae777a7d`（集成后的实现）与 `4dc245f35ebfa26035dda4307bc78176a2f5d7f7`（集成后的审计）经 `git merge-base --is-ancestor <SHA> HEAD` 均返回 0。基线 registry 的 P00-01 仍为 todo，与上述证据登记不同步；状态由总调度核对，本报告不自行改为 done。

## 本轮实际只读发现

命令均在上述工作树的本地 zsh 会话执行。没有读取私钥、token、env、Docker Env、认证头或完整 Nginx 配置。

| 实际操作 | 结果 | 能证明的范围 |
| --- | --- | --- |
| `command -v ssh`；`ssh -V` | `/usr/bin/ssh`；`OpenSSH_10.0p2 Debian-7+deb13u4, OpenSSL 3.5.7 9 Jun 2026` | 本地有 SSH 客户端，不代表 NAS 可达或认证可用 |
| `test -f /home/lijx/.ssh/config` | 不存在 | 历史 P1 使用的显式配置路径当前不可用 |
| `wc -c /mnt/c/Users/lijx/.ssh/config` | `0 /mnt/c/Users/lijx/.ssh/config` | Windows 侧现有配置为空，没有可选 NAS/OpenWrt Host 条目 |
| `test -f /home/lijx/.ssh/known_hosts` 与 Windows 对应路径的文件存在性检查 | 两侧文件均存在 | 不能仅凭信任记录推断本轮允许连接的账号、端口或目标；未解析/输出其内容 |
| `command -v fnos`；`command -v fnos-cli` | 均未找到 | 本地当前 PATH 没有这两个 CLI 入口，不推断所有厂商工具均不存在 |
| 对会话 `ALL_TOOLS` 的名称/说明筛选 `/fnos\|ssh\|nas\b/i` | `[]` | 本会话没有通过该筛选发现现成连接工具 |
| `rg --files /mnt/c/Users/lijx/.codex/skills /mnt/c/Users/lijx/.codex/plugins/cache` 后筛选 `fnos\|nas.*SKILL\|ssh.*SKILL` | 0 个匹配 | 未找到可应用的 fnOS/SSH/NAS 技能；不臆造厂商 API |

历史 P1 命令为 `ssh -F /home/lijx/.ssh/config fnos ...`（见 P01-06-checks）。当前明确配置缺失，因此本轮没有尝试该别名，也没有从 known_hosts 猜测连接目标、接受新主机指纹、登录新账号或注册授权。未运行远程命令。上述结果只描述本会话，不否定拥有者在其他环境中保留连接配置。

## 环境清单及验收状态

| 登记项 | 当前证据 / 状态 | 后续最小核验 |
| --- | --- | --- |
| fnOS CPU 架构、系统版本、容器平台架构 | 未知 / BLOCKED | 在既有可信会话只读执行 `uname -m`、限定系统版本字段及平台架构查询 |
| Hub 数据库宿主本地卷及文件系统 | 路径、文件系统未知 / BLOCKED | 对指定候选路径只读 `findmnt -T <path>` / `stat`；排除 SMB/NFS、rclone 等远程或用户态共享盘；确认 `/data/db/` 整目录持久化条件 |
| 附件/暂存卷、权限与空间 | 未知 / BLOCKED | 核对两个目录位于同一文件系统、非 root UID/GID 可用、可用空间；本轮不创建目录、不做写入探针 |
| Nginx 所在主机及版本 | 设计要求 OpenWrt；实际位置/版本未验证 / BLOCKED | 现成可信 OpenWrt 会话中 `nginx -v`；若实际位置不同，先交调度核对，不能擅改既定拓扑 |
| 外部 HTTPS Origin、域名和自定义端口 | 未提供可验证地址 / BLOCKED | 由拥有者登记测试 Origin（含 scheme、host、port）；不得从现网 MP 地址推导 Hub 地址 |
| Nginx 内网 upstream、Hub 端口及防火墙/NAT | 未知 / BLOCKED | 对限定测试 vhost 的监听/upstream 字段及网络边界只读核查；不输出完整配置或凭据 |
| TLS 证书覆盖、有效期与客户端信任 | NOT_RUN | 对确定测试 Origin 做证书链、主机名和端口实测；仅看 listen/证书路径不算 TLS 通过 |
| 独立测试实例、网络、目录、凭据、端口隔离 | 未确认 / BLOCKED | 拥有者明确已有测试资源，证明不复用现网 DB/附件/MP 状态和端口，不创建真实服务取证 |
| SQLite WAL/SHM 及持久化实际读写 | NOT_RUN | P12 在获准的独立环境测试；只读文件系统识别不能替代目录权限、锁和容器替换实测 |
| HTTPS / WS / 上传 / Long Polling 真实联调 | 全部 NOT_RUN | P11/P12 使用正式 Hub、受控测试配置及真实代理完成；不重做 P1/P2 PoC |

卡片完成标准逐项结论：

- **无公网明文端口：BLOCKED**。未建立服务或开放端口，但这不足以证明目标拓扑无公网明文暴露；尚缺测试监听、NAT/防火墙及外部可达性证据。
- **本地卷适用 SQLite：BLOCKED**。未知实际目标卷，未运行 WAL/SHM/权限/锁测试；历史 MP `/config` 路径不能直接指定为 Hub DB 卷。
- **不修改现网：PASS（本轮操作范围）**。本轮仅本地文档/配置存在性检查；零 SSH 连接、零远程命令、零部署、零重启、零挂载/反代变更。此项不表示其他运行验收通过。
- **执行受影响检查：PASS（报告校验）**；外部验收按上表保留 BLOCKED / NOT_RUN。
- **单卡提交及独立审计：本报告单独本地提交，精确 SHA 提交后交接；独立审计和集成未执行，不能标总卡 done。**

## 历史 P1 证据及现网保护约束

以下均来自仓库中 2026-10-21 的 P01-06 报告，**不是本轮远程核验结果**：

- P01-06-callback-live 记录 MP v3.1.5、`moviepilot-v2` 在修复后 healthy，使用本地恢复快照 `relaynest-local/moviepilot-recovery:20261021-callback`；Compose 仍声明 `jxxghp/moviepilot-v3:3.1.5`，两者镜像身份不同。
- 历史 Compose 路径为 `/vol1/1000/docker/media-stack/compose.yaml`。`/vol02/1000-1-12ecd637/media:/netdisk` 绑定因源目录缺失导致重启失败，用户随后授权移除。历史恢复后保留 `/config`、`/moviepilot/.cloakbrowser`、`/media`。
- 这些路径既不证明 fnOS CPU 架构，也不证明可用的 Hub 独立本地卷。不得按旧镜像信息重建/升级/恢复 `/netdisk`，不得把恢复快照当成正式已发布镜像。
- P1 是用户接受剩余缺口后的阶段放行；正式 Hub/Android 按钮端到端、持久恢复、TLS/WS/上传/Long Polling 不因此自动通过。本卡未重做 P1/P2，也未访问 RelayShelf 后端或数据库。

## 解除阻塞的最小输入及可继续分支

1. 一个**已配置且已信任**的只读 fnOS 连接入口（例如本会话可读的 SSH 配置文件路径及 Host 别名，能用 BatchMode 和严格主机校验）；若反代在独立 OpenWrt，再提供其现成只读入口。不要将密码、私钥、token 复制到报告。没有现成入口时，由拥有者通过安全方式配置，不能由本卡注册授权或接受新指纹。
2. 已有独立测试资源的非秘密清单：数据库/附件/暂存宿主目录、UID/GID、容器/网络标识、可用内网端口、测试 vhost 和 HTTPS Origin（含端口），以及与现网隔离的说明。没有现成资源时先登记“尚未创建”；创建或真实部署作为另行确认的最后一步。
3. 明确只读可核验的测试 vhost/网络规则范围，以便提取必要字段而非打印完整配置。后续测试凭据只需安全加载方式，不提交值。

当前可继续：本报告独立审计、由调度核对 P00-01 状态登记，以及其他不依赖真实 NAS 的已授权本地工作。真实环境核验依赖输入 1/2；P11/P12 运行验收依赖环境、正式组件和对应部署授权。未提出架构/需求变更，不以普通实现决策替代外部门槛。

## 报告验证与交接

- 已检查本报告只引用实跑本地命令；所有外部运行项均明确未运行。
- 提交前执行 `git diff --check` 和限定路径/状态检查；报告未含原始凭据、私钥内容、token、认证头或完整配置。
- 文档研究无源码变更，不运行业务测试；远程/TLS/WS/上传/Long Polling 测试受上述条件阻塞。
- `task_id: P00-05`；`outcome: blocked`（报告已交付，环境验收未完成）；基线如前；变更路径仅本报告。审计应由不同 agent 针对提交后的精确 SHA 核对证据真实性和范围，再交调度处理状态与后续门槛。
