# P03-05-F01-PLAN-A01 修复规划独立审计

```yaml
task_id: P03-05-F01-PLAN-A01
subject_id: P03-05-F01-PLAN
role: auditor
reviewed_base_sha: 1b4445891621aa41ffa4899664121574b762b851
reviewed_implementation_sha: b11b8287be2239662417a6f2ad847292d5b4b8d1
verdict: PASS
findings: []
report: docs/tasks/reports/P03-05-F01-plan-audit-01.md
```

工作树 `/tmp/relaynest-run2/api-next`，分支 `codex/api-next-run2`；初始干净、HEAD 精确匹配规划实现 SHA。本审计仅新增本报告，不修改修复卡、源码、原报告或 registry；报告独立提交，提交 SHA 在交接提供。PASS 只表示修复规划可派发，不表示 F01 已修复或父卡已完成。

## 核对结论

对照已读取的 AGENTS、CONTRIBUTING、WORKER_PROTOCOL、P03-05 卡/实施报告及原审计唯一发现 `P03-05-A01-F01`，逐项读取本次两份新增文档。固定差异仅为 `docs/tasks/cards/P03-05-F01.md` 与 `docs/tasks/reports/P03-05-F01-plan.md`。

- **发现覆盖**：修复要求包含共同 ContentRange 及具体206/416响应头三处模式的严格字符串终止，拒绝 LF、CR、CRLF 与其他尾缀；不依赖 Python 私有锚点作为共享 Schema 的唯一保障。扩展必须完整识别单区间与星号两类，解析失败产生 ValidationError，不能 trim 后接受。
- **独立门槛**：明确要求模式自身与扩展脱离 pattern 时分别验证，避免一层兜底掩盖另一层未修。保留单区间 `0 <= start <= end < size`、ASCII 数字、合法 `*/0`/`*/100`，206/416形态互换拒绝。
- **示例与非回归**：三条审计基串的 LF/CR/CRLF 变体在共同 Schema 与适用具体响应头两层均需反例；新例实际加入 examples，保留原219例及 Range 矩阵，不调低原预期或混淆 schema_valid/valid。要求独立临时探针、OpenAPI/原校验器/编译/差异检查；除三处模式和必要说明，其他契约解析后相等、依赖字节不变，覆盖前卡权限、幂等、CAS 等非回归。
- **最小路径与边界**：只允许修改相关模式、content_range、对应例子及必要局部说明/修复报告，扩展独立回归确有需要时可在现有流程加入局部断言。禁止其他 Schema、ByteRanges 算法、依赖、运行时源码或原审计/规划/registry 修改，没有引入产品、权限或架构变更。
- **依赖与交接**：登记建议以 subject_id 关联父卡，dependencies 为空并保留原卡前置关系；明确不得要求父卡 done。规划复核集成后由调度固定实际派发 SHA，避免旧基线覆盖文档。修复由另一实现者完成，随后针对精确最终 SHA 安排 P03-05-A02，审计 PASS 后另行集成；不由修复者自行关闭发现。

无需要修改规划的编号发现。

## 实际文档检查

| 检查 | 结果 |
| --- | --- |
| `git status --short`、`git rev-parse HEAD` | 初始干净；HEAD=`b11b8287be2239662417a6f2ad847292d5b4b8d1` |
| `git diff --stat/--name-status 1b4445891621aa41ffa4899664121574b762b851 b11b8287be2239662417a6f2ad847292d5b4b8d1` | 仅两份新增文档，共106行；逐份审阅内容 |
| `git diff --check 1b4445891621aa41ffa4899664121574b762b851 b11b8287be2239662417a6f2ad847292d5b4b8d1` | PASS |
| `PYTHONDONTWRITEBYTECODE=1 /tmp/relaynest-p0304-locked/bin/python -` 文档检查 | PASS：两文档标题、结尾换行、代码围栏配对、F01编号；13个本地 Markdown 链接目标存在 |
| 同一 stdin 脚本以 `git show <基线>:<路径>` 与当前文件字节比较 | PASS：openapi.yaml、validate.py、examples.json、README.md、requirements.in/txt 及 registry 均未变 |

脚本环境为已有锁环境 Python；检查只读取文件和 Git 对象，不安装依赖、不创建源码产物。未重跑 OpenAPI、示例、业务或运行时测试：本次没有源码变更，且派发明确仅审规划。后续修复及 A02 必须按卡真实执行规定检查，不能以本规划 PASS 代替修复结果、运行时 HTTP/权限/事务或客户端验证。
