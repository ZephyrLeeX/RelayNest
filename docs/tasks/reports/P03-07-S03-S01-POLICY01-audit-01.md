# P03-07-S03-S01-POLICY01-A01 独立规划审计

结论：**PASS**。审计基线 `3785ef9f02258ed10f8a83ce649d547edd946afe`，被审最终 SHA `7c2d995c8f045ecb31f87065b1a7e90eaf5f0e86`；独占工作树 `/home/lijx/.codex/worktrees/api-global-settings/RelayNest`，分支 `codex/api-global-settings`。本报告仅通过 POLICY01 技术规划，不放行 G-API、不标记 S01 实施完成。发现编号：无；无待用户批准项。

## 批准边界与技术核验

按本次用户授权核对 PRD §3.4/§3.6、设计 T3.2/T5.4/T6、S01 卡和 finalization 报告差异：正整数天/正整数字节，0 无立即清理、禁用或无限含义，默认30天/10MiB/50MiB不变，无新增业务上限。int64 存储表示边界和新全局资源的 CAS 安全上限属于已授权独立审计的技术细化；不新增业务参数、服务职责、权限或客户端总体架构。

定向读取 `server/api/openapi.yaml:2384` 的 Sequence、Version、UTCDateTime，`:3048` 的 Attachment、`:3705` 的设备版本：配置字符串正 int64 范围可容纳于 SQLite 有符号 int64，沿既有 Attachment.size 的表示方式，但排除 Sequence 允许的0；不是把版本改成序号。规范字符串还须拒绝符号、前导零、指数、小数、数字类型和边界外值。后续实现应先验证长度/语法再安全解析，不经 Web Number 中转；无需全客户端新 JSON 库。既有上传与设备契约保持不变，未声称解决其全部历史精度问题。

全局专用版本与公共 Version 求交集为 JSON integer 1～2^53−1，Web binary64 可精确表示全部合法值，Android Long/SQLite int64 可容纳。M−1 成功到 M，匹配且耗尽明确409及 retryable=false；先输入验证，再 CAS，再耗尽，过期且耗尽仍冲突重读。字段、时间、版本、变更同一事务；同值写入增版，耗尽无部分写入/事件/回绕。GET 可继续，未来恢复需确认迁移；未暗增 epoch 或归零权限。公共 Version 与设备版本未收窄，同步中此实体版本同样受上限约束。

UTC 整日年龄比较可避免巨大保留期乘 Duration 或反向生成越界 cutoff；需用安全日/秒分解并处理小数秒，未来时间不过期。独立边界例支持一天前1微秒、恰一天、未来及巨大保留期的判断；不能据此声称生产时间解析/清理已实现。原未完成/未 ACK/failed及必要父消息、附件、引用保护和删除事务复查未丢失。缩短期限仅影响后续批次，无立即清理或永久恢复承诺。

400 类型/结构、422 字段语义、413 体积沿现有分类；expected_version 的数字非整数/越界为422、错误类型为400有明确规划，后续 validator 必须显式区分。全局 settings.updated 仅版本与管理 GET 关联，设备通知分支、管理正文权限与同步权限不变；错误专用 schema、正反例和并发 CAS 均移交 S01。

## 实施输入与范围检查

S01 保留原 GET/PUT、CAS/冲突恢复、清理保护、changes 明确分支、P06-08及客户端交接、锁环境检查和共同 API 唯一 writer 约束。recipe 要求最终 main 的前置完整集成 SHA、实际 API 六文件指纹和交接语义，不能以本规划基线或历史899例计数替代未来基数。执行时还须遵循总调度 ORDER01：health 独立先行，settings 真正实施取 health 精确集成后的最新 main；本政策不要求 health 等待 settings。本次未改 ORDER01、依赖台账或冻结未来 API。

实际执行结果：

- `git diff --check 3785ef9f02258ed10f8a83ce649d547edd946afe 7c2d995c8f045ecb31f87065b1a7e90eaf5f0e86`：PASS。
- Perl v5.40.1 程序化比对 diff 路径集合：仅上述四文档；逐一比较 Git blob：六 API 字节相同、基线137份旧报告相同；四文档15个相对链接全部存在。
- Perl Math::BigInt 独立证明 int64/2^53−1 常量、正数规范边界样例及默认字节换算；binary64 高段相邻值碰撞和 UTC 整日边界/未来/巨大期限算例：PASS。它们仅为表示和算法可行性证明，不是运行时或客户端兼容测试。

限制：首次尝试 Python 检查因 `python3` 不存在未执行，随后以 Perl 完成上述文档/数值检查；SQLite 实际存取测试 NOT_RUN（本树环境无 Python/sqlite3）。API validator、899例、Web/Android JSON、生产清理/并发事务、真机及集成测试均 NOT_RUN。纯文档规划未修改 API 或运行时代码，不机械重跑业务测试，也不据本报告豁免 S01 必需检查。SQLite 范围依据既有冻结 Sequence/Attachment.size 契约，本轮不引入新数据库接口。

复验入口：未来若修改数值表示、版本上限/错误顺序、同步权限或安全时间算法，应对最终相关 SHA 重审；实现按 S01 专用正反例及原保护/权限验收执行。本轮只提交本审计报告，不修被审文档、不改 registry、不推送或部署。
