"""A local stand-in for the Anthropic Messages API (and Ollama's compatible one) that streams
scripted replies."""

from __future__ import annotations

import json
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


def _event(name: str, data: dict) -> bytes:
    return f"event: {name}\ndata: {json.dumps(data)}\n\n".encode()


def sse(blocks: list[dict], stop_reason: str) -> bytes:
    out = [_event("message_start", {"type": "message_start", "message": {
        "id": "msg_test", "type": "message", "role": "assistant", "model": "claude-opus-5-5",
        "content": [], "stop_reason": None, "stop_sequence": None,
        "usage": {"input_tokens": 5, "output_tokens": 1}}})]
    for i, block in enumerate(blocks):
        def start(cb):
            out.append(_event("content_block_start",
                              {"type": "content_block_start", "index": i, "content_block": cb}))

        def delta(d):
            out.append(_event("content_block_delta",
                              {"type": "content_block_delta", "index": i, "delta": d}))

        if block["type"] == "text":
            start({"type": "text", "text": ""})
            delta({"type": "text_delta", "text": block["text"]})
        elif block["type"] == "thinking":
            start({"type": "thinking", "thinking": "", "signature": ""})
            delta({"type": "thinking_delta", "thinking": block["thinking"]})
            delta({"type": "signature_delta", "signature": "sig-test"})
        elif block["type"] == "tool_use":
            start({"type": "tool_use", "id": block["id"], "name": block["name"], "input": {}})
            raw = json.dumps(block["input"])
            delta({"type": "input_json_delta", "partial_json": raw[:10]})
            delta({"type": "input_json_delta", "partial_json": raw[10:]})
        out.append(_event("content_block_stop", {"type": "content_block_stop", "index": i}))
    out.append(_event("message_delta", {"type": "message_delta",
                                        "delta": {"stop_reason": stop_reason, "stop_sequence": None},
                                        "usage": {"output_tokens": 10}}))
    out.append(_event("message_stop", {"type": "message_stop"}))
    return b"".join(out)


class MockAPI:
    """Serve `replies` in order: (blocks, stop_reason) or (status, error_json)."""

    def __init__(self, replies: list, tags: list | None = None):
        self.replies = list(replies)
        self.requests: list[dict] = []
        self.tags = tags  # Ollama's /api/tags model list; None answers 404
        api = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_GET(self):
                if self.path != "/api/tags" or api.tags is None:
                    self.send_error(404)
                    return
                payload = json.dumps({"models": api.tags}).encode()
                self.send_response(200)
                self.send_header("content-type", "application/json")
                self.send_header("content-length", str(len(payload)))
                self.end_headers()
                self.wfile.write(payload)

            def do_POST(self):
                body = self.rfile.read(int(self.headers.get("content-length", 0)))
                api.requests.append({"path": self.path, "headers": dict(self.headers),
                                     "body": json.loads(body)})
                reply = api.replies.pop(0) if api.replies else (500, {"error": "unscripted"})
                if isinstance(reply[0], int):
                    payload = json.dumps({"type": "error", "error": {
                        "type": reply[1].get("type", "api_error"),
                        "message": reply[1].get("message", "error")}}).encode()
                    self.send_response(reply[0])
                    self.send_header("content-type", "application/json")
                    self.send_header("content-length", str(len(payload)))
                    self.end_headers()
                    self.wfile.write(payload)
                    return
                data = sse(*reply)
                self.send_response(200)
                self.send_header("content-type", "text/event-stream")
                self.send_header("content-length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.url = f"http://127.0.0.1:{self.server.server_address[1]}"
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)

    def __enter__(self) -> "MockAPI":
        self.thread.start()
        return self

    def __exit__(self, *exc) -> None:
        self.server.shutdown()
        self.server.server_close()
