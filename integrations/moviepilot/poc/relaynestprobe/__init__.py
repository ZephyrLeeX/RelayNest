"""只处理固定合成消息的可移除探针，不接入任何 MoviePilot 业务命令。"""

import threading
from uuid import uuid4

try:
    from app.sdk.plugin import _PluginBase
except ModuleNotFoundError as error:
    # v3.0.0 使用旧基类入口；不隐藏新 SDK 自身的依赖缺失。
    if error.name != "app.sdk.plugin":
        raise
    from app.plugins import _PluginBase

from app.schemas.message import Message, MessageResponse
from app.schemas.notification import ChannelCapabilityManager


class RelayNestProbe(_PluginBase):
    """探测 source 路由与模块接口；不注册完整 RelayNest 渠道。"""

    plugin_name = "RelayNest 最小探针"
    plugin_desc = "仅处理内置合成消息，不连接 Hub，不执行下载或订阅。"
    plugin_version = "0.0.1"
    plugin_author = "RelayNest"
    plugin_config_prefix = "relaynestprobe_"
    plugin_order = -100
    auth_level = 1
    _sources = ("RelayNest-P1Probe:A", "RelayNest-P1Probe:B")
    _text = "relaynest-p1probe:synthetic"

    def init_plugin(self, config=None):
        """仅建立内存状态，不创建线程、网络连接或插件数据库。"""
        self._lock = threading.RLock()
        self._enabled = bool((config or {}).get("enabled", True))
        self._records = {}
        self._calls = []

    def get_state(self):
        return self._enabled

    def get_form(self):
        return [
            {"component": "VSwitch", "props": {"model": "enabled", "label": "启用合成消息探针"}},
            {"component": "VAlert", "props": {"type": "info", "text": "仅验证模块接口；未接入用户消息或业务命令。"}},
        ], {"enabled": True}

    def get_page(self):
        return []

    def get_api(self):
        """所有入口均要求 MP API 密钥，POST 只运行内置固定用例。"""
        return [
            {"path": "/status", "endpoint": self.status, "methods": ["GET"], "auth": "apikey", "summary": "读取探针状态"},
            {"path": "/probe", "endpoint": self.probe, "methods": ["POST"], "auth": "apikey", "summary": "运行固定合成消息探针"},
        ]

    def get_module(self):
        """声明五个独立入口，未命中固定测试身份时一律返回 None。"""
        return {
            "post_message": self.post_message,
            "post_medias_message": self.post_medias_message,
            "post_torrents_message": self.post_torrents_message,
            "send_direct_message": self.send_direct_message,
            "edit_message": self.edit_message,
        }

    def _matches(self, message):
        """完整匹配固定合成身份，避免截获用户消息或其它渠道。"""
        return bool(
            self._enabled
            and message is not None
            and message.channel is None
            and message.source in self._sources
            and message.userid == f"{message.source}:user"
            and message.text == self._text
            and message.save_history is False
        )

    def _record(self, method, message):
        self._calls.append({"method": method, "source": message.source, "userid": message.userid})
        self._calls = self._calls[-24:]

    def post_message(self, message, **kwargs):
        if not self._matches(message):
            return None
        with self._lock:
            self._record("post_message", message)
        # 此固定合成消息已被消费，阻止 legacy 调度继续进入其它 provider。
        return True

    def post_medias_message(self, message, medias):
        if not self._matches(message) or medias:
            return None
        with self._lock:
            self._record("post_medias_message", message)
        return True

    def post_torrents_message(self, message, torrents):
        if not self._matches(message) or torrents:
            return None
        with self._lock:
            self._record("post_torrents_message", message)
        return True

    def send_direct_message(self, message):
        """返回内存消息桩 ID，不能被当作真实 Hub 发送成功证据。"""
        if not self._matches(message):
            return None
        with self._lock:
            message_id = f"relaynest-probe-{uuid4().hex}"
            self._records[message_id] = {"source": message.source, "edited": False}
            self._record("send_direct_message", message)
            return MessageResponse(
                channel=None, source=message.source, message_id=message_id,
                chat_id=message.userid, success=True,
            )

    def edit_message(self, channel, source, message_id, chat_id, text,
                     title=None, buttons=None, metadata=None):
        """只允许编辑本轮内存消息桩，不操作真实消息或外部系统。"""
        with self._lock:
            record = self._records.get(message_id)
            if not (
                self._enabled and channel is None and source in self._sources
                and record and record["source"] == source
                and chat_id == f"{source}:user" and text == "relaynest-p1probe:edited"
            ):
                return None
            record["edited"] = True
            self._calls.append({"method": "edit_message", "source": source, "userid": chat_id})
            self._calls = self._calls[-24:]
            return True

    def status(self):
        with self._lock:
            return {"enabled": self._enabled, "messages": len(self._records), "calls": list(self._calls)}

    def probe(self):
        """通过宿主真实模块分发器运行固定用例，不经过 NoticeMessage 或业务链。"""
        with self._lock:
            if not self._enabled:
                return {"success": False, "reason": "探针已停用"}
            self._records.clear()
            self._calls.clear()
            results = []
            for source in self._sources:
                message = Message(channel=None, source=source, userid=f"{source}:user",
                                  title="RelayNest P1 probe", text=self._text, save_history=False)
                sent = self.chain.run_module("post_message", message=message)
                medias = self.chain.run_module("post_medias_message", message=message, medias=[])
                torrents = self.chain.run_module("post_torrents_message", message=message, torrents=[])
                response = self.chain.run_module("send_direct_message", message=message)
                owned = bool(response and response.message_id in self._records and response.source == source)
                edited = None
                if owned:
                    edited = self.chain.run_module(
                        "edit_message", channel=None, source=source,
                        message_id=response.message_id, chat_id=response.chat_id,
                        text="relaynest-p1probe:edited", title=None, buttons=None, metadata=None,
                    )
                results.append({"source": source, "post_message": sent is True,
                                "media_entry": medias is True, "torrent_entry": torrents is True,
                                "direct_owned_id": owned, "edit_entry": edited is True})
            rejected = self.send_direct_message(Message(
                source="OtherBot", userid="OtherBot:user", text=self._text, save_history=False)) is None
            return {
                "success": all(all(v for k, v in row.items() if k != "source") for row in results) and rejected,
                "kind": "module_dispatch_with_memory_stub", "results": results,
                "foreign_source_rejected": rejected,
                "native_callbacks": ChannelCapabilityManager.supports_callbacks(None),
                "native_editing": ChannelCapabilityManager.supports_editing(None),
                "calls": list(self._calls),
            }

    def stop_service(self):
        """停用后清除内存状态；探针没有需等待退出的工作线程。"""
        with self._lock:
            self._enabled = False
            self._records.clear()
            self._calls.clear()
