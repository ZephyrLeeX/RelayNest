# P03-05 集成 I01

日期：2026-10-22；角色：integrator；结论：PASS。目标 `main`，工作树 `/home/lijx/Projects/RelayNest`。本次只集成固定契约、F01 修复与 A02 独立复审，不实现运行时业务或后续接口。

## 提交映射及一致性

| 项目 | 原 SHA | main SHA |
| --- | --- | --- |
| 集成前基线 | — | `6d4ac84dd8d1b083b927505a9a97c975aa3e9d44` |
| 原实现 | `b1777a0290cac38b95f00e349bae676702cf7f17` | `95a24f6c87a14bc74e1cca81f7a357547ccf79dd` |
| F01 最终实现 | `5d170b84ef77cb6560d594341fa35bb0dab5bd7c` | `4ebf92410d27e552d8029459dfcc762748ca4533` |
| A02 PASS 报告 | `b7b02d023da5c25d0031d39bc833ec265a77b761` | `340e437583d1d3c96bc7f00dc28f85f7e1acf294` |

来源 `/tmp/relaynest-run3/api-fix` 开始时干净。三次 cherry-pick 均无冲突，无代码修改。原实施报告已由 `c0947f2b76c0f02664dc597bac3c663fb48c5485` 进入 main，本次同 blob 自动保持，原实现只产生四文件差异，没有改报告内容。原负审与修复规划已在 main，未重复集成。

最终 `server/api` Git tree 对象在被审实现和集成 HEAD 均为 `2a5abb6aa644f06ccaa38e6bf14864d48e37225f`；整个目录（包括依赖锁）完全相同。原实施报告、F01 实施报告、A02 报告分别逐路径 diff 为空。审计依据：[P03-05-audit-02.md](P03-05-audit-02.md)，关闭 `P03-05-A01-F01` 且无新增发现。没有代码漂移，无需重新代码审计。

## 当前组合实际验证

环境：Linux；复用已有哈希锁环境 `/tmp/relaynest-p0305-f01-locked/bin/python`（Python 3.13.15），未重建或升级依赖；Python 设置 `PYTHONDONTWRITEBYTECODE=1`。ECMA 检查使用已有 Node v22.23.2。临时探针 `/tmp/relaynest-run3/p0305-integration-probe.py`、pattern JSON 与 Node stdin 均不入库。

| 命令或检查 | 实际结果 |
| --- | --- |
| 锁环境 `python server/api/validate.py` | PASS：44 Schema、92 正例 + 146 反例、18 操作及权限声明 |
| 锁环境 `python -m openapi_spec_validator server/api/openapi.yaml` | PASS，`server/api/openapi.yaml: OK` |
| 锁环境独立 Content-Range 探针 | PASS：共享 Schema、206 头、416 头三层共 1152 单区间（size 0..5，start/end 0..7），与独立 `start<=end<size` 条件比较；18 星号边界和 81 异常尾缀符合预期 |
| 不带 pattern 的独立 `x-content-range` 扩展 | PASS：5 合法边界接受；21 异常尾缀与12种非法结构拒绝，覆盖末尾 LF/CR/CRLF、Unicode 行分隔符、NUL、逆序与越界 |
| Node 对从当前 YAML 提取的三条 pattern 执行 RegExp 默认及 Unicode 模式 | PASS：162 异常尾缀全部拒绝，正则均能编译 |
| 独立 Range 矩阵 | PASS：size 0..10，端点/长度 0..12，共1287组闭区间、开放结尾及 suffix；预期用资源字节集合求交，逐项比较 `range_intervals` |
| 八类声明负面变异，各在临时目录运行完整238例与声明校验 | PASS，全部非零拒绝：去掉跨Bot引用声明、Webhook误用BotAPIToken、下载加入WebCookie、上传幂等键变可选、callback去掉receive约束、编辑CAS变可选、公共404缓存改public、图片默认上限改1字节 |
| Git 整树/报告 blob 比较、范围及 `git diff --check` | PASS；变更仅 `server/api/**` 和本卡报告，Web、SQLite、Android及其他模块内容无差异 |

当前 main 已有 Web/SQLite/Android，本次 API 目录不参与它们的构建，且这些模块未改，因此没有重复运行 Go、Web、Android 测试，也不以此前测试替代本卡结果。验收中 expected_revision、幂等、大小限制和无 Android 编辑接口由当前校验器及独立负面变异检查；没有新增运行时接口。

## 限制、保护与交接

NOT_RUN：真实HTTP鉴权/跨Bot授权/凭据撤销、幂等墓碑与CAS并发事务、callback事务、流字节上限、fsync/rename/恢复及清理竞态、实际单/多段Range和ETag条件响应、客户端/设备、部署。这是契约校验通过，不声称以上运行时行为已通过。

没有修改需求、公共契约范围或其他模块，没有额外 agent、推送、发布或部署。开始前已有的 registry 修改不 stage、不覆盖；调度告知期间补充真实 agent_id 属于授权元数据变化，同样保留。未跟踪 `.directory`、`ORCHESTRATOR-CHECKPOINT.md` SHA-256 与开始前相同，不 stage。本集成报告单独提交，自身 SHA 在交接给出；最终 main 精确提交上另用 create_worktree 准备 P03-06 api-delivery 工作树，实际分支/持久路径在交接给出，仅准备不实现。
