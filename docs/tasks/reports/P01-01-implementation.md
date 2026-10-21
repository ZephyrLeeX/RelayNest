# P1 MoviePilot 插件兼容性初步验证

日期：2026-10-21。RelayNest 父基线：`f1d4e56e80ccec3daf5e15a8cab145bdbefc3085`。本报告提交 SHA 见任务交接。

## 结论与范围

**当前 G-MP 不通过，完整双向纯插件兼容性仍为 BLOCKED。** 已复核现成插件，并在用户现有 v3.1.5 镜像中验证独立渠道的模型与能力限制；未找到同时满足独立身份、原生结构化 callback、原生编辑和不改核心的完整扩展路径。该结论不等于证明所有可能的插件设计均不可行。

用户明确选择在现有 fnOS / Docker Compose 实例上做最小验证，覆盖原计划要求隔离实例的执行环境选择。本次仅 SSH 只读检查和容器内独立 Python 进程的模型检查：未安装插件、未重启服务、未修改配置或业务数据库、未调用消息业务链或发送真实通知。未连接下载器、订阅、正式 Hub、Android、FCM 或 RelayShelf。

本报告是 P01-01 初步研究证据，未做独立审计或集成验收，不更新任务台账，不将 P01-02～06 标为完成。

## 固定研究基线

| 对象 | 固定 ref / 实际观察 |
| --- | --- |
| MoviePilot v3.0.0 | `773d8490645e5b7dd3c07a397e6e6d5ac58ef48e`，仅源码对照 |
| MoviePilot v3.1.5 | `99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3`，主要研究及容器模型检查基线 |
| MoviePilot-Plugins | `7b6c84ce506791c85c1160d4be8766c8763870f4`，NapCatMsg 1.5.0、EmailMsg 1.3.4、WPushMsg 1.0.0 |
| MoviePilot-Frontend v3.1.5 | `21166c45fc1596af42978659ddf253b207b939cf`，源码检查，未操作生产 UI 保存配置 |
| 现有容器 | `moviepilot-v2`，镜像 `jxxghp/moviepilot-v3:3.1.5`，版本文件中后端和前端声明均为 v3.1.5 |
| 实际镜像 ID | `sha256:d9511187e90725fa8c408ad296eb5f0dbfa8b262cadd6a06b0a49d1b40175d61`，这是本地镜像 ID，不冒充 registry RepoDigest |

容器关键源码与固定 v3.1.5 原始文件的 SHA-256 逐项相同：

| 文件 | SHA-256 |
| --- | --- |
| `app/schemas/types.py` | `47c5a9f4a10d9350ac61ee64e31871880ba8571326b8df31237d4a748f543419` |
| `app/schemas/message.py` | `047d6f96ac36658e6deb4cf1b8736c6f33825b33f64a8bcfeea14acd4562f82a` |
| `app/schemas/notification.py` | `a83d9e1123fea976e30dad3bd258e63813157d1d2373ac9074ed55335983b31d` |
| `app/chain/_messaging.py` | `a098417f1b7b6abbc1d46e397c0181d44dba78e05ed2cba5b64eeccff77983a2` |
| `app/chain/message.py` | `af8fe2c615cc55c6e983c68b0b4f052ad2d42b9e83a109f1658a94b7aa03b0bb` |

## 1. 渠道与自定义配置

[前端通知设置](https://github.com/jxxghp/MoviePilot-Frontend/blob/21166c45fc1596af42978659ddf253b207b939cf/src/views/setting/AccountSettingNotification.vue) 的添加选项包括 `custom`；保存调用 `notification/config`。[配置对话框](https://github.com/jxxghp/MoviePilot-Frontend/blob/21166c45fc1596af42978659ddf253b207b939cf/src/components/dialog/NotificationChannelInfoDialog.vue) 的自定义分支只提供 type、name 输入，没有通用 Hub URL、令牌和 Bot 配置编辑表单。不能把该入口当作已经具备完整第三方通知渠道配置。

[后端配置端点](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/api/endpoints/notification.py) 校验名称、保存 `NotificationConf`，不会自动扩展渠道枚举。配置 type 是字符串，与消息 channel 类型是两个不同契约。

[通知服务目录](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/application/service.py) 通过运行通知模块的 `get_instances()` 发现服务；[模块基类](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/modules/__init__.py) 根据服务名称匹配配置 type。插件可读取配置、提供发送方法，但保存 type 并不会自动产生独立消息身份或通知模块实例。

[渠道枚举](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/schemas/types.py#L485) 无 RelayNest；[消息模型](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/schemas/message.py) 的入站、出站和直接发送响应 channel 均使用该枚举。`source` 是字符串，可承载插件/配置标识，但不是注册渠道或赋予渠道能力的接口。

## 2. NapCat、文本与定向回复

[NapCatMsg 源码](https://github.com/jxxghp/MoviePilot-Plugins/blob/7b6c84ce506791c85c1160d4be8766c8763870f4/plugins.v3/napcatmsg/__init__.py#L120) 确认 `get_module()` 只声明 `post_message`、`post_medias_message`、`post_torrents_message`。插件使用自己的配置表单，不以“自定义通知保存”作为独立渠道注册证据。

`_handle_inbound()` 直接调用 `MessageChain().handle_message()`，参数包括 `channel=NotificationChannel.QQ`、`source="NapCat"`、userid、username、original_message_id 和 original_chat_id。该实例证明插件存在入站入口，不能证明 RelayNest 可以独立于 QQ 接入。

定向发送主要检查 QQ channel 和 userid，广播通过 NoticeMessage 处理；QQ 定向和已包含 QQ targets 的事件由广播入口跳过。其定向模块未进一步严格按 source 区分不同 Bot，群会话补全又依赖 QQ 用户的活跃群记录，因此不能原样作为双 Bot 隔离方案。斜杠命令事件保留 user/channel/source，但未携带 original_chat_id，插件用活跃群记录补偿；不应把该补偿当作可靠的原会话映射契约。

MP 的部分会话状态按 userid 建索引，单独改变 source 不能证明会话隔离。专用插件仍需设计稳定的 Bot/user/chat/message 映射，并实测两个 Bot；不得将合成 userid 直接当作 MP 管理员身份，或给所有输入设置 `is_channel_admin=true`。

## 3. 候选与 callback

[消息发送链](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/chain/_messaging.py#L512) 的媒体、种子候选分别调用专用模块发送，不经过普通 NoticeMessage 广播路径。NapCat 分别实现这两个模块方法，把候选格式化为数字列表；数字文本继续走原生文本交互，不能据此声称结构化按钮回调通过。

[消息入站链](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/chain/message.py#L253) 接收 `callback_data`，也兼容 `CALLBACK:` 文本前缀，但两者都进入 `supports_callbacks(channel)` 判断。换成文本前缀不能绕过渠道能力判断。QQ 的能力配置包含 callback；RelayNest 未注册，查询返回 false。业务处理和授权仍必须留给 MP。

## 4. 直接发送、编辑与附件

宿主已有 `send_direct_message → MessageResponse` 和 `edit_message` 模块契约，不能据此视作插件已实现。[Telegram 模块](https://github.com/jxxghp/MoviePilot/blob/99ee18d0e6a27422a4affd3d4ec7d8d56772e4c3/app/modules/telegram/module.py) 是返回 message_id、chat_id、channel、source 的具体实现实例；本轮未连接 Telegram 或执行真实发送/编辑。

NapCat 未声明直接发送或编辑 provider，`_post_onebot()` 只返回成功状态和错误描述，没有把 OneBot 的返回消息 ID 映射成 `MessageResponse`。其图片发送直接转交 image URL，没有提供 RelayNest 所需的附件字节上传、文件持久化及 ID 映射证据。[EmailMsg](https://github.com/jxxghp/MoviePilot-Plugins/blob/7b6c84ce506791c85c1160d4be8766c8763870f4/plugins.v3/emailmsg/__init__.py) 有图片下载后 CID 内嵌的转换实例，仍不能证明 Hub 图片/文件链路通过。

原生交互更新还先检查编辑能力；因此即使单独实现编辑 provider，也不等于原生业务会选择编辑原消息。

## 5. 完整性与去重

| 路径 | 已核实入口 | 当前证据限制 |
| --- | --- | --- |
| 同步／异步主动通知 | 路由状态机 → NoticeMessage → post_message | 未实测，不应把每个事件视为一条独立原始通知 |
| 定向回复 | 原生消息链 → post_message | NapCat 使用 QQ；RelayNest 独立身份未成立 |
| 媒体／种子候选 | 两个专用发送方法 | 仅事件监听会遗漏 |
| 直接发送 | send_direct_message | NapCat 未实现；宿主契约及 Telegram 实例存在 |
| 编辑 | edit_message，加业务侧能力判断 | NapCat 未实现；RelayNest 能力查询失败 |

NapCat 的广播与模块入口有互斥过滤，这是可借鉴的路径划分，不是持久去重或故障重放实测证据。[EmailMsg](https://github.com/jxxghp/MoviePilot-Plugins/blob/7b6c84ce506791c85c1160d4be8766c8763870f4/plugins.v3/emailmsg/__init__.py) 和 [WPushMsg](https://github.com/jxxghp/MoviePilot-Plugins/blob/7b6c84ce506791c85c1160d4be8766c8763870f4/plugins.v3/wpushmsg/__init__.py) 均主要监听无 channel 的通知事件，未发现比 NapCat 更接近完整双向专用适配的实现。

不得按正文哈希永久去重。必须先为五条路径明确唯一发送责任，再验证重复窗口、双 Bot 隔离及对其他渠道的影响。

## 6. 版本差异

对照两个固定 commit：`app/schemas/notification.py` 字节相同，不能据此推断通知行为相同。

- v3.1.5 的 `Message` 新增私有 `notification_route_scope`。管理员可使用渠道默认目标，用户范围要求个人目标；无类型消息默认走管理员范围，未知路由也回退管理员。v3.0.0 的默认路由行为不同。
- scope 是私有运行态属性，不在 `model_dump()` 中；`_build_notice_message_data()` 通过 model_dump 构造事件，scope 因此不会直接传给事件插件。事件监听插件不能忽略此差异后宣称已经完整遵守通知权限与范围。
- v3.1.5 的通知模块基类 `check_message()`、目标读取方法使用 scope，插件 provider 需要显式研究同等路由约束，不能仅复制 channel/targets 判断。
- 两版消息模型均依赖固定 NotificationChannel；入站 callback 的能力门槛均存在。版本差异没有解除独立身份的限制。
- 还有导入差异：当前 NapCat 使用 `from app.sdk.plugin import _PluginBase`；固定 v3.0.0 源码树没有该单数路径，旧基类位于 `app/plugins/__init__.py`，`app/sdk/plugins.py` 是另一个复数路径，兼容清单也没有 `app.sdk.plugin` 映射。这是不能直接照搬 NapCat 并声称 v3.0.0 可加载的源码证据；本轮没有在 v3.0.0 中运行导入测试。
- 插件市场的 NapCat `system_version=">=3.0.0"` 只是声明，不是运行证据。专用插件仍需单独验证两版导入、生命周期和模块 ABI。

最低适配目标和拟议市场声明保留 `>=3.0.0`；主要验收基线仍为 v3.1.5。本轮仅 v3.1.5 模型检查有运行证据，v3.0.0 没有运行实例，也不宣称未来所有版本兼容。

## 实际验证及复现

SSH 默认客户端遇到本机系统配置权限错误；显式使用用户已有配置 `ssh -F /home/lijx/.ssh/config fnos` 后连接成功。未修改系统 SSH 配置。读取容器名称、镜像、挂载和版本，未读取 Compose 凭据或应用令牌。

以下程序通过标准输入交给现有镜像的临时进程执行：

```sh
ssh -F /home/lijx/.ssh/config -o BatchMode=yes fnos \
  'docker exec -i -e PYTHONDONTWRITEBYTECODE=1 -w /app moviepilot-v2 /opt/venv/bin/python -B -' \
  < probe_models.py
```

`probe_models.py` 的检查逻辑如下，可在本地临时目录重建，无需安装插件：

```python
import json
from pydantic import ValidationError
from app.schemas.types import NotificationChannel
from app.schemas.message import IncomingMessage, Message, MessageResponse
from app.schemas.notification import ChannelCapabilityManager

result = {"channel_registered": any(c.value == "RelayNest" for c in NotificationChannel)}
for model in (IncomingMessage, Message, MessageResponse):
    try:
        model(channel="RelayNest", source="RelayNest-P1")
        result[model.__name__] = "accepted"
    except ValidationError as exc:
        result[model.__name__] = [
            {"loc": list(e["loc"]), "type": e["type"]}
            for e in exc.errors(include_input=False, include_url=False)
        ]
result["callbacks"] = ChannelCapabilityManager.supports_callbacks("RelayNest")
result["editing"] = ChannelCapabilityManager.supports_editing("RelayNest")
result["source_without_channel"] = Message(
    source="RelayNest-P1", userid="p1-bot-a-user-1", save_history=False
).model_dump(include={"channel", "source", "userid"})
message = Message(save_history=False)
message.set_notification_route_scope("user")
result["scope_runtime"] = message.notification_route_scope
result["scope_serialized"] = "notification_route_scope" in message.model_dump()
print(json.dumps(result, ensure_ascii=False))
```

实际执行退出码 0：

| 检查 | 结果 |
| --- | --- |
| RelayNest 枚举成员 | false |
| 三种消息模型使用 RelayNest channel | 均为 `channel` 字段 `enum` 校验错误 |
| RelayNest callback / editing 能力 | false / false |
| 只设置 source、不设置 channel | Message 可构造；不代表消息链双向可用 |
| scope 运行态值／是否出现在序列化字段 | user / false |
| 五个关键源码文件 SHA-256 | 与固定 v3.1.5 一致 |

收尾检查：容器仍为 `running / healthy`，StartedAt 为 `2026-10-14T02:45:30.155912298Z`，本轮没有重启。报告内 Python 示例通过 compile 语法检查，五个文件哈希与报告一致，两个版本的能力模型文件逐字节相同；`git diff --check` 通过。没有执行业务测试套件，本轮无业务实现变更。

## 未运行与后续决策

以下全部 **NOT_RUN**：配置 UI 保存和重启保留、加载真实 RelayNest 插件、实际主动通知、真实文本入站与定向回复、媒体/种子候选、结构化 callback、引用、直接发送 ID、编辑、图片/文件上传、双 Bot 并发、生命周期、重放和多路径去重。模型检查成功不能替代这些验收。

当前应先解决独立渠道注册与能力扩展契约，再向现用主进程加载插件。可继续研究 source-only 的文本路径，但它不能满足已经确认的结构化 callback／编辑门槛，也不能据此豁免 AC-07。若最终需要核心渠道扩展、全局能力表修改、枚举猴子补丁或借用 QQ/WebAgent，应先提交具体最小方案及维护成本，等待用户决定；本次未实施任何此类变更。
