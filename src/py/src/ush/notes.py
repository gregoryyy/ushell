"""Notes on command output and files (TODO 2.25): `ush notes`, and Ctrl-N with the plugin.

What is sent, all of it redacted: the last command the plugin recorded (`--last`), input piped to
ush, and files named on the command line. A file is not sent if it is in a secret location (the
policy's list, by the name given and by the file it resolves to), is not a regular file, holds
binary data, or would take the material past LIMIT, the limit of piped input.
"""

from __future__ import annotations

import os
import stat
from dataclasses import dataclass

from .policy import is_secret

LIMIT = 200_000


@dataclass
class Material:
    label: str      # what it is, for the model and the disclosure: "the file app.log"
    text: str


def split_words(words: list[str]) -> tuple[list[str], str]:
    """Words naming existing paths are files; the rest is the instruction."""
    files = [w for w in words if w and os.path.lexists(w)]
    rest = [w for w in words if w and not os.path.lexists(w)]
    return files, " ".join(rest).strip()


def read_file(path: str, budget: int) -> tuple[str, str]:
    """The text of a file to send, or "" and why it isn't sent."""
    if is_secret(os.path.abspath(path)) or is_secret(os.path.realpath(path)):
        return "", "a secret location"
    try:
        fd = os.open(path, os.O_RDONLY | os.O_NONBLOCK)  # a FIFO doesn't block the open
    except OSError as exc:
        return "", f"can't read it ({exc.strerror})"
    info = os.fstat(fd)
    if not stat.S_ISREG(info.st_mode):
        os.close(fd)
        return "", ("a directory; name the files in it" if stat.S_ISDIR(info.st_mode) else
                    "not a regular file")
    if info.st_size > budget:
        os.close(fd)
        return "", too_large(info.st_size)
    with os.fdopen(fd, "rb") as f:
        data = f.read(budget + 1)
    if len(data) > budget:
        return "", too_large(len(data))
    if b"\0" in data:
        return "", "binary"
    return data.decode("utf-8", "replace"), ""


def too_large(size: int) -> str:
    return f"too large ({size_text(size)}; at most {size_text(LIMIT)} in all)"


def size_text(size: int) -> str:
    return f"{size} bytes" if size < 1000 else f"{size / 1000:.1f} KB".replace(".0 KB", " KB")
