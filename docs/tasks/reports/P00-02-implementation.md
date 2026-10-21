# P00-02 Git 与工作区交付条件核对

核对日期：2026-10-21。角色：WORKSPACE-01 / P00-02 实施者。仅核对工作区并记录证据，任务是否完成由独立审计与集成决定。

## 基线与隔离工作区

用户指定基线为 `b058474ce4e7e0b4321fe8dec174c23601ce30a7`，`git cat-file -t` 返回 `commit`。主目录 `/home/lijx/Projects/RelayNest` 当前为 `main`，HEAD 为该基线；不是设计文档旧记录中的占位仓库，无需重新初始化。架构 §9 和开发计划 P0 中“非 Git 仓库”属于历史状态，本报告记录本次实际条件，不修改已确认设计。

创建前 `git worktree list --porcelain` 仅列主目录，三个目标路径和目标分支均不存在。桌面 `list_artifacts` 返回空列表；`create_worktree` 无指定路径和分支参数，因此按用户授权使用 `git worktree add -b <分支> <路径> <精确基线>`。默认沙箱首次阻止 Git 元数据写入；升级权限后创建成功，未删除或覆盖旧工作。

| 工作区 | 分支 | 创建后 SHA | 创建后状态 |
| --- | --- | --- | --- |
| `/tmp/relaynest-worktrees/foundation` | `codex/foundation-20261021` | `b058474ce4e7e0b4321fe8dec174c23601ce30a7` | 干净；随后仅新增本报告 |
| `/tmp/relaynest-worktrees/push-plan` | `codex/push-plan-20261021` | `b058474ce4e7e0b4321fe8dec174c23601ce30a7` | 干净 |
| `/tmp/relaynest-worktrees/audit` | `codex/audit-20261021` | `b058474ce4e7e0b4321fe8dec174c23601ce30a7` | 干净 |

`git config user.name` 与 `git config user.email` 均可读且非空，使用现有本地身份；未修改身份配置。精确实现提交 SHA 在提交后交接，避免在报告内预写自身 SHA。

## 已有内容与旧锁保护

开始前主目录 `git status --porcelain=v1` 为 ` M docs/tasks/registry.json`、`?? .directory`；本实施者没有编辑、stage 或提交二者。台账由总调度并发维护，其文件哈希可能变化，不据此宣称内容全程相同。

工作树创建前后主目录 `.directory` SHA-256 均为 `d59437769bb9d12d6e13d8cd0e0a1ea6601f3bc32b2e7dc9906fb22929eea2ef`，`.git/index` 均为 `5471eb900f3874954bb3a1223174e27ebb1c9ccbd4be2f77a9ca57cfee991001`；主目录分支和 HEAD 未变化。

旧台账工作区锁仍记 `P00-02` / `git_preflight` / `/root/git_preflight`，旧基线为 `f1d4e56e80ccec3daf5e15a8cab145bdbefc3085`。本次 `collaboration.list_agents` 仅列总调度、push_planner、recovery、workspace，未列该旧 agent；可见进程只有当前命令执行链，无旧 Git 写入进程；`.git` 下未发现 `*.lock`。结合实际唯一旧工作树已在新基线，旧 owner/基线与当前不符，可以由总调度解除旧调度锁并登记新归属。本实施者不删除锁、不改台账。进程可见性受当前执行环境限制，不据此断言主机所有进程均不存在。

## 检查及结论

以下检查均实际运行，除首次受沙箱阻止的创建命令外均成功：

- `git status --porcelain=v1`、`git branch --show-current`、`git rev-parse HEAD`：主目录及三个隔离树可读，分支和 SHA 如上。
- `git worktree list --porcelain`、`git for-each-ref` 与目标路径检查：创建前无目标冲突，创建后隔离树存在。
- `git cat-file -t <基线>`、`git log -3`：真实已有提交历史与可解析基线。
- `git config user.name`、`git config user.email`：现有提交身份可用。
- Git 锁文件扫描、可见进程及 agent 清单：未发现旧写入者或 Git 锁；不覆盖旧工作。
- 主目录文件/index 哈希及状态复核：`.directory` 与 index 未变化，原有修改仍保留。

`git remote` 当前无配置；`git check-ignore --no-index .directory node_modules/ build/ .env` 未匹配现有规则（该命令按 Git 约定返回 1）。本次只交付本地文档提交，远程和工程忽略规则需在后续对应任务处理，不影响本卡本地提交条件。

G-GIT 的真实 checkout、固定已提交基线、隔离写入与本地提交条件已具备，可供后续使用。正式门槛状态仍由总调度在独立审计与集成后登记。本次没有业务代码变更，无需运行业务构建或真机测试；没有推送、发布、部署、修改 RelayShelf、创建自动化或额外 agent。
