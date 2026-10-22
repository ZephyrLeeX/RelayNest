# P03-07-S03-S01-POLICY01 规划续作

基线：`3785ef9f02258ed10f8a83ce649d547edd946afe`；分支 `codex/api-global-settings`。本轮仅规划，待不同 agent 对固定提交独立审计；不是 S01 API 实施完成或自审 PASS。[旧检查点](P03-07-S03-S01-POLICY01-implementation.md) 保留不改。

## 批准政策与范围

用户已批准保留期为正整数天、附件上限为正整数字节；0 不支持且不表示立即清理、禁用或无限。默认 30 天、图片 10 MiB、文件 50 MiB 不变，不新增业务性上限。计算/存储技术边界须契约明确并独立审计。本轮仅修改 [PRD §3.4/§3.6](../../PRD.md)、[设计 T3.2/T5.4/T6](../../TECHNICAL_DESIGN.md)、[S01 卡](../cards/P03-07-S03-S01.md) 和本报告；不改六 API、源码、registry、其他卡或旧报告。

## 定向证据与最小推荐

- 基线 `server/api/openapi.yaml` 的 `Version` 是 minimum=1 的 JSON integer，没有公共 maximum；`Sequence` 是 SQLite 非负 int64 规范十进制字符串，`Attachment.size` 引用它；`UTCDateTime` 是 RFC3339 Z。设备 `NotificationSettingsVersion` 是 0～int64 最大值，不能顺手收窄。公共类型、设备及已有上传契约均保持原样。
- `web/package.json` 使用 TypeScript/Vite，`web/src/main.ts` 仅骨架；`android/app/src/main/kotlin/top/jianxinli/relaynest/MainActivity.kt` 是 Compose 界面骨架，当前 app 未见全局设置 JSON 入口。不能声称已有客户端完成兼容验证，也没有依据要求全部客户端更换 JSON 库。
- 数值风险可局部复现：`perl -e 'printf "%.0f / %.0f\n", 9007199254740992.0, 9007199254740993.0'` 输出两个 `9007199254740992`。这是本机 binary64 精度演示，不是 Web/Android 运行测试。

| 版本方案 | 兼容与影响 |
| --- | --- |
| 新全局资源 JSON integer，1～2^53−1（推荐） | 与公共 Version 求交集，Web 常用 Number 可无损表示全部有效版本，Android Long/SQLite int64 能容纳；无需新增无损 JSON 依赖。有限版本耗尽须显式拒写。只约束尚未实现的全局资源，不收窄其他实体。 |
| JSON integer，1～2^63−1 | SQLite int64/Android Long 可容纳，但 Web 普通 JSON 数值在高段不能区分相邻 CAS 版本；需要专门的原始数值解析/序列化和全链路精度处理，普通 JSON.parse 后再转 BigInt 无法补救。尚无已确认实现或依赖，不应在本卡默认要求所有客户端迁移。 |

故采用第一方案作为待独立审计技术输入：新专用 schema 复用 `Version` 并加 maximum=9007199254740991，禁止用字符串版本绕过冻结。任何共享同步封装中的该实体版本同样遵守此上限。不是修改公共 Version，也不宣称其他既有 JSON integer 的精度问题已解决。耗尽后的恢复机制留待未来明确迁移，不能暗中换 epoch/归零或扩权限。

三配置字段建议为 `retention_days`、`image_max_bytes`、`file_max_bytes`：正 int64 十进制字符串 1～9223372036854775807，默认 `"30"`/`"10485760"`/`"52428800"`。该上限源于 SQLite int64 存储与既有大小表示，属于技术边界；不是业务配额，不把 T8 可配置初始大小固定成绝对上限。新 schema 排除公共 Sequence 允许的 0，不修改 Sequence。大字节值用安全整数/长度上限比较，不在 Web 经 Number 中转；未来 UI 可在字段局部使用字符串/BigInt，具体代码不在本轮。

保留期按 UTC 实际经过的整日年龄比较，日/秒分解且防溢出；不将最大天数乘 Duration 或从 now 生成越界 cutoff，未来 created_at 不过期。巨大合法保留期不满足年龄条件即可。默认、保护队列、必要父消息/附件与引用规则不变，缩短只影响后续批次，不承诺立即清理。此算法不更换时间字段。

## 错误、耗尽与实施验收

沿既有 400 格式/结构、422 字段语义、413 请求超限分类；字段字符串的非规范表示/0/越界为 422，错误类型或未知字段为 400。`expected_version` 为 JSON 数字但非整数或超专用范围时 422；字符串/其他类型为 400。S01 须将分类显式写入声明及 validator，避免 JSON Schema integer 与错误路由混淆。

顺序为输入验证 → CAS → 耗尽判断 → 同一事务写配置/服务端时间/版本与变更。过期 409 `GLOBAL_SETTINGS_CONFLICT`；最新版本已达上限且 expected_version 匹配则 409 `GLOBAL_SETTINGS_VERSION_EXHAUSTED`，`retryable=false`。两者返回最新版本及 `/admin/settings` 重读路径；过期且耗尽仍优先冲突，重读后可见耗尽。边界 M−1 的成功写入可到 M，此后即使同值也拒写，无部分写入、事件、回绕、归零。GET 仍可用，恢复写入需将来的契约迁移。

全局 `settings.updated` 明确类型分支携带版本和管理 GET 关联，正文权限不变，不改设备通知分支。全部字段、错误码和分支为 S01 技术输入，尚未写入 API；S01 从最终 main 取得已完整集成设备父卡、WS 和冻结 API 证据后再实施。正反例需包括最大配置值、M−1/M/M+1、过期且耗尽、同值写入、并发 CAS、清理保护和权限；本规划不提前写案例或更改旧上传限制。

以上属于已批准政策下的新全局资源安全表示细化，独立审计仍可否决技术方案；本轮没有新增需要用户决定的业务参数、外部服务接口或权限。若审计要求公共 Version、现有设备版本或全客户端 JSON 架构迁移，应另报具体影响。

## 检查与未执行项

- `git diff --check`：通过。
- 对基线六 API 路径执行 `git diff --exit-code`：通过，字节不变；旧报告及其余文件未修改。
- 四文档相对 Markdown 链接存在性、允许范围、政策/默认值/技术边界一致性：定向检查通过（交接附实际命令结果）。
- API validator、899 例、运行时、真机与清理集成测试：NOT_RUN；纯规划无 API 或运行时代码变更，不以数值演示代替兼容测试。

## 精简实施 recipe

1. 从最终 main 派发独占树，先核根规则、S01 卡、POLICY01 独立审计结论及设备父完整集成摘要（≤800字）；不要将本报告基线当实施基线。
2. 仅读本报告批准规则/推荐/错误段、PRD §3.4/§3.6、设计 T3.2/T5.4/T6 的对应新增段；不重读旧 checkpoint 或全设计。
3. 最终 API 定向定位 Version/Sequence/UTCDateTime/Attachment.size、SyncChange 相关 oneOf/ref/envelope/权限、Error 与设备既有分支，按需读取 validator 对应函数；不预读全协议或全部例子。
4. 冻结 settings/CAS/清理/change 闭环后只写 S01 允许文件；按卡运行锁环境 API 校验、必要正反例及 diff 检查，独立提交并交不同 agent 审计。输入目标≤8k、硬12k，材料保持定向，超预算留下明确 checkpoint。

## 交接摘要（≤800字）

批准规则已落 PRD：保留正整数天、附件正整数字节，0 不支持，默认30天/10MiB/50MiB不变，无新增业务性上限。规划推荐三值用正 int64 规范十进制字符串；全局 CAS 保持 Version JSON integer，仅新专用 schema 限1～2^53−1，兼容普通 Web Number/Android Long，避免全客户端无损 JSON 迁移；公共/设备版本不改。过期409冲突，匹配且耗尽409不可重试，拒写且不回绕；安全整日年龄比较避免 Duration/cutoff 溢出。S01 卡补充边界验收和最终 main 前置要求；全局同步仅版本与管理 GET 关联。六 API、源码、registry、旧报告不改；仅文档检查通过，运行时/API测试 NOT_RUN。待不同 agent 固定 SHA 独立审计，无新业务/权限决定；本报告不构成 API 实施或自审通过。
