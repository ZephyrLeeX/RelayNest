# P03-07-S03-S01 全局配置与全局 changes 实施

角色：开发 worker。工作树 `/home/lijx/.codex/worktrees/api-global-settings-impl/RelayNest`，分支 `codex/api-global-settings-impl`。实际基线 `46dc206bb061db3c32f29bac1589dc644b62d53e`；实施 SHA 由提交后的交接记录，本文不预写自身 SHA。

仅修改 API 契约四文件与本报告；不改运行时、需求/设计、卡、registry、两依赖锁或旧报告，不推送。前置使用调度确认已集成的设备 S02、S03 PLAN/POLICY01 及健康契约链。采用已批准正整数政策，无新增业务参数、业务上限或权限。

## 冻结结果

- 新增 `getGlobalSettings`/`updateGlobalSettings`：GET/PUT `/admin/settings`，唯一管理员 Bearer/Web Cookie；Cookie 写 CSRF 与同源，响应 no-store。
- 六个新 schema：`GlobalSettingsValue`、`GlobalSettingsVersion`、`GlobalSettings`、`GlobalSettingsUpdate`、`GlobalSettingsConflict`、`SyncGlobalSettings`。三配置使用正 int64 规范十进制字符串，默认 30/10485760/52428800；版本使用专用 JSON 整数 1～9007199254740991，公共/设备版本及 Attachment.size 不变。
- PUT 完整替换，要求 expected_version。结构/类型 400、字段语义 422、正文超 262144 字节 413；先验证输入，再 CAS，再耗尽。过期返回 `GLOBAL_SETTINGS_CONFLICT`；匹配但耗尽返回 `GLOBAL_SETTINGS_VERSION_EXHAUSTED`，均 409、retryable=false，带最新版本及重读路径。耗尽最新版本固定为 M。GET 继续可用，不自动换代或归零。
- 同值成功也增版；三字段、服务端时间、版本及唯一事件同事务提交。M−1 成功到 M；过期且耗尽优先冲突；无效输入/冲突/耗尽保持配置、时间、版本及事件不变。
- 整日年龄用 UTC 时间差的整日部分安全比较，不将保留天数乘 Duration 或构造远古 cutoff；巨大合法天数自然不满足清理条件，未来时间不清理。缩短只影响后续批次。删除事务重查保护队列、未 ACK、必要父消息/附件和其他消息/活动下载引用；不承诺立即清理或永久恢复，不授权静默丢弃受保护队列。

T6 changes 逐类核对：

| 类型 | Schema | 本卡处理 |
| --- | --- | --- |
| message.upsert | SyncMessageUpsert | 原样冻结 |
| message.deleted | SyncMessageDelete | 原样冻结，墓碑不带正文 |
| bot.upsert | SyncBotUpsert | 原样冻结 |
| bot.deleted | SyncBotDelete | 原样冻结 |
| read.updated | SyncReadUpdate | 原样冻结 |
| delivery.upsert | SyncDeliveryUpsert | 原样冻结 |
| device.notifications.updated | SyncDeviceNotifications | 原样冻结，不重做设备通知 |
| settings.updated | SyncGlobalSettings | 新增严格分支，仅版本与管理 GET 关联 |

全局事件字段为 type/change_seq/settings_version/recovery_path；不携带配置正文。正文读取沿管理权限；重复/旧事件不重应用但正常提交游标，旧 GET 响应不倒退新版本。无任意 object 或悬空 ref。旧 schema 中仅 `SyncChange` 的 oneOf 追加上述分支、描述去除待后卡补齐并补充已冻结语义；其余 108 个旧 schema 完全不变。

## 实际验证

解释器 `/tmp/relaynest-health-contract-locked/bin/python`，Python 3.13.15。用 importlib.metadata 对锁文件 18 个固定版本逐项比对通过，无安装或升级。

| 检查 | 真实结果 |
| --- | --- |
| `python -m openapi_spec_validator server/api/openapi.yaml`（上述锁解释器） | PASS，OpenAPI 3.1 |
| `python server/api/validate.py`（上述锁解释器） | PASS，115 schemas，39 operations，1229 例：568 正 / 661 反 |
| 新增案例 | 242 例：98 正 / 144 反 |
| 固定基线结构对照 | PASS：原 37 操作、987 例（470 正 / 517 反）逐项相等；109 旧 schema 仅上文 SyncChange 有意扩展；其余 components 和根字段逐项相等 |
| 两依赖锁字节对照 | PASS，无变化 |
| 声明破坏变异 | PASS，28 项全部被拒 |
| `git diff --check` | PASS |

原例中的 HTTP 101 无正文声明、WS 帧 schema、公开 health 纯文本与管理 JSON 分别沿原校验路径执行；新增 JSON 额外媒体不绕过原媒体安全 F01。此处仍为声明/夹具检查。

锁文件 SHA256：requirements.in `4d56f1903c85ca444dde0966a7fd2702f15242b2cbc5f35eb1fcd273a0317d7c`；requirements.txt `4e1992c8e79a37d1671671afd5e24f686837f719078ac7bdab06b5919231f184`。

新增例涵盖三配置 0/负数/前导零/符号/小数/指数/JSON 错类型/int64 上界外、最大合法值/巨大保留期；专用版本 1/M−1/M/M+1、同值写、并发 CAS 仅一成功、普通过期与过期且耗尽、结构/语义优先级、冲突/耗尽响应恢复信息及无部分写/事件/回绕；Cookie CSRF/同源、匿名及 Bot 越权；恰满天/差一秒/未来时间与全部清理保护；全局事件 union、完整同步页/高水位、禁正文、管理读取、重复/旧事件及 GET 版本防倒退。

28 项变异包括删除 PUT、弱化 CSRF/管理权限、篡改事务顺序/同值写/原子性、移除清理保护/改危险天数算法/宣称立即清理、去版本界限/允许0/数值配置/未知字段/缺必需字段、去冲突恢复版本/耗尽可重试/耗尽未达 M、事件带正文/任意字段/无界版本/错误 GET、union 缺全局或任意 object、缓存放开、新旧 JSON 额外媒体及旧公开 health 媒体变化。基线先执行成功，再对文档深拷贝逐项破坏并调用同一 validator；未改动真实工作树执行变异。

复核脚本位于本地临时文件 `/tmp/relaynest-global-regression.py` 与 `/tmp/relaynest-global-mutations.py`；前者从精确基线 git show 读取并逐项比较，后者在临时目录复用真实 examples 调用校验器。它们不是提交文件，也不是运行时测试。

初期探查 examples 容器时曾误用 list(e)，触发大输出截断；未将截断内容视为全文已读，随后停止初始取材并改为类型/字段与定向节点读取。未记录可靠的精确输入 token 数。生成阶段的 YAML 锚点/缩进与旧例 lone-surrogate 重序列化问题已修正：新 YAML 不使用重复锚点，旧 JSON 内容原样保留后追加；最终解析、全部例及冻结对照通过。

## 后续交接与限制

P06-08 实现配置 CAS/清理必须沿上述 keys、错误顺序、安全整日比较与事务保护；P4/P5/P8/P9 复用全局 GET/PUT、版本上限/耗尽恢复及 settings.updated 分支，界面缩短时明确后续清理影响，不把默认附件大小当绝对上限。

HTTP/真实鉴权/CSRF、SQLite 原子提交/并发与故障恢复、真实清理任务/附件租约、Room 事务、真实 WS 帧、Android/手机/OPPO/NAS：全部 NOT_RUN。声明与合成检查不能替代这些验收，不放行 G-API、P1/P2 或后续实现/正式验收门槛。待不同 agent 对最终精确 SHA 独立审计；当前开发角色不自行销单或集成。
