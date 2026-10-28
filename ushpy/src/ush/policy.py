"""Decide which shell commands ush may run without asking the user.

The policy is a convenience gate, not a sandbox. A command line runs without a
prompt only if ush can parse it completely, every simple command in it is a known
read-only invocation, and no file-content reader touches a known secret location.
Everything else needs explicit approval. Unknown syntax is never auto-approved.
"""

from __future__ import annotations

import glob
import os
import re
from dataclasses import dataclass
from typing import Callable, Optional

# Commands from these directories may be named by absolute path.
SYSTEM_DIRS = ("/bin", "/usr/bin", "/sbin", "/usr/sbin", "/usr/local/bin", "/opt/homebrew/bin")

SEPARATORS = {"|", "|&", "&&", "||", ";"}
OUTPUT_REDIRECTS = {">", ">>", ">|", "&>", "&>>"}
# Unquoted runs of these characters are operators; quoted ones stay inside words.
OPERATOR_CHARS = set("();<>|&")
FORBIDDEN_FRAGMENTS = {
    "`": "command substitution",
    "$(": "command substitution",
    "${": "parameter expansion",
    "$'": "ANSI-C quoting",
    "$[": "arithmetic expansion",
    "<(": "process substitution",
    ">(": "process substitution",
    "=(": "process substitution",
}
MAX_GLOB_MATCHES = 2000


@dataclass(frozen=True)
class Verdict:
    auto: bool
    reason: str = ""
    sudo: bool = False
    rule: str = ""          # the user's allow rule that let a command run, if one did


def _reject(reason: str, sudo: bool = False) -> Verdict:
    return Verdict(False, reason, sudo)


# ---------------------------------------------------------------------------
# Secret locations. Reading their contents (not listing them) requires approval.

SECRET_COMPONENTS = {
    ".ssh", ".gnupg", ".aws", ".azure", ".kube", ".docker", ".password-store",
    "Keychains", ".netrc", ".pgpass", ".git-credentials", ".npmrc", ".pypirc",
    ".vault-token", "1Password", "Bitwarden", ".terraform.d", "gcloud",
}
SECRET_PATH_PARTS = ("/.config/ush", "/.config/anthropic", "/.config/gh", "/.local/state/ush")
SECRET_NAME_PATTERNS = re.compile(
    r"^(id_[a-z0-9_]+|.*\.(pem|key|p12|pfx|kdbx|keychain(-db)?|gpg|asc)|\.env(\..*)?"
    r"|.*(secret|token|passw|credential).*|master\.passwd|shadow|gshadow|sudoers"
    r"|ssh_host_.*_key)$",
    re.IGNORECASE,
)
SECRET_ABSOLUTE = (
    "/etc/sudoers.d", "/private/etc/sudoers.d", "/var/db/dslocal", "/private/var/db/dslocal",
    "/Library/Keychains", "/var/root", "/root", "/etc/ssh", "/private/etc/ssh",
)
HOME_PARENTS = ("/Users", "/home")


def _is_secret(path: str) -> bool:
    if any(path == p or path.startswith(p + "/") for p in SECRET_ABSOLUTE):
        return True
    if any(part in path + "/" for part in (p + "/" for p in SECRET_PATH_PARTS)):
        return True
    parts = [p for p in path.split("/") if p]
    if any(p in SECRET_COMPONENTS for p in parts):
        return True
    return bool(parts) and bool(SECRET_NAME_PATTERNS.match(parts[-1]))


def _covers_secrets(directory: str) -> bool:
    """True if recursively reading `directory` would reach a secret location."""
    d = directory.rstrip("/") or "/"
    if d == "/":
        return True
    if _is_secret(d):
        return True
    roots = SECRET_ABSOLUTE + HOME_PARENTS + (os.path.expanduser("~"),)
    homes = [os.path.join(h, n) for h in HOME_PARENTS if os.path.isdir(h)
             for n in _listdir(h)]
    return any(r == d or r.startswith(d + "/") for r in (*roots, *homes))


def _listdir(path: str) -> list[str]:
    try:
        return os.listdir(path)
    except OSError:
        return []


def _check_paths(words: list[str], recursive: bool) -> Optional[str]:
    """Check expanded arguments that are paths: existing files or words with a '/'.

    Other words (search patterns, keys) are skipped, so `grep token f` is not
    mistaken for reading a file named "token".
    """
    for word in words:
        if word.startswith("-") or not word or not ("/" in word or os.path.lexists(word)):
            continue
        path = os.path.realpath(word)
        if _is_secret(path):
            return f"reads a secret location ({path})"
        if recursive and os.path.isdir(path) and _covers_secrets(path):
            return f"reads {path} recursively, which includes secret locations"
    return None


# ---------------------------------------------------------------------------
# Known commands. A validator returns None if the arguments are acceptable, or
# the reason the invocation needs approval.

Validator = Callable[[list[str]], Optional[str]]
META, CONTENT, RECURSIVE = "meta", "content", "recursive"


def _any(_: list[str]) -> Optional[str]:
    return None


def _no_args(args: list[str]) -> Optional[str]:
    return "arguments change its behavior" if args else None


def _short_flags(args: list[str]) -> set[str]:
    flags = set()
    for a in args:
        if a.startswith("-") and not a.startswith("--") and len(a) > 1:
            flags.update(a[1:])
    return flags


def _forbid(*options: str, letters: str = "") -> Validator:
    def check(args: list[str]) -> Optional[str]:
        for a in args:
            base = a.split("=", 1)[0]
            if base in options:
                return f"option {base} may change the system"
        bad = _short_flags(args) & set(letters)
        if bad:
            return f"option -{''.join(sorted(bad))} may change the system"
        return None
    return check


def _first_arg_in(*allowed: str, extra: Optional[Validator] = None) -> Validator:
    def check(args: list[str]) -> Optional[str]:
        words = [a for a in args if not a.startswith("-")]
        if not words or words[0] not in allowed:
            return "only some subcommands are known to be read-only"
        return extra(args) if extra else None
    return check


def _prefix_in(*allowed: tuple[str, ...]) -> Validator:
    def check(args: list[str]) -> Optional[str]:
        if any(tuple(args[: len(p)]) == p for p in allowed):
            return None
        return "only some subcommands are known to be read-only"
    return check


def _find(args: list[str]) -> Optional[str]:
    bad = {"-delete", "-exec", "-execdir", "-ok", "-okdir", "-fprint", "-fprint0",
           "-fprintf", "-fls"}
    for a in args:
        if a in bad:
            return f"find {a} runs commands or changes files"
    return None


SED_PRINT = re.compile(r"^((\d+|\$)(,(\d+|\$))?|/[^/]*/)?[pdq]$")
SED_SUBST = re.compile(r"^s(?P<d>[^\w\s\\])(?:\\.|(?!(?P=d)).)*(?P=d)(?:\\.|(?!(?P=d)).)*(?P=d)[gIip0-9]*$")


def _sed(args: list[str]) -> Optional[str]:
    scripts, positional, expect_script = [], [], False
    for a in args:
        if expect_script:
            scripts.append(a)
            expect_script = False
        elif a in ("-e", "--expression"):
            expect_script = True
        elif a in ("--quiet", "--silent", "--regexp-extended") or (
                a.startswith("-") and not a.startswith("--") and len(a) > 1 and set(a[1:]) <= set("nEru")):
            pass
        elif a.startswith("-"):
            return f"sed option {a} is not known to be read-only"
        else:
            positional.append(a)
    if not scripts:
        if not positional:
            return "sed without a script"
        scripts = positional[:1]
    for script in scripts:
        for part in script.split(";"):
            if not (SED_PRINT.match(part.strip()) or SED_SUBST.match(part.strip())):
                return "sed script is not a simple print or substitution"
    return None


def _awk(args: list[str]) -> Optional[str]:
    program = next((a for a in args if not a.startswith("-")), "")
    if any(a.startswith("-f") for a in args):
        return "awk programs from files are not checked"
    for word in ("system", "getline", "|", ">", "close", "fflush", "ENVIRON", "printf >"):
        if word in program:
            return f"awk program uses {word!r}"
    return None


def _jq(args: list[str]) -> Optional[str]:
    if any(("env" in a or "$ENV" in a or "input_filename" in a) for a in args if not a.startswith("-")):
        return "jq program reads the environment"
    return None


def _xargs_split(args: list[str]) -> tuple[list[str], list[str]]:
    takes_value = {"-n", "-I", "-P", "-L", "-s", "-d", "-E", "-a", "--max-args", "--max-procs",
                   "--delimiter", "--arg-file"}
    i = 0
    while i < len(args) and args[i].startswith("-"):
        i += 2 if args[i] in takes_value else 1
    return args[:i], args[i:]


def _ps(args: list[str]) -> Optional[str]:
    for a in args:
        if a.startswith("-") and "E" in a[1:] or (not a.startswith("-") and "e" in a):
            return "ps would print process environments"
    return None


def _command(args: list[str]) -> Optional[str]:
    return None if args[:1] in (["-v"], ["-V"]) else "command runs another command"


def _top(args: list[str]) -> Optional[str]:
    return None if ("-l" in args or "-b" in args) else "top needs -l (macOS) or -b (Linux) to exit"


def _date(args: list[str]) -> Optional[str]:
    if all(a.startswith("+") or a in ("-u", "-R", "-I", "--utc") for a in args):
        return None
    return "date with arguments may set the clock"


def _sysctl(args: list[str]) -> Optional[str]:
    if "w" in _short_flags(args) or any("=" in a for a in args):
        return "sysctl would change a kernel setting"
    return None


def _ifconfig(args: list[str]) -> Optional[str]:
    if not args or args in (["-a"], ["-l"]) or (len(args) == 1 and not args[0].startswith("-")):
        return None
    return "ifconfig with arguments may reconfigure an interface"


def _ip(args: list[str]) -> Optional[str]:
    mutating = {"add", "del", "delete", "change", "replace", "flush", "set", "append",
                "prepend", "exec", "save", "restore", "up", "down"}
    return "ip would change network configuration" if mutating & set(args) else None


def _positional_at_most(n: int) -> Validator:
    def check(args: list[str]) -> Optional[str]:
        if len([a for a in args if not a.startswith("-")]) > n:
            return "an extra file argument would be written"
        return None
    return check


def _all(*validators: Validator) -> Validator:
    def check(args: list[str]) -> Optional[str]:
        for v in validators:
            reason = v(args)
            if reason:
                return reason
        return None
    return check


COMMANDS: dict[str, tuple[str, Validator]] = {
    # metadata and system state
    "ls": (META, _any), "du": (META, _any), "df": (META, _any), "stat": (META, _any),
    "file": (META, _any), "wc": (META, _any), "find": (META, _find), "tree": (META, _any),
    "which": (META, _any), "whereis": (META, _any), "type": (META, _any),
    "basename": (META, _any), "dirname": (META, _any), "realpath": (META, _any),
    "readlink": (META, _any), "pwd": (META, _any), "cd": (META, _any),
    "command": (META, _command), "whoami": (META, _any),
    "id": (META, _any), "groups": (META, _any), "hostname": (META, _no_args),
    "uname": (META, _any), "sw_vers": (META, _any), "uptime": (META, _any),
    "date": (META, _date), "sysctl": (META, _sysctl), "vm_stat": (META, _any),
    "vmstat": (META, _any), "iostat": (META, _any), "ps": (META, _ps),
    "pgrep": (META, _any), "top": (META, _top), "lsof": (META, _any),
    "mount": (META, _no_args), "nproc": (META, _any), "free": (META, _any),
    "lsblk": (META, _any), "findmnt": (META, _any), "lscpu": (META, _any),
    "lspci": (META, _any), "lsusb": (META, _any), "lsmod": (META, _any),
    "getconf": (META, _any), "locale": (META, _any), "echo": (META, _any),
    "printf": (META, _any), "true": (META, _any), "false": (META, _any),
    "test": (META, _any), "[": (META, _any), "sleep": (META, _any),
    "md5": (META, _any), "md5sum": (META, _any), "shasum": (META, _any),
    "sha256sum": (META, _any), "cksum": (META, _any),
    "netstat": (META, _any), "ss": (META, _any), "ip": (META, _ip),
    "ifconfig": (META, _ifconfig), "dig": (META, _any), "host": (META, _any),
    "nslookup": (META, _any),
    # macOS
    "diskutil": (META, _prefix_in(("list",), ("info",), ("apfs", "list"),
                                  ("apfs", "listSnapshots"), ("apfs", "listVolumeGroups"))),
    "tmutil": (META, _first_arg_in("listlocalsnapshots", "listlocalsnapshotdates",
                                   "listbackups", "destinationinfo", "status", "latestbackup",
                                   "machinedirectory", "isexcluded")),
    "log": (CONTENT, _first_arg_in("show")),
    "system_profiler": (META, _any),
    "pmset": (META, lambda a: None if a[:1] == ["-g"] else "only pmset -g is read-only"),
    "launchctl": (META, _first_arg_in("list", "print", "print-disabled", "blame")),
    "defaults": (CONTENT, _first_arg_in("read", "read-type", "domains")),
    "mdutil": (META, lambda a: None if set(a) <= {"-s", "-a", "-sa", "-as"} or
               (a[:1] == ["-s"]) else "only mdutil -s is read-only"),
    "mdfind": (META, _any), "csrutil": (META, _first_arg_in("status")),
    "spctl": (META, lambda a: None if a[:1] in (["--status"], ["--assess"], ["-a"])
              else "only spctl --status/--assess are read-only"),
    "ioreg": (META, _any),
    "networksetup": (META, lambda a: None if a and (a[0].startswith("-get") or
                     a[0].startswith("-list")) else "only -get*/-list* are read-only"),
    "scutil": (META, lambda a: None if a and a[0] in ("--dns", "--proxy", "--nwi", "--get")
               else "only scutil --dns/--proxy/--nwi/--get are read-only"),
    "fdesetup": (META, _first_arg_in("status", "isactive")),
    "kmutil": (META, _first_arg_in("showloaded")),
    "softwareupdate": (META, lambda a: None if a[:1] in (["-l"], ["--list"], ["--history"])
                       else "only softwareupdate --list/--history are read-only"),
    "pkgutil": (META, lambda a: None if a and a[0] in ("--pkgs", "--pkg-info", "--files",
                "--file-info", "--pkgs-plist") else "only pkgutil queries are read-only"),
    "xcode-select": (META, lambda a: None if a in (["-p"], ["--print-path"], ["-v"])
                     else "only xcode-select -p is read-only"),
    "nvram": (META, lambda a: None if a and set(a) <= {"-p", "-x", "-xp", "-px"}
              else "nvram with arguments may change firmware variables"),
    "plutil": (CONTENT, lambda a: None if a[:1] in (["-p"], ["-lint"])
               else "only plutil -p/-lint are read-only"),
    # Linux services
    "journalctl": (CONTENT, _forbid("--vacuum-size", "--vacuum-time", "--vacuum-files",
                                    "--rotate", "--flush", "--sync", "--relinquish-var",
                                    "--setup-keys", "--follow", letters="f")),
    "systemctl": (META, _first_arg_in("status", "list-units", "list-unit-files",
                                      "list-timers", "show", "is-active", "is-enabled",
                                      "is-failed", "cat", "list-dependencies")),
    "dmesg": (CONTENT, _forbid("--clear", "--read-clear", "--console-off", "--console-on",
                               "--console-level", "--follow", letters="cCDEnw")),
    # tools with read-only subcommands
    "docker": (META, _prefix_in(("system", "df"), ("ps",), ("images",), ("image", "ls"),
                                ("volume", "ls"), ("container", "ls"), ("info",),
                                ("version",))),
    "brew": (META, lambda a: None if a and (a[0] in ("list", "ls", "info", "config",
             "--cache", "--prefix", "--cellar", "--repository", "outdated", "leaves",
             "deps", "uses", "doctor", "--version") or (a[0] == "cleanup" and
             ("-n" in a or "--dry-run" in a))) else "this brew command changes packages"),
    "xcrun": (META, _prefix_in(("simctl", "list"))),
    # file contents
    "cat": (CONTENT, _any), "head": (CONTENT, _any),
    "tail": (CONTENT, _forbid("--follow", letters="fF")),
    "grep": (CONTENT, _any), "egrep": (CONTENT, _any), "fgrep": (CONTENT, _any),
    "rg": (RECURSIVE, _forbid("--pre", "--pre-glob")), "strings": (CONTENT, _any),
    "od": (CONTENT, _any), "hexdump": (CONTENT, _any),
    "xxd": (CONTENT, _all(_forbid(letters="r"), _positional_at_most(1))),
    "sort": (CONTENT, _forbid("--output", "--compress-program", letters="o")),
    "uniq": (CONTENT, _positional_at_most(1)),
    "cut": (CONTENT, _any), "tr": (CONTENT, _any), "column": (CONTENT, _any),
    "nl": (CONTENT, _any), "fold": (CONTENT, _any), "paste": (CONTENT, _any),
    "comm": (CONTENT, _any), "diff": (CONTENT, _any), "cmp": (CONTENT, _any),
    "base64": (CONTENT, _forbid("--output", letters="o")),
    "sed": (CONTENT, _sed), "awk": (CONTENT, _awk), "jq": (CONTENT, _jq),
}


def _recursive(name: str, args: list[str]) -> bool:
    if name == "rg":
        return True
    if name in ("grep", "egrep", "fgrep"):
        return bool({"r", "R"} & _short_flags(args)) or any(
            a in ("--recursive", "--dereference-recursive", "--directories=recurse") for a in args)
    return False


# ---------------------------------------------------------------------------
# Parsing. The tokenizer follows `zsh -f` word rules closely enough to see the
# arguments a command will actually receive: quotes, backslashes, $NAME, leading
# ~, and unquoted globs are expanded here. Constructs it does not model (brace
# expansion, special parameters, a leading `=`) make the command need approval.


class Unsupported(ValueError):
    pass


@dataclass
class Token:
    op: bool
    text: str = ""                    # operator text
    words: tuple[str, ...] = ()       # expanded words
    adjacent_op: bool = False         # a word immediately followed by an operator


def _variable(command: str, i: int) -> tuple[str, int]:
    match = re.match(r"\$([A-Za-z_][A-Za-z0-9_]*)", command[i:])
    if not match:
        raise Unsupported("uses a special shell parameter")
    return os.environ.get(match.group(1), ""), i + match.end()


def _word(command: str, i: int) -> tuple[Token, int]:
    n = len(command)
    literal: list[str] = []
    pattern: list[str] = []   # glob pattern: quoted and expanded text escaped
    magic = False
    start = i
    while i < n and command[i] not in " \t" and command[i] not in OPERATOR_CHARS:
        c = command[i]
        if c == "'":
            end = command.find("'", i + 1)
            if end < 0:
                raise Unsupported("has an unterminated quote")
            text, i = command[i + 1:end], end + 1
            literal.append(text)
            pattern.append(glob.escape(text))
        elif c == '"':
            i += 1
            buf: list[str] = []
            while True:
                if i >= n:
                    raise Unsupported("has an unterminated quote")
                c = command[i]
                if c == '"':
                    i += 1
                    break
                if c == "\\" and i + 1 < n and command[i + 1] in '"\\$`':
                    buf.append(command[i + 1])
                    i += 2
                elif c == "$":
                    value, i = _variable(command, i)
                    buf.append(value)
                else:
                    buf.append(c)
                    i += 1
            literal.append("".join(buf))
            pattern.append(glob.escape("".join(buf)))
        elif c == "\\":
            if i + 1 >= n:
                raise Unsupported("ends with a backslash")
            literal.append(command[i + 1])
            pattern.append(glob.escape(command[i + 1]))
            i += 2
        elif c == "$":
            value, i = _variable(command, i)  # zsh neither splits nor globs the value
            literal.append(value)
            pattern.append(glob.escape(value))
        elif c == "{" and command.startswith("{}", i):
            literal.append("{}")  # empty braces are literal in zsh: find -exec ... {} +
            pattern.append(glob.escape("{}"))
            i += 2
            continue
        elif c in "{}":
            raise Unsupported("uses brace expansion")
        elif c == "#" and i == start:
            raise Unsupported("contains a comment")
        elif c == "=" and i == start:
            raise Unsupported("uses = expansion")
        else:
            magic = magic or c in "*?["
            literal.append(c)
            pattern.append(c)
            i += 1
    text, pat = "".join(literal), "".join(pattern)
    if command[start] == "~":  # unquoted leading tilde
        head, sep, tail = text.partition("/")
        home = os.path.expanduser(head)
        text = home + sep + tail
        pat = glob.escape(home) + pat[len(head):]
    if magic:
        matches = sorted(glob.glob(pat, recursive="**" in pat))
        if len(matches) > MAX_GLOB_MATCHES:
            raise Unsupported("uses a pattern that matches too many files to check")
        words = tuple(matches)  # no match: zsh reports an error and runs nothing
    else:
        words = (text,)
    return Token(False, words=words, adjacent_op=i < n and command[i] in OPERATOR_CHARS), i


def _tokens(command: str) -> list[Token]:
    tokens, i, n = [], 0, len(command)
    while i < n:
        if command[i] in " \t":
            i += 1
        elif command[i] in OPERATOR_CHARS:
            j = i
            while j < n and command[j] in OPERATOR_CHARS:
                j += 1
            tokens.append(Token(True, text=command[i:j]))
            i = j
        else:
            token, i = _word(command, i)
            tokens.append(token)
    return tokens


# ---------------------------------------------------------------------------
# The user's allowlist: rules that let commands the table doesn't know run without approval.
# A rule is a command name and leading arguments ("git status", "kubectl get"); it matches a
# simple command whose words start with it. Rules extend the table and never override it: a
# command the table knows keeps its validator, and commands that run other commands can't be
# allowed, since that would allow anything. Everything else in the policy still applies: shell
# syntax, redirections, secret paths, the model's "change", --confirm, and the checker.

RULE_WORD = re.compile(r"^[A-Za-z0-9._:@%+=,-]+$")
RULE_WORDS = 8
RUNNERS = frozenset({
    "sh", "bash", "zsh", "dash", "ksh", "mksh", "fish", "csh", "tcsh", "busybox",
    "env", "sudo", "doas", "su", "xargs", "nice", "nohup", "timeout", "time", "exec", "eval",
    "source", ".", "command", "builtin", "watch", "parallel", "script", "expect", "chroot",
    "ssh", "osascript", "open", "launchctl", "at", "batch", "crontab", "screen", "tmux",
    "python", "perl", "ruby", "node", "deno", "bun", "php", "lua", "tclsh", "R", "Rscript",
    "java", "go", "make", "npm", "npx", "pip", "uv", "uvx", "git-shell",
})
INTERPRETER = re.compile(r"^(python|perl|ruby|node|php|lua|pip)[0-9.]*$")
# Commands whose purpose is to change or send something. A rule would let an injected "read"
# claim run them unasked; ush asks for them every time.
CHANGERS = frozenset({
    "rm", "rmdir", "mv", "cp", "ln", "dd", "install", "mkfs", "newfs", "fdisk", "truncate",
    "shred", "srm", "chmod", "chown", "chgrp", "chflags", "xattr", "touch", "mkdir", "tee",
    "kill", "killall", "pkill", "shutdown", "reboot", "halt", "systemctl", "service",
    "rsync", "scp", "sftp", "ftp", "curl", "wget", "nc", "ncat", "socat", "telnet", "mail",
    "sendmail", "security", "passwd", "useradd", "userdel", "usermod", "dscl", "visudo",
})


def parse_rule(text: str) -> tuple[str, ...]:
    """The rule's words; ValueError with the reason if `text` can't be an allow rule."""
    words = tuple(text.split())
    if not words:
        raise ValueError("an allow rule needs a command name")
    if len(words) > RULE_WORDS:
        raise ValueError(f"an allow rule has at most {RULE_WORDS} words")
    for word in words:
        if not RULE_WORD.match(word):
            raise ValueError(f"{word!r}: allow rules are plain words, without paths, quotes, "
                             "globs, variables, or shell syntax")
    name = words[0]
    if name.startswith("-"):
        raise ValueError("an allow rule starts with a command name")
    if name in RUNNERS or INTERPRETER.match(name):
        raise ValueError(f"{name} runs other commands or code, so allowing it would allow "
                         "anything")
    if name in CHANGERS:
        raise ValueError(f"{name} changes or sends data; ush asks before it every time")
    if name in COMMANDS:
        raise ValueError(f"{name} is in ush's own read-only table; its check decides, and a rule "
                         "can't widen it")
    return words


def _allowed(name: str, args: list[str], rules: tuple[tuple[str, ...], ...]) -> str:
    """The rule that allows this simple command, or ""."""
    words = [name, *args]
    for rule in rules:
        if words[:len(rule)] == list(rule):
            return " ".join(rule)
    return ""


def _simple_command(words: list[str], rules: tuple[tuple[str, ...], ...] = ()) -> Verdict:
    if not words:
        return _reject("empty command in a pipeline or list")
    if re.match(r"^[A-Za-z_][A-Za-z0-9_]*=", words[0]):
        return _reject("sets environment variables for the command")
    name, args = words[0], words[1:]
    if name == "sudo":
        return _reject("runs a command with sudo", sudo=True)
    if "/" in name:
        if os.path.dirname(name) not in SYSTEM_DIRS:
            return _reject(f"runs {name}, which is not a known system command")
        name = os.path.basename(name)
    if name == "xargs":
        _, rest = _xargs_split(args)
        if not rest:
            return _reject("xargs without a command")
        inner = _simple_command(rest)
        if not inner.auto:
            return inner
        kind = COMMANDS.get(os.path.basename(rest[0]), (None, None))[0]
        if kind != META:
            return _reject("xargs passes unchecked paths to a file reader")
        return Verdict(True)
    entry = COMMANDS.get(name)
    if entry is None:
        rule = _allowed(name, args, rules)
        if not rule:
            return _reject(f"{name} is not a known read-only command")
        reason = _check_paths(args, recursive=False)
        if reason:
            return _reject(reason)
        return Verdict(True, rule=rule)
    kind, validator = entry
    reason = validator(args)
    if reason:
        return _reject(reason)
    if kind in (CONTENT, RECURSIVE):
        reason = _check_paths(args, recursive=kind == RECURSIVE or _recursive(name, args))
        if reason:
            return _reject(reason)
    return Verdict(True)


def classify(command: str, rules: tuple[tuple[str, ...], ...] = ()) -> Verdict:
    """Return whether `command` (one shell command line) may run without approval. `rules` is
    the user's allowlist (parse_rule); a root session passes none."""
    if not command.strip():
        return _reject("empty command")
    if "\n" in command or "\r" in command:
        return _reject("spans several lines")
    for fragment, what in FORBIDDEN_FRAGMENTS.items():
        if fragment in command:
            return _reject(f"uses {what}")
    try:
        tokens = _tokens(command)
    except Unsupported as exc:
        return _reject(str(exc))

    commands: list[list[str]] = [[]]
    sudo = False
    i = 0
    while i < len(tokens):
        tok = tokens[i]
        if not tok.op:
            commands[-1].extend(tok.words)
            i += 1
            continue
        target = tokens[i + 1] if i + 1 < len(tokens) and not tokens[i + 1].op else None
        if tok.text in SEPARATORS:
            commands.append([])
        elif tok.text in OUTPUT_REDIRECTS or tok.text == ">&":
            previous = tokens[i - 1] if i else None
            if previous and not previous.op and previous.adjacent_op and \
                    len(previous.words) == 1 and previous.words[0].isdigit():
                commands[-1].pop()  # the file descriptor of "2>"
            if target is None:
                return _reject("has a redirection without a target")
            if not (target.words == ("/dev/null",) or
                    (tok.text == ">&" and len(target.words) == 1 and target.words[0].isdigit())):
                return _reject("redirects output to a file")
            i += 1
        elif tok.text == "<":
            if target is None:
                return _reject("has a redirection without a target")
            reason = _check_paths(list(target.words), recursive=False)
            if reason:
                return _reject(reason)
            i += 1
        else:
            return _reject(f"uses shell syntax {tok.text!r}")
        i += 1

    used = []
    for words in commands:
        verdict = _simple_command(words, rules)
        sudo = sudo or verdict.sudo
        if not verdict.auto:
            return _reject(verdict.reason, sudo)
        if verdict.rule and verdict.rule not in used:
            used.append(verdict.rule)
    return Verdict(True, rule=", ".join(used))
