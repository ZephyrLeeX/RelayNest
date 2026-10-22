# P03-10 集成 I01

日期：2026-10-22；角色：integrator；结论：本地精确集成及组合检查 PASS。目标 `main`，工作树 `/home/lijx/Projects/RelayNest`；远端 GitHub 行为仍 NOT_RUN。

## 精确提交

| 项目 | 原 SHA | main SHA |
| --- | --- | --- |
| 原实现基线 | `8536dfddfbff2bc6fb3e126b66f380ba7ef88a7b` | — |
| 本卡集成前 main | — | `95f50154daa92597574c6ce63d163ab0b07cd194` |
| 实现 | `500c6e2259ffb0a99163114870b2bc0484b91547` | `e9ba0dcd59cd159b0e02e77a9f5b28be0a068b9c` |
| 独立审计 PASS | `0ece340fce0dcdad19a13eccf642fd6de2401e7c` | `fae5c03c35aa6552ef531de0eb6b4d2579df431a` |

来源受管web-ci工作树开始时干净，两次cherry-pick无冲突。workflow及实施报告与原实现逐文件diff为空，审计报告与固定审计提交同blob；workflow blob=`f8788ba86d0a1bc1e7bdd6af55786e1acd0b4f76`。集成差异仅上述三文件，本报告另行独立提交；其他CI、Web源文件/版本/锁及其他模块无内容变化，不产生代码漂移或重审需求。审计证据：[P03-10-audit-01.md](P03-10-audit-01.md)。本报告提交SHA在交接给出。

## 当前 main 真实检查

环境：Linux amd64，现有Node v22.23.2、npm 10.9.8、actionlint 1.7.7，未安装/升级工具。临时Python驱动读取当前YAML的全部五组run，用 `bash --noprofile --norc -eo pipefail` 顺序执行，首个非零即停止。驱动及日志 `/tmp/relaynest-run3/p0310-integration-check.py`、`p0310-integration-check.log` 不入库。

| 检查 | 实际结果 |
| --- | --- |
| `actionlint -oneline .github/workflows/web-check.yml`、五组run逐段Bash语法 | PASS，退出0；没有执行shellcheck |
| 先将main原dist移出源码树并断言不存在，执行当前workflow版本断言 → npm ci → typecheck → build/非空index断言 → 清单/锁diff | PASS，五组退出码 `[0,0,0,0,0]`，本次重建Go嵌入资源，不依赖其他工作流或旧dist |
| 加stale.txt再完整执行五组真实命令 | PASS，退出码仍全0，stale.txt消失；HTML/CSS/JS三产物名称/字节摘要相同，清单/锁未改 |
| 当前HEAD通过git archive导出隔离副本，初始无node_modules/dist；加入string=42类型错误 | 预期失败PASS：`[0,0,2]`、真实TS2322，没有执行build |
| 隔离副本恢复TS后加入不存在CSS import | 预期失败PASS：`[0,0,0,1]`，类型检查通过而真实Vite报Could not resolve，构建失败阻止输出断言 |
| 隔离副本恢复源码，仅package.json TypeScript改5.9.2、保留5.9.3锁 | 预期失败PASS：`[0,1]`，真实npm EUSAGE，未执行typecheck/build |
| Git精确范围、blob、锁diff及 `git diff --check` | PASS，负例只在自动回收隔离副本；main tracked源码和锁不变 |

三产物SHA-256：index.html=`134555eec33cfa5d98e63e065bdb46032ba69540873da83a68de1a094e2bd7de`，CSS=`f821e8a944558d9713eff25f5a792221e47478398b70af7d52d17683309bb1cf`，JS=`6d52fc8dd4a8b45582b191ec3b8e11c0bed55a5d9c8b47b2d6116a477c7d5dfe`。结果与固定审计记录一致。本轮npm缓存复用已有环境，不声称下载缓存为空；main原dist备份位于 `/tmp/p0310-prior-dist-ihsujcfa/dist`，本次新产物仍忽略、不提交。

## 限制及保护

NOT_RUN：真实GitHub Actions、托管runner及Node下载、push/PR路径事件、fork权限隔离、远端失败界面、并发取消和超时。静态配置/本地命令通过不能替代这些远端证据。未重跑Go/Android、浏览器、漏洞扫描、设备、部署或发布；其他模块与CI无变化。

本次没有改工作流、锁版本或其他CI，不执行发布步骤。registry保留用户及授权调度差异、不stage、不覆盖；`.directory`及已提交checkpoint摘要未变。未派额外agent，未推送/部署/发布；本卡只提交指定实现、审计和独立集成报告。
