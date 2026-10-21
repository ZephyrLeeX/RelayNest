"""P01-05 临时 HTTP 协议桩；不是 Hub，也不提供持久可靠性。"""

import asyncio
import base64
from contextlib import contextmanager
import hashlib
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
from pathlib import Path
import tempfile
import threading
import urllib.request
from uuid import uuid4


PNG = base64.b64decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=")
FILE = b"RelayNest-P1Probe:file\n"


def request(url, payload=None):
    """不使用环境代理、不携带宿主凭据；地址仅由本轮桩生成。"""
    req = urllib.request.Request(url, data=json.dumps(payload).encode() if payload is not None else None,
                                 headers={"Content-Type": "application/json"})
    with urllib.request.build_opener(urllib.request.ProxyHandler({})).open(req, timeout=3) as response:
        return json.load(response)


@contextmanager
def receiver():
    """有限生命周期 loopback 服务与夹具目录，异常也关闭线程并删除文件。"""
    state = {"messages": {}, "keys": {}, "creates": 0, "edits": 0, "replays": 0}

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_GET(self):
            if self.path != "/fixture.png":
                self.send_error(404)
                return
            self.send_response(200)
            self.send_header("Content-Length", str(len(PNG)))
            self.end_headers()
            self.wfile.write(PNG)

        def do_POST(self):
            try:
                size = int(self.headers.get("Content-Length", "0"))
                if not 0 < size <= 65536:
                    raise ValueError("请求大小")
                data = json.loads(self.rfile.read(size))
                if data.get("source") not in ("RelayNest-P1Probe:A", "RelayNest-P1Probe:B"):
                    raise ValueError("来源")
                # 沿用 P01-04 的 phase/Bot 命名空间，不把它误判成真实用户。
                prefixes = (data["source"] + ":",) + tuple(
                    f"RelayNest-P1Probe:{phase}:{data['source'][-1]}:" for phase in ("media", "torrent"))
                if not str(data.get("userid", "")).startswith(prefixes):
                    raise ValueError("身份")
                if self.path == "/messages":
                    key = data["key"]
                    if key in state["keys"]:
                        mid = state["keys"][key]
                        if state["messages"][mid]["initial"] != data:
                            raise ValueError("重放载荷冲突")
                        state["replays"] += 1
                    else:
                        # 哈希只校验附件完整性，消息身份来自本轮显式键。
                        for item in data["attachments"]:
                            raw = base64.b64decode(item["bytes"], validate=True)
                            expected = PNG if item["kind"] == "image" else FILE
                            if raw != expected or hashlib.sha256(raw).hexdigest() != item["sha256"]:
                                raise ValueError("附件字节")
                        mid = "p105-" + uuid4().hex
                        state["keys"][key] = mid
                        state["messages"][mid] = {"initial": data, "text": data["text"], "revision": 1}
                        state["creates"] += 1
                elif self.path == "/edit":
                    mid = data["message_id"]
                    row = state["messages"][mid]
                    if any(row["initial"][field] != data[field] for field in ("source", "userid")):
                        raise ValueError("跨目标编辑")
                    row.update(text=data["text"], revision=row["revision"] + 1)
                    state["edits"] += 1
                else:
                    raise ValueError("路径")
                body = json.dumps({"message_id": mid, "success": True}).encode()
                self.send_response(200)
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
            except (ValueError, KeyError, TypeError):
                self.send_error(400)

    with tempfile.TemporaryDirectory(prefix="relaynest-p105-") as folder:
        fixture = Path(folder) / "fixture.txt"
        fixture.write_bytes(FILE)
        server = HTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, name="relaynest-p105-receiver", daemon=True)
        thread.start()
        try:
            yield {"url": f"http://127.0.0.1:{server.server_port}", "file": str(fixture), "state": state}
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=3)
            if thread.is_alive():
                raise RuntimeError("临时接收线程未退出")


class TransportProbeMixin:
    def _transport_send(self, path, message, items=None):
        """仅由探针已经通过身份/夹具检查的 provider 调用。"""
        active = self._transport
        if not active:
            return None
        attachments = []
        for kind, ref in (("image", getattr(message, "image", None)),
                          ("file", getattr(message, "file_path", None))):
            if not ref:
                continue
            if kind == "image":
                if ref != active["url"] + "/fixture.png":
                    raise ValueError("拒绝非夹具图片 URL")
                with urllib.request.build_opener(urllib.request.ProxyHandler({})).open(ref, timeout=3) as response:
                    raw = response.read(1025)
            else:
                if ref != active["file"]:
                    raise ValueError("拒绝非夹具文件")
                raw = Path(ref).read_bytes()
            if raw != (PNG if kind == "image" else FILE):
                raise ValueError("夹具内容变化")
            attachments.append({"kind": kind, "bytes": base64.b64encode(raw).decode(),
                                "sha256": hashlib.sha256(raw).hexdigest()})
        payload = {"key": f"{active['run']}:{len(active['sent'])}", "path": path,
                   "source": message.source, "userid": message.userid,
                   "text": message.text, "title": message.title, "count": len(items or []),
                   "attachments": attachments}
        result = request(active["url"] + "/messages", payload)
        active["sent"].append({"payload": payload, "message_id": result["message_id"]})
        return result["message_id"]

    def transport_probe(self):
        """固定最小矩阵；不传无目标通知、不重复执行业务输入。"""
        from app.chain.message import MessageChain
        from app.schemas.message import Message

        if not self._enabled:
            return {"success": False, "reason": "探针已停用"}
        if self._transport_result is not None:
            return self._transport_result
        if self._transport is not None:
            return {"success": False, "reason": "测试正在运行"}
        preflight = self.text_preflight()
        if not preflight["success"]:
            return {"success": False, "preflight": preflight}
        # 旧缓存没有传输证据，禁止把先前的内存结果算入本轮。
        if self._text_result is not None or self._candidate_result is not None:
            return {"success": False, "reason": "请在新加载实例运行，避免复用未传输缓存"}
        checks, error_type = {}, None
        with receiver() as active:
            active.update(run=uuid4().hex, sent=[])
            self._transport = active
            chain = MessageChain()
            try:
                for source in self._sources:
                    message = Message(channel=None, source=source, userid=f"{source}:user",
                                      title="RelayNest P1 probe", text=self._text, save_history=False)
                    before = len(active["sent"])
                    chain.post_message(message)
                    checks[f"sync:{source}"] = len(active["sent"]) == before + 1
                    before = len(active["sent"])
                    asyncio.run(chain.async_post_message(message))
                    checks[f"async:{source}"] = len(active["sent"]) == before + 1
                    message.image = active["url"] + "/fixture.png"
                    message.file_path = active["file"]
                    message.file_name = "relaynest-probe.txt"
                    response = chain.send_direct_message(message)
                    mid = response.message_id if response else None
                    checks[f"direct:{source}"] = mid in active["state"]["messages"]
                    checks[f"attachments:{source}"] = bool(mid and len(active["state"]["messages"][mid]["initial"]["attachments"]) == 2)
                    edited = chain.edit_message(channel=None, source=source, message_id=mid,
                                                chat_id=f"{source}:user", text="relaynest-p1probe:edited")
                    checks[f"edit:{source}"] = bool(edited and active["state"]["messages"][mid]["revision"] == 2
                                                    and active["state"]["messages"][mid]["text"] == "relaynest-p1probe:edited")
                text = self.text_probe()
                candidates = self.candidate_probe()
                checks["native_text"] = text["success"]
                checks["native_candidates"] = candidates["success"]
                sent = active["sent"]
                checks["path_counts"] = (len(sent) == 24 and sum(row["payload"]["path"] == "post_message" for row in sent) == 4
                                         and sum(row["payload"]["path"] == "reply" for row in sent) == 2
                                         and sum(row["payload"]["path"].startswith("candidate:") for row in sent) == 16)
                first = sent[0]
                replay = request(active["url"] + "/messages", first["payload"])
                checks["explicit_replay"] = replay["message_id"] == first["message_id"] and active["state"]["creates"] == 24
                checks["same_text_distinct"] = sent[0]["payload"]["text"] == sent[1]["payload"]["text"] and sent[0]["message_id"] != sent[1]["message_id"]
            except Exception as error:
                error_type = type(error).__name__
            finally:
                self._transport = None
            state = active["state"]
            result = {"success": error_type is None and all(checks.values()), "error_type": error_type,
                      "kind": "main_process_to_temporary_loopback_protocol_stub", "checks": checks,
                      "counts": {key: state[key] for key in ("creates", "edits", "replays")},
                      "deliveries": [{"path": row["payload"]["path"], "source": row["payload"]["source"],
                                      "count": row["payload"]["count"], "attachments": len(row["payload"]["attachments"])} for row in active["sent"]],
                      "text_success": locals().get("text", {}).get("success"),
                      "candidate_success": locals().get("candidates", {}).get("success"),
                      "broadcast": "NOT_RUN", "persistent_replay": "NOT_RUN"}
        result["temporary_receiver_closed"] = True
        result["fixture_removed"] = not Path(active["file"]).exists()
        self._transport_result = result
        return result
