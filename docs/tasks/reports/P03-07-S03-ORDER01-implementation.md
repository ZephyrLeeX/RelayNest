# P03-07-S03-ORDER01 健康先行依赖核对

## 结论与边界

基线：`2ac12d12a9a57fc45dad24d32516ad314b006c57`；工作树 `/home/lijx/.codex/worktrees/api-health-order/RelayNest`，分支 `codex/api-health-order`，开始时干净。角色：规划。结论：健康可独立先行；原 S01→健康边属于共同 API 写入顺序，不是新增 settings Schema 的真实技术依赖。本次仅调整三张卡的调度描述，未实现或冻结新接口。

证据：

- [技术设计 T8](../../TECHNICAL_DESIGN.md) 第284、286行分别定义公开健康及管理员只读状态：live 进程、ready 数据库/挂载；管理投影覆盖队列、等待、失败、连接、附件恢复、DB/推送脱敏摘要，没有 settings 字段或配置版本读取前置。FCM 降级规则独立于配置产品参数。
- [路由](../../../server/internal/httpapi/router.go) 第12～13行直接注册根 `/health/live`、`/health/ready`。[健康处理器](../../../server/internal/health/health.go) 仅接收 Probe，Live/Ready/respond 未引用 settings；正文安全输出及 no-store/nosniff 已存在。此处只核对既有实现，不将未来管理员投影描述为已实现。
- [OpenAPI](../../../server/api/openapi.yaml) 根 servers 当前为 `/api/v1`；`Version`（2390）、`Error`（2416）、`AdminBearer`（4153）、`WebCookie`（4157）已有共享定义。Version 是实体版本；Error 是受保护 JSON 错误；管理员身份已支持唯一管理员设备会话/Web Cookie。health 可复用既有管理员身份/错误/时间定义，无需等 S01 新增配置 Schema；公开纯文本仍单独声明，不能落入 `/api/v1` 或套用 JSON Error。
- [原父验收](../cards/P03-07.md) 与 [S03八项验收](../cards/P03-07-S03.md) 保持完整：settings 的配置/CAS/清理/同步分支仍由 S01 完成；health 的公开探针及管理员投影仍由 S02 完成；全 G-API 仍由 S03-S03 同时汇合两者。

## 调度与 registry 最小建议（本轮不写 registry）

1. 健康 `P03-07-S03-S02.dependencies` 改为 `[P03-07-S02, P03-07-S03-PLAN01, P03-07-S03-ORDER01]`，全部指完整独立审计集成；本 ORDER01 未审计集成前不得按新依赖启动。
2. 最终证据卡 `P03-07-S03-S03.dependencies` 明确为 `[P03-07-S03-S01, P03-07-S03-S02]`，不再依赖旧传递边隐含 settings。
3. S01 卡/依赖/产品决策由另一规划者维护，本轮不修改。原规划与设备父仍是共同真实前置；ORDER01 仅核对调度，不依赖未来 settings/health 完成。派生 A/I 使用 subject_id，不用被审卡 done，不能引入父子完成环。
4. 共同四份 API 始终单写者，锁文件只读。健康先行审计精确集成后，下一张实施卡从当时 main 最新实际 SHA 派发，带六文件指纹与健康摘要重验全部非回归；若 settings 已先集成，健康同样重新取其最新基线。两张实施卡不得同时写共同 API，取消语义依赖不等于取消资源串行。

调度只有在将本建议登记并完成独立审计集成后才能派发健康。本报告不放行 G-API、P4 运行时、真机、OPPO、NAS、签名或发布，不修改 PRD/技术设计、权限、监控平台或任何配置产品参数。

## 验证与限制

环境：当前工作树 zsh、Git、Perl；无 `python3`，首次用于文档替换的 Python 命令未执行（退出127），随后使用 apply_patch 完成修改。纯文档任务不恢复 API Python 环境，不运行业务全测/运行时或外部验证。

实际检查：

- `git diff --check`：通过。
- Perl 定向检查三张改动卡及本报告的相对文档链接：通过。
- Perl 对比基线与当前三卡全部 `- [ ]` 验收行，逐字逐序相同：通过（父8项、health4项、最终5项）。原 P03-07/S01 卡与旧报告未修改。
- Perl 从三子卡建议依赖提取局部 DAG，加入 ORDER01/PLAN01/设备父已完成前置，深度优先检查无环，并断言最终卡显式依赖两分支：通过。此次未改前置内部依赖，未重审全仓调度台账。
- `git diff --exit-code 2ac12d12a9a57fc45dad24d32516ad314b006c57 -- server/api/openapi.yaml server/api/examples.json server/api/validate.py server/api/README.md server/api/requirements.in server/api/requirements.txt`：通过，六文件字节不变；未重跑 OpenAPI 校验，不宣称新增 API 已验证。
- 范围检查：仅三张授权卡及本报告；显式 stage 独立本地提交，不 push/部署。提交 SHA 在提交后交接提供，不在报告预写自身 SHA。

读取材料接近硬上限12k，超过目标6k；已停止扩读，只读了协议/四卡、T8、父验收和局部源码/共享身份版本错误，未整读 API 或设计。无未决产品问题；运行时及新契约验证留正式实施卡。

## 下一卡最小上下文

健康无需等待 settings 规划：先确认设备父、原规划、本 ORDER01 完整审计集成，再取得共同 API 独占窗口和最新 main SHA。只读 T8 两段、现有根 health/router、AdminBearer/WebCookie/Error/Version/UTCDateTime 与相关验证 keys；依卡补公开根路径安全纯文本及管理员 JSON 状态投影。不要加入 settings 参数/版本到 health，不预冻未实现配置字段。移交精确 API keys/六文件指纹/集成摘要给后续 settings 与最终证据卡；最终必须同时取得 settings 和 health 两支证据。
