"""使用轻量宿主桩检查探针的拒绝边界；不冒充真实 MP 验证。"""

import importlib.util
from pathlib import Path
import sys
import tempfile
from types import ModuleType, SimpleNamespace


def main():
    """临时替换导入并在结束后恢复，避免污染调用进程。"""
    names = ["app", "app.sdk", "app.sdk.plugin", "app.schemas", "app.schemas.message", "app.schemas.notification",
             "app.application", "app.application.notification", "app.application.plugin",
             "app.application.plugin.runtime", "app.application.messaging", "app.application.messaging.webagent",
             "app.application.messaging.webagent.events", "app.schemas.types", "app.chain", "app.chain.message"]
    previous = {name: sys.modules.get(name) for name in names}
    try:
        for name in names:
            sys.modules[name] = ModuleType(name)

        class Message(SimpleNamespace):
            def __init__(self, **kwargs):
                super().__init__(**{"channel": None, "source": None, "userid": None,
                                   "text": None, "title": None, "buttons": None, "save_history": True, **kwargs})

            def model_dump(self):
                return vars(self).copy()

        sys.modules["app.sdk.plugin"]._PluginBase = object
        sys.modules["app.schemas.message"].Message = Message
        sys.modules["app.schemas.message"].MessageResponse = SimpleNamespace
        sys.modules["app.schemas.notification"].ChannelCapabilityManager = SimpleNamespace(
            supports_callbacks=lambda channel: False, supports_editing=lambda channel: False)
        spec = importlib.util.spec_from_file_location("relaynestprobe_under_test", Path(__file__).parent / "relaynestprobe/__init__.py", submodule_search_locations=[str(Path(__file__).parent / "relaynestprobe")])
        module = importlib.util.module_from_spec(spec)
        sys.modules[spec.name] = module
        spec.loader.exec_module(module)
        plugin = module.RelayNestProbe()
        plugin.init_plugin()
        plugin.chain = SimpleNamespace(run_module=lambda name, **kwargs: plugin.get_module()[name](**kwargs))
        result = plugin.probe()
        assert result["success"] and len(result["calls"]) == 10
        assert len(plugin._records) == 2
        assert plugin.probe()["success"] and len(plugin._records) == 2

        good = dict(channel=None, source=plugin._sources[0], userid=f"{plugin._sources[0]}:user",
                    text=plugin._text, save_history=False)
        for change in [{"source": "OtherBot"}, {"channel": "QQ"}, {"userid": "another-user"},
                       {"text": "/download"}, {"save_history": True}]:
            message = Message(**{**good, **change})
            assert plugin.post_message(message) is None
            assert plugin.post_medias_message(message, []) is None
            assert plugin.post_torrents_message(message, []) is None
            assert plugin.send_direct_message(message) is None
        assert plugin.post_medias_message(Message(**good), [object()]) is None
        assert plugin.post_torrents_message(Message(**good), [object()]) is None
        message_id, record = next(iter(plugin._records.items()))
        other = next(source for source in plugin._sources if source != record["source"])
        assert plugin.edit_message(None, other, message_id, f"{other}:user", "relaynest-p1probe:edited") is None

        # 这里的链是测试桩：只检查固定输入、缓存和捕获边界，不宣称宿主文本链通过。
        handlers, providers, configs, inputs = [], {}, [], []
        plugin.eventmanager = SimpleNamespace(visualize_handlers=lambda: handlers)
        sys.modules["app.schemas.types"].EventType = SimpleNamespace(NoticeMessage=SimpleNamespace(value="notice"))
        sys.modules["app.application.notification"].get_notification_configs = lambda **kwargs: configs
        sys.modules["app.application.plugin.runtime"].get_plugin_manager = lambda: SimpleNamespace(get_plugin_modules=lambda: providers)
        assert plugin.text_preflight()["success"]
        handlers.append({"event_type": "notice", "status": "enabled", "handler_identifier": "OtherPlugin.send"})
        assert not plugin.text_preflight()["success"]
        handlers.clear()
        providers[("OtherPlugin", "other")] = {"post_message": object()}
        assert not plugin.text_preflight()["success"]
        providers.clear()
        configs.append(SimpleNamespace(name=plugin._sources[0]))
        assert not plugin.text_preflight()["success"]
        configs.clear()
        handlers.append({"event_type": "notice", "status": "enabled",
                         "handler_identifier": "app.application.messaging.webagent.events.dispatch_web_agent_message_event"})
        events_module = sys.modules["app.application.messaging.webagent.events"]
        events_module.extract_web_agent_message_from_event_data = lambda data: None
        assert plugin.text_preflight()["success"] and plugin.text_preflight()["webagent_rejects_probe"]
        events_module.extract_web_agent_message_from_event_data = lambda data: object()
        assert not plugin.text_preflight()["success"]
        handlers.clear()

        class NativeChainStub:
            def handle_message(self, **kwargs):
                assert kwargs["text"] == "/noai" and kwargs["is_channel_admin"] is False
                assert kwargs["channel"] is None
                inputs.append(kwargs)
                reply = dict(source=kwargs["source"], userid=kwargs["userid"],
                             title=plugin._reply_title, text=None, save_history=False)
                for change in [{"channel": "QQ"}, {"source": "OtherBot"}, {"userid": "other-user"},
                               {"title": "other-title"}, {"text": "business-reply"}, {"save_history": True}]:
                    assert plugin.post_message(Message(**{**reply, **change})) is None
                assert plugin.post_message(Message(**reply)) is True

        sys.modules["app.chain.message"].MessageChain = NativeChainStub
        native = plugin.text_probe()
        assert native["success"] and len(inputs) == 2
        assert all(row["reply_count"] == 1 for row in native["results"])
        assert all(not row["replies"][0]["chat_context_preserved"] for row in native["results"])
        assert plugin.text_probe() is native and len(inputs) == 2
        plugin.probe()
        assert plugin.text_probe() is native and len(inputs) == 2
        for unsafe in ("/noai 0", "/noai 1", "1", "/download", "/noai 自动选择"):
            try:
                plugin._candidate_input(plugin._sources[0], {}, unsafe)
            except ValueError:
                pass
            else:
                raise AssertionError("危险候选输入未被拒绝")
        source = plugin._sources[0]
        context = {"userid": "fixture-user", "chat_id": "fixture-chat", "message_id": "fixture-message",
                   "fixture_prefix": "RelayNest-P1Probe:fixture:"}
        plugin._candidate_pending[source] = context
        candidate_message = dict(source=source, userid=context["userid"], save_history=False)
        media_items = [SimpleNamespace(title=context["fixture_prefix"] + "1")]
        assert plugin.post_medias_message(Message(**candidate_message), media_items) is True
        torrents = [SimpleNamespace(torrent_info=media_items[0])]
        assert plugin.post_torrents_message(Message(**candidate_message), torrents) is True
        assert plugin.post_medias_message(Message(**candidate_message), [SimpleNamespace(title="real-media")]) is None
        assert plugin.post_medias_message(Message(**candidate_message), media_items * 9) is None
        for change in ({"source": "OtherBot"}, {"userid": "other-user"}, {"channel": "QQ"}, {"save_history": True}):
            assert plugin.post_medias_message(Message(**{**candidate_message, **change}), media_items) is None
        assert plugin.post_message(Message(**candidate_message, title="输入有误！")) is True
        assert plugin.post_message(Message(**candidate_message, title="真实业务回复")) is None
        assert not plugin._candidate_calls[0]["chat_context_preserved"]
        # 真实 loopback HTTP 仅验证协议桩；不计为 MP 主进程或正式 Hub 证据。
        transport = sys.modules["relaynestprobe_under_test.transport"]
        with transport.receiver() as active:
            active.update(run="local-test", sent=[])
            plugin._transport = active
            message = Message(**good, image=active["url"] + "/fixture.png", file_path=active["file"])
            response = plugin.send_direct_message(message)
            assert response.message_id in active["state"]["messages"]
            row = active["state"]["messages"][response.message_id]
            assert len(row["initial"]["attachments"]) == 2
            assert plugin.edit_message(None, source, response.message_id, f"{source}:user", "relaynest-p1probe:edited")
            assert row["revision"] == 2 and row["text"] == "relaynest-p1probe:edited"
            payload = active["sent"][0]["payload"]
            assert transport.request(active["url"] + "/messages", payload)["message_id"] == response.message_id
            assert active["state"]["creates"] == 1 and active["state"]["replays"] == 1
            # 回归：原生候选使用 phase/Bot 前缀，必须被临时接收桩接受。
            context["userid"] = "RelayNest-P1Probe:media:A:local:user"
            plugin._candidate_pending[source] = context
            assert plugin.post_medias_message(Message(source=source, userid=context["userid"], save_history=False), media_items)
            assert active["state"]["creates"] == 2
            assert active["sent"][-1]["payload"]["path"] == "candidate:media"
            message.image = "http://untrusted.invalid/image"
            try:
                plugin.send_direct_message(message)
            except ValueError:
                pass
            else:
                raise AssertionError("未拒绝非夹具 URL")
            plugin._transport = None
        assert not Path(active["file"]).exists()
        assert not any(t.name == "relaynest-p105-receiver" for t in transport.threading.enumerate())
        # 专用渠道测试入口仅能提交自有request的翻页，下载/选择及外部ID都拒绝。
        for unsafe in ("media:fixture:download:0", "media:fixture:select:1", "media:foreign:page-next", "update:restart"):
            try:
                plugin._callback_input(source, {}, unsafe, ("fixture",))
            except ValueError:
                pass
            else:
                raise AssertionError("不安全callback未被拒绝")
        relaynest_channel = object()
        sys.modules["app.schemas.types"].NotificationChannel = SimpleNamespace(RelayNest=relaynest_channel)
        callback_context = dict(userid=f"{source}:callback:local:user", fixture_prefix="RelayNest-P1Probe:callback:local:")
        plugin._callback_pending[source] = callback_context
        callback_message = Message(source=source, userid=callback_context["userid"], channel=relaynest_channel, save_history=False)
        callback_items = [SimpleNamespace(title=callback_context["fixture_prefix"] + "1")]
        assert plugin._capture_callback("media", callback_message, callback_items)
        for change in ({"channel": None}, {"source": "OtherBot"}, {"userid": "other-user"}, {"save_history": True}):
            assert not plugin._capture_callback("media", Message(**{**callback_message.model_dump(), **change}), callback_items)
        assert not plugin._capture_callback("media", callback_message, [SimpleNamespace(title="real-media")])
        plugin._callback_result = {"test_marker": True}
        plugin._candidate_result = {"test_marker": True}
        plugin.stop_service()
        assert not plugin.get_state() and plugin.status()["messages"] == 0
        assert plugin.status()["text_replies"] == 0 and plugin.status()["text_result"] is None
        assert not plugin._text_pending
        assert not plugin._candidate_pending and plugin.status()["candidate_calls"] == 0
        assert plugin.status()["candidate_result"] is None
        assert not plugin._callback_pending and not plugin._callback_calls and plugin._callback_result is None
        assert plugin.post_message(Message(**good)) is None
        assert not plugin.probe()["success"]
        plugin.init_plugin({"enabled": False})
        assert not plugin.get_state()
        restart_spec = importlib.util.spec_from_file_location("restart_preflight_under_test", Path(__file__).parent / "restart_preflight.py")
        restart_module = importlib.util.module_from_spec(restart_spec)
        restart_spec.loader.exec_module(restart_module)
        with tempfile.TemporaryDirectory() as folder:
            bind_rows = restart_module.check_bind_sources([
                {"Type": "bind", "Source": folder, "Destination": "/config"},
                {"Type": "bind", "Source": str(Path(folder) / "missing"), "Destination": "/netdisk"}])
            assert bind_rows[0]["available"] and not bind_rows[1]["available"] and bind_rows[1]["errno"] == 2
        print("PASS: 重启挂载预检、回调危险动作拒绝、临时 HTTP ID/编辑/附件/重放/清理、模块及文本桩、候选捕获、危险数字拒绝、传播边界、身份限制、缓存及停用清理；仅宿主桩验证")
    finally:
        for name in list(sys.modules):
            if name.startswith("relaynestprobe_under_test"):
                sys.modules.pop(name, None)
        for name, value in previous.items():
            if value is None:
                sys.modules.pop(name, None)
            else:
                sys.modules[name] = value


if __name__ == "__main__":
    main()
