# P1 最小可移除插件实测

日期：2026-10-21。RelayNest 父基线：`61d9dcc`。实际实现 SHA 见本地提交交接。

## 结论

**已在现有 MoviePilot v3.1.5 主进程加载探针并完成固定模块用例，随后停用、卸载及删除远端源码。** `channel=None + 独立 source` 可以经过真实宿主调度器进入插件的五个发送／编辑模块入口。这补充并收窄 P01-01 的结论：新渠道字符串的枚举校验失败，不代表无 channel 的插件模块路径不可用。

发送结果使用插件内存桩，媒体与种子候选为空列表。没有接入用户消息、业务命令或真实 Hub，因此不是完整双向测试；原生 callback 和自动编辑能力仍为 false。**G-MP 及完整 P01-02 验收仍未通过。** 本次是用户明确要求的最小探针，不把任务依赖、审计或全部生命周期门槛写成已完成。

## 范围与实现

- 新增 `integrations/moviepilot/poc/relaynestprobe/__init__.py`，类名 `RelayNestProbe`，版本 0.0.1。
- 新增宿主桩验证脚本、固定路径 API 客户端和复现说明。
- 声明 `post_message`、`post_medias_message`、`post_torrents_message`、`send_direct_message`、`edit_message`。非探针身份返回 None，不拦截其它消息。
- 只有 MP API 密钥保护的 status/probe 接口，没有任意输入或命令执行 API。
- 无轮询、线程、事件监听、插件自有数据库或新增依赖；内存记录可重复清空。仅使用 RLock 保护 API 并发访问。
- 不改变固定枚举和能力表；不修改 MP 核心，不占用已有渠道，不创建正式 Hub。

用户已明确授权在现有 fnOS / Docker Compose 实例进行最小测试，覆盖原计划要求独立实例的环境选择。本次未扩展到真实业务操作、FCM、Android 或 RelayShelf。

## 环境与操作

沿用 P01-01 已固定的 v3.1.5 commit、镜像和关键文件哈希。容器为 `moviepilot-v2`，镜像 `jxxghp/moviepilot-v3:3.1.5`。远端仅新增探针目录 `/app/app/plugins/relaynestprobe`，源码 SHA-256：`7e8ae2e99c37b85b3be737e9450c5aabfff6592d7c204a82d8370e7045037e00`。

通过 SSH 在容器内读取现有 API_TOKEN，并只用于 `127.0.0.1` 的请求头；未输出、落盘或修改该凭据。没有修改 app.env、Compose 或通知配置。

首次仅写入源码并调用 `POST /api/v1/plugin/reload/RelayNestProbe` 返回 HTTP 200、`success=false`，提示插件加载失败。容器独立进程的类导入和底层加载器均成功，但这不能替代主进程加载。继续核对实际运行源码，发现 `runtime/extensions/plugin/runtime.py` 的运行入口会拒绝未登记启用本体的插件。

随后走官方 API：

1. `POST /api/v1/plugin/instance/RelayNestProbe/enabled`，body 为 `{"enabled":true}`，成功。
2. `POST /api/v1/plugin/reload/RelayNestProbe`，成功；刷新动态 API 后 status 可访问。
3. GET status、POST probe，确认由现有主进程处理。
4. 再重载一次并运行 probe，核对无重复调用。
5. 官方启停 API 设置 enabled=false；status 确认内存清空。
6. `DELETE /api/v1/plugin/RelayNestProbe` 成功，status 返回 404。
7. 校验远端源码哈希及目录内容，仅删除本次探针目录。

启用和卸载操作通过 MP 自己的 API 写入插件管理元数据，未直接改数据库或原有插件配置。MP 正常卸载会保留本体停用记录；本次没有绕过正常机制直接删除该记录。

## 结果

| 验证项 | 实际结果 | 证据边界 |
| --- | --- | --- |
| 主进程加载 | 成功，reload 响应 success=true，status HTTP 200 | 不是容器另起进程冒充加载 |
| 两组 source | A/B 均命中各自 provider 记录 | 固定合成身份，不等于业务会话隔离已验收 |
| 普通消息模块 | 两组均 true | 直接 run_module，不经过主动通知路由或 NoticeMessage |
| 媒体／种子入口 | 两组均 true | 空列表，只测接口，不测真实候选内容和数字选择 |
| 直接发送 ID | 两组均返回属于自身 source 的桩 ID | 内存桩，不是 Hub 或外部渠道返回 ID |
| 手动编辑 provider | 两组均 true | 显式调用模块；不是原生业务自动选择编辑 |
| 外部 source 拒绝 | true；本地另测固定用户、正文、已有渠道及历史标记拒绝 | 不验证其他渠道的真实业务非回归 |
| 重载后再次测试 | success=true，仍为 10 次调用、2 个 source | 无重复 provider 命中；未测容器重启 |
| 原生 callback 能力 | false | 仍需解决 |
| 原生自动编辑能力 | false | 手动编辑接口可调用不改变能力判断 |
| 无密钥访问 status | HTTP 401 | 未开放无鉴权入口 |
| 停用 | status 显示 enabled=false、messages=0、calls=[] | 没有工作线程需要退出 |
| 卸载及删除源码 | 官方卸载成功、API 404、目录不存在 | 仍按 MP 机制保留停用元数据和正常操作日志 |
| 原有安装清单 | 测试前后 8 个插件 ID 和顺序一致 | 未修改安装清单或原有插件设置 |
| 容器状态 | running / healthy | StartedAt 仍为 `2026-10-14T02:45:30.155912298Z`，未重启 |

probe 的实际响应摘要：

```json
{
  "success": true,
  "kind": "module_dispatch_with_memory_stub",
  "results": [
    {"source": "RelayNest-P1Probe:A", "post_message": true, "media_entry": true, "torrent_entry": true, "direct_owned_id": true, "edit_entry": true},
    {"source": "RelayNest-P1Probe:B", "post_message": true, "media_entry": true, "torrent_entry": true, "direct_owned_id": true, "edit_entry": true}
  ],
  "foreign_source_rejected": true,
  "native_callbacks": false,
  "native_editing": false
}
```

## 检查与未验证项

本地 `PYTHONDONTWRITEBYTECODE=1 python integrations/moviepilot/poc/verify_probe.py` 通过：固定合成流程、重复运行、外部身份拒绝、跨 source 编辑拒绝、停用清理。该脚本使用宿主桩，不依赖 pytest，也不将桩验证当作真实 MP 行为。真实模块行为另由上述主进程 HTTP 探针记录支持。

全部新增 Python 文件通过语法编译检查；报告 JSON、探针源码哈希及 README 相对链接检查通过。仓库中的固定客户端另在卸载后运行 status，返回预期 HTTP 404、退出码 1；这表示接口已移除，不是客户端检查通过后仍有服务残留。

NOT_RUN：v3.0.0 运行兼容；“通知→添加→自定义”保存／重启保留；真实文本入站和原有业务回复；主动广播／用户通知路由；真实媒体／种子候选；结构化 callback；业务权限及危险确认；图片和文件；消息 ACK／重放／持久去重；实际 Hub／客户端；容器重启恢复；独立审计。

建议下一步只在探针内增加受控的原生文本链验证，先明确 NoticeMessage 对无 channel 回复的传播风险，以及 MP 身份／权限映射。不能直接把本探针的内存桩成功转成正式插件兼容性通过，也不因手动编辑成功而宣称按钮回调问题已解决。
