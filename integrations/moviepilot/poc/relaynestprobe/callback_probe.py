"""专用RelayNest渠道的固定回调测试，只允许翻页，不执行有效业务选择。"""

from uuid import uuid4
from .transport import receiver


class CallbackProbeMixin:
    def _capture_callback(self, kind, message, items=None):
        """只收集本轮明确目标的合成列表和原生失效/边界提示。"""
        if message is None or not self._callback_pending:
            return False
        from app.schemas.types import NotificationChannel
        with self._lock:
            context = self._callback_pending.get(message.source)
            if not (self._enabled and context and message.channel == NotificationChannel.RelayNest
                    and message.userid == context["userid"] and message.save_history is False):
                return False
            if kind == "post_message":
                if message.title not in {"交互已失效，请重新搜索或订阅", "已经是第一页了！", "已经是最后一页了！"} or message.text:
                    return False
            else:
                if not items or len(items) > 8:
                    return False
                titles = [getattr(item if kind == "media" else item.torrent_info, "title", "") for item in items]
                if not all(title.startswith(context["fixture_prefix"]) for title in titles):
                    return False
            self._transport_send("callback:" + kind, message, items)
            self._callback_calls.append({"kind": kind, "source": message.source, "userid": message.userid,
                                         "count": len(items or []), "title": message.title,
                                         "buttons": message.buttons or []})
            return True

    @staticmethod
    def _callback_input(source, context, data, owned_ids):
        """白名单只包含本轮request的前后翻页；拒绝select/download及其0号。"""
        allowed = {f"media:{request_id}:{action}" for request_id in owned_ids
                   for action in ("page-next", "page-prev")}
        if data not in allowed:
            raise ValueError("回调探针仅允许本轮夹具翻页")
        from app.chain.message import MessageChain
        from app.schemas.types import NotificationChannel
        MessageChain().handle_message(
            channel=NotificationChannel.RelayNest, source=source, userid=context["userid"],
            username=context["userid"], text="", is_channel_admin=False, callback_data=data,
            original_chat_id=context["userid"], original_message_id=context["fixture_prefix"],
        )

    def callback_probe(self):
        """原生生成按钮，经HTTP桩传输，再以同一callback_data进入原生handle_message。"""
        from app.application.messaging.media import media_interaction_manager as manager
        from app.chain.interaction import MediaInteractionChain
        from app.domain.context import Context, MediaInfo, TorrentInfo
        from app.schemas.notification import ChannelCapabilityManager
        from app.schemas.types import NotificationChannel

        with self._lock:
            if not self._enabled:
                return {"success": False, "reason": "探针已停用"}
            if self._callback_result is not None:
                return self._callback_result
            channel = getattr(NotificationChannel, "RelayNest", None)
            if not channel or not ChannelCapabilityManager.supports_buttons(channel) or not ChannelCapabilityManager.supports_callbacks(channel):
                return {"success": False, "reason": "专用渠道按钮/回调能力尚未生效"}
            preflight = self._message_preflight(channel)
            if not preflight["success"] or self._transport is not None:
                return {"success": False, "reason": "传播检查或并发状态不允许测试", "preflight": preflight}
            chain = MediaInteractionChain()
            if chain._page_size(channel) != 8:
                return {"success": False, "reason": "分页契约变化"}
            self._callback_calls.clear()
            controls, owned = [], []
            error_type = None
            run_id = uuid4().hex[:12]
            with receiver() as active:
                active.update(run=run_id, sent=[])
                self._transport = active
                try:
                    for phase in ("media", "torrent"):
                        contexts, requests, next_data = {}, {}, {}
                        for source in self._sources:
                            prefix = f"{source}:callback:{phase}:{run_id}"
                            context = {"userid": prefix + ":user", "fixture_prefix": prefix}
                            contexts[source] = context
                            items = []
                            for index in range(9):
                                media = MediaInfo(); media.title = f"{prefix}:{index + 1}"
                                if phase == "media":
                                    items.append(media)
                                else:
                                    torrent = TorrentInfo(); torrent.title = media.title
                                    item = Context(); item.media_info, item.torrent_info = media, torrent
                                    items.append(item)
                            request = manager.create_or_replace(context["userid"], channel, source, context["userid"],
                                                                "Search", "probe", title="RelayNest callback probe", items=items)
                            request.phase = phase
                            owned.append(request); requests[source] = request
                            self._callback_pending[source] = context
                            before = len(self._callback_calls)
                            chain._render_interaction(request, channel, source, context["userid"])
                            calls = self._callback_calls[before:]
                            nav = [b["callback_data"] for row in calls for group in row["buttons"] for b in group
                                   if b.get("callback_data") == f"media:{request.request_id}:page-next"]
                            next_data[source] = nav[0] if len(nav) == 1 else ""
                            controls.append({"phase": phase, "source": source, "step": "initial_buttons",
                                             "passed": len(calls) == 1 and calls[0]["count"] == 8 and len(nav) == 1})
                        ids = tuple(request.request_id for request in requests.values())
                        for step in ("wrong_user", "cross_bot", "next", "repeat_next", "previous", "expired"):
                            for source in self._sources:
                                context, request = contexts[source], requests[source]
                                other = self._sources[1] if source == self._sources[0] else self._sources[0]
                                other_request = manager.get_by_user(contexts[other]["userid"])
                                other_page = other_request.page if other_request else None
                                target_context = context
                                data = next_data[source]
                                if step == "wrong_user":
                                    target_context = dict(context, userid=context["userid"] + ":wrong")
                                elif step == "cross_bot":
                                    # 使用当前Bot的合法测试身份，提交另一Bot的请求ID。
                                    data = next_data[other]
                                elif step == "previous":
                                    # 从上一轮已经传给HTTP桩的原生按钮中取上一页数据。
                                    buttons = [b["callback_data"] for row in active["sent"] if row["payload"]["source"] == source
                                               for group in (row["payload"].get("buttons") or []) for b in group]
                                    data = f"media:{request.request_id}:page-prev"
                                    if data not in buttons:
                                        raise ValueError("未从原生输出获得上一页按钮")
                                elif step == "expired":
                                    manager.remove(request.request_id)
                                self._callback_pending[source] = target_context
                                before = len(self._callback_calls)
                                self._callback_input(source, target_context, data, ids)
                                calls = self._callback_calls[before:]
                                self._callback_pending[source] = context
                                expected_title = ("已经是最后一页了！" if step == "repeat_next" else "交互已失效，请重新搜索或订阅")
                                passed = len(calls) == 1 and calls[0]["source"] == source and calls[0]["userid"] == target_context["userid"]
                                if step in {"next", "previous"}:
                                    passed = passed and calls[0]["kind"] == phase and calls[0]["count"] == (1 if step == "next" else 8)
                                    passed = passed and request.page == (1 if step == "next" else 0)
                                else:
                                    passed = passed and calls[0]["kind"] == "post_message" and calls[0]["title"] == expected_title
                                    if step != "expired":
                                        passed = passed and request.page == (1 if step == "repeat_next" else 0)
                                other_after = manager.get_by_user(contexts[other]["userid"])
                                passed = passed and (other_after.page if other_after else None) == other_page
                                controls.append({"phase": phase, "source": source, "step": step, "passed": bool(passed)})
                        self._callback_pending.clear()
                except Exception as error:
                    error_type = type(error).__name__
                finally:
                    self._callback_pending.clear()
                    for request in owned:
                        manager.remove(request.request_id)
                    self._transport = None
                cleaned = all(manager.get_by_id(request.request_id, request.user_id) is None for request in owned)
                result = {"success": error_type is None and len(controls) == 28 and all(row["passed"] for row in controls)
                          and active["state"]["creates"] == 28 and cleaned,
                          "kind": "patched_channel_native_callback_to_temporary_http_stub", "preflight": preflight,
                          "error_type": error_type, "controls": controls, "http_creates": active["state"]["creates"],
                          "owned_sessions_removed": cleaned, "native_editing": ChannelCapabilityManager.supports_editing(channel),
                          "business_selection_executed": False}
            result.update(temporary_receiver_closed=True)
            self._callback_result = result
            return result
