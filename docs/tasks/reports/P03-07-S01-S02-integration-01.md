# P03-07-S01-S02-I01 bootstrap 最终组合精确集成

日期：2026-10-22；角色：integrator；结果：implemented（契约集成）。BS-A01-01 经 A02 在契约/合成状态范围关闭；不表示运行时、父 S01 或 G-API 完成。

## 精确组合

目标 `/home/lijx/Projects/RelayNest`、`main`；开始 HEAD 精确匹配 `146928a408d4544e2dc6d5b7f9a80573be74689d`。依次 cherry-pick 以下源提交，无冲突或内容调整：

| 源提交 | 目标提交 | 职责 |
| --- | --- | --- |
| `9cb95c41488fdf29895900b8288b7c15aacc5abe` | `8c2b06a0e6ff99762fc92d4a43414e14047e3fcb` | 原实现 |
| `532ea26e791b9774bcd6c6246769499a40823b26` | `f8376d747bc6eac3dbbcaff7d44053a0c788801a` | 负面 A01，保留 CHANGES_REQUIRED |
| `7922f0f6704a6b52df7f2b290b4595cf6d50fc68` | `52feb51994f381c95a5fa1a1c1a24743f84beb81` | F01 规划 |
| `8bff2ea39183cb3bb5d29f0b61691bbfbbe3785b` | `448b96e41c4a80d6ef0edc271ef4be869144147d` | 规划独审 |
| `ab5fe9a1c4f4821877d677095d8ca251a7d93e27` | `417aba854984a6e8597ad833c0d790d6e0132d94` | 分支规划集成 |
| `a379d9620c9a0b616c575d749fd05030376437c3` | `0d6276c1d4b960be8793ddf28e99fda873bcc913` | F01 最终修复 |
| `0993016dd18b120ce4c300c7b86087474e5eee9e` | `ff960f7d1b2cec2aafe335e70bc316b16f7febab` | 独立 A02 PASS |

[A02](P03-07-S01-S02-audit-02.md)固定审计最终 F01，结论关闭 BS-A01-01；未改写[A01](P03-07-S01-S02-audit-01.md)。四 API 及完整规划/报告共12文件均与 A02 源提交逐字节一致，无需集成后技术内容重审。额外仅新增本报告，独立提交；最终 main SHA 提交后交接，不预写自身 SHA。

## 实际验证

Linux，解释器 `/tmp/relaynest-p0305-f01-locked/bin/python`，实际 Python 3.13.15；未恢复或升级依赖。

- `.../python -m openapi_spec_validator server/api/openapi.yaml`：PASS，OK。
- `.../python server/api/validate.py`：PASS，77 schemas，240 valid + 288 invalid examples，26 operations/security。
- 锁环境只读脚本：精确12路径组合和源 blob 一致；对 `33cc7585e873a5debaddc5b93c8657b60d5a73a8`，旧25操作/全部旧路径、全部旧 components 分组（含73 Schema）、顶层元数据及446例原值/顺序完整保留，两锁逐字节一致；当前26操作/77 Schema/528例。
- 组合文档本地 Markdown 文件链接6处均存在，不验证锚点/外链；原负面审计仍含 CHANGES_REQUIRED。
- `git diff --check <集成前SHA> HEAD`：PASS；提交本报告前另检查暂存差异和范围。
- registry 既有/调度未提交改动及未跟踪 `.directory` 保留，未覆盖、暂存或提交。

本轮未重复 A02 的隔离破坏矩阵，精确组合无 API 变动；完整矩阵见 A02。本轮 HTTP/令牌签发防篡改、SQLite事务/并发、Android/Room、真实WS、Go/真机/厂商/NAS/现网均 NOT_RUN。契约声明不替代后续运行时实现。未 push、部署、发布或派 agent。

## 后续与释放

下一 WS/父联合收口继续复用已审 sync/read 与修复后的 bootstrap，需独立开发、不同 agent 审计、精确集成及父验收映射；父 S01 未收口前不放行设备实施，G-API 继续未通过。

本报告提交后释放主树写入权，仅从最终 main 精确 SHA 准备 P03-07-S01-S03 独占 managed worktree 与 `codex/` 分支，另交路径/分支/干净基线；不实现或测试下一卡，不 reset 已审源树。
