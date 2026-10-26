"""Identity, settings, and API key storage.

Under sudo, ush runs as root but uses the invoking user's key and settings
(found through SUDO_USER), while history is written to root's own state
directory. Environment overrides of the provider endpoint are ignored as root.
"""

from __future__ import annotations

import json
import os
import pwd
import re
import stat
import subprocess
import sys
from dataclasses import dataclass, field
from typing import Callable, Optional

DEFAULT_MODEL = "claude-opus-5-5"
EFFORTS = ("low", "medium", "high", "xhigh", "max")
KEY_ENV = ("USH_API_KEY", "ANTHROPIC_API_KEY")
KEYCHAIN_SERVICE = "ush"
KEYCHAIN_ACCOUNT = "anthropic"
SECURITY = "/usr/bin/security"
KEY_PATTERN = re.compile(r"^[A-Za-z0-9_\-]{20,300}$")


class ConfigError(Exception):
    pass


@dataclass(frozen=True)
class Identity:
    uid: int
    name: str
    home: str
    owner_uid: int          # user whose key and settings are used
    owner_name: str
    owner_home: str

    @property
    def is_root(self) -> bool:
        return self.uid == 0

    @property
    def via_sudo(self) -> bool:
        return self.uid != self.owner_uid

    def config_dir(self) -> str:
        xdg = os.environ.get("XDG_CONFIG_HOME") if not self.is_root else None
        return os.path.join(xdg or os.path.join(self.owner_home, ".config"), "ush")

    def state_dir(self) -> str:
        xdg = os.environ.get("XDG_STATE_HOME") if not self.is_root else None
        return os.path.join(xdg or os.path.join(self.home, ".local", "state"), "ush")


def identity(environ: Optional[dict] = None) -> Identity:
    env = os.environ if environ is None else environ
    uid = os.geteuid()
    me = pwd.getpwuid(uid)
    owner = me
    sudo_user = env.get("SUDO_USER")
    if uid == 0 and sudo_user and sudo_user != "root":
        try:
            owner = pwd.getpwnam(sudo_user)
        except KeyError:
            pass
    return Identity(uid, me.pw_name, me.pw_dir, owner.pw_uid, owner.pw_name, owner.pw_dir)


# ---------------------------------------------------------------------------
# Settings


@dataclass
class Settings:
    model: str = DEFAULT_MODEL
    effort: str = "medium"
    max_steps: int = 30
    command_timeout: int = 120     # default seconds per command
    max_timeout: int = 900         # upper bound a request may ask for
    output_limit: int = 32_000     # bytes of each output sent to the model
    show_lines: int = 12           # output lines shown in the terminal per command
    extra: dict = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: dict) -> "Settings":
        s = cls()
        if isinstance(data.get("model"), str) and re.match(r"^[a-z0-9.\-]{3,64}$", data["model"]):
            s.model = data["model"]
        if data.get("effort") in EFFORTS:
            s.effort = data["effort"]
        for name, low, high in (("max_steps", 1, 200), ("command_timeout", 1, 3600),
                                ("max_timeout", 1, 7200), ("output_limit", 1000, 500_000),
                                ("show_lines", 0, 10_000)):
            value = data.get(name)
            if isinstance(value, int) and low <= value <= high:
                setattr(s, name, value)
        return s

    def to_dict(self) -> dict:
        return {"model": self.model, "effort": self.effort, "max_steps": self.max_steps,
                "command_timeout": self.command_timeout, "max_timeout": self.max_timeout,
                "output_limit": self.output_limit, "show_lines": self.show_lines}


def load_settings(ident: Identity) -> Settings:
    path = os.path.join(ident.config_dir(), "config.json")
    try:
        with open(path, encoding="utf-8") as f:
            data = json.load(f)
    except FileNotFoundError:
        return Settings()
    except (OSError, ValueError) as exc:
        raise ConfigError(f"cannot read {path}: {exc}") from None
    return Settings.from_dict(data if isinstance(data, dict) else {})


def save_settings(ident: Identity, settings: Settings) -> str:
    path = os.path.join(_private_dir(ident.config_dir(), ident), "config.json")
    _write_private(path, json.dumps(settings.to_dict(), indent=2) + "\n", ident)
    return path


# ---------------------------------------------------------------------------
# Private files


def _private_dir(path: str, ident: Identity) -> str:
    os.makedirs(path, mode=0o700, exist_ok=True)
    if ident.is_root and ident.via_sudo:
        _chown_to_owner(path, ident)
    return path


def _chown_to_owner(path: str, ident: Identity) -> None:
    try:
        os.chown(path, ident.owner_uid, -1)
    except OSError:
        pass


def _write_private(path: str, text: str, ident: Identity) -> None:
    tmp = path + ".tmp"
    fd = os.open(tmp, os.O_WRONLY | os.O_CREAT | os.O_TRUNC | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as f:
        f.write(text)
    if ident.is_root and ident.via_sudo:
        _chown_to_owner(tmp, ident)
    os.replace(tmp, path)


def read_private(path: str, allowed_owners: set[int]) -> Optional[str]:
    """Read a file that must be a regular, non-symlink file readable only by its owner."""
    try:
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    except FileNotFoundError:
        return None
    except OSError as exc:
        raise ConfigError(f"cannot open {path}: {exc.strerror}") from None
    with os.fdopen(fd, encoding="utf-8") as f:
        info = os.fstat(f.fileno())
        if not stat.S_ISREG(info.st_mode):
            raise ConfigError(f"{path} is not a regular file")
        if info.st_uid not in allowed_owners:
            raise ConfigError(f"{path} is owned by uid {info.st_uid}; refusing to use it")
        if info.st_mode & 0o077:
            raise ConfigError(f"{path} is accessible by other users; run: chmod 600 {path}")
        return f.read()


# ---------------------------------------------------------------------------
# API key


Runner = Callable[..., subprocess.CompletedProcess]


@dataclass
class KeyStore:
    ident: Identity
    environ: dict = field(default_factory=lambda: dict(os.environ))
    runner: Runner = subprocess.run
    platform: str = sys.platform

    @property
    def credentials_path(self) -> str:
        return os.path.join(self.ident.config_dir(), "credentials")

    def _keychain_available(self) -> bool:
        return self.platform == "darwin" and os.access(SECURITY, os.X_OK)

    def _security(self, args: list[str], stdin: Optional[str] = None):
        kwargs = {}
        if self.ident.is_root and self.ident.via_sudo:
            # Use the invoking user's login keychain.
            kwargs = {"user": self.ident.owner_uid,
                      "env": {"HOME": self.ident.owner_home, "PATH": "/usr/bin:/bin"}}
        return self.runner([SECURITY, *args], input=stdin, capture_output=True, text=True,
                           timeout=20, **kwargs)

    def resolve(self) -> tuple[Optional[str], str]:
        """Return (key, source). A missing key returns (None, "none")."""
        for name in KEY_ENV:
            value = self.environ.get(name, "").strip()
            if value:
                return value, f"environment ({name})"
        if self._keychain_available():
            try:
                result = self._security(["find-generic-password", "-s", KEYCHAIN_SERVICE,
                                         "-a", KEYCHAIN_ACCOUNT, "-w"])
                if result.returncode == 0 and result.stdout.strip():
                    return result.stdout.strip(), "macOS keychain"
            except (OSError, subprocess.SubprocessError):
                pass
        text = read_private(self.credentials_path, {self.ident.owner_uid, 0})
        if text:
            try:
                key = json.loads(text).get("anthropic_api_key", "")
            except (ValueError, AttributeError):
                raise ConfigError(f"{self.credentials_path} is not valid JSON") from None
            if key:
                return key, f"file ({self.credentials_path})"
        return None, "none"

    def store(self, key: str, prefer_keychain: bool = True) -> str:
        if not KEY_PATTERN.match(key):
            raise ConfigError("that does not look like an API key")
        if prefer_keychain and self._keychain_available():
            command = (f'add-generic-password -U -s {KEYCHAIN_SERVICE} -a {KEYCHAIN_ACCOUNT} '
                       f'-l "ush API key" -w "{key}"\n')
            result = self._security(["-i"], stdin=command)  # stdin keeps the key out of argv
            if result.returncode == 0:
                self._remove_file()
                return "macOS keychain"
        _private_dir(self.ident.config_dir(), self.ident)
        _write_private(self.credentials_path, json.dumps({"anthropic_api_key": key}) + "\n",
                       self.ident)
        return f"file ({self.credentials_path})"

    def clear(self) -> list[str]:
        removed = []
        if self._keychain_available():
            result = self._security(["delete-generic-password", "-s", KEYCHAIN_SERVICE,
                                     "-a", KEYCHAIN_ACCOUNT])
            if result.returncode == 0:
                removed.append("macOS keychain")
        if self._remove_file():
            removed.append(self.credentials_path)
        return removed

    def _remove_file(self) -> bool:
        try:
            os.unlink(self.credentials_path)
            return True
        except FileNotFoundError:
            return False


def mask(key: str) -> str:
    return key[:7] + "…" + key[-4:] if len(key) > 16 else "…"
