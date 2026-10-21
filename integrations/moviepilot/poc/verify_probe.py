"""使用轻量宿主桩检查探针的拒绝边界；不冒充真实 MP 验证。"""

import importlib.util
from pathlib import Path
import sys
from types import ModuleType, SimpleNamespace


def main():
    """临时替换导入并在结束后恢复，避免污染调用进程。"""
    names = ["app", "app.sdk", "app.sdk.plugin", "app.schemas", "app.schemas.message", "app.schemas.notification"]
    previous = {name: sys.modules.get(name) for name in names}
    try:
        for name in names:
            sys.modules[name] = ModuleType(name)

        class Message(SimpleNamespace):
            def __init__(self, **kwargs):
                super().__init__(**{"channel": None, "source": None, "userid": None,
                                   "text": None, "save_history": True, **kwargs})

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
        plugin.stop_service()
        assert not plugin.get_state() and plugin.status()["messages"] == 0
        assert plugin.post_message(Message(**good)) is None
        assert not plugin.probe()["success"]
        plugin.init_plugin({"enabled": False})
        assert not plugin.get_state()
        print("PASS: 合成模块流程、重复运行、外部身份拒绝、跨 source 编辑拒绝、停用和清理；仅宿主桩验证")
    finally:
        for name, value in previous.items():
            if value is None:
                sys.modules.pop(name, None)
            else:
                sys.modules[name] = value


if __name__ == "__main__":
    main()
