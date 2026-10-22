# P03-10 Web 检查 CI 实施报告

日期：2026-10-22；角色：developer；结果：implemented，等待独立审计和集成。

- 基线：`8536dfddfbff2bc6fb3e126b66f380ba7ef88a7b`。
- 工作树：`/home/lijx/.codex/worktrees/web-ci/RelayNest`；分支：`codex/web-ci-run3`；开始时干净。
- 仅新增 `.github/workflows/web-check.yml` 和本报告；没有修改依赖锁、工具版本、业务、其他 CI 或台账，没有新增 helper。最终提交 SHA 在提交后交接，避免报告预写自身 SHA。
- 已读根约定、CONTRIBUTING、WORKER_PROTOCOL、P03-10、ARCHITECTURE §1、DEVELOPMENT_PLAN P3、Web README/check.sh/mise/package/锁文件局部、Vite/TS 配置及 P03-02 审计 CI-01。

## 实现与验收对应

| 条目 | 实现和证据 |
| --- | --- |
| 锁文件安装、类型及构建 | 同一 `ubuntu-24.04` job 校验 Node `22.23.2`/npm `10.9.8`，依次执行 `npm --prefix web ci --no-audit --no-fund`、`npm --prefix web run typecheck`、`npm --prefix web run build`；最后断言 `server/internal/webui/dist/index.html` 非空、清单/锁未改写。沿用 build 内已有的类型检查，不改变应用脚本。 |
| 干净构建可复现 | 本地无 node_modules/dist/npm 缓存的 archive 实测 PASS，第二次安装/构建三文件内容及名称完全一致，见下表；插入的旧产物标记被 Vite 清除。工作流不依赖 Go workflow 或共享工作目录。 |
| 失败传播 | 无 continue-on-error、忽略退出码或发布后备分支；bash 默认失败即退出。真实 TS 错误退出 2、Vite 解析错误退出 1、锁不一致退出 1；本地按步骤运行时均停止后续命令。远端 GitHub 失败 UI 未实跑。 |
| 路径触发 | push/pull_request 均覆盖 `web/**`（含 npm 锁、Node mise、npmrc、Vite/TS 配置）、`.github/workflows/web*`、根 `mise.toml` 和 `.gitignore`；静态核对 PASS，远端事件触发 NOT_RUN。 |
| 不发布、只读权限 | `contents: read`，checkout `persist-credentials: false`；无 secrets 引用、发布、部署、上传、写权限、pull_request_target 或 workflow_run。Actions 使用默认只读 GitHub token 获取源码/工具元数据，无发布凭据配置。静态核对 PASS；真实 fork PR NOT_RUN。 |
| 缓存与旧产物 | 显式 `package-manager-cache: false`，不启用 Actions 依赖/产物缓存；每次 ci 重装，Vite 现有 emptyOutDir 清理专有 dist。没有缓存 key 回退或复用旧 dist。 |

设置 10 分钟超时及同 workflow/ref 并发取消；没有引入 Go 检查职责，也没有调用包含 Go 检查的 `web/check.sh`。

## 官方 Actions 核验

与已审计 Go CI 选择一致，真实运行下列公开只读查询，返回完整 SHA；另以 curl 获取这些 SHA 的官方 `action.yml` 与 README 核对输入、runtime、缓存开关：

```sh
git ls-remote https://github.com/actions/checkout.git refs/tags/v6.1.0
git ls-remote https://github.com/actions/setup-node.git refs/tags/v6.5.0
```

| Action | 固定提交 | 官方来源 |
| --- | --- | --- |
| checkout v6.1.0 | `d23441a48e516b6c34aea4fa41551a30e30af803` | [action.yml](https://github.com/actions/checkout/blob/d23441a48e516b6c34aea4fa41551a30e30af803/action.yml)、[README](https://github.com/actions/checkout/blob/d23441a48e516b6c34aea4fa41551a30e30af803/README.md) |
| setup-node v6.5.0 | `249970729cb0ef3589644e2896645e5dc5ba9c38` | [action.yml](https://github.com/actions/setup-node/blob/249970729cb0ef3589644e2896645e5dc5ba9c38/action.yml)、[README](https://github.com/actions/setup-node/blob/249970729cb0ef3589644e2896645e5dc5ba9c38/README.md) |

两者 Action runtime 为 Node 24，官方 README 要求 runner 至少 2.327.1；checkout 对 Docker 内认证命令另要求 2.329.0，本工作流没有该用途。使用 GitHub 托管 ubuntu-24.04；项目编译仍固定 Node 22.23.2。没有声称本地已验证 GitHub 托管 runner 版本或工具下载；这些需要远端运行确认。最初查过 v4 标签，最终文件使用上述 v6 SHA；浏览器访问 v6 tree 页受限后，用官方 raw 内容实际核验成功。

## 实际检查

环境：Linux amd64 / WSL2 `6.18.33.2-microsoft-standard-WSL2`，已有 `/home/lijx/.local/share/mise/installs/node/22.23.2/bin`；实测 `node --version` 为 v22.23.2、`npm --version` 为 10.9.8。复用 `/tmp/relaynest-ci-tools/actionlint` v1.7.7（go1.26.0 编译）。未安装或升级工程工具、未读取凭据或连接远端仓库运行。

临时驱动 `/tmp/relaynest-run3/web-ci-check/verify.mjs` 从本工作树的 workflow 提取实际 run 命令（Git diff 另在真实工作树执行），用 `bash --noprofile --norc -e -o pipefail -c` 逐步执行。它通过 `git archive HEAD` 将精确基线导出至全新 `/tmp/relaynest-run3/web-ci-check/pristine-MRih24`，确认不存在 node_modules/dist/npm-cache；使用空 npm 用户/全局配置和全新 npm 缓存。该驱动只用于本地证据，不是 Actions 解释器；不把本地调度等同 GitHub 实际步骤调度。

| 真实命令或检查 | 结果 |
| --- | --- |
| `/tmp/relaynest-ci-tools/actionlint -oneline .github/workflows/web-check.yml` | PASS，最终 v6 配置退出 0，覆盖 YAML、workflow 结构、表达式和输入；无 shellcheck，未声称运行 shellcheck。 |
| `node /tmp/relaynest-run3/web-ci-check/verify.mjs`（使用上述固定 Node） | PASS，驱动退出 0；日志 `/tmp/relaynest-run3/web-ci-check/verify.log`。四条 workflow run 步骤的成功/失败均为真实进程结果。 |
| 工具链两个 test、`npm --prefix web ci --no-audit --no-fund` | PASS，首次公网锁安装 16 包，退出 0；未更改清单或锁，未运行 audit。 |
| `npm --prefix web run typecheck`、`npm --prefix web run build`、`test -s server/internal/webui/dist/index.html` | PASS，均退出 0；TypeScript 5.9.3/Vite 7.3.7，构建四模块、输出三文件至精确 embed 目录。 |
| 第二次 ci/typecheck/build，预先加 dist/stale.txt，再比较文件清单及 SHA-256 | PASS，旧标记消失、三个资源完全相同、锁内容未变。 |
| 临时副本加 `const ciFailure: string = 42;` 后按同顺序执行 | 预期失败 PASS；tsc 报 TS2322、退出 2；驱动停止，未运行 build。 |
| 恢复类型负例，临时 main.ts 加 `import './ci-missing.css';` 后按同顺序执行 | 预期失败 PASS；typecheck 退出 0，Vite 报 Could not resolve，build 退出 1；bash 未执行紧随其后的 dist 断言，即使旧 dist 存在也不能掩盖失败。 |
| 恢复源码，仅临时 package.json TypeScript 改 5.9.2、保留 5.9.3 锁后按同顺序执行 | 预期失败 PASS；npm ci 报 EUSAGE / lock 不满足声明、退出 1；驱动停止，未运行 typecheck/build。未提交或保留此工程改动。 |
| `git diff --exit-code -- web/package.json web/package-lock.json`、`git diff --check` | PASS，退出 0；仅本卡两个新增文件。 |

透明记录：最初探测 Python `import yaml` 失败（没有 PyYAML），最终使用 actionlint 完成 YAML 校验；临时驱动首次误将 run 步骤数量断言为 5（实际排除 Git diff 后为 4），在执行构建前退出 1，修正临时断言后上述完整检查通过。没有修改产品代码、弱化类型或构建检查。

| 重复构建产物 | SHA-256 |
| --- | --- |
| index.html | `134555eec33cfa5d98e63e065bdb46032ba69540873da83a68de1a094e2bd7de` |
| assets/index-BC4YGnT-.css | `f821e8a944558d9713eff25f5a792221e47478398b70af7d52d17683309bb1cf` |
| assets/index-o9XC6usj.js | `6d52fc8dd4a8b45582b191ec3b8e11c0bed55a5d9c8b47b2d6116a477c7d5dfe` |

## 未执行边界与交接

- NOT_RUN：GitHub Actions 真正执行、托管 runner 下载 Node、push/PR 路径过滤、fork PR 权限隔离、并发取消、超时及失败界面；本地静态/命令验证不冒充远端证据。
- NOT_RUN：Go 组合构建、浏览器、部署、发布、依赖漏洞扫描；Go 的 CI 职责由 P03-09 承接。
- 不存在待用户决策或已知实现阻断；上述本地验收 PASS 仅表示可交付独立审计，不标任务 done，不豁免后续真实 CI 验证。
- 无推送、发布、部署、凭据读取或额外 agent。临时负例、缓存、日志和构建产物均不入库。
