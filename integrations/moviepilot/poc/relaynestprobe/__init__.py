"""只处理固定测试消息的可移除探针，不执行下载或订阅业务。"""

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


from .transport import TransportProbeMixin, request


class RelayNestProbe(TransportProbeMixin, _PluginBase):
    """探测 source 路由与模块接口；不注册完整 RelayNest 渠道。"""

    plugin_name = "RelayNest 最小探针"
    plugin_desc = "仅处理内置合成消息，不连接 Hub，不执行下载或订阅。"
    plugin_version = "0.0.4"
    plugin_author = "RelayNest"
    plugin_config_prefix = "relaynestprobe_"
    plugin_order = -100
    auth_level = 1
    _sources = ("RelayNest-P1Probe:A", "RelayNest-P1Probe:B")
    _text = "relaynest-p1probe:synthetic"
    _reply_title = "请输入要使用传统交互处理的内容"

    def init_plugin(self, config=None):
        """仅建立内存状态，不创建线程、网络连接或插件数据库。"""
        self._lock = threading.RLock()
        self._enabled = bool((config or {}).get("enabled", True))
        self._records = {}
        self._calls = []
        self._text_pending = {}
        self._text_replies = []
        self._text_result = None
        self._candidate_pending = {}
        self._candidate_calls = []
        self._candidate_result = None
        self._transport = None
        self._transport_result = None

    def get_state(self):
        return self._enabled

    def get_form(self):
        return [
            {"component": "VSwitch", "props": {"model": "enabled", "label": "启用合成消息探针"}},
            {"component": "VAlert", "props": {"type": "info", "text": "仅验证模块接口和固定 /noai 输入；不接受任意业务命令。"}},
        ], {"enabled": True}

    def get_page(self):
        return []

    def get_api(self):
        """所有入口均要求 MP API 密钥，POST 只运行内置固定用例。"""
        return [
            {"path": "/transport_probe", "endpoint": self.transport_probe, "methods": ["POST"], "auth": "apikey", "summary": "运行临时 HTTP 传输验证"},
            {"path": "/status", "endpoint": self.status, "methods": ["GET"], "auth": "apikey", "summary": "读取探针状态"},
            {"path": "/probe", "endpoint": self.probe, "methods": ["POST"], "auth": "apikey", "summary": "运行固定合成消息探针"},
            {"path": "/text_preflight", "endpoint": self.text_preflight, "methods": ["GET"], "auth": "apikey", "summary": "检查固定文本测试的传播边界"},
            {"path": "/text_probe", "endpoint": self.text_probe, "methods": ["POST"], "auth": "apikey", "summary": "运行固定原生文本入站测试"},
            {"path": "/candidate_probe", "endpoint": self.candidate_probe, "methods": ["POST"], "auth": "apikey", "summary": "运行无下载的固定候选控制测试"},
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
        if self._capture_candidate("post_message", message):
            return True
        if self._capture_text_reply(message):
            return True
        if not self._matches(message):
            return None
        with self._lock:
            self._record("post_message", message)
            self._transport_send("post_message", message)
        # 此固定合成消息已被消费，阻止 legacy 调度继续进入其它 provider。
        return True

    def _capture_text_reply(self, message):
        """仅收集本轮待处理身份的固定原生提示，不转发通知事件。"""
        if message is None:
            return False
        with self._lock:
            expected = self._text_pending.get(message.source)
            if not (
                self._enabled and expected and message.channel is None
                and message.userid == expected["userid"]
                and message.title == self._reply_title and not message.text
                and message.save_history is False
            ):
                return False
            self._transport_send("reply", message)
            self._text_replies.append({
                "source": message.source, "userid": message.userid,
                "chat_context_preserved": getattr(message, "original_chat_id", None) == expected["chat_id"],
                "message_context_preserved": getattr(message, "original_message_id", None) == expected["message_id"],
            })
            return True

    def text_preflight(self):
        """只读检查扩散入口；仅排除已核实不接收本次消息的 WebAgent 桥。"""
        from app.application.notification import get_notification_configs
        from app.application.plugin.runtime import get_plugin_manager
        from app.schemas.types import EventType

        notice_handlers = [
            row["handler_identifier"] for row in self.eventmanager.visualize_handlers()
            if row["event_type"] == EventType.NoticeMessage.value and row["status"] == "enabled"
        ]
        webagent_handler = "app.application.messaging.webagent.events.dispatch_web_agent_message_event"
        webagent_rejects_probe = None
        blocked_handlers = list(notice_handlers)
        if webagent_handler in notice_handlers:
            from app.application.messaging.webagent.events import extract_web_agent_message_from_event_data

            # 该桥在提取结果为空时立即返回；用真实消息模型校验本次 channel=None 的范围。
            webagent_rejects_probe = all(
                extract_web_agent_message_from_event_data(Message(
                    channel=None, source=source, userid=f"{source}:user:preflight",
                    title=self._reply_title, save_history=False,
                ).model_dump()) is None
                for source in self._sources
            )
            if webagent_rejects_probe:
                blocked_handlers.remove(webagent_handler)
        other_providers = [
            identity[0] for identity, methods in get_plugin_manager().get_plugin_modules().items()
            if {"post_message", "post_medias_message", "post_torrents_message",
                "send_direct_message", "edit_message"}.intersection(methods)
            and identity[0] != type(self).__name__
        ]
        collisions = [
            conf.name for conf in get_notification_configs(include_disabled=True)
            if conf.name in self._sources
        ]
        return {"success": self._enabled and not (blocked_handlers or other_providers or collisions),
                "active_notice_handlers": notice_handlers, "other_post_providers": other_providers,
                "source_collisions": collisions, "blocked_notice_handlers": blocked_handlers,
                "webagent_rejects_probe": webagent_rejects_probe}

    def text_probe(self):
        """原样调用宿主 handle_message；只发送两条固定 /noai，不替换核心或事件。"""
        from app.chain.message import MessageChain

        with self._lock:
            if not self._enabled:
                return {"success": False, "reason": "探针已停用"}
            # 同一实例重复请求只返回本轮结果，避免多次写入测试消息历史。
            if self._text_result is not None:
                return self._text_result
            preflight = self.text_preflight()
            if not preflight["success"]:
                return {"success": False, "reason": "传播边界检查未通过", "preflight": preflight}
            self._text_replies.clear()
            results = []
            run_id = uuid4().hex[:12]
            for source in self._sources:
                context = {"userid": f"{source}:user:{run_id}",
                           "chat_id": f"{source}:chat:{run_id}",
                           "message_id": f"{source}:message:{run_id}"}
                self._text_pending[source] = context
                before = len(self._text_replies)
                error_type = None
                try:
                    MessageChain().handle_message(
                        channel=None, source=source, userid=context["userid"],
                        username=context["userid"], text="/noai", is_channel_admin=False,
                        original_message_id=context["message_id"], original_chat_id=context["chat_id"],
                    )
                except Exception as error:
                    # 只报告异常类型，不把宿主原始异常或配置带入响应。
                    error_type = type(error).__name__
                finally:
                    self._text_pending.pop(source, None)
                replies = self._text_replies[before:]
                results.append({"source": source, "input": "/noai", "is_channel_admin": False,
                                "error_type": error_type, "reply_count": len(replies),
                                "directed_reply": error_type is None and len(replies) == 1
                                and replies[0]["source"] == source and replies[0]["userid"] == context["userid"],
                                "replies": replies})
            self._text_result = {
                "success": all(row["directed_reply"] for row in results),
                "kind": "native_handle_message_fixed_noai", "preflight": preflight, "results": results,
                "native_callbacks": ChannelCapabilityManager.supports_callbacks(None),
                "native_editing": ChannelCapabilityManager.supports_editing(None),
            }
            return self._text_result

    def post_medias_message(self, message, medias):
        if self._capture_candidate("media", message, medias):
            return True
        if not self._matches(message) or medias:
            return None
        with self._lock:
            self._record("post_medias_message", message)
        return True

    def post_torrents_message(self, message, torrents):
        if self._capture_candidate("torrent", message, torrents):
            return True
        if not self._matches(message) or torrents:
            return None
        with self._lock:
            self._record("post_torrents_message", message)
        return True

    def _capture_candidate(self, kind, message, items=None):
        """只捕获本轮夹具候选及固定控制回复，不记录真实媒体或种子。"""
        if message is None:
            return False
        with self._lock:
            context = self._candidate_pending.get(message.source)
            if not (self._enabled and context and message.channel is None
                    and message.userid == context["userid"] and message.save_history is False):
                return False
            if kind == "post_message":
                if message.title not in {"输入有误！", "媒体交互已结束"} or message.text:
                    return False
            else:
                if not items or len(items) > 8:
                    return False
                titles = [getattr(item if kind == "media" else item.torrent_info, "title", "") for item in items]
                if not all(title.startswith(context["fixture_prefix"]) for title in titles):
                    return False
            self._transport_send(f"candidate:{kind}", message, items)
            self._candidate_calls.append({
                "kind": kind, "source": message.source, "userid": message.userid,
                "count": len(items or []), "buttons_present": bool(message.buttons),
                "chat_context_preserved": getattr(message, "original_chat_id", None) == context["chat_id"],
                "message_context_preserved": getattr(message, "original_message_id", None) == context["message_id"],
                "title": message.title if kind == "post_message" else None,
            })
            return True

    @staticmethod
    def _candidate_input(source, context, text):
        """固定白名单不含 0 或有效序号，防止探针进入下载和订阅分支。"""
        if text not in {"/noai 999999", "/noai n", "/noai q"}:
            raise ValueError("候选探针仅允许越界数字、下一页和退出")
        from app.chain.message import MessageChain

        MessageChain().handle_message(
            channel=None, source=source, userid=context["userid"], username=context["userid"],
            text=text, is_channel_admin=False, original_chat_id=context["chat_id"],
            original_message_id=context["message_id"], reply_to_message_id=context["fixture_prefix"],
        )

    def candidate_probe(self):
        """播种自有内存夹具，再走原生列表渲染和受控文本；不执行有效下载选择。"""
        from app.application.messaging.media import media_interaction_manager as manager
        from app.chain.interaction import MediaInteractionChain
        from app.chain.message import MessageChain
        from app.domain.context import Context, MediaInfo, TorrentInfo

        with self._lock:
            if not self._enabled:
                return {"success": False, "reason": "探针已停用"}
            if self._candidate_result is not None:
                return self._candidate_result
            preflight = self.text_preflight()
            if not preflight["success"]:
                return {"success": False, "preflight": preflight}
            chain = MediaInteractionChain()
            if chain._page_size(None) != 8:
                return {"success": False, "reason": "宿主候选分页契约改变，未执行测试"}
            self._candidate_calls.clear()
            run_id = uuid4().hex[:12]
            owned = []
            controls = []
            error_type = None
            callback_error = None
            callback_returned = False
            collision = False
            try:
                # 只使用随机测试用户，直接观察宿主 store 是否按 source 分区。
                shared_user = f"RelayNest-P1Probe:shared:{run_id}"
                for source in self._sources:
                    request = manager.create_or_replace(shared_user, None, source, shared_user, "Search", "probe")
                    owned.append(request)
                collision = (manager.get_by_user(shared_user).request_id == owned[-1].request_id
                             and manager.get_by_id(owned[-2].request_id, shared_user) is None)
                manager.remove(owned[-1].request_id)
                for phase in ("media", "torrent"):
                    contexts = {}
                    requests = {}
                    for source in self._sources:
                        prefix = f"RelayNest-P1Probe:{phase}:{source[-1]}:{run_id}"
                        context = {"userid": f"{prefix}:user", "chat_id": f"{prefix}:chat",
                                   "message_id": f"{prefix}:initial", "fixture_prefix": prefix}
                        contexts[source] = context
                        items = []
                        for index in range(9):
                            media = MediaInfo()
                            media.title = f"{prefix}:{index + 1}"
                            if phase == "media":
                                items.append(media)
                            else:
                                torrent = TorrentInfo()
                                torrent.title = f"{prefix}:{index + 1}"
                                item = Context()
                                item.media_info, item.torrent_info = media, torrent
                                items.append(item)
                        request = manager.create_or_replace(context["userid"], None, source, context["userid"],
                                                            "Search", "probe", title="RelayNest P1 probe", items=items)
                        request.phase = phase
                        owned.append(request)
                        requests[source] = request
                        self._candidate_pending[source] = context
                        before = len(self._candidate_calls)
                        chain._render_interaction(request, None, source, context["userid"],
                                                  original_message_id=context["message_id"],
                                                  original_chat_id=context["chat_id"])
                        calls = self._candidate_calls[before:]
                        controls.append({"phase": phase, "source": source, "step": "initial",
                                         "passed": len(calls) == 1 and calls[0]["kind"] == phase
                                         and calls[0]["count"] == 8, "calls": calls})
                    # 两个会话同时存在，交错操作，检查每步不会更改另一组会话。
                    for step, text in (("invalid", "/noai 999999"), ("next", "/noai n"), ("quit", "/noai q")):
                        for source in self._sources:
                            context, request = contexts[source], requests[source]
                            other = self._sources[1] if source == self._sources[0] else self._sources[0]
                            other_before = manager.get_by_user(contexts[other]["userid"])
                            other_snapshot = (other_before.request_id, other_before.page) if other_before else None
                            context["message_id"] = f"{context['fixture_prefix']}:{step}"
                            before = len(self._candidate_calls)
                            self._candidate_input(source, context, text)
                            calls = self._candidate_calls[before:]
                            current = manager.get_by_user(context["userid"])
                            other_after = manager.get_by_user(contexts[other]["userid"])
                            other_unchanged = other_snapshot == ((other_after.request_id, other_after.page) if other_after else None)
                            expected_title = "输入有误！" if step == "invalid" else "媒体交互已结束"
                            delivered = len(calls) == 1 and calls[0]["source"] == source and calls[0]["userid"] == context["userid"]
                            if step == "next":
                                passed = delivered and calls[0]["kind"] == phase and calls[0]["count"] == 1 and current.page == 1
                            else:
                                passed = delivered and calls[0]["title"] == expected_title
                                passed = passed and (current is None if step == "quit" else current.page == 0)
                            controls.append({"phase": phase, "source": source, "step": step,
                                             "passed": bool(passed and other_unchanged), "calls": calls})
                    self._candidate_pending.clear()
                try:
                    MessageChain().handle_message(
                        channel=None, source=self._sources[0], userid=f"RelayNest-P1Probe:callback:{run_id}",
                        username="RelayNest probe", text="", is_channel_admin=False,
                        callback_data=f"media:relaynest-probe-{run_id}:noop",
                    )
                    callback_returned = True
                except Exception as error:
                    callback_error = type(error).__name__
            except Exception as error:
                error_type = type(error).__name__
            finally:
                self._candidate_pending.clear()
                # 仅清理本轮记录下来的 request_id，不清空宿主其它用户的会话。
                for request in owned:
                    manager.remove(request.request_id)
            cleaned = all(manager.get_by_id(request.request_id, request.user_id) is None for request in owned)
            self._candidate_result = {
                "success": error_type is None and len(controls) == 16 and all(row["passed"] for row in controls) and cleaned,
                "kind": "native_candidate_fixture_controls", "error_type": error_type,
                "same_user_replaces_previous_source": collision, "controls": controls, "owned_sessions_removed": cleaned,
                "callback": {"supported": ChannelCapabilityManager.supports_callbacks(None),
                             "returned_without_error": callback_returned, "error_type": callback_error},
                "valid_selection_executed": False,
            }
            return self._candidate_result

    def send_direct_message(self, message):
        """返回本轮临时 HTTP 桩或旧内存桩 ID，均非正式 Hub 证据。"""
        if not self._matches(message):
            return None
        with self._lock:
            message_id = self._transport_send("direct", message) or f"relaynest-probe-{uuid4().hex}"
            self._records[message_id] = {"source": message.source, "edited": False}
            self._record("send_direct_message", message)
            return MessageResponse(
                channel=None, source=message.source, message_id=message_id,
                chat_id=message.userid, success=True,
            )

    def edit_message(self, channel, source, message_id, chat_id, text,
                     title=None, buttons=None, metadata=None):
        """只允许编辑本轮自有桩消息，不操作真实业务消息。"""
        with self._lock:
            record = self._records.get(message_id)
            if not (
                self._enabled and channel is None and source in self._sources
                and record and record["source"] == source
                and chat_id == f"{source}:user" and text == "relaynest-p1probe:edited"
            ):
                return None
            if self._transport:
                request(self._transport["url"] + "/edit", {"source": source, "userid": chat_id,
                        "message_id": message_id, "text": text})
            record["edited"] = True
            self._calls.append({"method": "edit_message", "source": source, "userid": chat_id})
            self._calls = self._calls[-24:]
            return True

    def status(self):
        with self._lock:
            return {"enabled": self._enabled, "messages": len(self._records), "calls": list(self._calls),
                    "text_replies": len(self._text_replies), "text_result": self._text_result,
                    "transport_active": self._transport is not None, "transport_result": self._transport_result,
                    "candidate_calls": len(self._candidate_calls), "candidate_result": self._candidate_result}

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
            self._text_pending.clear()
            self._text_replies.clear()
            self._text_result = None
            self._candidate_pending.clear()
            self._candidate_calls.clear()
            self._candidate_result = None
        self._transport = None
        self._transport_result = None
