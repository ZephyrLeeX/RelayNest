# P03-07-S02-S02-I01 注册生命周期精确集成

日期：2026-10-22；角色：integrator；结果：implemented（仅契约集成）。父 S02 与 G-API 不自动通过。

## 精确组合

目标 `/home/lijx/Projects/RelayNest`、`main`；集成前 HEAD 为 `d0343fa3d2a72b1f5f5a7c4db9087b4bb4105402`。源工作树 `/home/lijx/.codex/worktrees/api-push-registration/RelayNest` 检查干净。

源实现 `6901bc09de53995435102d11f2dfad9bd3f46059` 的父提交精确为 `876935a219c3a1bf828aeaabb2e8a82ca2bb5d4a`；独立审计 `14081bcb11a5ce8c7ce7b700b6b1ca6e10bdf1e1` 的父提交精确为该实现。原基线至集成前 main 仅调度检查点变化，API 未变。

| 源提交 | main 目标提交 | 职责 |
| --- | --- | --- |
| `6901bc09de53995435102d11f2dfad9bd3f46059` | `02c2386a3f3bc7a6bd76a99345ecdd5cfa083aac` | 注册生命周期实现 |
| `14081bcb11a5ce8c7ce7b700b6b1ca6e10bdf1e1` | `28ae9c980bf1ff0d2a3c5db3719a0e53f55648e1` | 独立 A01 PASS |

按序 cherry-pick，无冲突、无 API 内容调整；四个 API 文件与实施/审计报告共六个路径均与审计源提交逐字节一致。[独立审计](P03-07-S02-S02-audit-01.md)固定审查上述源实现，PASS/findings=[]。精确内容未变，无需内容重审。本报告独立提交，最终 main SHA 在提交后交接，避免自指。

## 实际检查

Linux；解释器 `/tmp/relaynest-p0305-f01-locked/bin/python`，实际版本 Python 3.13.15；本轮未安装或升级依赖。

- `.../python -m openapi_spec_validator server/api/openapi.yaml`：PASS，OK。
- `.../python server/api/validate.py`：PASS，91 schemas、371 valid + 394 invalid =765 examples、32 operations/security。
- 锁解释器只读比较脚本：六个路径源 blob 一致；相对原基线全部旧29操作结构、84 Schema、全部100组件、687例原值及顺序保持；两 requirements 锁字节一致。最终32操作/91 Schema/765例。
- `git diff --check d0343fa HEAD`：PASS；报告提交前另检查 status、限定暂存路径和 cached diff。
- 主树 registry.json 与未跟踪 `.directory` 保留，未覆盖、暂存或提交；提交前后核对 SHA256。

本轮未重复独立审计的19破坏探针及追加断言。真实 HTTP、数据库 CAS/事务/并发、多worker、Android 来源回调、SDK、外发/撤回、平台展示、手机/NAS和厂商现场均 NOT_RUN，合成模型不证明运行时。未 push、部署、发布或派额外 agent。

## 后续与释放

后卡保留同 device_id 多 Session、Session.id 精确归属、provider 独立版本、初始 CAS=0、同版同 token 先于 CAS 幂等、删除持久屏障、耗尽不回绕和旧结果来源隔离。恢复投影绝不回显 token。通知偏好 CAS/模式、同步设置 union 与完整发送快照由 P03-07-S02-S03 联合收口，不提前判定父 S02/G-API 通过。

提交后释放主树写入权，从最终 main 精确 SHA 创建下一卡独占 managed worktree 与 `codex/` 分支，实际干净路径/分支/基线另行交接，不实施下一卡。源注册树保留。
