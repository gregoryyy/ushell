"""A local stand-in for OpenAI's Responses API (TODO 2.29.6): `POST /v1/responses` answers from
scripted replies, streamed as server-sent events or as one JSON response, and `GET /v1/models`
lists models."""

from __future__ import annotations

import json
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

SYNTHETIC_OPENAI_KEY = "sk-proj-test-000000000000000000000000000000SYNTHETIC"


def text(t: str, ident: str = "msg_1") -> dict:
    return {"type": "message", "id": ident, "role": "assistant", "status": "completed",
            "content": [{"type": "output_text", "text": t, "annotations": []}]}


def call(name: str, arguments: dict | str, call_id: str = "call_1", ident: str = "fc_1") -> dict:
    raw = arguments if isinstance(arguments, str) else json.dumps(arguments)
    return {"type": "function_call", "id": ident, "call_id": call_id, "name": name,
            "arguments": raw, "status": "completed"}


def reasoning(ident: str = "rs_1", summary: str = "") -> dict:
    return {"type": "reasoning", "id": ident, "encrypted_content": "gAAAA-opaque-" + ident,
            "summary": [{"type": "summary_text", "text": summary}] if summary else [],
            "status": "completed"}


class MockOpenAI:
    """Replies, in order: a list of output items (a completed response), a dict
    `{"output": [...], "status": "incomplete", "reason": "max_output_tokens"}`, a list of raw
    events (`{"events": [...]}`), or `(status, message)` for an HTTP error (`(status, message,
    headers)` adds headers)."""

    def __init__(self, replies: list, models: list | None = None):
        self.replies = list(replies)
        self.requests: list[dict] = []
        self.models = models  # GET /v1/models: [{"id": …, "created": …}]; None answers 404
        api = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def _json(self, status: int, payload: dict, headers: dict | None = None):
                raw = json.dumps(payload).encode()
                self.send_response(status)
                self.send_header("content-type", "application/json")
                self.send_header("content-length", str(len(raw)))
                for k, v in (headers or {}).items():
                    self.send_header(k, v)
                self.end_headers()
                self.wfile.write(raw)

            def do_GET(self):
                api.requests.append({"path": self.path, "headers": dict(self.headers)})
                if self.path != "/v1/models" or api.models is None:
                    self._json(404, {"error": {"message": "not found", "type": "invalid_request_error"}})
                    return
                self._json(200, {"object": "list", "data": [
                    {"object": "model", "owned_by": "openai", **m} for m in api.models]})

            def do_POST(self):
                body = json.loads(self.rfile.read(int(self.headers.get("content-length", 0))))
                api.requests.append({"path": self.path, "headers": dict(self.headers),
                                     "body": body})
                reply = api.replies.pop(0) if api.replies else (500, "unscripted")
                if isinstance(reply, tuple):
                    self._json(reply[0], {"error": {"message": reply[1], "type": "error",
                                                    "code": None, "param": None}},
                               reply[2] if len(reply) > 2 else None)
                    return
                if isinstance(reply, dict) and "events" in reply:
                    events = reply["events"]
                else:
                    output = reply["output"] if isinstance(reply, dict) else reply
                    status = reply.get("status", "completed") if isinstance(reply, dict) else "completed"
                    response = {"id": "resp_1", "object": "response", "status": status,
                                "model": body.get("model"), "output": output,
                                "usage": {"input_tokens": 11, "output_tokens": 7}}
                    if status == "incomplete":
                        response["incomplete_details"] = {"reason": reply.get("reason")}
                    if not body.get("stream"):
                        self._json(200, response)
                        return
                    events = [{"type": "response.created", "response": {**response, "output": [],
                                                                         "status": "in_progress"}}]
                    for i, item in enumerate(output):
                        events.append({"type": "response.output_item.added", "output_index": i,
                                       "item": {**item, "status": "in_progress"}})
                        for part in item.get("content") or []:
                            if part.get("type") == "output_text":
                                for piece in (part["text"][:5], part["text"][5:]):
                                    if piece:
                                        events.append({"type": "response.output_text.delta",
                                                       "item_id": item.get("id"),
                                                       "output_index": i, "content_index": 0,
                                                       "delta": piece})
                        if item.get("type") == "function_call":
                            raw = item["arguments"]
                            for piece in (raw[:7], raw[7:]):
                                events.append({"type": "response.function_call_arguments.delta",
                                               "item_id": item.get("id"), "output_index": i,
                                               "delta": piece})
                        events.append({"type": "response.output_item.done", "output_index": i,
                                       "item": item})
                    events.append({"type": f"response.{'incomplete' if status == 'incomplete' else 'completed'}",
                                   "response": response})
                self.send_response(200)
                self.send_header("content-type", "text/event-stream")
                self.end_headers()
                for n, event in enumerate(events):
                    self.wfile.write(f"event: {event.get('type')}\ndata: {json.dumps(event)}\n\n".encode())
                    self.wfile.flush()

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.url = f"http://127.0.0.1:{self.server.server_address[1]}"
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)

    def __enter__(self) -> "MockOpenAI":
        self.thread.start()
        return self

    def __exit__(self, *exc) -> None:
        self.server.shutdown()
        self.server.server_close()
