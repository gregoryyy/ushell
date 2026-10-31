"""Propose mode between two `//` calls (ARCH: propose mode, TODO 2.17).

ush proposes one command; the zsh plugin puts it on the user's command line, and the user runs
it in their own shell, edited or not, or doesn't. The plugin records what ran, and the next `//`
continues the conversation with it.

- The handoff directory is the plugin's: made with `mktemp -d` when the plugin loads, passed as
  USH_HANDOFF, removed when the shell exits. ush writes `proposal` (a mode line, `capture` or
  `interactive`, then the command); the plugin writes `ran` (the exit status, then the command
  line that ran) and `output` (what it printed, unless the proposal was interactive). ush uses
  the directory only if it is a real directory of the user's, closed to others, and never as
  root. When `//` takes `ran` and `output`, they stay as `last-ran` and `last-output` for notes
  (`ush notes --last`, Ctrl-N, TODO 2.25), until the next run replaces them or the shell exits.
- The conversation is saved per shell in the state directory, `sessions/KEY.json`, mode 0600,
  where KEY is the handoff directory's name or, without the plugin, the parent process. It holds
  what was sent to the provider, so redacted command output, and expires after an hour unused.
- Explanations (`ush explain`, Down on a proposal) are kept in the state directory,
  `explanations.json`, mode 0600: the model's redacted answer under the SHA-256 of the command
  line, never the command line itself, so a command recalled from history isn't sent again.
"""

from __future__ import annotations

import hashlib
import json
import os
import re
import stat
import time
from dataclasses import dataclass
from typing import Optional

from .config import Identity
from .execute import _Collector

SESSION_TTL = 3600
EXPLANATIONS_KEPT = 500
KEY = re.compile(r"^[A-Za-z0-9._-]{1,64}$")


@dataclass
class Ran:
    """What the user ran after a proposal."""
    command: str
    exit_code: Optional[int]
    output: str = ""
    captured: bool = False      # False: an interactive command, its output was not recorded
    truncated: bool = False


def handoff_dir(ident: Identity, environ: Optional[dict] = None) -> Optional[str]:
    """The plugin's handoff directory, if usable: never as root, only the user's own, mode 0700."""
    env = os.environ if environ is None else environ
    path = env.get("USH_HANDOFF", "")
    if ident.is_root or not path or not os.path.isabs(path):
        return None
    try:
        info = os.lstat(path)
    except OSError:
        return None
    if not stat.S_ISDIR(info.st_mode) or info.st_uid != ident.uid or info.st_mode & 0o077:
        return None
    return path


def write_proposal(directory: str, command: str, interactive: bool) -> None:
    """Leave the proposal for the plugin's next prompt; replaces an earlier one atomically."""
    for name in ("ran", "output"):
        _remove(os.path.join(directory, name))
    temporary = os.path.join(directory, f".proposal.{os.getpid()}")
    _remove(temporary)
    fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as f:
        f.write(("interactive" if interactive else "capture") + "\n" + command)
    os.replace(temporary, os.path.join(directory, "proposal"))


def take_ran(directory: str, limit: int) -> Optional[Ran]:
    """What the plugin recorded since the proposal, then kept only for notes; None if nothing
    ran."""
    ran = _load_ran(directory, "", limit)
    if ran is None:
        return None
    for name in ("ran", "output"):
        path = os.path.join(directory, name)
        if os.path.lexists(path):
            os.replace(path, os.path.join(directory, "last-" + name))
        else:
            _remove(os.path.join(directory, "last-" + name))
    _remove(os.path.join(directory, "proposal"))
    return ran


def last_ran(directory: str, limit: int) -> Optional[Ran]:
    """The last run the plugin recorded, whether or not `//` has taken it; removes nothing."""
    return _load_ran(directory, "", limit) or _load_ran(directory, "last-", limit)


def _load_ran(directory: str, prefix: str, limit: int) -> Optional[Ran]:
    text = _read(os.path.join(directory, prefix + "ran"))
    if text is None:
        return None
    status, _, command = text.partition("\n")
    exit_code = int(status) if status.strip().lstrip("-").isdigit() else None
    collector = _Collector(limit, None)
    captured = False
    try:
        fd = os.open(os.path.join(directory, prefix + "output"), os.O_RDONLY | os.O_NOFOLLOW)
    except OSError:
        pass
    else:
        with os.fdopen(fd, "rb") as f:
            if stat.S_ISREG(os.fstat(f.fileno()).st_mode):
                captured = True
                while chunk := f.read(65536):
                    collector.feed(chunk)
    output, truncated = collector.text()
    return Ran(command.rstrip("\n"), exit_code, output, captured, truncated)


def session_key(handoff: Optional[str]) -> str:
    """One conversation per shell: the plugin's directory, or the parent process without it."""
    if handoff and KEY.match(os.path.basename(handoff)):
        return os.path.basename(handoff)
    return f"ppid-{os.getppid()}"


class Sessions:
    """Saved conversations in propose mode, one per shell."""

    def __init__(self, state_dir: str, key: str):
        if not KEY.match(key):
            raise ValueError(f"bad session key {key!r}")
        self.directory = os.path.join(state_dir, "sessions")
        self.path = os.path.join(self.directory, key + ".json")

    def load(self) -> Optional[dict]:
        try:
            if time.time() - os.lstat(self.path).st_mtime > SESSION_TTL:
                self.clear()
                return None
        except OSError:
            return None
        text = _read(self.path)
        try:
            data = json.loads(text) if text else None
        except ValueError:
            data = None
        return data if isinstance(data, dict) else None

    def save(self, data: dict) -> None:
        os.makedirs(self.directory, mode=0o700, exist_ok=True)
        self._prune()
        temporary = f"{self.path}.{os.getpid()}"
        _remove(temporary)
        fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        with os.fdopen(fd, "w", encoding="utf-8") as f:
            json.dump(data, f)
        os.replace(temporary, self.path)

    def clear(self) -> None:
        _remove(self.path)

    def _prune(self) -> None:
        """Remove conversations of other shells that expired."""
        now = time.time()
        try:
            names = os.listdir(self.directory)
        except OSError:
            return
        for name in names:
            path = os.path.join(self.directory, name)
            try:
                if name.endswith(".json") and now - os.lstat(path).st_mtime > SESSION_TTL:
                    os.remove(path)
            except OSError:
                pass


class Explanations:
    """Explanations given before, by command line; the newest EXPLANATIONS_KEPT."""

    def __init__(self, state_dir: str):
        self.directory = state_dir
        self.path = os.path.join(state_dir, "explanations.json")

    @staticmethod
    def key(command: str) -> str:
        return hashlib.sha256(command.strip().encode("utf-8", "surrogatepass")).hexdigest()

    def _load(self) -> dict:
        text = _read(self.path)
        try:
            data = json.loads(text) if text else {}
        except ValueError:
            data = {}
        return data if isinstance(data, dict) else {}

    def get(self, command: str) -> Optional[str]:
        entry = self._load().get(self.key(command))
        answer = entry.get("answer") if isinstance(entry, dict) else None
        return answer if isinstance(answer, str) and answer else None

    def put(self, command: str, answer: str) -> None:
        """Keep an answer; a failure to write only means it is asked for again."""
        data = self._load()
        data.pop(self.key(command), None)
        data[self.key(command)] = {"answer": answer, "time": int(time.time())}
        if len(data) > EXPLANATIONS_KEPT:  # oldest first: dicts keep insertion order
            data = dict(list(data.items())[-EXPLANATIONS_KEPT:])
        try:
            os.makedirs(self.directory, mode=0o700, exist_ok=True)
            for _ in range(100):
                temporary = f"{self.path}.{os.urandom(6).hex()}.tmp"
                try:
                    fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW,
                                 0o600)
                    break
                except FileExistsError:
                    continue
            else:
                return
            with os.fdopen(fd, "w", encoding="utf-8") as f:
                json.dump(data, f)
            os.replace(temporary, self.path)
        except OSError:
            pass


def _read(path: str) -> Optional[str]:
    try:
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    except OSError:
        return None
    with os.fdopen(fd, encoding="utf-8", errors="replace") as f:
        if not stat.S_ISREG(os.fstat(f.fileno()).st_mode):
            return None
        return f.read()


def _remove(path: str) -> None:
    try:
        os.remove(path)
    except FileNotFoundError:
        pass
