# P03-07-S02-S01-I01 设备会话最终组合精确集成

日期：2026-10-22；角色：integrator；结果：implemented（契约集成）。DS-A01-01 经独立 A02 契约复验通过，可交调度登记关闭；不表示运行时、父 S02 或 G-API 通过。

## 精确组合

目标 `/home/lijx/Projects/RelayNest`、`main`；开始 HEAD 精确匹配 `cde59a4e489dc741022e50049a3b959604c47433`。按序 cherry-pick，无冲突或 API 内容调整：

| 源提交 | 目标提交 | 职责 |
| --- | --- | --- |
| `12d9cee328eda93e16a689fa4638c0eba6b06482` | `a32f038b16af1ef271e90e82b0e3fdd68867529c` | 原实现 |
| `de2a6f915e20354e16738f3d17736d50f16894c8` | `a8f6027a564fcc4d2f5082e992aebb16958b2994` | 负面 A01 |
| `7938f479a2ed05c6c27e2bfb631b3ec44a288f9d` | `46a4b8e5a08debd2c86d7dad354972023d0cfa8a` | 修复规划 |
| `8f203ee84212a5003e47b1fdac429262c7a1ddc5` | `d7f41e2facd09ae59caa62958f94bea2a3e64e13` | 规划独审 |
| `6928f4cd79d461a199abfae997ec1ed9e69ee642` | `3a6fd1df77789c58eb0d3d6cc72e0f01de6d5d5a` | 分支规划集成 |
| `a63bd59c8f50dcd83353e279b044f25f4383780c` | `40de39fe18ebf74ad89da40b851f27d9c7121c7b` | 最终 F01 |
| `c05a5b93b64b5348bb98270ade4856fc23061fac` | `f17b94f0eeb533021c1a2650406a3af3c41db2f5` | 独立 A02 PASS |

最终四 API、修复卡和完整报告链共12文件与 A02 源提交逐字节一致，无需集成后内容重审。[负面 A01](P03-07-S02-S01-audit-01.md)仍保留 CHANGES_REQUIRED，[A02](P03-07-S02-S01-audit-02.md)固定审计最终 F01，不覆盖历史结论。本报告额外新增并独立提交，最终 main SHA 提交后交接，不预写自身 SHA。

## 实际验证

Linux，`/tmp/relaynest-p0305-f01-locked/bin/python`，实测 Python 3.13.15；未恢复或升级依赖。

- `.../python -m openapi_spec_validator server/api/openapi.yaml`：PASS，OK。
- `.../python server/api/validate.py`：PASS，84 schemas，331 valid + 356 invalid，29 operations/security。
- 锁环境只读脚本：精确12路径及源 blob 一致；相对 `5cdaf15f7e01b663dd0f2b68fc044d9e875aaa79`，旧27操作/全部旧路径、全部旧 components 分组（含83 Schema）、顶层元数据、637例原值及顺序完整保留，两 requirements 锁字节一致；最终29/84/687。
- 组合 Markdown 本地文件链接11处存在；未验证锚点或外链。原负面审计仍含 CHANGES_REQUIRED。
- `git diff --check <集成前SHA> HEAD`：PASS；提交本报告前另检查暂存差异和限定范围。
- registry 既有/调度未提交内容及未跟踪 `.directory` 保留，未覆盖、暂存或提交。

精确组合未变，不重复 A02 的独立权限/目标矩阵和隔离探针。本轮 HTTP、真实数据库事务/并发、多worker、WS断连、SDK、真机/NAS及现网均 NOT_RUN，合成模型不证明真实撤销效果。未 push、部署、发布或派额外 agent。

## 后续与释放

后卡必须保留同 device_id 多 Session 合法，DELETE 的 `{id}` 精确指向 Session.id，不得用设备唯一会话假设或迟到旧请求回退撤销新会话；注册/偏好归属及条件更新、恢复读和 job 快照仍按 T9.3 独立核对，具体摘要见 F01 与 A02。父 S02 其余验收及 G-API 继续待后卡收口。

提交后释放主树写入权，仅从最终 main 精确 SHA 为 P03-07-S02-S02 准备新的独占 managed worktree / `codex/` 分支，另交路径/分支/干净基线；不实现或测试下一卡，不 reset 已审源树。
