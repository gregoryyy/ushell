"""Test doubles: a scripted provider and a scripted terminal."""

from __future__ import annotations

import io
import os
import sys
import tempfile
import unittest
from typing import Callable

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "src"))

from ush.config import Identity, Settings  # noqa: E402
from ush.history import History  # noqa: E402
from ush.provider import Turn  # noqa: E402
from ush.ui import Choice, Terminal  # noqa: E402

SYNTHETIC_KEY = "sk-ant-test-0000000000000000000000000000SYNTHETIC"


def tool_use(command: str, effect: str = "read", ident: str = "", timeout: int = 30,
             purpose: str = "test") -> dict:
    return {"type": "tool_use", "id": ident or f"toolu_{abs(hash((command, effect))) % 10**8}",
            "name": "run_command",
            "input": {"command": command, "purpose": purpose, "effect": effect,
                      "timeout_seconds": timeout}}


def script_use(source: str, language: str = "sh", ident: str = "toolu_script") -> dict:
    return {"type": "tool_use", "id": ident, "name": "run_script",
            "input": {"language": language, "source": source, "purpose": "test",
                      "effect": "read", "timeout_seconds": 30}}


def text(value: str) -> dict:
    return {"type": "text", "text": value}


class FakeProvider:
    """Returns scripted turns. A turn may be a callable taking the message list."""

    name = "Fake"
    model = "fake-model"

    def __init__(self, turns: list):
        self.turns = list(turns)
        self.requests: list[list[dict]] = []

    def turn(self, system, tools, messages, on_text, on_progress) -> Turn:
        import copy
        self.requests.append(copy.deepcopy(messages))
        if not self.turns:
            raise AssertionError("provider called more often than scripted")
        item = self.turns.pop(0)
        if callable(item):
            item = item(messages)
        if isinstance(item, Turn):
            turn = item
        else:
            stop = "tool_use" if any(b["type"] == "tool_use" for b in item) else "end_turn"
            turn = Turn(item, stop)
        for block in turn.content:
            if block["type"] == "text":
                on_text(block["text"])
        return turn


class FakeTerminal(Terminal):
    def __init__(self, choices: list | None = None, can_prompt: bool = True):
        super().__init__(out=io.StringIO(), err=io.StringIO())
        self.choices = list(choices or [])
        self._can_prompt = can_prompt
        self.reasons: list[str] = []

    @property
    def can_prompt(self) -> bool:
        return self._can_prompt

    def confirm(self, reason: str, editable: bool) -> Choice:
        self.reasons.append(reason)
        if not self.choices:
            raise AssertionError(f"unexpected approval prompt: {reason}")
        choice = self.choices.pop(0)
        return choice if isinstance(choice, Choice) else Choice(choice)

    def read_line(self, prompt: str, prefill: str = ""):
        return None


def fake_identity(home: str, uid: int | None = None) -> Identity:
    uid = os.getuid() if uid is None else uid
    return Identity(uid, "tester", home, uid, "tester", home)


class TempDirTest(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.tmp = os.path.realpath(self._tmp.name)
        self.addCleanup(self._tmp.cleanup)

    def make_agent(self, turns: list, choices=None, can_prompt=True, **option_values):
        from ush.agent import Agent, Options
        provider = FakeProvider(turns)
        ui = FakeTerminal(choices, can_prompt)
        settings = Settings(show_lines=0)
        history = History(os.path.join(self.tmp, "state"))
        agent = Agent(provider, ui, settings, fake_identity(self.tmp), history,
                      Options(**option_values), (SYNTHETIC_KEY,))
        return agent, provider, ui, history


def last_tool_results(provider: FakeProvider, index: int = -1) -> list[dict]:
    messages = provider.requests[index]
    return messages[-1]["content"]


def results_text(results: list[dict]) -> str:
    return "\n".join(r["content"] for r in results)


Callback = Callable[[list[dict]], list[dict]]
