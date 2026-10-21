# RelayNest 最小 MoviePilot 探针

此探针验证 `channel=None + 独立 source` 的模块分发和受控文本入站。0.0.1 已验证模块入口，0.0.2 已在用户现有 MoviePilot v3.1.5 主进程验证固定 `/noai` 输入及原生定向提示回复。每轮测试后均卸载并移除了远端源码。不作为正式插件发布，不代表 P1 完整双向验收通过。

0.0.3 增加原生媒体／种子候选控制测试：合成候选首屏、翻页、越界输入和退出共 16 项通过；同 userid 跨 source 会覆盖会话，原生空 channel 回调返回 AttributeError。详见 [候选执行报告](../../../docs/tasks/reports/P01-04-implementation.md)。

## 能力边界

- 无第三方新增依赖；使用 MP 自带 SDK、消息模型和标准库。
- 只处理 source 为 `RelayNest-P1Probe:A/B`、对应固定 userid、固定正文、`save_history=False` 且 channel 为空的合成消息。
- 不监听 NoticeMessage、不启动后台线程、不接 Hub、不处理任意用户输入。文本测试只原样调用 `MessageChain.handle_message(text="/noai")`，不替换宿主核心、权限或事件实现。
- 基础模块 probe 的两类候选只传空列表；candidate-probe 在内存播种 9 条合成媒体／种子候选，由原生渲染及交互控制处理。直接发送 ID 和编辑结果由内存消息桩生成，没有发送到真实设备。
- 不修改 NotificationChannel、能力表或 MP 核心，不借用 QQ/WebAgent。
- v3.0.0 仅准备旧基类导入回退，未在该版本运行；目前没有市场版本兼容承诺。

## 本地检查

在 RelayNest 仓库根执行：

```sh
PYTHONDONTWRITEBYTECODE=1 python integrations/moviepilot/poc/verify_probe.py
```

该脚本使用宿主桩验证拒绝边界和状态清理。真实 MP 的模块证据见 [模块执行报告](../../../docs/tasks/reports/P01-02-implementation.md)，原生文本证据见 [文本执行报告](../../../docs/tasks/reports/P01-03-implementation.md)。

## 在当前 fnOS 实例复现

以下操作只针对新目录 `relaynestprobe`。若目录已存在，应先确认归属，不能覆盖已有文件。所有路径和容器名均针对本次环境；不要使用市场安装接口下载不存在的插件。

先建立一个全新的插件目录，再复制唯一源码文件：

```sh
ssh -F /home/lijx/.ssh/config fnos 'docker exec moviepilot-v2 mkdir /app/app/plugins/relaynestprobe'
ssh -F /home/lijx/.ssh/config fnos \
  'docker exec -i moviepilot-v2 sh -c "cat > /app/app/plugins/relaynestprobe/__init__.py"' \
  < integrations/moviepilot/poc/relaynestprobe/__init__.py
```

目录创建失败时停止，不能继续复制命令。测试时实际采用了目标不存在及文件哈希校验，未覆盖旧文件。

使用下列命令，依次把末尾动作替换为 `enable`、`load`、`status`、`probe`：

```sh
ssh -F /home/lijx/.ssh/config fnos \
  'docker exec -i -e PYTHONDONTWRITEBYTECODE=1 -w /app moviepilot-v2 /opt/venv/bin/python -B - enable' \
  < integrations/moviepilot/poc/probe_api.py
```

v3.1.5 要求先通过官方 API 设置插件本体启用位，再重载以注册 API；仅放源码并调用重载会报告加载失败。`probe_api.py` 只允许固定探针路径，从容器内现有配置读取 API_TOKEN，通过 loopback 请求头使用，不输出凭据。无凭据访问探针 API 实测返回 HTTP 401。

如需复现本轮文本测试，先使用 `text-preflight`，再使用 `text-probe`。传播检查会拒绝未知的活动 NoticeMessage 监听器、其它 post_message 插件 provider 和同名通知配置。唯一经过源码核对的内置 WebAgent 桥，只有在其真实提取函数拒绝本次 channel=None 消息时才被排除；不禁用任何现有监听器。

`text-probe` 没有自定义输入参数，只运行两次固定 `/noai`，显式使用非渠道管理员身份。MP 原生流程会记录两条测试输入历史；同一探针实例的重复请求返回缓存，不再入站。重载会清除缓存，所以不要为了查看结果反复重载后运行文本测试。测试历史和 MP 正常保留的停用元数据不会被探针直接删除。

`candidate-probe` 同样没有自定义输入，先执行传播检查，再播种独立测试用户的内存会话，交错发送固定 `/noai 999999`、`/noai n`、`/noai q`。明确禁止 `0` 和有效序号：种子阶段的 0 是自动下载。每个阶段 A/B 各 3 条输入，本轮留下 12 条测试历史；重复调用只返回缓存。所有自有 request_id 在 finally 中清理，绝不清空宿主全部交互状态。

候选 endpoint 的 success 只表示列表和控制用例通过；须单独查看 callback 字段。本轮 callback 为 supported=false、returned_without_error=false、error_type=AttributeError。没有执行有效候选对应的搜索、订阅或下载，也没有把候选控制通过记为完整选择执行成功。

测试结束依次执行 `disable`、`status`、`unload`。停用后的 status 应显示 `enabled=false`、messages 为 0；卸载后 status 返回 HTTP 404。MP 按正常机制保留该插件的停用元数据，不直接删除业务数据库行。

最后核对远端目录文件内容及归属，只删除本次建立的 `/app/app/plugins/relaynestprobe`。本次删除前校验了 `__init__.py` 的 SHA-256，确认目录内只有该文件及 Python 缓存，再删除整个探针目录。不要修改 `/config/app.env`、通知配置、安装清单或其他插件目录。

## 如何解读成功结果

`success=true` 表示两组固定 source 均命中了普通消息、两个候选模块入口、直接发送以及手动编辑 provider。`native_callbacks=false`、`native_editing=false` 是仍未解决的宿主能力门槛。

模块 probe 的成功不能替代文本入站、业务权限、真实候选选择、按钮回调、主动广播、多 Bot 会话、真实附件、通知去重或容器重启后的恢复验证。

`text-probe` 的 `success=true` 仅表示固定原生输入在两组独立测试身份下各获得一次 source/user 正确的提示回复。本轮 chat_context_preserved 和 message_context_preserved 均为 false；插件尚未补全这些上下文。它不能代表任意用户消息、实际业务命令、权限映射、群会话或引用回复均可用。
