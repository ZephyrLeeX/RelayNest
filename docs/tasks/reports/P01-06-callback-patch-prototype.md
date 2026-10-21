# 方案 A：插件内一键回调补丁管理原型

日期：2026-10-21。关联探针基线 `061c0b8`，门槛报告 `409101b`；提交前仓库HEAD `bcc42505b8a0335d328d1ac42f1f647c5e7759c4`。本轮是用户选择方案A后的补丁助手实现，不将原P01-06文档任务或G-MP改为通过，也不修改台账。

## 用户决定与本轮结果

用户明确选择“新增专用RelayNest渠道＋小范围MP核心补丁”，并提出在插件界面提供一键注入。用户另将完整上下文恢复及自动编辑列为可选项；按钮所属用户、Bot和交互请求的必要身份校验仍是callback正确处理的一部分，不能随之省略。

已完成本地补丁管理原型，供后续正式插件集成/现网安装前评审。界面有检查、应用、恢复三个按钮；有管理员鉴权、显式点击确认、精确版本/源码校验、备份、跨请求锁、重复应用与恢复能力。原型不自动应用或重启，不修改进程内类或能力字典。

**现有实例核心文件未修改，未安装本助手，未重启容器，真实按钮callback仍未通过验收。** 之前G-MP的技术结论仍有效；用户调整优先级后，也不能将尚未运行的完整链路记为PASS。

## 文件与职责

| 文件 | 职责 |
| --- | --- |
| integrations/moviepilot/poc/relaynestpatch/core_patch.py | 固定两文件的应用、检查、恢复和备份；无MP进程热重载 |
| integrations/moviepilot/poc/relaynestpatch/__init__.py | 独立RelayNestPatch插件助手原型，三个界面按钮及管理员API |
| integrations/moviepilot/poc/verify_patch.py | 临时夹具、故障恢复、拒绝边界、界面配置及鉴权依赖回归 |
| integrations/moviepilot/poc/README.md | 使用边界及本地验证入口 |

旧RelayNestProbe未改动；本助手没有消息provider、任意业务命令、后台线程或Hub连接。不是已经完成的RelayNest正式插件。

## 一键行为与限制

1. “检查补丁状态”只读源码/版本并读取当前进程能力，不创建备份或改核心。
2. “一键应用回调补丁”先由当前MP管理员确认，后端检查 `version.py` 的APP_VERSION为v3.1.5，以及两个目标的完整SHA-256；同时生成并编译校验两份补丁内容。
3. 所有原件先备份到插件持久数据路径 `relaynest-callback-v1/v3.1.5/`，写入prepared操作清单，再逐个原子替换目标并刷盘。文件路径固定，不接受请求传入路径、代码或命令。
4. 同版本原始源码或只包含本补丁的源码可以识别；重复点击不重复插入。未知版本、未知改动、符号链接、备份冲突拒绝写入；发现partial时禁止继续应用，先恢复。
5. “恢复原文件”仅恢复精确识别的原件/本补丁版本，不覆盖之后的用户修改。原文件内容由指纹验证；备份已损坏时拒绝恢复。
6. 应用前确保本进程已导入原枚举与能力模块，减少写盘后首次导入新能力表遇到旧枚举的风险；不执行模块reload。应用/恢复后页面提示重启MP。插件重载不能更新所有已经导入枚举类的消息模型，因此不实现热注入；助手不调用重启接口、shell或Docker socket。

磁盘original/patched/partial/blocked与 `runtime_callback_active` 分开显示。磁盘patched、进程false意味着待重启；恢复磁盘后进程仍可能true，也须重启。助手声明auth=bear，并在每个端点签名显式依赖MP的get_current_active_superuser；没有匿名或仅普通业务用户写核心的入口。

两文件整体不是原子事务。进程中断/第二文件写入失败会留下可识别的partial和备份，不能宣称已回滚或自动恢复；管理员应先点击恢复，确认完整状态后再重启。与MP升级同时修改源码、只读核心目录、进行中容器替换等运维场景未实测。

## 精确补丁

只改两处：

- `app/schemas/types.py::NotificationChannel` 新增 `RelayNest = "RelayNest"`。
- `app/schemas/notification.py::ChannelCapabilityManager._capabilities` 新增该渠道，capabilities仅INLINE_BUTTONS、CALLBACK_QUERIES，fallback_enabled=true；无MESSAGE_EDITING。

原始源码要求：

| 文件 | SHA-256 |
| --- | --- |
| app/schemas/types.py | `47c5a9f4a10d9350ac61ee64e31871880ba8571326b8df31237d4a748f543419` |
| app/schemas/notification.py | `a83d9e1123fea976e30dad3bd258e63813157d1d2373ac9074ed55335983b31d` |

源码内容已变化即使版本号没变也拒绝。市场最低目标v3.0.0仍未运行验证，本助手目前仅导入v3.1.5新SDK/鉴权路径；不能安装到v3.0.0后宣称受支持。后续每个MP版本需要核验并增加适配，按钮只是省去手工编辑，不代表万能跨版本补丁。

更新/重新创建容器可能还原核心文件；保留插件数据目录可保留备份，但不能让新镜像自动带补丁。升级后重新检查，只有已适配版本才能一键应用，然后重启。正常重启本身不一定丢失补丁，实际取决于MP更新/容器文件层是否重建或源码被覆盖。

## 接口查证

固定Frontend commit `21166c45fc1596af42978659ddf253b207b939cf`：`src/components/render/FormRender.vue`支持on事件字符串；`src/api/index.ts`将已登录认证客户端暴露为window.MoviePilotAPI。现有EmailMsg也使用VBtn.onclick调用插件POST接口。使用宿主现有表单渲染，不新增前端依赖。

实际宿主SDK的get_data_path为插件数据目录入口；插件API声明的bear认证之外又显式添加超级用户依赖。以上是源码查证及本地声明回归，尚未替代真实浏览器/权限集成测试。

## 实际验证

- `PYTHONDONTWRITEBYTECODE=1 python integrations/moviepilot/poc/verify_patch.py --source-root /tmp/relaynest-p1-research/3.1.5`：11项回归及固定真实源码副本应用/恢复通过。11项为只读inspect、重复应用/恢复、未知版本、同版本源码冲突、备份冲突、恢复拒绝用户后续修改、第二文件写入失败后的partial恢复、目标符号链接拒绝、version符号链接拒绝、权限位保留/无效操作拒绝、UI/管理员依赖声明。UI生成的3个JavaScript处理函数还在本地Node24中通过语法编译。
- `PYTHONDONTWRITEBYTECODE=1 python integrations/moviepilot/poc/verify_probe.py`：既有探针回归通过。
- 本轮在容器独立进程中，把实际types/notification/message/common及version复制至 `/tmp/relaynest-callback-copy-*`，只在副本运行本补丁管理器；新子进程导入副本后，3个实际消息模型都接受channel=RelayNest，buttons/callbacks=true、editing=false。随后恢复两目标，字节与/app原件一致，TemporaryDirectory退出后不存在。
- 上述临时副本脚本可由仓库中的验证器重新生成：`PYTHONDONTWRITEBYTECODE=1 python integrations/moviepilot/poc/verify_patch.py --emit-container-check > /tmp/relaynest-callback-copy-check.py`，不依赖/tmp永久保留。随后通过stdin执行：`ssh -F /home/lijx/.ssh/config fnos 'docker exec -i -e PYTHONDONTWRITEBYTECODE=1 -w /app moviepilot-v2 /opt/venv/bin/python -B -' < /tmp/relaynest-callback-copy-check.py`。脚本SHA-256 `763574f87ce5b57cec56943d569626fbbdb0ac0b9f198a48146d38ecc65a2d3f`。这是容器独立进程组件/模型验证，**不是修改主进程或原生callback实测**。
- 实际容器两目标SHA仍为上表原始指纹；容器running/healthy，StartedAt `2026-10-14T02:45:30.155912298Z`，未重启。远端临时副本目录无残留，没有修改MP业务数据库或正常用户数据。
- Python语法编译及git diff --check交付前通过。

本地源文件SHA-256：helper `0edd81c5ca36afacf45935a6e510ac35712ed1bcd7c1fc468bb22562d7e69e63`；manager `e850b2c9489d98f6831976d2ab63792802a73ba9d63c09c75b9518515915cb1c`。

## 后续部署与验收边界

下一步可安装助手到当前实例供管理员检查界面。应用两文件并安排一次MP重启后，才可验证主进程确实加载独立渠道，随后把受控消息探针适配到该渠道，测试合成媒体/种子按钮生成和翻页callback。用户此次要求的是便捷补丁管理；本轮将本地原型做成可审查结果，尚未代用户点击应用或中断现有服务。

NOT_RUN：真实助手安装/页面点击/实际管理员鉴权集成；现网核心应用与恢复；MP重启后能力加载；真实callback路由；错误用户/跨Bot/过期/重复按钮事件；有效业务选择；正式Hub及Android按钮链；v3.0.0；其它MP版本；独立审计。

原生callback不等于只打开两项能力就验收完成；现有探针仍只接受空channel，正式插件也尚未实现RelayNest channel的按钮转发。所有后续测试仍需MP负责业务权限及危险确认，Hub只透明传输通用交互事件。未推送或发布，未改台账、已有Android证据或.directory；无子代理或新聊天。
