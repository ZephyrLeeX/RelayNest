# 方案 A 实例部署、重启恢复及原生 callback 实测

日期：2026-10-21。助手基线 `faae4ee`；提交前HEAD `879aaebffb1e312db78caac1978e4fea23b80d4d`（包含其它独立Android任务）。本轮只提交MoviePilot探针、助手本地仓库清单、验证及本报告；不修改台账或其它任务文件。

## 结论

**在用户授权的当前v3.1.5实例应用方案A后，主进程已具备独立RelayNest渠道的按钮/callback能力，媒体及种子原生翻页回调28项检查通过。** 原先空channel在原生callback入口受阻的问题，在这次显式核心文件补丁方案下得到限定运行证据。

助手已通过官方本地仓库入口正式安装，配置表单返回“检查补丁状态”“一键应用回调补丁”“恢复原文件”三个按钮；补丁已生效，自动编辑能力仍false。尚未验证真实浏览器点击及Android/正式Hub端到端；未执行有效选择、搜索、订阅或下载，不能声称所有业务按钮已通过。

重启发生过部署事故：既有/netdisk绑定源缺失导致Docker停止容器后无法启动。用户随后明确授权“移除这个挂载”，已仅移除该绑定并恢复MP为healthy。故本轮不再声称“未改部署、未重启、镜像和容器ID不变”。

## 授权和范围

用户先选择方案A，并把完整上下文恢复、自动编辑列为可选项；本轮明确允许安装助手、应用补丁并重启一次以验证callback。挂载故障后又单独授权移除/netdisk。没有把上述授权解释成允许升级MP、停用正常插件、清除数据、真实下载/订阅、推送代码或修改其它服务。

本轮使用显式SSH配置连接fnOS；API_TOKEN只从容器既有配置读入内存，通过容器loopback请求头使用，不打印或保存。没有直接修改MP数据库。未启动子代理或新聊天。

## 实际补丁应用与能力变化

1. 安装前确认原容器running/healthy，原StartedAt为`2026-10-14T02:45:30.155912298Z`，两个目标仍为固定原始指纹，助手和探针目录不存在；原安装清单8项。
2. 新建助手目录、复制并核对`__init__.py`和`core_patch.py`，通过官方启用/重载API加载。
3. helper status/apply无凭据访问均HTTP401；带现有API凭据的status成功。表单HTTP200、包含3个按钮；这不是浏览器实际点击证据。
4. 通过助手同一个POST patch/apply端点应用补丁，HTTP200/success=true。备份与manifest核对通过，operation=completed_apply，每份backup符合原指纹，目标文件符合manifest中的patched哈希。
5. 重启前status为disk_state=patched、runtime_callback_active=false。重建恢复服务并重新加载助手后，为disk_state=patched、runtime_callback_active=true、runtime_editing_active=false。

| 目标 | 原始SHA-256 | 当前补丁SHA-256 |
| --- | --- | --- |
| app/schemas/types.py | `47c5a9f4a10d9350ac61ee64e31871880ba8571326b8df31237d4a748f543419` | `b536fb434f26e45a43a6c173ee7648c19c9af70e0083717a8783812a57e4292c` |
| app/schemas/notification.py | `a83d9e1123fea976e30dad3bd258e63813157d1d2373ac9074ed55335983b31d` | `4503e3f57d355d329439be8253bdfc74662a22bcfd1c989d784cbca844b48286` |

只新增RelayNest枚举和INLINE_BUTTONS/CALLBACK_QUERIES。未改MessageChain业务实现、原生权限或确认流程，也未运行时热改枚举/能力字典。None渠道的能力限制没有因此被全局打开。

## 重启故障、影响与恢复

首次`docker restart --time 30 moviepilot-v2`已停止旧容器，但Docker报：

```text
Cannot restart container moviepilot-v2:
error while creating mount source path '/vol02/1000-1-12ecd637/media':
mkdir ...: operation not permitted
```

进一步只读检查发现：rclone根挂载存在且allow_other，但其media子目录不存在；root与普通用户stat都得到ENOENT。再次直接start同样失败。补丁备份和写入在停止前均已校验，此报错发生在Docker绑定源准备阶段，未进入MP应用启动。

这次重启前没有检查全部宿主绑定源，导致服务停止后才发现问题，是本轮运维预检缺口。新增`restart_preflight.py`：只读读取指定容器Mounts并逐一stat绑定源，不读取Env、不执行重启；本地缺失路径拒绝测试及恢复后3个真实绑定源检查均通过。

用户授权移除该挂载后，发现旧容器可写层包含插件/依赖安装与删除记录；直接从原镜像重建会丢失这些变化，因此未执行该破坏性重建。恢复过程：

- 保留旧文件层为本地恢复镜像；镜像配置中的全部运行Env值及标签值清空并校验，入口设为/bin/false，供文件层恢复，不作为含原凭据元数据的备份发布。配置/媒体等bind卷数据没有复制进恢复镜像。
- 修改`/vol1/1000/docker/media-stack/compose.yaml`，只删除`/vol02/1000-1-12ecd637/media:/netdisk`这一行。修改前后解析后的完整Compose结构比较，只有目标服务这一项volume删除；其它服务配置不变。
- 从恢复文件层创建新MP容器；原Config/HostConfig在内存中复用，移除/netdisk绑定并使用恢复镜像，保留原运行环境、端口、重启策略、网络和其它绑定。未拉取或升级镜像，实际MP版本仍v3.1.5。
- 新容器healthy、原有插件及文件层保留后清理旧的exited容器；保留本地恢复镜像供后续回退。

| 项目 | 实际值 |
| --- | --- |
| 旧容器ID | `23fee037592320c95c0d938f183974fd68d36c77590a8d8d4234b936563a378c` |
| 旧容器停止时间 | `2026-10-21T08:29:30.764408393Z`（北京时间16:29:30） |
| 新容器ID | `08a6046e4c274048a0f0bb0ac8048be46e2d0d291c387f73fd46a54612313be8` |
| 新StartedAt | `2026-10-21T08:40:14.136058624Z`（北京时间16:40:14） |
| 当前状态 | running / healthy |
| 原始镜像ID | `sha256:d9511187e90725fa8c408ad296eb5f0dbfa8b262cadd6a06b0a49d1b40175d61` |
| 当前本地恢复镜像 | `relaynest-local/moviepilot-recovery:20261021-callback` |
| 当前镜像ID | `sha256:2d41a349e3a8b72f02a6b82a29d3c74b1b63e5f1d21a8fdda0a568c45d1e94d1` |

停止到新容器启动约10分43秒；没有记录精确HTTP恢复时刻，不能将此间隔当作精确业务不可用时长。当前绑定保留/config、/moviepilot/.cloakbrowser、/media，/netdisk不再可用，依赖它的功能需后续重新配置正确路径。

Compose仍声明原`jxxghp/moviepilot-v3:3.1.5`，当前容器使用恢复快照，二者镜像身份不同。后续Compose重建/升级可能回到声明镜像并还原核心文件，应重新检查、在已适配版本上应用补丁；不把恢复快照写成已发布的正式衍生镜像。

## 原生callback调用链

```text
原生MediaInteractionManager创建自有合成会话
 → 原生MediaInteractionChain生成媒体/种子列表及按钮
 → 探针专用provider严格匹配RelayNest渠道、source、userid、夹具
 → 临时HTTP桩接收完整buttons数组及列表数量摘要

探针取得同一原生按钮的callback_data
 → 未修改的MessageChain.handle_message(channel=RelayNest, is_channel_admin=false)
 → 原生能力检查 → 原生InteractionRouter → 原生媒体交互处理器
 → 原生列表或边界/失效提示 → provider → 临时HTTP桩
```

回调输入由固定探针生成，不是Android/Hub提交的入站HTTP事件。白名单只有自有request_id的page-next/page-prev，拒绝select、download（含0）、外部请求ID及update等动作。没有直接绕过MessageChain调用底层业务handler。

传播预检对RelayNest渠道本身使用真实WebAgent提取函数，确认它拒绝本次消息；只有该已核实桥被排除。其它活动NoticeMessage handler、消息/候选/直接发送/编辑provider及source同名配置会阻止测试。最终均为空，仅保留原有WebAgent桥，未关闭它或正常插件。

## 最终28项结果

两种phase各9条合成候选，首屏8条、下一页1条；A/B使用各自命名空间userid，交错操作并逐步核对另一会话page不变。

| 检查 | media A/B | torrent A/B |
| --- | --- | --- |
| 首屏原生按钮，含next回调 | 通过/通过 | 通过/通过 |
| 错误userid提交自有请求 | 原生失效提示，原会话不变 | 同左 |
| 当前Bot身份提交另一Bot请求ID | 原生失效提示，两组会话不变 | 同左 |
| 原生next按钮 | 1条候选，page=1 | 同左 |
| 同一next再次提交 | 已是最后一页，page保持1 | 同左 |
| 原生previous按钮 | 8条候选，page=0 | 同左 |
| 移除自有会话后提交旧按钮 | 原生失效提示 | 同左 |

每组7项，合计28项；最终HTTP桩创建28行、error_type=null、自有会话全部移除、临时receiver关闭。自动编辑false，交互结果作为新消息发送。重复请求callback-probe返回逐字段相同的缓存，没有再次调用原生回调。

此跨Bot检查依赖独立命名空间userid，不代表宿主已经按source校验相同userid的请求。正式插件仍需绑定可信Bot/user/request身份。两页条件下重复next的边界通过，也不代表任意按钮、有效业务执行或持久事件重放具有幂等性。

首轮探针在完成10项检查后因自身遍历`buttons=None`的辅助代码触发TypeError；顶层success=false，已清理、卸载并删除该轮源码。修正为迭代`buttons or []`后全新加载复测，28项通过。没有把首轮失败写成MP业务失败，也没有隐藏该失败。回调分支源码不写普通用户输入历史，本轮未发送普通业务文本，也未用数据库行数验证“零新增历史”。

## 助手持久安装登记

最初只有enable/load，重启后助手API404；手动reload后API正常，但不在原8项安装清单中，不能认为已经完成持久安装。

核对宿主本地仓库机制后，新增持久目录`/config/relaynest-local-plugins`：根package.v3.json只含RelayNestPatch，版本0.0.1、system_version=`==3.1.5`；两个源文件位于plugins.v3/relaynestpatch。通过官方`POST /system/env`仅追加PLUGIN_LOCAL_REPO_PATHS，保留原有路径；未改其它环境配置或直接写app.env。

官方来源接口确认selected/local、库存完整，来源为`local://RelayNestPatch?version=v3`，随后官方安装成功。首次误用POST安装返回405，核对实际端点为GET后修正；405没有安装副作用。

最终安装清单为原有顺序8项加RelayNestPatch：P115OfflineDownloader、PluginUnInstall、RemoteIdentifiers、CategoryEditor、PersonMeta、BangumiColl、SubtitleManualUpload、P115StrmHelper、RelayNestPatch。助手实际/app载荷与持久本地仓库逐字节相同。未使用伪造在线市场、远程下载补丁或数据库写入登记。

登记完成后没有为复测再次重启MP；自动随启动装载/从原镜像重新部署恢复仍为NOT_RUN，不能只凭清单多一项宣称升级恢复已实测。

## 清理、检查与剩余范围

两轮回调测试均执行官方disable/status/unload；停用状态所有计数和结果清空；卸载后status404。按源码哈希及目录内容核对后删除临时relaynestprobe，临时HTTP夹具目录无残留。原生自有request_id在finally移除，没有清空宿主全部会话。用户授权保留的助手、补丁、备份和本地仓库不作为临时探针删除。

| 部署探针源码 | SHA-256 |
| --- | --- |
| __init__.py | `0105b9c5f766e82cdaab51e004e7a29d539a4b35b09ae5468d0f7837072fc2a1` |
| callback_probe.py（最终） | `b9bd524f60fa4c54fbbf80a690301a2bce55f1b59523ab6197951afdb77fb950` |
| transport.py | `8468f246f68040276d1ed9cf301e98838096dc5d2b826eea5863dc59111e4007` |

本地`verify_probe.py`（新增callback白名单/身份拒绝、清理、缺失挂载拒绝）、`verify_patch.py --source-root /tmp/relaynest-p1-research/3.1.5`（11项及真实源码副本）、Python语法和git diff --check通过。实际API应用/检查/表单/安装、重启恢复、28项主进程回调、缓存及卸载结果如上。

本轮对MP侧原生导航callback给出局部PASS，不自动将P01-06、P1/G-MP或AC-07改为通过。上下文恢复和自动编辑按用户新决定作为可选；仍需验证正式Hub/Android按钮链、有效业务选择及权限/危险确认、自定义配置绑定/广播、持久事件处理、v3.0.0和其它版本。真实浏览器点击、线上restore并重启、正式登记后的再启动、独立审计均NOT_RUN。

提交只包含本任务文件，不推送。用户既有registry.json、.directory及期间其它任务的提交/证据保留。
