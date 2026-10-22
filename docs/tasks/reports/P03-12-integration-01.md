# P03-12 集成 I01

日期：2026-10-22；角色：integrator；结论：固定本地检查CI精确集成与组合验证 PASS，不代表真实MP或远端CI通过。目标 `main`，工作树 `/home/lijx/Projects/RelayNest`。

## 精确提交映射

| 项目 | 原 SHA | main SHA |
| --- | --- | --- |
| 原实现基线 | `4cf1304839d7dfdb55dd211f7f4deabb13846349` | — |
| 集成前main | — | `57ff401f95d9f747ec85938cbb521190463ae3c2` |
| 实现 | `e3fa7343369a7af6599146e134efbbdd5a4a393a` | `64c2a3566b089500581ed3fe8bc34239702d48dd` |
| 独立审计PASS | `ab6530f93c300ca870d6aeb8cb4bdd79c226b951` | `845aeb3821465a29d7fda4b6a77c4d91740c332f` |

来源受管moviepilot-ci树开始时干净。两次cherry-pick无冲突；workflow/实施报告及审计报告分别与精确来源逐文件diff为空。workflow blob=`db7fe03d8c938f2977b4af87889091f36133cb52`，没有代码漂移或重审需求。差异仅此三文件，本报告独立提交；插件、锁、MP核心、其他CI及模块内容不变。独立证据：[P03-12-audit-01.md](P03-12-audit-01.md)。本报告自身SHA提交后交接。

## 当前 main 亲自执行

环境：Linux amd64；既有Python 3.13.15、Node v24.21.0和actionlint 1.7.7。没有安装/升级依赖。临时驱动读取当前workflow三组run并用 `bash --noprofile --norc -eo pipefail` 原样执行，日志 `/tmp/relaynest-run3/p0312-integration-check.log`，脚本同目录 `.py`；均不提交。

| 检查 | 实际结果 |
| --- | --- |
| `actionlint -shellcheck= .github/workflows/moviepilot-check.yml` | PASS，退出0；ShellCheck未启用，明确NOT_RUN，不声称全量shell静态审查 |
| 三组run分别 `/bin/bash -n` | PASS，3/3 |
| workflow原Python heredoc，经 `python -I -B -` | PASS，精确版本3.13.15、mise工具声明断言、现有9个Python文件compile语法通过，不导入操作脚本、不写字节码 |
| `python -I -B integrations/moviepilot/poc/verify_probe.py` | PASS，仅本地宿主桩及127.0.0.1接收器，无真实实例参数 |
| `python -I -B integrations/moviepilot/poc/verify_patch.py` | PASS，11项临时夹具全部通过；PATH包含现有Node v24.21.0，可选UI JavaScript解析分支执行；未传source-root/emit-container-check |
| 从当前HEAD archive导出隔离副本，分别注入非法语法、probe入口异常、patch入口异常，再执行对应原run | 预期失败PASS，三个均exit1且错误可见；只改临时副本，未改仓库实现 |
| 原blob/范围、源码状态与 `git diff --check` | PASS，无额外实现变更，无生成物入库 |

## 限制与保护

本卡不连接MP实例、不读取源码秘密、不下载MP源码。静态/宿主桩/协议桩与合成夹具不等于AC-07、P10/P11、真实业务或市场发布通过。沿用用户已经接受的P1/G-MP例外；方案A仅固定v3.1.5指纹及助手显式授权应用，v3.0.0未实测，不能推广所有>=3.0.0兼容。P01-06原有NOT_RUN与部署限制保持。

NOT_RUN：ShellCheck、GitHub远端runner、真实fork PR与事件触发、远端工具下载、正式MP/Hub/Firebase/Android/业务/设备、发布部署。本地通过不豁免这些后续门槛。

registry用户及授权调度差异保留，不stage、不覆盖；`.directory`与已提交checkpoint未修改，摘要同开始前。未派额外agent、推送、发布或部署。本卡单独交接后再开始下一张历史证据集成，不混合提交。
