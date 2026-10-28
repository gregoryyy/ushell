"""Terminal interaction. Status goes to stderr, answers to stdout.

Prompts read from the terminal even when stdin is a pipe (`dmesg | ush //...`).
Without a terminal, ush cannot ask, so commands that need approval are declined.
"""

from __future__ import annotations

import os
import sys
from dataclasses import dataclass
from typing import Optional, TextIO

from .check import Verdict
from .execute import Result


@dataclass
class Choice:
    action: str        # "yes" | "no" | "edit" | "abort"
    text: str = ""     # edited command, or the reason for declining


class Terminal:
    def __init__(self, out: TextIO = sys.stdout, err: TextIO = sys.stderr):
        self.out, self.err = out, err
        self.color = err.isatty() and not os.environ.get("NO_COLOR")
        self._tty: Optional[TextIO] = None
        self._answer_open = False
        self._status_open = False

    # -- styling -----------------------------------------------------------
    def _style(self, code: str, text: str) -> str:
        return f"\x1b[{code}m{text}\x1b[0m" if self.color else text

    def dim(self, text: str) -> str:
        return self._style("2", text)

    def bold(self, text: str) -> str:
        return self._style("1", text)

    def _emit(self, text: str) -> None:
        self._end_answer()
        self.err.write(text + "\n")
        self.err.flush()

    # -- messages ----------------------------------------------------------
    def info(self, text: str) -> None:
        self._emit(self.dim(text))

    def warn(self, text: str) -> None:
        self._emit(self._style("33", "ush: " + text))

    def error(self, text: str) -> None:
        self._emit(self._style("31", "ush: " + text))

    def progress(self, text: str) -> None:
        self._emit(self.dim("… " + " ".join(text.split())))

    def answer(self, chunk: str) -> None:
        if not self._answer_open:
            self._answer_open = True
        self.out.write(chunk)
        self.out.flush()

    def _end_answer(self) -> None:
        if self._answer_open:
            self._answer_open = False
            self.out.write("\n")
            self.out.flush()

    def finish_answer(self) -> None:
        self._end_answer()

    def show_command(self, command: str, purpose: str, note: str) -> None:
        self._emit(self._style("36", "$ ") + self.bold(command) +
                   (self.dim(f"   # {purpose}") if purpose else ""))
        if note:
            self._emit(self.dim(f"  {note}"))

    def show_proposal(self, command: str, purpose: str, notes: list[str]) -> None:
        """A proposed command: the user runs it, not ush. Warnings in yellow."""
        self._emit(self._style("36", "→ ") + self.bold(command) +
                   (self.dim(f"   # {purpose}") if purpose else ""))
        for note in notes:
            self._emit(self._style("33", f"  {note}"))

    def show_script(self, language: str, source: str, purpose: str, note: str) -> None:
        self._emit(self._style("36", f"$ {language} script") +
                   (self.dim(f"   # {purpose}") if purpose else ""))
        for n, line in enumerate(source.splitlines(), 1):
            self._emit(self.dim(f"{n:4} │ ") + line)
        if note:
            self._emit(self.dim(f"  {note}"))

    def show_check(self, model: str, verdict: Verdict) -> None:
        """The checker's verdict under the proposal: dim when it has no objection."""
        if verdict.ok:
            self._emit(self.dim(f"  checker {model}: no objection"))
        else:
            label = "flagged" if verdict.status == "flag" else "no verdict"
            self._emit(self._style("33", f"  checker {model}: {label}: {verdict.reason}"))

    def show_output(self, result: Result, lines: int) -> None:
        text = result.output.rstrip("\n")
        shown = text.splitlines()
        if lines and len(shown) > lines:
            hidden = len(shown) - lines
            shown = shown[:lines] + [f"… {hidden} more lines (use -v to see all)"]
        elif not lines:
            shown = []
        for line in shown:
            self._emit(self.dim("  │ " + line))
        status = result.summary()
        if result.timed_out or result.exit_code:
            self._emit(self._style("33", f"  └ {status}, {result.duration:.1f}s"))
        else:
            self._emit(self.dim(f"  └ {status}, {result.duration:.1f}s"))

    # -- input -------------------------------------------------------------
    def _terminal(self) -> Optional[TextIO]:
        if sys.stdin.isatty():
            return sys.stdin
        if self._tty is None:
            try:
                self._tty = open("/dev/tty", "r+", encoding="utf-8")
            except OSError:
                return None
        return self._tty

    @property
    def can_prompt(self) -> bool:
        return self._terminal() is not None

    def read_line(self, prompt: str, prefill: str = "") -> Optional[str]:
        """Read one line; None on end of input. `prefill` is put on the line to edit, where
        readline is available (a session in propose mode)."""
        tty = self._terminal()
        if tty is None:
            return None
        self._end_answer()
        if tty is sys.stdin:
            hook = _prefill(prefill)
            try:
                return input(prompt)
            except EOFError:
                return None
            finally:
                if hook is not None:
                    hook.set_pre_input_hook(None)
        self.err.write(prompt)
        self.err.flush()
        line = tty.readline()
        return line.rstrip("\n") if line else None

    def confirm(self, reason: str, editable: bool) -> Choice:
        if reason:
            self._emit(self._style("33", f"  needs approval: {reason}"))
        options = "[y]es / [n]o / [e]dit / [a]bort" if editable else "[y]es / [n]o / [a]bort"
        while True:
            reply = self.read_line(f"  run it? {options}: ")
            if reply is None:
                return Choice("abort")
            reply = reply.strip().lower()
            if reply in ("y", "yes"):
                return Choice("yes")
            if reply in ("a", "abort", "q", "quit"):
                return Choice("abort")
            if reply in ("n", "no", ""):
                why = self.read_line("  reason, for the assistant (optional): ")
                return Choice("no", (why or "").strip())
            if editable and reply in ("e", "edit"):
                edited = self.read_line("  command: ")
                if edited and edited.strip():
                    return Choice("edit", edited.strip())


def _prefill(text: str):
    """Put `text` on the next input() line; returns the readline module to reset, or None."""
    if not text:
        return None
    try:
        import readline
    except ImportError:
        return None

    def insert() -> None:
        readline.insert_text(text)
        readline.redisplay()

    readline.set_pre_input_hook(insert)
    return readline
