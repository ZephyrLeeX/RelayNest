# P08-04-D01-M01 离线维护实现交接

- 角色 developer；结果 implemented（限本基线实现，等待固定 SHA 独审与真实 0006 集成组合验证）。
- 基线 `fad32a126a4c0b95dde20c5cdc7b358f8a0dd188`；工作树 `/home/lijx/Projects/RelayNest-recovery-20261024/instance-maintenance`，分支 `codex/instance-maintenance-20261025`。实施 SHA 在提交后交接，不预写自身 SHA。
- C01 冻结来源 `09deddba6ace42261c227b57254f8ae9ebbd1a2a`，main 组合 `f72a11c147269b4c81f0e319998292a3228a83f9`；沿用 S01 三 Tx API，不改 API/SQL/checksum/原 Open。
- SCOPE01 依据主树 `592ea2d39b377838f1476dacf6343e42c691b275` 所含范围卡/追加和 I01，精确组合 `2c5c3e8d2657875a5664eb0a4144ae38a6474f87`；仅采用其授权的最小 offline.go 及本版 HTTP 生命周期 flock。

## 实现与证据

四个主要 Go 文件：main.go 分派冻结命令并持 shared flock；instance.go 严格参数解析、同 inode exclusive flock、单 Store.Write 换 ID/推进代次/删全部 sessions；auth/instance_rotation.go 原子撤销辅助；storage/offline.go 只用 mode=rw 打开现存库，逐一核对当前全部嵌入迁移连续版本/name/checksum/数量并严格读取身份。无 CREATE/ALTER、migrate、enableWAL 或自动补台账。

退出码严格 0/2/3/4/1；expected/身份失败与随机、UPDATE、撤销、提交失败区分。SQLite BUSY/LOCKED 为 3。外部只给固定脱敏文本。提交后 Close 错误仍返回 0 并提示停服复核，不谎称回滚。密码、Bot 摘要、sync_epoch 保留。

新增 instance_test.go 使用真实临时 SQLite：Web/Android 会话由现有 SessionStore.Create 生成；恢复/克隆后旧 token 失效，正常原库搬迁保留 ID/会话。恢复测试在备份后撤销源会话，确保维护清除备份带回的旧状态。随机 Reader 故障、身份 UPDATE trigger、会话 DELETE trigger、deferred FK 导致真实 Commit 失败、写锁忙、代次 MaxInt64 均核对 ID/代次/会话/密码/Bot/epoch 完整回滚。

不存在、空库、缺 ledger、部分迁移、未知版本、name/checksum 不符、缺身份/损坏身份分别拒绝，并比较关闭连接的 DB 文件字节无变化。没有以重新正常 Open 自动迁移掩盖失败。

受控子进程执行生产 main/runInstance：在随机读取处暂停真实维护事务，硬链接别名的第二个维护退出 3、HTTP 启动拒绝；HTTP live=200 时维护退出 3，SIGTERM 排空退出后维护成功。锁只协调本版同机 inode；人工停全部进程/自动重启、隔离入口仍是必需条件，旧 admin/旧版本/他机不在自动证明范围。

## 实际检查

环境：WSL Linux amd64，固定 Go 1.27.3 `/home/lijx/.local/share/mise/installs/go/1.27.3/bin/go`，本树既有 Web embed 构建可用，无额外依赖、网络或部署。以下命令均在 server/ 实际执行，以下为本次真实结果：

- `go test ./cmd/relaynest ./internal/auth ./internal/storage -count=1`：PASS（15.455s/11.484s/0.608s）；包括既有认证/密码重置非回归。
- `go test -race ./cmd/relaynest ./internal/auth ./internal/storage -count=1`：PASS（56.354s/39.055s/5.283s）。
- 最后追加生产入口退出码 0/4/2 的子进程测试；`go test ./cmd/relaynest -run '^TestInstanceMainProcess$' -count=1` 与同命令加 `-race` 均 PASS（0.019s/2.126s）；之后 vet 重跑 PASS。
- `go vet ./cmd/relaynest ./internal/auth ./internal/storage`：PASS。
- `gofmt` 和 `git diff --check`：PASS。

## 推送门禁与未执行限制

本树真实迁移仅 0001_initial/0002_admin/0003_sessions/0004_bots/0005_instance_identity，生产无 push 注册映射/仓储。未虚构映射表，也未删除未来换 ID 必须同事务撤销全部实际映射要求。未来注册、待办和发送必须绑定实例及 auth_generation，并补恢复/克隆旧注册失效测试。

主树真实 `0006_messages.sql` 已有 `push_jobs(session_id,device_id)` → `sessions(id,device_id) ON DELETE CASCADE`。当前未纳入此工作树，未复制迁移、未 rebase，**真实 0006 组合回归 NOT_RUN，必须由最终 I 在精确组合重跑，不能仅靠本次 PASS 放行**。测试 fixture 已在实际表存在时填入真实 pending，恢复/克隆核对清空、正常搬迁保留、失败矩阵核对 pending 回滚；无表时状态标记 -1，不冒充已测级联。

未执行真实 NAS/现网维护、外部推送注册投递、Close/输出故障注入及非配合进程停机检测。没有 push/部署、派子 agent、改 registry 或其他任务文件。新接口为内部 OpenOffline、RevokeInstanceSessionsTx，冻结命令/三个 Tx API 不变。
