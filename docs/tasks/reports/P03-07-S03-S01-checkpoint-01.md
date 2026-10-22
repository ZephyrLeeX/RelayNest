# P03-07-S03-S01 实施前检查点

角色：developer。结果：needs_user / checkpoint，未完成实现，不作为验收或 G-API 放行证据。

## 基线与范围

- 派发基线：`42cd918c01c34bb7baa918420dad0daa2b4d6a98`。
- 独占工作树：`/home/lijx/.codex/worktrees/api-global-settings/RelayNest`；分支 `codex/api-global-settings`；开始时干净。
- 已核对设备三个集成节点 `876935a`、`82ebd62`、`6d93961` 及 S03 规划 `e6b0565` 均为基线祖先。没有改写前设备契约。
- 本轮仅新增本检查点；API 四文件、两锁文件、运行时、registry、设计、任务卡均未修改。未派 agent、push、部署或发布。

## 必须先确定的最小产品输入

PRD §3.4（77行）仅规定全局历史保留默认30天、管理员可修改；§3.6（100行）仅规定默认图片10MiB、文件50MiB、全局可调。T3.2（112行）规定 `expected_version`，T5.4（232～236行）规定后续清理与保护，均未定义以下允许范围：

1. 历史保留是否只允许正整数天；0是否有意义；是否支持无限保留。
2. 图片/文件大小是否只允许正整数 bytes；0是否代表禁用；是否存在产品最大值。

这些决定会改变可接受配置及清理行为，不能用普通请求体资源上限替代。已向调度返回具体缺口，等待决定。建议选项为：A，仅正整数天/bytes，不引入0或无限模式；B，明确支持特殊值及对应清理/上传政策后再冻结。这里仅列选项，不擅自选择。产品政策与技术上限分开：正数/0/无限及产品最小值由用户确认；安全时间计算、大小计算与存储可表示范围由后续技术角色提出确切上限、溢出规则及依据，经审计后冻结。既有 `Version` 只有 integer/minimum=1，无maximum，不是已经批准的产品配置或安全计算上限；本检查点没有确认任何数值范围。

## 可继续的独立准备

后续仅需基线 keys：`Version`、`UTCDateTime`、`Error`、`SyncChange`、`SyncPage`、`SyncDeviceNotifications`、`NotificationSettingsConflict` 及管理员路由共用安全声明。计划新增全局 settings/更新/CAS冲突/同步分支的具体命名由实施决定，不视为已冻结字段。

已确认约束：唯一管理员 Bearer/Web Cookie；Cookie 写必须同源与CSRF；CAS冲突返回最新版本并重新 GET 恢复，服务端生成版本/时间；全局设置同步恢复仍需管理权限，不携带任意配置对象或绕过授权。T6消息upsert/删除、Bot upsert/删除、已读、投递及前设备通知分支均保留，仅有意扩展全局设置分支。

P06-08清理交接依据：缩短只影响后续清理；以created_at计算、不因编辑/已读重置；删除事务复核pending/leased/retry_wait/failed及必要父消息/附件；仍有消息/投递/活动下载上传引用时不删附件；普通引用不永久保护父消息；离线设备/未读/FCM不阻止普通清理；保护记录无隐含TTL，压力拒绝写入，不自动丢弃。设置成功不承诺同步清理完成或恢复已经删除的数据。UI缩短期限必须显示明确影响。

## 实际验证

环境：Linux；`/tmp/relaynest-p0305-f01-locked/bin/python`；实际 `Python 3.13.15`，未安装/升级依赖。

- `.../python -m openapi_spec_validator server/api/openapi.yaml`：PASS，`server/api/openapi.yaml: OK`。
- `.../python server/api/validate.py`：PASS，100 schemas；450 valid +449 invalid =899 examples；34 operations/security。
- 六份API文件逐字节与派发基线 blob 比较：全部相等，故全部旧操作/组件/899例及两锁无差异。没有新增操作或例子。
- `git diff --check`：PASS（检查点写入前）；提交前再次执行。

未执行：本卡新Schema/正反例、CAS/隔离破坏探针（尚未实施）；真实HTTP、DB并发、清理、客户端及真机均 NOT_RUN。基线PASS不能替代本卡验收。

## 上下文退出与后续

初始批量读取的工具结果发生截断，未将其视为完整证据；关键PRD/T3/T5.4/T6/Version/Error原文另行定向读取。累计初始材料估算已触及或超过12k硬限，不宣称符合8k/12k预算，依WORKER_PROTOCOL §3退出，不继续扩大读取或实施。精确token计数未知。

下一轮最小输入：本检查点、调度确认的允许值决定、精确工作树HEAD/干净状态，随后定向读取管理员既有操作与validate入口，独立完成API四文件、本卡例子/声明探针、全部旧值非回归和报告。当前卡未完成，需同卡续派；不得跳到健康/状态/最终G-API索引。
