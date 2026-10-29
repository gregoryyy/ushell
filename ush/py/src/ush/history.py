"""Append-only request history: what was asked, which commands ran, the answer.

History stores commands, scripts, decisions, and exit codes, plus a redacted,
shortened answer. Command output is not stored. The file is private (0600).
"""

from __future__ import annotations

import json
import os
import time
from dataclasses import asdict, dataclass, field
from typing import Iterator, Optional

ANSWER_LIMIT = 4000


@dataclass
class Step:
    kind: str                       # "command" | "script" | "explicit"
    command: str = ""               # command line, or script source
    language: str = ""              # script language
    purpose: str = ""
    decision: str = ""              # auto | approved | edited | declined | dry-run | explicit | skipped
    check: str = ""                 # the checker's verdict: ok | flag: reason | none: reason
    rule: str = ""                  # the user's allow rule that let it run without approval
    exit_code: Optional[int] = None
    timed_out: bool = False
    duration: float = 0.0


@dataclass
class Entry:
    id: str
    time: float
    user: str
    cwd: str
    request: str
    model: str
    steps: list[Step] = field(default_factory=list)
    answer: str = ""
    outcome: str = ""               # answered | refused | aborted | error | dry-run

    def to_json(self) -> str:
        return json.dumps(asdict(self), ensure_ascii=False)

    @classmethod
    def from_dict(cls, data: dict) -> "Entry":
        steps = [Step(**{k: v for k, v in s.items() if k in Step.__dataclass_fields__})
                 for s in data.get("steps", [])]
        fields = {k: v for k, v in data.items() if k in cls.__dataclass_fields__ and k != "steps"}
        return cls(steps=steps, **fields)


class History:
    def __init__(self, directory: str):
        self.directory = directory
        self.path = os.path.join(directory, "history.jsonl")

    def new_entry(self, request: str, model: str, user: str) -> Entry:
        stamp = time.time()
        return Entry(id=time.strftime("%Y%m%d-%H%M%S", time.localtime(stamp)) +
                     f"-{int(stamp * 1000) % 1000:03d}", time=stamp, user=user,
                     cwd=os.getcwd(), request=request, model=model)

    def append(self, entry: Entry) -> None:
        if len(entry.answer) > ANSWER_LIMIT:
            entry.answer = entry.answer[:ANSWER_LIMIT] + " […]"
        os.makedirs(self.directory, mode=0o700, exist_ok=True)
        fd = os.open(self.path, os.O_WRONLY | os.O_APPEND | os.O_CREAT | os.O_NOFOLLOW, 0o600)
        with os.fdopen(fd, "a", encoding="utf-8") as f:
            f.write(entry.to_json() + "\n")

    def entries(self) -> Iterator[Entry]:
        try:
            with open(self.path, encoding="utf-8") as f:
                for line in f:
                    try:
                        yield Entry.from_dict(json.loads(line))
                    except (ValueError, TypeError):
                        continue  # a damaged line does not hide the others
        except FileNotFoundError:
            return

    def recent(self, n: int) -> list[Entry]:
        return list(self.entries())[-n:]

    def find(self, ident: str) -> Optional[Entry]:
        entries = list(self.entries())
        if ident in ("last", "-1") and entries:
            return entries[-1]
        matches = [e for e in entries if e.id == ident or e.id.startswith(ident)]
        return matches[-1] if matches else None
