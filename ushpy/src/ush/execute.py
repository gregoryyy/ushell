"""Run shell commands and auxiliary scripts with time and output bounds."""

from __future__ import annotations

import os
import shutil
import signal
import subprocess
import sys
import tempfile
import threading
import time
from dataclasses import dataclass
from typing import Callable, Optional

# Environment variables never passed to commands: provider credentials and the
# settings that would let a command read them back.
SECRET_ENV_PREFIXES = ("ANTHROPIC_", "USH_API_KEY", "CLAUDE_API_KEY", "OPENAI_API_KEY")
NON_INTERACTIVE_ENV = {"PAGER": "cat", "GIT_PAGER": "cat", "SYSTEMD_PAGER": "", "NO_COLOR": "1",
                       "HOMEBREW_NO_AUTO_UPDATE": "1"}

SCRIPT_LANGUAGES = ("sh", "zsh", "python3")


class Cancelled(Exception):
    """The user interrupted a running command."""


@dataclass
class Result:
    exit_code: Optional[int]
    output: str            # stdout and stderr, interleaved and bounded
    truncated: bool = False
    timed_out: bool = False
    duration: float = 0.0

    def summary(self) -> str:
        if self.timed_out:
            return "timed out"
        return f"exit {self.exit_code}"


def find_shell() -> list[str]:
    for path in ("/bin/zsh", "/usr/bin/zsh", shutil.which("zsh") or ""):
        if path and os.access(path, os.X_OK):
            return [path, "-f", "-o", "pipefail", "-c"]
    return ["/bin/sh", "-c"]


def command_env(base: Optional[dict] = None) -> dict:
    env = {k: v for k, v in (base if base is not None else os.environ).items()
           if not k.startswith(SECRET_ENV_PREFIXES)}
    env.update(NON_INTERACTIVE_ENV)
    return env


class _Collector:
    """Keep the head and tail of a stream within `limit` bytes."""

    def __init__(self, limit: int, echo: Optional[Callable[[bytes], None]]):
        self.head_limit = limit * 3 // 4
        self.tail_limit = limit - self.head_limit
        self.head = bytearray()
        self.tail = bytearray()
        self.dropped = 0
        self.echo = echo

    def feed(self, chunk: bytes) -> None:
        if self.echo:
            self.echo(chunk)
        room = self.head_limit - len(self.head)
        if room > 0:
            self.head += chunk[:room]
            chunk = chunk[room:]
        if chunk:
            self.tail += chunk
            if len(self.tail) > self.tail_limit:
                excess = len(self.tail) - self.tail_limit
                self.dropped += excess
                del self.tail[:excess]

    def text(self) -> tuple[str, bool]:
        if not self.dropped:
            return (bytes(self.head) + bytes(self.tail)).decode("utf-8", "replace"), False
        marker = f"\n[... {self.dropped} bytes omitted ...]\n".encode()
        return (bytes(self.head) + marker + bytes(self.tail)).decode("utf-8", "replace"), True


def run(argv: list[str], *, timeout: float, limit: int, interactive: bool = False,
        cwd: Optional[str] = None, env: Optional[dict] = None,
        echo: Optional[Callable[[bytes], None]] = None) -> Result:
    """Run `argv`, collecting combined output.

    Non-interactive commands get /dev/null as stdin and their own process group,
    so a timeout or Ctrl-C stops the whole pipeline. Interactive commands (user
    approved, e.g. for a sudo password prompt) keep the terminal's stdin and
    process group; the terminal delivers Ctrl-C to them directly.
    """
    start = time.monotonic()
    proc = subprocess.Popen(
        argv,
        stdin=None if interactive else subprocess.DEVNULL,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        cwd=cwd,
        env=command_env() if env is None else env,
        start_new_session=not interactive,
    )
    collector = _Collector(limit, echo)

    def pump() -> None:
        assert proc.stdout is not None
        fd = proc.stdout.fileno()
        for chunk in iter(lambda: os.read(fd, 65536), b""):
            collector.feed(chunk)

    reader = threading.Thread(target=pump, daemon=True)
    reader.start()
    timed_out = False
    try:
        try:
            proc.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            timed_out = True
            _stop(proc, interactive)
    except KeyboardInterrupt:
        _stop(proc, interactive)
        raise Cancelled() from None
    finally:
        reader.join(2)
        if proc.stdout:
            proc.stdout.close()
    output, truncated = collector.text()
    return Result(proc.returncode, output, truncated, timed_out, time.monotonic() - start)


def _stop(proc: subprocess.Popen, interactive: bool) -> None:
    def send(sig: int) -> None:
        try:
            if interactive:
                proc.send_signal(sig)
            else:
                os.killpg(proc.pid, sig)
        except (ProcessLookupError, PermissionError):
            pass

    send(signal.SIGTERM)
    try:
        proc.wait(timeout=2)
    except subprocess.TimeoutExpired:
        send(signal.SIGKILL)
        proc.wait()


def run_command(command: str, **kwargs) -> Result:
    return run([*find_shell(), command], **kwargs)


def run_script(language: str, source: str, **kwargs) -> Result:
    if language not in SCRIPT_LANGUAGES:
        raise ValueError(f"unsupported script language {language!r}")
    workdir = tempfile.mkdtemp(prefix="ush-")  # mode 0700
    try:
        path = os.path.join(workdir, "script")
        fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(fd, "w") as f:
            f.write(source)
        if language == "python3":
            python = shutil.which("python3") or sys.executable
            argv = [python, "-I", path]
        elif language == "zsh":
            argv = [find_shell()[0], "-f", path]
        else:
            argv = ["/bin/sh", path]
        return run(argv, **kwargs)
    finally:
        shutil.rmtree(workdir, ignore_errors=True)
