"""独立的补丁助手原型；不接管业务、不自动修改核心或重启。"""

import importlib

from fastapi import Depends
from app.api.dependencies.auth import get_current_active_superuser
from app.api.principal import ApiPrincipal
from app.sdk.plugin import _PluginBase
from .core_patch import CorePatch


class RelayNestPatch(_PluginBase):
    plugin_name = "RelayNest 回调补丁助手"
    plugin_desc = "检查、应用和恢复已适配版本的文件补丁，重启后生效。"
    plugin_version = "0.0.1"
    plugin_author = "RelayNest"
    plugin_config_prefix = "relaynestpatch_"
    plugin_order = 100
    auth_level = 1

    def init_plugin(self, config=None):
        # 构造函数不检查/写入核心；不得在启动或保存配置时偷偷应用。
        self._patch = CorePatch("/app", self.get_data_path("RelayNestPatch"))

    def get_state(self):
        return True

    def get_api(self):
        return [{"path": "/patch/" + action, "endpoint": endpoint,
                 "methods": ["GET" if action == "status" else "POST"], "auth": "bear",
                 "summary": "RelayNest补丁" + action}
                for action, endpoint in (("status", self.patch_status), ("apply", self.patch_apply),
                                         ("restore", self.patch_restore))]

    def patch_status(self, _user: ApiPrincipal = Depends(get_current_active_superuser)):
        return {"success": True, "data": self._status()}

    def patch_apply(self, _user: ApiPrincipal = Depends(get_current_active_superuser)):
        return self._change("apply")

    def patch_restore(self, _user: ApiPrincipal = Depends(get_current_active_superuser)):
        return self._change("restore")

    def _change(self, action):
        if action == "apply":
            # 先缓存同一进程中的旧枚举和能力模块，避免写文件后才首次导入新表。
            # 这不是热重载；全部引用仍只能在重启后可靠切换。
            try:
                importlib.import_module("app.schemas.types")
                importlib.import_module("app.schemas.notification")
            except (ImportError, AttributeError):
                return {"success": False, "message": "当前进程模块状态异常，请先恢复原文件。",
                        "data": {"reason": "runtime_module_unavailable"}}
        result = self._patch.change(action)
        message = ("文件已应用，请重启MP后检查生效状态。" if action == "apply"
                   else "原文件已恢复，请重启MP后检查状态。") if result["success"] else "操作未完成，请先检查状态，确认前不要重启。"
        return {"success": result["success"], "message": message, "data": result}

    def _status(self):
        from app.schemas.notification import ChannelCapabilityManager
        from app.schemas.types import NotificationChannel
        channel = getattr(NotificationChannel, "RelayNest", None)
        result = self._patch.inspect()
        result["runtime_callback_active"] = bool(channel and ChannelCapabilityManager.supports_buttons(channel)
                                                 and ChannelCapabilityManager.supports_callbacks(channel))
        result["runtime_editing_active"] = bool(channel and ChannelCapabilityManager.supports_editing(channel))
        return result

    def get_form(self):
        """使用固定v3.1.5前端支持的VBtn onclick与已登录的MoviePilotAPI。"""
        forms = [{"component": "VAlert", "props": {"type": "info", "text":
                  "只适配已核对的v3.1.5。应用/恢复后需重启MP；插件重载不足以生效。升级后先检查。"}}]
        actions = (("status", "检查补丁状态", ""),
                   ("apply", "一键应用回调补丁", "将修改MP两个核心文件，备份后写入，重启后生效。继续？"),
                   ("restore", "恢复原文件", "将恢复已识别补丁的原文件，重启后生效。继续？"))
        for action, label, confirmation in actions:
            confirm_js = ("if (!window.confirm(" + repr(confirmation) + ")) return; ") if confirmation else ""
            method = "get" if action == "status" else "post"
            handler = ("function(e) { " + confirm_js
                       + "window.MoviePilotAPI." + method + "('plugin/RelayNestPatch/patch/" + action + "')"
                       + ".then(function(r) { if (r.disk_state) { "
                       + "var labels = {original:'尚未应用补丁',patched:'磁盘补丁已应用',partial:'补丁未完成，请先恢复原文件',blocked:'版本或源码不匹配，已停止'}; "
                       + "alert((labels[r.disk_state] || '状态未知') + '\\n' + (r.runtime_callback_active ? '当前进程回调能力已生效' : '当前进程回调能力未生效')); "
                       + "} else { alert('文件操作已完成，请重启MP后再检查状态。'); } })"
                       + ".catch(function() { alert('操作失败，请重新检查补丁状态。'); }); }")
            forms.append({"component": "VBtn", "props": {"color": "primary", "variant": "tonal", "onclick": handler},
                          "text": label})
        return forms, {}

    def get_page(self):
        return []

    def stop_service(self):
        # 无服务线程；停用不擅自恢复核心，待重启的补丁状态由管理员显式处理。
        pass
