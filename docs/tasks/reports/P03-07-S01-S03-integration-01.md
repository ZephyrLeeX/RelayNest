# P03-07-S01-S03-I01 WS 与父同步域契约精确集成

日期：2026-10-22；角色：integrator；结果：implemented（契约集成）。父 S01 三卡契约验收可由调度收口，不代表运行时或完整 G-API 通过。

## 精确组合及检查

- 目标 `/home/lijx/Projects/RelayNest`、`main`；集成前 SHA：`e6b0565ec3be85997d9b7daa66bce9ad7cec2ab8`，起始 HEAD 精确匹配。
- 源实现 `169fd8043e48e37762f11e845f00f56ce8024933`，不同 agent PASS 审计 `91215226a724679ca163be5279c049cb6bba3ad7`。
- 顺序 cherry-pick 无冲突或 API 调整；目标实现 `4b5e24677f44d53a04f2ff56f480d43937674d72`，目标审计/精确组合 `a751df6aa3bfc3eda26b8cd352fcd49398be4c13`。四 API 及实施/审计报告恰六文件，与源审计 blob 逐字节一致，无需集成后内容重审。
- 本报告额外新增并独立提交；最终 main SHA 提交后交接，不预写自身 SHA。

Linux，锁解释器 `/tmp/relaynest-p0305-f01-locked/bin/python`，实测 Python 3.13.15，未升级依赖。实际检查：

- `.../python -m openapi_spec_validator server/api/openapi.yaml`：PASS，OK。
- `.../python server/api/validate.py`：PASS，83 schemas，295 valid + 342 invalid，27 operations/security。
- 锁环境只读脚本对 `3cdb5e228b7d124890d45250728b260b2091a6db`：旧26操作/全部旧路径、全部旧 components 分组（含77 Schema）、顶层元数据、528例原值及顺序逐值不变，两 requirements 锁字节一致；最终27/83/637。
- 精确六路径和被审 blob 比较通过；前两卡 main 集成 SHA 均为当前祖先，两份集成报告存在。
- `git diff --check <集成前SHA> HEAD`：PASS；本报告提交前另检查暂存差异及限定范围。
- registry 既有/调度未提交内容与未跟踪 `.directory` 保留，未覆盖、暂存或提交。

## 父 S01 三卡链与八项验收

| 子卡 | 固定实现/审计 | main 集成证据 |
| --- | --- | --- |
| sync/read | `966066831a6b1b75720cae131b2e5d17c22e6e35` / `d51bceebcf4a81637920efd9c60b802a36943ee1` | `33cc7585e873a5debaddc5b93c8657b60d5a73a8`，[集成报告](P03-07-S01-S01-integration-01.md) |
| bootstrap 最终修复 | `a379d9620c9a0b616c575d749fd05030376437c3` / A02 `0993016dd18b120ce4c300c7b86087474e5eee9e` | `3cdb5e228b7d124890d45250728b260b2091a6db`，[集成报告](P03-07-S01-S02-integration-01.md) |
| WS/父联合验收 | `169fd8043e48e37762f11e845f00f56ce8024933` / `91215226a724679ca163be5279c049cb6bba3ad7` | 本次精确组合 `a751df6aa3bfc3eda26b8cd352fcd49398be4c13` |

[WS 独立审计](P03-07-S01-S03-audit-01.md)已逐项核对父八项：四操作权限/握手、固定H/epoch/floor、当前投影/六类变更、bootstrap恢复、read max/未读、WS事件/生命周期、数字/隔离/越权、keys/错误/时间/no-store/锁。三链与当前组合一致，前卡内容冻结复验通过，故父 S01 **契约验收可收口**。BS-A01-01仅在契约/合成状态范围由 A02 关闭，原负面审计保留；设备与全局设置分支继续由后卡补齐。

未重跑独立审计42项破坏矩阵；精确 API 未改，证据见审计。本轮 HTTP/WS真实握手、Accept/Origin部署、令牌签发防篡改、计时/慢端网络、SQLite事务并发、Android/Room、真机/OPPO/NAS及现网均 NOT_RUN，契约/合成声明不替代运行时。本次和旧报告中的读取预算偏差保留，不改写旧报告或声称预算全部达标。不放行 G-API，不改变 P1/P2 放行。未 push、部署、发布或派额外 agent。

## 释放与后续

本报告提交后释放主树写入权，仅从最终 main SHA 为设备会话契约 P03-07-S02-S01 准备新独占 managed worktree 与 `codex/` 分支；创建后另交路径/分支/干净基线，不实现或测试下一卡，不 reset 已审源树。
