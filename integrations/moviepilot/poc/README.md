# RelayNest 最小 MoviePilot 探针

此探针验证 `channel=None + 独立 source` 的模块分发和受控文本入站。0.0.1 已验证模块入口，0.0.2 已在用户现有 MoviePilot v3.1.5 主进程验证固定 `/noai` 输入及原生定向提示回复。每轮测试后均卸载并移除了远端源码。不作为正式插件发布，不代表 P1 完整双向验收通过。

0.0.3 增加原生媒体／种子候选控制测试：合成候选首屏、翻页、越界输入和退出共 16 项通过；同 userid 跨 source 会覆盖会话，原生空 channel 回调返回 AttributeError。详见 [候选执行报告](../../../docs/tasks/reports/P01-04-implementation.md)。

0.0.4 增加 `transport-probe`：MP 主进程经真实发送方法向临时 loopback HTTP 接收桩传输，返回 ID、同消息编辑、夹具图片/文件字节、定向同步/异步通知及原生回复/候选路径检查通过。它是协议桩，不是正式 Hub 或 Android；广播、持久重放、容器重启仍未运行。完整结果及失败轮次见 [P01-05 报告](../../../docs/tasks/reports/P01-05-implementation.md)。

## 能力边界

- 无第三方新增依赖；使用 MP 自带 SDK、消息模型和标准库。
- 常规探针只处理 source 为 `RelayNest-P1Probe:A/B`、固定身份和正文、`save_history=False` 且 channel 为空的合成消息；0.0.5 的 callback-probe 另只接受已打补丁的 RelayNest 渠道及自有候选夹具。
- 不监听 NoticeMessage；常规探针不启动后台线程，transport-probe 仅在请求期间启动临时 HTTP 服务线程并在 finally 关闭；不接正式 Hub、不处理任意用户输入。文本测试只原样调用 `MessageChain.handle_message(text="/noai")`，不替换宿主核心、权限或事件实现。
- 基础模块 probe 的两类候选只传空列表；candidate-probe 在内存播种 9 条合成媒体／种子候选，由原生渲染及交互控制处理。旧 probe 的直接发送 ID 和编辑仍为内存桩；transport-probe 返回临时 HTTP 接收桩生成的 ID 并更新其内存行，没有发送到真实设备。
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

先建立一个全新的插件目录，再复制两个源码文件（均须使用本地同版本文件）：

```sh
ssh -F /home/lijx/.ssh/config fnos 'docker exec moviepilot-v2 mkdir /app/app/plugins/relaynestprobe'
ssh -F /home/lijx/.ssh/config fnos \
  'docker exec -i moviepilot-v2 sh -c "cat > /app/app/plugins/relaynestprobe/__init__.py"' \
  < integrations/moviepilot/poc/relaynestprobe/__init__.py
```

目录创建失败时停止，不能继续复制命令。测试时实际采用了目标不存在及文件哈希校验，未覆盖旧文件。

另用相同方式将本地 `integrations/moviepilot/poc/relaynestprobe/transport.py` 复制到新目录内的 `transport.py`。加载前校验两文件 SHA-256；不能仅部署 `__init__.py`。

使用下列命令，依次把末尾动作替换为 `enable`、`load`、`status`、`probe`：

```sh
ssh -F /home/lijx/.ssh/config fnos \
  'docker exec -i -e PYTHONDONTWRITEBYTECODE=1 -w /app moviepilot-v2 /opt/venv/bin/python -B - enable' \
  < integrations/moviepilot/poc/probe_api.py
```

v3.1.5 要求先通过官方 API 设置插件本体启用位，再重载以注册 API；仅放源码并调用重载会报告加载失败。`probe_api.py` 只允许固定探针路径，从容器内现有配置读取 API_TOKEN，通过 loopback 请求头使用，不输出凭据。无凭据访问探针 API 实测返回 HTTP 401。

如需复现本轮文本测试，先使用 `text-preflight`，再使用 `text-probe`。传播检查会拒绝未知的活动 NoticeMessage 监听器、其它五类发送/编辑插件 provider 和同名通知配置。唯一经过源码核对的内置 WebAgent 桥，只有在其真实提取函数拒绝本次 channel=None 消息时才被排除；不禁用任何现有监听器。

`text-probe` 没有自定义输入参数，只运行两次固定 `/noai`，显式使用非渠道管理员身份。MP 原生流程会记录两条测试输入历史；同一探针实例的重复请求返回缓存，不再入站。重载会清除缓存，所以不要为了查看结果反复重载后运行文本测试。测试历史和 MP 正常保留的停用元数据不会被探针直接删除。

`candidate-probe` 同样没有自定义输入，先执行传播检查，再播种独立测试用户的内存会话，交错发送固定 `/noai 999999`、`/noai n`、`/noai q`。明确禁止 `0` 和有效序号：种子阶段的 0 是自动下载。每个阶段 A/B 各 3 条输入，本轮留下 12 条测试历史；重复调用只返回缓存。所有自有 request_id 在 finally 中清理，绝不清空宿主全部交互状态。

候选 endpoint 的 success 只表示列表和控制用例通过；须单独查看 callback 字段。本轮 callback 为 supported=false、returned_without_error=false、error_type=AttributeError。没有执行有效候选对应的搜索、订阅或下载，也没有把候选控制通过记为完整选择执行成功。

测试结束依次执行 `disable`、`status`、`unload`。停用后的 status 应显示 `enabled=false`、messages 为 0；卸载后 status 返回 HTTP 404。MP 按正常机制保留该插件的停用元数据，不直接删除业务数据库行。

最后核对远端目录文件内容及归属，只删除本次建立的 `/app/app/plugins/relaynestprobe`。本次删除前校验了两个源码文件的 SHA-256，确认目录内只有本轮两个源码文件及 Python 缓存，再删除整个探针目录。不要修改 `/config/app.env`、通知配置、安装清单或其他插件目录。

## 如何解读成功结果

`success=true` 表示两组固定 source 均命中了普通消息、两个候选模块入口、直接发送以及手动编辑 provider。`native_callbacks=false`、`native_editing=false` 是仍未解决的宿主能力门槛。

模块 probe 的成功不能替代文本入站、业务权限、真实候选选择、按钮回调、主动广播、多 Bot 会话、真实附件、通知去重或容器重启后的恢复验证。

`text-probe` 的 `success=true` 仅表示固定原生输入在两组独立测试身份下各获得一次 source/user 正确的提示回复。本轮 chat_context_preserved 和 message_context_preserved 均为 false；插件尚未补全这些上下文。它不能代表任意用户消息、实际业务命令、权限映射、群会话或引用回复均可用。

## P01-05 最小传输复现

启用并加载全新探针实例后，先 `text-preflight`，再运行 `transport-probe`，不要先运行 text-probe 或 candidate-probe（旧缓存没有传输证据，入口会拒绝）。客户端动作固定，无用户输入参数。常规 probe 仍只用旧内存桩。

transport-probe 只绑定随机 loopback 端口，临时目录为 `/tmp/relaynest-p105-*`，请求结束关闭端口、join 服务线程并删除夹具。它先运行两个 Bot 各一次同步和异步的**明确目标通知**，再直接发送含固定图片和文件的消息、编辑原 ID，复用既有原生文本和候选控制探针。图片仅允许本轮桩生成的精确 URL，文件仅允许本轮创建的精确路径，不下载任意 URL 或读取用户文件。接收桩只接受固定测试 source 和两种测试 userid 命名空间。

预期创建 24 行、编辑 2 次、显式同键重放 1 次；同正文不同键保留不同 ID。键由本轮 run ID 和发送序号产生，只有同一已构造载荷的重传复用该键，不能将它描述成 MP 稳定事件键或持久幂等。附件哈希只检查字节完整性。候选只发送类型、标题、正文、目标和候选数量，不是完整候选对象协议。

同一实例再请求 transport-probe 返回缓存，不重复入站。重载清空缓存，重载后不要为查看结果再次执行原生测试。收尾沿用 disable → status 清空 → unload → status 404 → 哈希/目录校验 → 删除源码，并检查临时目录不存在、容器 healthy、StartedAt 和原有插件清单不变。测试历史保留，不手工清除数据库行。进行中并发停用/重载、故障恢复及重启均未验证。

## 回调补丁助手原型（方案 A）

用户已选择独立 RelayNest 渠道的文件补丁方案；上下文恢复、自动编辑作为可选项。新增独立目录 `relaynestpatch`，类名 `RelayNestPatch`，版本0.0.1。它只有补丁管理界面，不是正式消息插件；旧探针的channel=None限制和既有结果保持原样。

插件配置页提供“检查补丁状态”“一键应用回调补丁”“恢复原文件”三个按钮。打开配置或保存配置不会自动改核心。写操作使用已登录管理员身份，点击后确认；应用和恢复均须重启MP进程后再检查，插件reload不足以生效。助手不执行重启。

首版仅支持 `APP_VERSION='v3.1.5'` 且 `app/schemas/types.py`、`notification.py` 与已核对SHA-256一致的实例。只新增RelayNest枚举以及按钮/callback两项能力，不开启自动编辑。未知版本、第三方源码修改、符号链接或备份冲突均拒绝覆盖；重复应用不会重复插入。备份和操作清单按补丁ID/MP版本保存于宿主分配的插件数据目录，请保持该目录持久化。

两文件逐个原子替换，整体不具备跨文件事务；操作中断可能显示partial，此时先恢复，确认状态前不要重启。恢复只处理已识别的原文件/本补丁，不覆盖用户后续修改。容器重建可能清除核心文件补丁，升级后先点击检查；新MP版本须先更新已验证的适配指纹，不能保证任意升级后都能直接套用。

本地验证（只写临时目录）：

```sh
PYTHONDONTWRITEBYTECODE=1 python integrations/moviepilot/poc/verify_patch.py
# 可选：对固定研究源码副本核对真实指纹及应用/恢复；源目录只读
PYTHONDONTWRITEBYTECODE=1 python integrations/moviepilot/poc/verify_patch.py --source-root /tmp/relaynest-p1-research/3.1.5
```

原型阶段的实现记录见[补丁助手报告](../../../docs/tasks/reports/P01-06-callback-patch-prototype.md)。后续已获用户授权部署并应用，MP重启恢复后原生callback最小检查通过，详见[实际部署与回调报告](../../../docs/tasks/reports/P01-06-callback-live.md)。真实浏览器点击及正式Hub/Android链路仍未验证。


## 已部署助手的持久本地仓库

当前实例的 `/config/relaynest-local-plugins` 是本助手的本地仓库，根目录使用源码中的 `relaynestpatch/package.v3.json`，源文件放 `plugins.v3/relaynestpatch/`。清单只声明 `==3.1.5`，不等于正式RelayNest插件的最低版本承诺。

本轮通过官方 `/system/env` 仅追加 `PLUGIN_LOCAL_REPO_PATHS` 中这一条路径，随后通过官方插件来源/安装接口登记；原有8个安装ID保留，新增RelayNestPatch。只有 enable/load 会遗漏正式安装登记，重启后API可能不存在；不能把reload成功当作持久安装。`probe_api.py` 提供固定的 patch-register-local、patch-source、patch-install、patch-form、patch-status 动作；patch-install使用该宿主实际要求的GET安装接口。调用前先确认本地候选唯一且可信，不直接编辑MP数据库或伪造市场源。

本地仓库与备份必须保留在持久配置卷。登记后的再次重启/容器替换尚未复测，不将其写成完整升级恢复保证。

## 已打补丁渠道的callback-probe

0.0.5新增callback-probe：部署 `relaynestprobe` 的三个Python文件，在新加载实例调用固定端点。只从本轮原生媒体/种子按钮中使用page-next、page-prev；明确拒绝select、download（含0）、外部request_id和其它动作。两组命名空间userid、两种phase各检查首屏按钮、错误用户、跨Bot请求、下一页、重复下一页、上一页、失效请求，共28项。按钮完整数组会发送到临时HTTP桩；回调由探针调用原生handle_message提交同一按钮数据，不代表Android/Hub入站事件已验证。

重复请求返回本轮缓存；每轮finally仅移除自有request_id、关闭临时HTTP服务。结束仍须disable→确认状态清空→unload→确认404→按源码哈希删除探针目录。补丁助手是用户授权的常驻安装，不随临时探针一起删除。

后续在Docker宿主重启前先运行只读绑定源检查：

```sh
ssh -F /home/lijx/.ssh/config fnos 'python3 -' < integrations/moviepilot/poc/restart_preflight.py
```

它不读取容器环境或执行启停；发现绑定源缺失时非零退出。一次检查不能保证未来挂载始终可用。本轮重启因旧/netdisk源缺失中断，已按用户授权移除该绑定并恢复服务；当前运行于保留原文件层的本地恢复镜像，后续Compose重建可能回到原镜像，须再检查补丁状态。
