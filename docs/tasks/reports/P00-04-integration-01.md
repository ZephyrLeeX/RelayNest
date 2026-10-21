# P00-04 集成报告（I01）

日期：2026-10-21。角色：integrator，独立于本卡开发及审计。结论 PASS；无冲突或内容变更，无需重审。

## 精确目标与映射

目标为 `/home/lijx/Projects/RelayNest` 的 `main`；集成前 HEAD 为 `b0b614723d8055247d77fbeaa9c6dd88214d3c27`。

| 对象 | 原 SHA | main SHA / 处理 |
| --- | --- | --- |
| 实现 | `7cf1a3148ffebfd05cc8f4d6d5a6ae9b5ed6fadc` | 同 SHA 已为 main 祖先，未重复 pick |
| 独立审计 PASS | `7812dccb9801c53ef844360c6ecddc6a0e5811f9` | `8cbfe77086af3e3f0e65d294d9a4f9eb1edb3c5a` |

审计报告集成后、本报告提交前 HEAD 为 `8cbfe77086af3e3f0e65d294d9a4f9eb1edb3c5a`。本报告独立提交，提交 SHA 后续交接。

## 实际验证

- 读取 P00-04 卡与独立审计报告；`git show --stat` 核对两个对象存在、范围各只有对应报告，审计准确引用原实现及其基线，verdict 为 PASS。
- `git merge-base --is-ancestor <实现> HEAD` 返回 0；Python 对比原实现的环境报告与当前文件，逐字节一致。
- `git cherry-pick <审计>` 成功、无冲突；Python 对比原审计提交报告 blob 与 main 文件，逐字节一致。
- `git diff --check <集成前> HEAD` 通过；除新增审计报告外，完整 index entries 与开始时一致；registry 和 `.directory` SHA-256 均与本卡开始时一致，没有编辑、stage 或提交二者。
- 本次仅报告集成，未运行业务构建、真机、FCM、OAuth 或签名验证；旧工具链版本和旧网络/设备记录属于历史证据，不能解释成当前现场复现通过。

## 限制与交接

P00-04 环境登记的独立审计证据已精确进入 main，可用于后续 Android 基础工程依赖收口；任务状态由总调度登记。审计 E01 历史证据粒度、E02 正式签名缺口、E03 网络及设备覆盖限制全部保留；不替代 P2 审计、G-FCM 或发布门槛。未改实现、台账、产品或技术设计，未推送、发布或部署。集成完成后按调度要求从最终 main HEAD 新建独立 Android 工作树，仅准备不实现。
