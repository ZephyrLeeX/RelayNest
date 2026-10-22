# P03-07-S03-PLAN01-I01 最终域预规划精确集成

日期：2026-10-22；角色：integrator；结果：implemented（纯文档预规划集成）。不表示 S03 已实施、设备父任务完成或 G-API 通过。

## 精确组合

- 目标 `/home/lijx/Projects/RelayNest`、`main`；集成前 SHA：`3cdb5e228b7d124890d45250728b260b2091a6db`，开始 HEAD 精确匹配。
- 源规划：`6665555c9b715e1f9bbbcffff645b6ca114c3c39`；不同 agent 独立 PASS 审计：`abe849921e26bbcedd57c236c32a37ab8e8ce10f`，见[审计报告](P03-07-S03-PLAN01-audit-01.md)。
- 顺序 cherry-pick，无冲突、无内容调整；目标规划：`ee76ff243d4efcd6627187419788b159ce287af7`，目标审计/精确组合：`b5ad917510ab25ea0added3925a4d15cdfc9bcb1`。
- 六源文档为父卡、三子卡、规划实施及审计报告，与被审源提交逐字节一致，无需内容重审。额外仅新增本集成报告，独立提交；最终 main SHA 在提交后交接，不预写自身 SHA。

## 实际检查

Linux/Git，系统 `python3` 执行只读检查：

- 差异路径精确为六源文档，工作区内容与源审计固定 blob 一致。
- 父卡所有八项 checkbox 与集成前逐字、逐序一致；设置、健康及最终证据三域串行归属与父验收保留。
- 六文档 Markdown 本地文件链接30处均存在；不验证锚点和外链。
- 其他371个既有文件 Git blob 不变；API四文件和两 requirements 锁工作区字节未变。未纳入另树 WS/API 内容。
- `git diff --check <集成前SHA> HEAD`：PASS；提交本报告前另检查暂存差异及限定路径。
- registry 既有/调度未提交改动及用户未跟踪 `.directory` 保留，未覆盖、暂存或提交。

纯文档组合，不跑 OpenAPI/validate.py、业务全测、运行时/并发、WS、Android/真机、厂商/NAS/现网矩阵，均 NOT_RUN；不豁免未来实施检查。未 push、部署、发布或派额外 agent。

## 门槛与释放

本次 PASS 不解锁 S03 实施或 G-API，仍等待 S02 父完整独立审计和精确集成。未来派发采用最终 main、WS/设备实际摘要和基数，当前规划不抢冻字段。原 P03-07/G-API 最终证据及运行时门槛继续保留。

无新增集成冲突、技术内容修改或用户决策。提交后释放主树写入权给调度；本轮不准备新工作树。
