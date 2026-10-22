# P03-09 集成 I01

日期：2026-10-22；角色：integrator；结论：本地精确集成与组合检查 PASS，远端 GitHub 门槛仍 NOT_RUN。目标分支 `main`，工作树 `/home/lijx/Projects/RelayNest`，没有修改工作流或其他 CI。

## 精确提交映射

| 项目 | 原 SHA | main SHA |
| --- | --- | --- |
| 原实现基线 | `6d4ac84dd8d1b083b927505a9a97c975aa3e9d44` | — |
| 集成前 main | — | `8536dfddfbff2bc6fb3e126b66f380ba7ef88a7b` |
| 实现 | `ae32f8753a5f4cff109f1fb28b739857b3655e62` | `29b6516946e67d1ff6b8de7ce86c0bd00242cad4` |
| A01 报告 | `a6865b889eae055dfc8d2346cc17d99c9e0b89c0` | `a149dec078fefb47fe72263872e968df53a9933a` |
| A01 证据措辞澄清 | `282152c29994b67aa31e3ec046e0df0768d1a8f7` | `9133cf6409f3708e88307c2e825a5766d82acc9a` |

来源受管 GoCI 工作树开始时干净。三次 cherry-pick 均无冲突；workflow 和实施报告与原实现逐文件 diff 为空，最终审计报告与原澄清提交逐文件 diff 为空。workflow blob 为 `1e6a1178e78efb9038aeacbbb51ba111ceca77a5`。除此三文件，集成前后其他模块无内容差异，没有代码漂移或重新审计需求。独立审计：[P03-09-audit-01.md](P03-09-audit-01.md)，PASS 仅针对固定本地配置，不代表远端验收已完成。本报告独立提交，自身 SHA 提交后交接。

## 当前 main 亲自执行

环境：Linux amd64；现有 Node v22.23.2/npm 10.9.8/Go go1.27.3；`GOTOOLCHAIN=local`、`GOFLAGS=-mod=readonly`；actionlint 1.7.7。临时 Python 驱动从当前 YAML 提取 run，使用 `bash --noprofile --norc -eo pipefail` 按顺序执行并记录时间，复用现有 Python 哈希锁环境，不升级工具。脚本及日志分别为 `/tmp/relaynest-run3/p0309-integration-check.py`、`p0309-integration-check.log`，均不入库。

开始前将原有 dist 移至 `/tmp/p0309-prior-dist-_en7jaby/dist`，显式断言源码目录无 dist，再执行 npm 安装/构建。未用残留 dist 代替本次构建；新生成物仍被忽略，不提交。

| 检查 | 实际结果 |
| --- | --- |
| `actionlint -oneline .github/workflows/server-check.yml`；所有 run 的 Bash 语法 | PASS，退出0；没有执行 shellcheck |
| workflow 中 Node/npm/Go 精确版本断言 | PASS，均与固定值相等 |
| `npm --prefix web ci --no-audit --no-fund` → `npm --prefix web run build` | PASS，安装16包；tsc与Vite 7.3.7构建成功，本次生成 HTML/CSS/JS |
| server 下 `go mod download`、`go mod verify`、原格式 run 块、`go vet ./...` | PASS，依赖全部 verified、格式及vet退出0 |
| server 下 `CGO_ENABLED=1 go test -race -count=1 ./...` | PASS，cmd/health/httpapi/storage/webui五包实际重跑，migrations无测试文件，耗时2.18秒 |
| server 下原 CGO_ENABLED=0 构建块 | PASS，4.10秒；原块的 `GOCACHE=$(mktemp -d)` 创建 `/tmp/tmp.aY7b77Wg9A` 后，本地观察断言 `test -z "$(ls -A "$GOCACHE")"` 实际通过，再 `go build -trimpath -o "$RUNNER_TEMP/relaynest" ./cmd/relaynest`；原trap清理该目录。未修改workflow；模块下载缓存沿用环境，不声称模块缓存为空 |
| `go version -m /tmp/p0309-integration-runner-j01yxogn/relaynest` | PASS，go1.27.3、CGO_ENABLED=0、trimpath=true、linux/amd64；revision=9133cf6409f3708e88307c2e825a5766d82acc9a、modified=true（保留用户/调度未提交文件），不声称干净源码二进制 |
| 原格式 run 块在自动回收临时目录的正/负例 | PASS，已格式化源码退出0；未格式化源码退出1、输出unformatted.go，后续标记文件未生成；没有改仓库源码或弱化检查 |
| 原锁文件diff步骤、范围/blob检查、`git diff --check` | PASS；go.mod/go.sum、Web清单/锁均未改写，其他CI/业务模块不变 |

首次临时驱动用 rename 将旧dist移到/tmp时因跨设备返回EXDEV，检查尚未启动；随后仅将临时驱动改为跨设备可用的move，完整上述流程退出0。没有修改产品或工作流，也没有把失败尝试写作通过。GOCACHEPROG 实测为空；缓存初态和耗时是本次实际观察，未把独立审计对开发证据的核实说成其亲自冷缓存重跑。

## 限制与保护

路径过滤涵盖 server/**、web/**、本工作流；PR只读权限、无发布秘密、失败硬退出等配置与已审blob完全一致。NOT_RUN：远端GitHub Actions、真实push/PR路径触发、fork权限、checkout/setup下载执行、远端失败展示和分支保护。没有推送/发布/部署，不依据本地结果关闭远端门槛。

registry保留原用户及授权调度变化，不stage、不覆盖；`.directory`和已提交checkpoint摘要与开始前一致且不修改。只有本卡实现/审计与集成报告提交；不派额外agent。本报告提交后按最终main精确ref，依次用create_worktree准备p2-evidence、moviepilot-ci持久工作树，实际路径/分支在交接给出，只准备不实现或审计，不覆盖旧引用。
