# P04-05-S03-F01 修复实施报告

- 角色：修复 agent；对应发现 `P04-05-S03-A01-F01`（P2）。结论：implemented，待不同实现者最终组合 A02；不自行关闭发现或修改台账。
- 实施基线：`775dde33d6800cf2cde26f6b1c1b8836307dce21`；完整业务审查基线仍为 `79c4601deb71d5865e3539ddc7da7f138737ee15`。
- 工作树：`/home/lijx/Projects/RelayNest-recovery-20261024/device-admin`；分支 `codex/device-admin-recovery-20261024`。
- 恢复起点仅 `auth/request_security.go` 和 `devices/http.go` 存在前次中断未提交修改；核对后继续该实现。最终实现 SHA 随独立本地提交后交接，不在报告内预写自身 SHA。

## 最小修复

冻结 DELETE 契约见 OpenAPI `x-error-codes` 401 REQUIRED。公共 `AuthorizeRequired` / `WriteSessionErrorRequired` 固定选 REQUIRED，既有入口固定选 FAILED；共享私有检查与错误分类。DELETE 请求检查和撤销事务错误两处使用新入口，GET/auth/Bot 保持旧入口。未改来源、凭据、CSRF、Inspect 顺序、SQL、存储事务、Cookie 或 no-store 规则；未改 API/README、依赖、registry 或根恢复报告。

改动限定五个授权 Go 文件和本报告。沿用真实 SQLite/mux 与 production 装配，不新增生产测试钩子、不截获 HTTP 正文改码。

## 回归与负面证据

新增 `TestDeviceHTTPRequiredCredentials` 使用有效目标 Session.id，覆盖无、空、格式错误、随机、无效 Cookie、重复 Bearer/Cookie、混合及真实 Bot API Token/Webhook Key。逐项对照 DELETE 401 REQUIRED 和 GET 401 FAILED，检查 no-store、无 Set-Cookie、会话快照不变、无 hook。

`TestDeviceHTTPRequiredLifecycleAndTransactionRecheck` 在真实库设置撤销、idle/absolute 到期、管理员代次变化。分别在请求之前及 Authorize 成功后读取受控请求体返回 EOF 时变更；后一种随后进入真实 RevokeDeviceSession 复核，全部401 REQUIRED。比较注入后的会话快照，排除将主动 actor 变更当作业务副作用；目标不撤销、无 hook/Cookie。关闭真实数据库分别覆盖 Inspect 与业务事务503，保持 SERVICE_UNAVAILABLE。原设备403/404、精确撤销、代理和 Cookie 测试保留。

公共测试将原完整拒绝矩阵同时施加两种入口，明确401选择、403 FORBIDDEN、503 SERVICE_UNAVAILABLE、错误信封及无副作用。另覆盖有效管理员、真实 Bot 两种凭据、ErrInvalidSession/ErrNotInitialized、包装错误和数据库错误映射。production 装配新增缺失/错误凭据以及真实 Bot 凭据的 DELETE/GET、auth session/logout、Bot 路由精确码，保留同安装多会话、分页、父会话隔离、重登旧ID、启动关闭重开等原矩阵。

旧版反例：临时 Go overlay 仅用 `git show 775dde3:server/internal/{auth/request_security.go,devices/http.go}` 替换两份生产源码；当前新增设备测试不变。命令 `go test -count=1 -overlay=<临时文件> ./internal/devices -run '^TestDeviceHTTPRequired'` 退出1，18处精确码失败，实际均 FAILED：10种入口凭据、4种请求前状态、4种授权后状态；数据库503检查不失败。完整临时日志在 `/tmp/relaynest-f01-old.log`，未入库敏感正文或凭据。保留原 A01 overlay 负面证据，本报告不覆盖旧报告。

初次运行的 overlay 路径误指向 server/server，未形成有效旧源码证据；随后校正绝对映射再运行。测试编写中 absolute 单列设0违反既有 idle<=absolute 检查，已改为同时到期；production 新测试误用 `/auth/current` 得404，已改为既有 `/auth/session`。这些失败均未计 PASS。最终旧版反例和新源码检查分别重新运行。

事务内部 tail/self-final/queued invalidation 使用既有 auth 真实受控时钟测试（含 `TestDeviceSessionRevocationCommitBoundaries`）随五包全量重跑；公共 REQUIRED 映射与 DELETE 的唯一事务错误出口另有断言。这里的 HTTP 延迟注入覆盖事务入口前失效，不声称实现了 HTTP 层事务尾检/自撤最终复核的独立时序注入。

## 实际检查

环境：mise 安装 Go 1.27.3，`GOTOOLCHAIN=local`，linux/amd64；SQLite 3.53.4（临时文件数据库）。现有 embed 资源可用，无需 npm ci/build。

- `go test -count=1 ./internal/devices ./internal/auth ./internal/httpapi ./cmd/relaynest ./internal/bots`：PASS，五包非缓存；随后 production 追加旧 auth/Bot 凭据对照后，`go test -count=1 ./cmd/relaynest` 再次 PASS。
- `go test -race -count=1 ./internal/devices ./internal/auth ./internal/httpapi ./cmd/relaynest ./internal/bots`：PASS；production 最终补充断言后 `go test -race -count=1 ./cmd/relaynest` 再次 PASS（48.007s）。
- `go vet ./internal/devices ./internal/auth ./internal/httpapi ./cmd/relaynest ./internal/bots`：PASS，最终断言修改后重新运行。
- 五个改动 Go 文件 gofmt：PASS；根目录 `git diff --check`：PASS。

普通/race/vet 输出分别保留临时 `/tmp/relaynest-f01-test.log`、`/tmp/relaynest-f01-race.log`、`/tmp/relaynest-f01-vet.log`；最终 cmd 输出 `/tmp/relaynest-f01-cmd-final.log`、`/tmp/relaynest-f01-cmd-race-final.log`。这些为本地复核辅助，交付结论不依赖日志永久存在。

## 交接与限制

A02 必须针对 `79c4601..最终实现SHA` 的完整源码组合，包含 `03b95101c3bf0671118aa54b1c70f7b56d9389ef`、`013823a684645755cb49557e103360c30b31d05e` 及本次公共 helper 修复；检查所有消费者和原父子验收，不能沿用旧 source audit PASS。最终 I 精确选提交，排除根恢复报告提交 `de9f2ca91536f442a4230a68dd653053e6524a17`，不整分支合入。

真实 WS/provider/Android 真机仍 NOT_RUN；未消费未冻结 G1 草稿 API/header。无 push、部署或用户可见新聊天，无新需求决定；遗留为最终 A02/I 与调度收口。
