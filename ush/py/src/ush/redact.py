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
    re.compile(r"-----BEGIN [A-Z0-9 ]*PRIVATE KEY( BLOCK)?-----.*?"
               r"(-----END [A-Z0-9 ]*PRIVATE KEY( BLOCK)?-----|\Z)", re.DOTALL),
    # Provider and platform tokens
    re.compile(r"\bsk-(?:ant-)?[A-Za-z0-9_\-]{20,}"),
    re.compile(r"\b(?:AKIA|ASIA)[0-9A-Z]{16}\b"),
    re.compile(r"\bgh[pousr]_[A-Za-z0-9]{30,}\b"),
    re.compile(r"\bgithub_pat_[A-Za-z0-9_]{40,}\b"),
    re.compile(r"\bxox[abposr]-[A-Za-z0-9\-]{10,}"),
    re.compile(r"\bAIza[0-9A-Za-z_\-]{35}\b"),
    re.compile(r"\bglpat-[A-Za-z0-9_\-]{20,}\b"),
    re.compile(r"\beyJ[A-Za-z0-9_\-]{10,}\.eyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}"),
    re.compile(r"\b[rs]k_(?:live|test)_[A-Za-z0-9]{16,}"),                 # Stripe
    re.compile(r"\bya29\.[A-Za-z0-9_\-]{20,}"),                            # Google OAuth
    re.compile(r"\bnpm_[A-Za-z0-9]{36}\b"),                                  # npm
    re.compile(r"\bpypi-AgE[A-Za-z0-9_\-]{40,}"),                           # PyPI
    re.compile(r"\bSG\.[A-Za-z0-9_\-]{16,}\.[A-Za-z0-9_\-]{16,}"),          # SendGrid
    re.compile(r"\bhooks\.slack\.com/services/[A-Za-z0-9/]+"),             # Slack webhook
    re.compile(r"\bdop_v1_[a-f0-9]{64}\b"),                                 # DigitalOcean
    re.compile(r"\bhf_[A-Za-z0-9]{30,}\b"),                                 # Hugging Face
    # HTTP authorization values: Bearer and Basic credentials
    re.compile(r"(?i)(?<=\bbearer )[A-Za-z0-9._~+/\-]{12,}=*"),
    re.compile(r"(?i)(?<=\bbasic )[A-Za-z0-9+/]{12,}=*"),
    # .netrc and similar: "password VALUE"
    re.compile(r"(?i)(?<=\bpassword )(?!\[redacted\])\S+"),
    # Credentials embedded in URLs: scheme://user:password@host
    re.compile(r"(?<=://)[^/\s:@]+:[^/\s@]+(?=@)"),
]

# NAME=value, NAME: value, "name": "value" for secret-like names. The name is kept. The parts
# around the keyword are bounded, and a name starts only at the start of a word: otherwise a long
# run of word characters made the search quadratic, and a large output stalled ush (SECTEST F3).
_ASSIGNMENT = re.compile(
    r"""(?ix)
    (?<![\w.\-"'])          # a name starts a word: one try per word, not per character
    (?P<name>["']?[\w.\-]{0,40}(?:password|passwd|passphrase|secret|token|api[_\-]?key|
        access[_\-]?key|private[_\-]?key|account[_\-]?key|auth|credential|session[_\-]?id)
        [\w.\-]{0,40}["']?)
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
