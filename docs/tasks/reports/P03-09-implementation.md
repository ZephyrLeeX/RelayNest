# P03-09 Go 检查 CI 实施报告

日期：2026-10-22；角色：developer；结果：implemented（本地配置与检查完成，等待独立审计/集成；GitHub 实际运行门槛 NOT_RUN）。

## 基线与范围

- 基线：`6d4ac84dd8d1b083b927505a9a97c975aa3e9d44`。
- 工作树：`/home/lijx/.codex/worktrees/go-ci/RelayNest`，分支 `codex/go-ci-run3`；开始时干净。
- 仅新增 `.github/workflows/server-check.yml` 和本报告，均属于卡内允许路径。没有修改业务、依赖/工具版本、迁移、Schema、其他 CI、任务卡或 registry，没有新增 helper。
- 已读根约定、CONTRIBUTING、WORKER_PROTOCOL、任务卡、ARCHITECTURE §1、DEVELOPMENT_PLAN P3 和 Web README/check.sh、P03-02 审计 CI-01、server/go.mod 与局部工具锁。
- 调度确认 P03-01/P03-08 已审计集成，且已把 P03-02 登记为实际前置。卡原始元数据仅列 P03-01/P03-08；P03-02 审计 [CI-01](P03-02-audit-01.md) 和 `web/README.md`/`web/check.sh` 明确 Go embed 编译输入为 Web 生成的 `server/internal/webui/dist/`，因此本工作流包含 Web 前置并监控 `web/**`。没有自行修改依赖台账。

## 实现与验收对应

| 验收 | 实现与结论 |
| --- | --- |
| 按路径触发 | push/pull_request 均包含 `server/**`、`web/**`、`.github/workflows/server*`，覆盖 go.mod/go.sum、package.json/package-lock.json、Web 工具锁及工作流自身；另提供 workflow_dispatch。静态检查 PASS，GitHub 实际事件触发 NOT_RUN。 |
| Web 必须在 Go 前重建 | 单 job/同工作空间先断言 Node 22.23.2/npm 10.9.8，然后顺序执行 `npm --prefix web ci --no-audit --no-fund` 和 `npm --prefix web run build`；不存在下载或恢复 dist 的步骤。 |
| 固定 Go 和检查 | Go 1.27.3，GOTOOLCHAIN=local、GOFLAGS=-mod=readonly；核对实际 Go/go.mod 版本，下载/校验模块；格式、vet、race -count=1；最后 CGO_ENABLED=0 从新建空 GOCACHE 编译。 |
| 失败阻止成功结论 | 无 continue-on-error、条件绕过、忽略退出码或汇总伪成功；bash 默认失败传播。格式失败输出文件名并退出 1。版本断言、安装、构建、测试和最后锁文件 diff 均是硬失败。静态检查与格式负例 PASS，托管 job 失败展示 NOT_RUN。 |
| PR 不读取发布秘密 | 仅 pull_request（没有 pull_request_target/workflow_run）；contents: read，其余权限未授予；checkout persist-credentials=false；无 secrets 引用、发布/上传/部署步骤。只使用官方 Actions 默认 GitHub 只读 token 获取源码/工具元数据，不配置发布凭据。静态检查 PASS，真实 fork PR NOT_RUN。 |
| 缓存版本隔离 | 本最小工作流关闭 setup-node 自动包管理器缓存及 setup-go 缓存，没有 actions/cache，也不缓存 node_modules/dist。最终构建额外使用独立空 Go 构建缓存，避免旧产物；无需维护跨版本 cache key。 |

运行环境选 `ubuntu-24.04`，job 超时 20 分钟；普通测试阶段 CGO_ENABLED=1 使用 runner 的 C 编译器进行 Go race 检查，最终构建 CGO_ENABLED=0。构建产物只落 RUNNER_TEMP，不发布、不入库。最后断言 Go/Web 清单与锁文件没有被改写。

## 官方 Actions 来源核验

以下标签均通过真实 `git ls-remote https://github.com/actions/<name>.git 'refs/tags/v6*'` 核验为完整 SHA，并从对应 SHA 的官方 `action.yml` 核对输入、Node 24 Actions runtime 与缓存开关。没有使用浮动标签作为 uses 引用；Actions runtime 与项目 Node 22 工具链分别固定。

| Action | 标签与完整提交 | 官方核验来源 |
| --- | --- | --- |
| checkout | v6.1.0 / `d23441a48e516b6c34aea4fa41551a30e30af803` | [发布](https://github.com/actions/checkout/releases/tag/v6.1.0)、[action.yml](https://github.com/actions/checkout/blob/d23441a48e516b6c34aea4fa41551a30e30af803/action.yml) |
| setup-node | v6.5.0 / `249970729cb0ef3589644e2896645e5dc5ba9c38` | [发布](https://github.com/actions/setup-node/releases/tag/v6.5.0)、[action.yml](https://github.com/actions/setup-node/blob/249970729cb0ef3589644e2896645e5dc5ba9c38/action.yml) |
| setup-go | v6.5.0 / `924ae3a1cded613372ab5595356fb5720e22ba16` | [发布](https://github.com/actions/setup-go/releases/tag/v6.5.0)、[action.yml](https://github.com/actions/setup-go/blob/924ae3a1cded613372ab5595356fb5720e22ba16/action.yml) |

## 实际本地验证

环境：Linux amd64 / WSL2 `6.18.33.2-microsoft-standard-WSL2`，gcc 14.2.0；使用既有 `/home/lijx/.local/share/mise/installs/node/22.23.2/bin`、`/home/lijx/.local/share/mise/installs/go/1.27.3/bin`，实测 Node v22.23.2/npm 10.9.8/Go go1.27.3；未安装或升级工具。既有 actionlint 为 `/tmp/relaynest-ci-tools/actionlint` v1.7.7（go1.26.0 编译）。

本地验证脚本和日志仅在 `/tmp/relaynest-run3/go-ci-check/`，不入库。脚本以 `bash set -euo pipefail` 顺序执行工作流对应命令；首次断言 web/node_modules 和 dist 均不存在，Go GOCACHE/GOMODCACHE 使用该新建临时目录内的空目录。npm registry 与 Go 模块下载可用，未复用已有 Web 产物或 Go 模块缓存。

| 实际执行 | 结果 |
| --- | --- |
| `actionlint -oneline .github/workflows/server-check.yml`；`bash -n /tmp/relaynest-run3/go-ci-check/check.sh` | PASS，退出 0；actionlint 覆盖 YAML 解析、工作流结构、表达式/Action 输入检查。没有可用 shellcheck，不声称执行过 shellcheck。 |
| Node/npm 版本输出和等值断言；`npm --prefix web ci --no-audit --no-fund` | PASS，安装 16 包；清单/锁文件无变化。 |
| `npm --prefix web run build` | PASS，TypeScript 类型检查与 Vite 7.3.7 构建四模块；在精确 embed 目录生成 HTML/CSS/JS。 |
| Go 版本及 go.mod 等值断言；`go mod download`；`go mod verify`（server 目录） | PASS，`all modules verified`。 |
| `unformatted=$(gofmt -l .)`，非空则输出并退出 1（server 目录） | PASS，无格式文件输出。另在临时目录用未格式化 Go 文件运行相同格式块，实际退出 1、输出 unformatted.go，后续标记未执行；负例验证 PASS。 |
| `go vet ./...`（server 目录） | PASS。 |
| `CGO_ENABLED=1 go test -race -count=1 ./...`（server 目录） | PASS，cmd/relaynest、internal/health、httpapi、storage、webui 五包通过；migrations 无测试文件，没有缓存测试结论。 |
| 新 `GOCACHE=$(mktemp -d)`，`CGO_ENABLED=0 go build -trimpath -o "$RUNNER_TEMP/relaynest" ./cmd/relaynest`（server 目录） | PASS，空构建缓存真实编译成功，产物 10,110,596 字节；临时 GOCACHE 由 trap 清理。 |
| `git diff --exit-code -- server/go.mod server/go.sum web/package.json web/package-lock.json`；`git diff --check` | PASS。完整对应命令脚本退出 0，构建后 tracked 源码没有变化。 |

Go 全程设置 GOTOOLCHAIN=local、GOFLAGS=-mod=readonly；与托管工作流相同的命令顺序真实执行完成。额外空 GOMODCACHE 是本地验证加强，不意味着 CI 启用了依赖缓存。

`go version -m` 另确认产物为 go1.27.3、CGO_ENABLED=0、trimpath=true、linux/amd64。该产物在提交工作流前生成，因此 VCS revision 为基线，modified=true（存在本卡新增未跟踪文件）；“空缓存构建”不声称该本地二进制对应提交后的干净 checkout。

透明记录：探测本机 Python 的 `import yaml` 失败（没有 PyYAML），没有安装依赖来绕过；最终使用既有 actionlint 完成 YAML 与工作流校验，该工具真实退出 0。本地对应命令脚本、格式负例和二进制未提交。

## 限制与交接

- GitHub Actions 实际运行、checkout/setup Actions 在托管 runner 上的下载、fork PR 的权限行为、真实 push/PR 路径过滤与远端失败展示均为 **NOT_RUN**，本地成功不代表这些验收已经通过。尚未获得对应远端执行条件，没有远端 push 或 workflow dispatch。
- GitHub 分支保护/必需检查配置不在本卡工作流修改范围；本报告不声称已配置远端合并门禁。
- 未运行发布、容器、fnOS、部署或外部业务系统验证，没有读取发布秘密。未运行 npm 漏洞扫描，不声称依赖安全审计通过。
- 等待独立审计和调度集成；本卡没有自行标 done。最终提交 SHA 由提交后交接给出，不在报告预写自身 SHA。
