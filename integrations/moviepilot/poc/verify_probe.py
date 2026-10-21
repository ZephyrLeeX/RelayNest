"""使用轻量宿主桩检查探针的拒绝边界；不冒充真实 MP 验证。"""

import importlib.util
from pathlib import Path
import sys
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
                                   "text": None, "save_history": True, **kwargs})

            def model_dump(self):
                return vars(self).copy()

        sys.modules["app.sdk.plugin"]._PluginBase = object
        sys.modules["app.schemas.message"].Message = Message
        sys.modules["app.schemas.message"].MessageResponse = SimpleNamespace
        sys.modules["app.schemas.notification"].ChannelCapabilityManager = SimpleNamespace(
            supports_callbacks=lambda channel: False, supports_editing=lambda channel: False)
        spec = importlib.util.spec_from_file_location("relaynestprobe_under_test", Path(__file__).parent / "relaynestprobe/__init__.py")
        module = importlib.util.module_from_spec(spec)
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
        plugin.stop_service()
        assert not plugin.get_state() and plugin.status()["messages"] == 0
        assert plugin.status()["text_replies"] == 0 and plugin.status()["text_result"] is None
        assert not plugin._text_pending
        assert plugin.post_message(Message(**good)) is None
        assert not plugin.probe()["success"]
        plugin.init_plugin({"enabled": False})
        assert not plugin.get_state()
        print("PASS: 模块及固定文本桩、传播边界拒绝、身份拒绝、重复请求缓存、跨 source 编辑拒绝、停用清理；仅宿主桩验证")
    finally:
        for name, value in previous.items():
            if value is None:
                sys.modules.pop(name, None)
            else:
                sys.modules[name] = value


if __name__ == "__main__":
    main()
