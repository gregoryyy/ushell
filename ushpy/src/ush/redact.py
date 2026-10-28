"""Remove likely secrets from text before it leaves the machine or is stored.

Redaction applies to command output sent to the AI provider, to piped input, and
to answers saved in history. What the user sees in the terminal is not redacted.
Pattern matching is best effort; it reduces, but cannot rule out, disclosure.
"""

from __future__ import annotations

import re
from typing import Iterable

MARK = "[redacted]"

_PATTERNS = [
    # Private key blocks
    re.compile(r"-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?(-----END [A-Z0-9 ]*PRIVATE KEY-----|\Z)",
               re.DOTALL),
    # Provider and platform tokens
    re.compile(r"\bsk-(?:ant-)?[A-Za-z0-9_\-]{20,}"),
    re.compile(r"\b(?:AKIA|ASIA)[0-9A-Z]{16}\b"),
    re.compile(r"\bgh[pousr]_[A-Za-z0-9]{30,}\b"),
    re.compile(r"\bgithub_pat_[A-Za-z0-9_]{40,}\b"),
    re.compile(r"\bxox[abposr]-[A-Za-z0-9\-]{10,}"),
    re.compile(r"\bAIza[0-9A-Za-z_\-]{35}\b"),
    re.compile(r"\bglpat-[A-Za-z0-9_\-]{20,}\b"),
    re.compile(r"\beyJ[A-Za-z0-9_\-]{10,}\.eyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}"),
    # Credentials embedded in URLs: scheme://user:password@host
    re.compile(r"(?<=://)[^/\s:@]+:[^/\s@]+(?=@)"),
]

# NAME=value, NAME: value, "name": "value" for secret-like names. The name is kept.
_ASSIGNMENT = re.compile(
    r"""(?ix)
    (?P<name>["']?[\w.\-]*(?:password|passwd|passphrase|secret|token|api[_\-]?key|
        access[_\-]?key|private[_\-]?key|auth|credential|session[_\-]?id)[\w.\-]*["']?)
    (?P<sep>\s*[:=]\s*)
    (?P<value>"[^"\n]*"|'[^'\n]*'|[^\s,;}\]]+)
    """
)


def redact(text: str, known: Iterable[str] = ()) -> str:
    for secret in known:
        if secret and len(secret) >= 8:
            text = text.replace(secret, MARK)
    for pattern in _PATTERNS:
        text = pattern.sub(MARK, text)

    def assignment(m: re.Match) -> str:
        value = m.group("value")
        if value.strip("\"'") in ("", MARK) or value.lower().strip("\"'") in (
                "true", "false", "null", "none", "yes", "no"):
            return m.group(0)
        return m.group("name") + m.group("sep") + MARK

    return _ASSIGNMENT.sub(assignment, text)
