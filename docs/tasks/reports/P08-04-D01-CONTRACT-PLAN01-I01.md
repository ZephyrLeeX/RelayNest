# P08-04-D01-CONTRACT-PLAN01-I01 精确集成

- 角色：integration；目标分支 `main`；集成前 `79c4601deb71d5865e3539ddc7da7f138737ee15`。
- 被集成规划 `ca8cd805b0dc7f13aba8fed2a4ece5319eff92d6` → `14cff188221148e12162d6b59866871ca8fe8f1e`。
- 独审 PASS `b98a023733dc2c26ef54f9db1b07a50447d43953` → 组合 `f0e9dd348c6415d4e4036bd18f5c0ecbe21a6cb7`，见[独审报告](P08-04-D01-CONTRACT-PLAN01-A01.md)。仅两次精确 cherry-pick，无冲突、内容修订或重审需求。本报告独立提交，准确 SHA 提交后交接；最终 main 为随后独立 snapshot45 提交。

## 实际验证

- 来源 `/home/lijx/.codex/worktrees/instance-contract-plan/RelayNest`、`codex/instance-contract-plan`：开始及交付核验 HEAD 为上述独审 SHA、工作区干净。
- Perl 只读核验：12 个文档路径的 Git blob 全部等于来源独审 HEAD；其中规划 11 文件的 71 个相对链接均存在；父卡与 S04 的集成前全文仍为当前文件原样前缀，原验收未删改。
- 人工核对八卡链 C01→S01→M01→B01→N01→K01→R01→V01→S04→父；每卡最终 SHA 经另人 A/PASS/I，A/I 用 subject_id，不等待被审任务 done。设备共享入口、完整 P04-05/P03-03 和 Android 串行依赖仍在，无环。
- `git diff --check 79c4601 HEAD` PASS；差异仅八新卡、父/S04追加、规划与独审报告。API、源码、SQL、迁移号均未变。纯文档未跑业务、新契约、Go、Android或真机矩阵，不能据此宣称新门槛通过。

## 当前授权与后续边界

用户 G1 残余窗口、生命周期、具体契约/兼容成本三项已批准，以 root 当前 registry 的 answered 决定为准；旧提案与旧 snapshot 的 pending 属历史，不覆盖新决定。旧 G-API 仅对应原 39 operations / 115 schemas / 1229 cases，新 C01 尚未实施或冻结。root 登记后下一步是 C01；本轮仅准备干净工作树，不研究或实施。

Bot 管理 HTTP 已本地收口；设备 S03 与 P04-07 规划原路径不可访问，当前未完成、由专门恢复 agent 核查，不能写成正常运行或自动释放锁。V01/S04、父卡以及 phone/NAS/OPPO/Keystore/签名发布门槛仍保留。

registry 仅 root 写，既有 dirty 未 stage/覆盖；用户 `.directory` 未动。源规划树不 reset/clean；无 push、部署、新聊天或额外 agent。仅本报告和随后 checkpoint 显式 stage、独立提交，交接后释放 main index。
