# P03-06 集成 I01

日期：2026-10-22；角色：integrator；结论：契约精确集成及本地组合校验 PASS。目标 `main`，工作树 `/home/lijx/Projects/RelayNest`，不代表运行时投递或事务通过。

## 精确提交映射

| 项目 | 原 SHA | main SHA |
| --- | --- | --- |
| 原实现基线 | `38d52c3bae74010167d4defe0e681e843d497fc8` | — |
| 本卡集成前 main | — | `701bbc60c5665d05d6b6ebbf167038e291a39096` |
| 实现 | `fd0f3cb9526637fa25d472ad801f995d88f0d426` | `5de8b58c2709348e35636c99a0d7d00a227441c5` |
| 独立审计 PASS | `2d2c633366742011a96e9cac5297c7b4bb30eafd` | `6171037585da3a0a951bf9ddc60f06d542eda11f` |

来源受管 api-delivery 工作树开始时干净。两次 cherry-pick 无冲突，未修改代码或Schema；最终 `server/api` 整树在原实现及main均为 `012104539161ac1397243ba74d009c0761fbf252`。实施报告、审计报告与来源逐文件diff为空；其他模块与CI内容不变，没有代码漂移，无需重审。证据：[P03-06-audit-01.md](P03-06-audit-01.md)。本报告独立提交，自身SHA在交接给出。

## 当前 main 实际校验

环境：Linux；既有 `/tmp/relaynest-p0305-f01-locked/bin/python`，Python 3.13.15，依赖未升级或重建；设置 `PYTHONDONTWRITEBYTECODE=1`。只读取当前契约、合成示例和固定前卡版本，不连接设备或业务服务。

| 检查 | 实际结果 |
| --- | --- |
| `python -m openapi_spec_validator server/api/openapi.yaml` | PASS，`server/api/openapi.yaml: OK` |
| `python server/api/validate.py` | PASS：61 Schema、136正例+233反例（369例）、23操作/权限声明 |
| 从当前实施报告最后Python代码块原样提取 `/tmp/relaynest-run3/p0306-integration-probe.py` 并实际执行 | PASS，前卡全部paths/components/servers解析相等，原18操作/44 Schema/238例逐项保留，两依赖文件字节相同 |
| 同一探针14类声明破坏变异，各在自动回收临时目录执行真实校验器 | PASS，14/14非零拒绝且无Traceback：Webhook领取、管理员ACK、send续约、retry缺CSRF、管理正文/未知字段、幂等键可选/缺重放、累计续约无界、缺ACK重放、claim无上界、数字代次、缺错误码、管理响应引用载荷 |
| 探针py_compile | PASS，pyc仅输出 `/tmp/relaynest-p0306-validate.pyc`，不入库 |
| Git API整树/报告blob及范围核对、`git diff --check` | PASS；实际变更仅四份API文件、实施/审计与本集成报告，不改锁/台账/其他模块 |

锁文件SHA-256：requirements.in=`4d56f1903c85ca444dde0966a7fd2702f15242b2cbc5f35eb1fcd273a0317d7c`；requirements.txt=`4e1992c8e79a37d1671671afd5e24f686837f719078ac7bdab06b5919231f184`。本次重跑的是上述结构、369例、14变异与非回归；没有把独立审计额外122探针记录冒称为本次重跑。

## callback 与后续实现交接

保留 T2.3 最小不可变callback快照：message_id/revision、button_id、callback_data、管理员/设备/时间，event_id位于领取条目；与包含sender=administrator完整Message快照的message.created事件区分。不增加父消息正文、Bot历史枚举或其他读取权限。

P05-11仍需验证点击时不可变参数与必要父消息/依赖；P05-06仍需验证未ACK/failed投递的父消息/附件引用保护、ACK事务释放引用、领取事件附件授权，不能把合法UUID当授权，也不能重投时从最新消息拼旧载荷。当前callback没有附件引用清单，尚不能证明“发现必要附件→授权下载→持久化→ACK”已闭环。若后续接入发现载荷缺口，应按实际最小需求交调度核对，不据本PASS擅自扩充历史权限。

NOT_RUN：真实HTTP receive/跨Bot权限与撤销竞态、Long Polling、数据库CAS并发领取、ACK丢回复与同租约事务恢复、renew预算/并发、退避、手动retry幂等/溢出/事务、实际管理响应脱敏、附件保护/清理及接入闭环。没有运行时实现，不把声明变异说成真实鉴权或事务测试；无设备、外部系统、发布或部署验证。

registry原用户及授权调度差异保留、不stage、不覆盖；`.directory`与已提交checkpoint摘要未变。未派额外agent，未推送/部署/发布；本卡不准备P03-07实现树，等待范围规划及新卡审计集成后由调度安排。
