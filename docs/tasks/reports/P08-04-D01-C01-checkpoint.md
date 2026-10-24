# P08-04-D01-C01 实例身份契约草稿 checkpoint

- 角色：developer；结果：**needs_user（仅新增混合附件鉴权顺序决定），C01未完成/未冻结**。
- 原基线/当前草稿基线：`8138b794e8023dd2eff4a1e03ec1e0a8c07f022c`。
- 工作树：`/home/lijx/.codex/worktrees/instance-contract-freeze/RelayNest`；分支 `codex/instance-contract-freeze`。开始时路径存在、HEAD准确且干净；本轮未出现路径消失。
- 本轮按调度要求仅提交 [两操作混合鉴权提案](P08-04-D01-C01-mixed-auth-proposal.md) 和本checkpoint；下面六个路径保持**未提交草稿**。本报告不预写自身提交SHA，精确docSHA提交后交接。

## 已完成草稿及真实检查

六路径草稿已包含：正式设计的公开probe/G1残余窗口/生命周期/Tx接口/离线命令与退出码/Android门禁和旧版本阻断；公开probe的3个新Schema及无凭据信封；31个Android login/AdminBearer操作的条件头与事务声明；独立封闭AndroidSession、原Session及设备投影/Web响应兼容；完整认证分支和probe/Session正反例。原1229例保留，仅三个既有Android响应值加入required instance_id；新增961例，总2190例。**其中混合操作按已知分支建模，未实现wire分支选择，不能冒充问题已闭合。**

检查环境：项目mise Python **3.13.15**。原 `/tmp/relaynest-health-contract-locked` 不存在，实际执行以下命令重建，未升级依赖：

```sh
/home/lijx/.local/bin/mise exec -- python -m venv /tmp/relaynest-health-contract-locked
/tmp/relaynest-health-contract-locked/bin/python -m pip install --require-hashes -r server/api/requirements.txt
```

锁文件18个依赖已全部成功安装；使用 importlib.metadata 对 requirements.txt 的每个 `name==version` 逐项比较，结果 `locked dependencies 18 version mismatches []`。未修改requirements或运行时依赖。

最终停止扩展后重新实际运行：

```sh
/tmp/relaynest-health-contract-locked/bin/python -m openapi_spec_validator server/api/openapi.yaml
/tmp/relaynest-health-contract-locked/bin/python server/api/validate.py
/tmp/relaynest-health-contract-locked/bin/python /home/lijx/.codex/checkpoints/instance-contract-freeze-C01-8138b79/declaration_mutations.py
git diff --check
```

结果分别：`server/api/openapi.yaml: OK`；`PASS: OpenAPI 3.1; 118 schemas; 1406 valid + 784 invalid examples; 40 operations/security`；`PASS: 20 declaration mutations rejected; repeat check stable`；diff check无输出、退出0。20种声明变异覆盖probe凭据/超时/1024上限/限流/redirect/cache/错误、ID格式、封闭Schema/required ID、公共Session/唯一Bearer、header/绑定顺序/409分支、Web/Bot权限；脚本保存在下述非managed快照供复跑。

以上均是**草稿机器契约/合成状态检查**；不是HTTP、SQLite事务/并发、Go、Web、Android、WS、Room/outbox或真实设备验证。旧源码audit不因设计改变而自动满足新运行时门槛。phone/NAS/OPPO/签名/Keystore本轮NOT_RUN；G-INSTANCE-CONTRACT等待最终实施SHA独立A01/I01，runtime/composition、S04/父gate不改变。

## 未提交草稿与SHA-256

| 路径 | SHA-256 |
| --- | --- |
| docs/TECHNICAL_DESIGN.md | ddef815720e7cdf4208932efdb89ef8265403b9d94c70cf7dc43ec1b6e79c1aa |
| docs/ARCHITECTURE.md | cf4ecd80ab6d2bb194508b86759cd662b81f4f1ac25cf5fa5d0ea5c62f8c485d |
| server/api/openapi.yaml | c81ce0782fe3189fc3f33b2dbad44ea1fde99cfc574cad3c69df64fe35485886 |
| server/api/examples.json | f2e9168f92b414a6491b5b24149f2a0078a23d32bb218e0154f6e34332597b90 |
| server/api/validate.py | 1daa43c88a28d84c0f949eac4994da68730aefafce5948c5343a1c31a5d9ee2e |
| server/api/README.md | bfdff89e27079ec4b75561d8e2e1d2b242388a41fc1bd0164c2c1a3bda153ee1 |

非managed文件级快照：`/home/lijx/.codex/checkpoints/instance-contract-freeze-C01-8138b79/`，保留上述六路径原相对目录、`draft.patch` 与 `declaration_mutations.py`。`draft.patch` SHA-256为 `9bb676b3f0096cc745f1dd04e0acdb376b93d2d55b51e541e2701df109573abe`，只含六路径相对原base差异，无其他用户文件/凭据。恢复前先比对base及现有文件，禁止直接覆盖其他修改；若managed树消失，由恢复任务在非managed树按此快照恢复，不重做实现。

当前dirty仅六路径草稿，加本报告和提案两新文档；仅后两者显式stage。无registry/runtime/SQL/Android改动、迁移号分配、外部接口写入、现网维护、push、部署或新聊天。文档提交后六路径仍dirty是刻意保留，不可误清理或stage成冻结实现。

## 尚缺项与继续入口

1. 新提案先由不同agent独立核对固定docSHA与原base两附件操作/凭据事实，随后用户决定是否批准Bot-first狭窄例外（含无效Bot错误优先级及503候选映射），或选择显式分支协议。旧三项批准继续有效，不重问。
2. 获准后在当前六路径草稿最小修改混合附件规则；当前统一 `x-instance-binding` 的ID先行顺序不够表达推荐例外，必须正式设计/机器声明同步，不能只改说明。补有效/停用/越权/不匹配Bot、Bot存储故障、旧Android缺头、Admin事务ID/代次变化的精确分支选择正反例，不以当前合成branch参数代替。
3. 补公开probe的 declaration 安全正反例：当前通用 `declaration` 夹具分支只特殊允许health的空security；虽然getInstance主权限检查为公开且无凭据请求模型已覆盖，仍需使通用声明例覆盖新公开probe，再跑全部原例。此处已识别、停止扩展后未继续改代码，不隐藏为已完成。
4. 完成 `P08-04-D01-C01-implementation.md`，按最终新计数/命令记证，审查六路径局部diff/消费者/Tx和命令接口，再全量OpenAPI CLI+validator+diff check并显式stage允许路径，本地独立实现提交。最终实施SHA需不同agent A01/PASS/I01，不能以本提案docSHA冻结契约。
5. 后卡使用最终C01契约，当前候选接口为 storage.InitInstanceIDTx / ReadInstanceIDTx / RotateInstanceIDTx（ctx,*sql.Tx；rotate另expectedID，均返回string,error）和 `relaynest instance rotate-id --db PATH --expected-instance-id UUID --confirm-offline`。它们在草稿中，不能作为已独审冻结接口派发S01/M01。迁移编号仍未分配。

本agent交接后释放该树写入；六路径/快照保持原样，等待调度安排后续审查与继续。
