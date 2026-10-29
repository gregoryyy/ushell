"""Terminal interaction. Status goes to stderr, answers to stdout.

Prompts read from the terminal even when stdin is a pipe (`dmesg | ush //...`).
Without a terminal, ush cannot ask, so commands that need approval are declined.
"""

from __future__ import annotations

import os
import re
import shutil
import subprocess
import sys
from dataclasses import dataclass
from typing import Optional, TextIO

from .check import Verdict
from .execute import Result

# What a terminal would act on instead of showing: C0 controls (but tab and newline), DEL, C1
# controls, and Unicode format characters that reorder or hide text (bidi overrides and isolates,
# zero-width characters, BOM). SECTEST F1.
_HIDDEN = re.compile("[\x00-\x08\x0b-\x1f\x7f-\x9f\u061c\u200b-\u200f\u2028-\u202e"
                     "\u2060-\u2069\ufeff]")


def visible(text: str) -> str:
    """Text as the terminal should show it: control and format characters written out (^[ for
    ESC, ^M for a carriage return, <U+202E> for a bidi override), never acted on."""
    def show(m: re.Match) -> str:
        c = ord(m.group())
        if c < 0x20:
            return "^" + chr(c + 0x40)
        if c == 0x7f:
            return "^?"
        if c <= 0x9f:
            return f"<{c:#04x}>"
        return f"<U+{c:04X}>"
    return _HIDDEN.sub(show, text)


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
        self.verbose = False   # -v: explanations from the line editor in full
        self.history: list[str] = []   # session lines, for Up in the line editor

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
        self._emit(self.dim(visible(text)))

    def warn(self, text: str) -> None:
        self._emit(self._style("33", "ush: " + visible(text)))

    def error(self, text: str) -> None:
        self._emit(self._style("31", "ush: " + visible(text)))

    def progress(self, text: str) -> None:
        self._emit(self.dim("… " + visible(" ".join(text.split()))))

    def answer(self, chunk: str) -> None:
        if not self._answer_open:
            self._answer_open = True
        self.out.write(visible(chunk))
        self.out.flush()

    def _end_answer(self) -> None:
        if self._answer_open:
            self._answer_open = False
            self.out.write("\n")
            self.out.flush()

    def finish_answer(self) -> None:
        self._end_answer()

    def show_command(self, command: str, purpose: str, note: str) -> None:
        self._emit(self._style("36", "$ ") + self.bold(visible(command)) +
                   (self.dim(f"   # {visible(purpose)}") if purpose else ""))
        if note:
            self._emit(self.dim(f"  {visible(note)}"))

    def show_proposal(self, command: str, purpose: str, notes: list[str]) -> None:
        """A proposed command: the user runs it, not ush. Warnings in yellow."""
        self._emit(self._style("36", "→ ") + self.bold(visible(command)) +
                   (self.dim(f"   # {visible(purpose)}") if purpose else ""))
        for note in notes:
            self._emit(self._style("33", f"  {visible(note)}"))

    def show_script(self, language: str, source: str, purpose: str, note: str) -> None:
        self._emit(self._style("36", f"$ {language} script") +
                   (self.dim(f"   # {visible(purpose)}") if purpose else ""))
        for n, line in enumerate(source.split("\n"), 1):
            self._emit(self.dim(f"{n:4} │ ") + visible(line))
        if note:
            self._emit(self.dim(f"  {visible(note)}"))

    def show_check(self, model: str, verdict: Verdict) -> None:
        """The checker's verdict under the proposal: dim when it has no objection."""
        if verdict.ok:
            self._emit(self.dim(f"  checker {model}: no objection"))
        else:
            label = "flagged" if verdict.status == "flag" else "no verdict"
            self._emit(self._style("33", f"  checker {model}: {label}: {visible(verdict.reason)}"))

    def show_output(self, result: Result, lines: int) -> None:
        text = result.output.rstrip("\n")
        shown = text.split("\n")
        if lines and len(shown) > lines:
            hidden = len(shown) - lines
            shown = shown[:lines] + [f"… {hidden} more lines (use -v to see all)"]
        elif not lines:
            shown = []
        for line in shown:
            self._emit(self.dim("  │ " + visible(line)))
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

    def read_line(self, prompt: str, prefill: str = "", session: bool = False) -> Optional[str]:
        """Read one line; None on end of input. A session line (`ush>`) is read with zsh's line
        editor: the session's earlier lines as history (Up), a proposal pre-filled, and the
        plugin's keys on it (Down explains, Up or Esc closes the explanation, Esc clears).
        Python's readline can't pre-fill with libedit, macOS's default."""
        tty = self._terminal()
        if tty is None:
            return None
        self._end_answer()
        if session or prefill:
            used, line = _vared(prompt, prefill, tty, self.verbose, self.history[-500:])
            if used:
                if line and line.strip() and session:
                    self.history.append(line)
                return line
        if tty is sys.stdin:
            try:
                import readline  # noqa: F401  line editing and arrow keys for input()
            except ImportError:
                pass
            try:
                return input(prompt)
            except EOFError:
                return None
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


# zsh's vared edits a session line, pre-filled with a proposal or empty, with the session's
# earlier lines as history. The result comes back on the descriptor in $3. Exit 2: no zle, so the
# caller falls back to a plain prompt; exit 1: end of input (Ctrl-D on an empty line).
VARED = r"""
zmodload zsh/zle 2>/dev/null || exit 2
line=$1 prompt=$2 out=$3 offered=$4 n=$5
shift 5
HISTSIZE=1000
local h
for h in "${@[1,n]}"; do
  print -s -r -- "$h"
done
shift $n
typeset -ga ush=("$@")
typeset -gi explained=0
typeset -g kept_line= kept_text=  # the last explanation and the line it explains
# The prompt is drawn as PREDISPLAY, so an explanation can take its place above the command.
_ush_init() {
  PREDISPLAY=$prompt
}
_ush_close() {
  explained=0
  PREDISPLAY=$prompt
}
_ush_esc() {
  if (( explained )); then
    _ush_close
  elif [[ -n $offered ]]; then
    BUFFER= CURSOR=0
    offered=
  fi
}
_ush_up() {
  if (( explained )); then
    _ush_close
  else
    zle up-line-or-history
  fi
}
_ush_down() {
  if [[ -n $offered && $HISTNO == $HISTCMD && $RBUFFER != *$'\n'* &&
        -n ${BUFFER//[[:space:]]/} ]]; then
    if [[ -z $kept_text || $BUFFER != $kept_line ]]; then
      PREDISPLAY="ush: explaining…"$'\n'"→ "
      zle -R
      local text
      if text=$("${ush[@]}" explain --inline -- "$BUFFER" 2>&1 </dev/null); then
        kept_line=$BUFFER kept_text=$text
      else
        kept_line= kept_text=
      fi
    else
      local text=$kept_text
    fi
    explained=1
    PREDISPLAY="$text"$'\n'"→ "
  else
    zle down-line-or-history
  fi
}
_ush_finish() {
  PREDISPLAY=$prompt
}
zle -N _ush_esc
zle -N _ush_up
zle -N _ush_down
zle -N zle-line-init _ush_init
zle -N zle-line-finish _ush_finish
bindkey -e
bindkey $'\e' _ush_esc
bindkey $'\e[A' _ush_up
bindkey $'\eOA' _ush_up
bindkey $'\e[B' _ush_down
bindkey $'\eOB' _ush_down
vared -h -e -p '' line || exit 1
print -rn -- "$line" >&$out
"""


def _vared(prompt: str, prefill: str, tty: TextIO, verbose: bool,
           history: list[str]) -> tuple[bool, Optional[str]]:
    """Edit a session line with zsh's vared. Returns (True, line or None at end of input), or
    (False, None) if zsh can't edit here."""
    zsh = shutil.which("zsh")
    if not zsh:
        return False, None
    ush = [sys.executable, *(["-I"] if sys.flags.isolated else []), "-m", "ush",
           *(["-v"] if verbose else [])]
    streams = {} if tty is sys.stdin else {"stdin": tty, "stdout": tty, "stderr": tty}
    read, write = os.pipe()
    try:
        args = [prefill, prompt, str(write), "1" if prefill else "", str(len(history)),
                *history, *ush]
        proc = subprocess.Popen([zsh, "-f", "-c", VARED, "zsh", *args], pass_fds=(write,),
                                **streams)
        os.close(write)
        write = -1
        data = b""
        try:
            while chunk := os.read(read, 65536):
                data += chunk
        finally:
            code = proc.wait()
    finally:
        os.close(read)
        if write >= 0:
            os.close(write)
    if code == 2:
        return False, None
    return True, (data.decode("utf-8", "replace") if code == 0 else None)
