import json
import os
import pwd
import subprocess
import unittest
from unittest import mock

from helpers import SYNTHETIC_KEY, TempDirTest, fake_identity
from ush.config import (ConfigError, Identity, KeyStore, Settings, apply_environment,
                        check_ollama_url, identity, load_settings, mask, read_private,
                        save_settings)


class FakeSecurity:
    """Stands in for /usr/bin/security; records calls."""

    def __init__(self):
        self.items: dict = {}
        self.calls: list = []

    def __call__(self, argv, input=None, **kwargs):
        self.calls.append((argv, input, kwargs))
        if argv[1] == "-i":
            # add-generic-password ... -w "KEY"
            self.items["ush"] = input.rsplit('-w "', 1)[1].split('"')[0]
            return subprocess.CompletedProcess(argv, 0, "", "")
        if argv[1] == "find-generic-password":
            if "ush" in self.items:
                return subprocess.CompletedProcess(argv, 0, self.items["ush"] + "\n", "")
            return subprocess.CompletedProcess(argv, 44, "", "not found")
        if argv[1] == "delete-generic-password":
            found = self.items.pop("ush", None) is not None
            return subprocess.CompletedProcess(argv, 0 if found else 44, "", "")
        raise AssertionError(argv)


class IdentityTest(unittest.TestCase):
    def test_sudo_uses_the_invoking_users_key_and_roots_history(self):
        me = pwd.getpwuid(os.getuid())
        root = pwd.getpwuid(0)
        with mock.patch("os.geteuid", return_value=0):
            ident = identity({"SUDO_USER": me.pw_name})
        self.assertTrue(ident.is_root)
        self.assertTrue(ident.via_sudo)
        self.assertEqual(ident.owner_home, me.pw_dir)
        self.assertEqual(ident.config_dir(), os.path.join(me.pw_dir, ".config", "ush"))
        self.assertTrue(ident.state_dir().startswith(root.pw_dir))

    def test_root_ignores_xdg_overrides(self):
        ident = Identity(0, "root", "/var/root", 501, "u", "/Users/u")
        with mock.patch.dict(os.environ, {"XDG_CONFIG_HOME": "/tmp/evil"}):
            self.assertEqual(ident.config_dir(), "/Users/u/.config/ush")

    def test_unknown_sudo_user_falls_back_to_root(self):
        with mock.patch("os.geteuid", return_value=0):
            ident = identity({"SUDO_USER": "no-such-user-ush"})
        self.assertFalse(ident.via_sudo)


class KeyStoreTest(TempDirTest):
    def store(self, platform="linux", environ=None, runner=None):
        ident = fake_identity(self.tmp)
        return KeyStore(ident, environ=environ or {}, runner=runner or FakeSecurity(),
                        platform=platform)

    def setUp(self):
        super().setUp()
        patcher = mock.patch.dict(os.environ, {"XDG_CONFIG_HOME": os.path.join(self.tmp, "cfg")})
        patcher.start()
        self.addCleanup(patcher.stop)

    def test_environment_takes_precedence(self):
        keys = self.store(environ={"ANTHROPIC_API_KEY": SYNTHETIC_KEY})
        self.assertEqual(keys.resolve(), (SYNTHETIC_KEY, "environment (ANTHROPIC_API_KEY)"))

    def test_file_store_is_private_and_round_trips(self):
        keys = self.store()
        self.assertEqual(keys.resolve(), (None, "none"))
        where = keys.store(SYNTHETIC_KEY)
        self.assertIn("file", where)
        info = os.stat(keys.credentials_path)
        self.assertEqual(info.st_mode & 0o777, 0o600)
        self.assertEqual(os.stat(os.path.dirname(keys.credentials_path)).st_mode & 0o777, 0o700)
        self.assertEqual(keys.resolve()[0], SYNTHETIC_KEY)
        self.assertEqual(keys.clear(), [keys.credentials_path])
        self.assertEqual(keys.resolve(), (None, "none"))

    def test_insecure_or_linked_credentials_are_refused(self):
        keys = self.store()
        keys.store(SYNTHETIC_KEY)
        os.chmod(keys.credentials_path, 0o644)
        with self.assertRaisesRegex(ConfigError, "chmod 600"):
            keys.resolve()
        os.chmod(keys.credentials_path, 0o600)
        link = os.path.join(self.tmp, "link")
        os.symlink(keys.credentials_path, link)
        with self.assertRaises(ConfigError):
            read_private(link, {os.getuid()})
        with self.assertRaisesRegex(ConfigError, "owned by"):
            read_private(keys.credentials_path, {12345})

    def test_keychain_store_keeps_key_out_of_argv(self):
        security = FakeSecurity()
        keys = self.store(platform="darwin", runner=security)
        with mock.patch("os.access", return_value=True):
            self.assertEqual(keys.store(SYNTHETIC_KEY), "macOS keychain")
            self.assertEqual(keys.resolve(), (SYNTHETIC_KEY, "macOS keychain"))
            self.assertEqual(keys.clear(), ["macOS keychain"])
        for argv, _, _ in security.calls:
            self.assertNotIn(SYNTHETIC_KEY, " ".join(argv))
        self.assertIn(SYNTHETIC_KEY, security.calls[0][1])

    def test_keychain_lookup_under_sudo_runs_as_the_invoking_user(self):
        security = FakeSecurity()
        ident = Identity(0, "root", "/var/root", 501, "alice", "/Users/alice")
        keys = KeyStore(ident, environ={}, runner=security, platform="darwin")
        with mock.patch("os.access", return_value=True), \
                mock.patch("ush.config.read_private", return_value=None):
            keys.resolve()
        _, _, kwargs = security.calls[0]
        self.assertEqual(kwargs["user"], 501)
        self.assertEqual(kwargs["env"]["HOME"], "/Users/alice")

    def test_malformed_keys_are_rejected_before_storage(self):
        keys = self.store(platform="darwin")
        for bad in ("short", 'abc" ; delete-keychain x "' + "a" * 20, "a b" * 10):
            with self.subTest(bad=bad), self.assertRaises(ConfigError):
                keys.store(bad)

    def test_mask(self):
        self.assertNotIn(SYNTHETIC_KEY[10:-4], mask(SYNTHETIC_KEY))


class SettingsTest(TempDirTest):
    def test_round_trip_and_validation(self):
        with mock.patch.dict(os.environ, {"XDG_CONFIG_HOME": os.path.join(self.tmp, "cfg")}):
            ident = fake_identity(self.tmp)
            self.assertEqual(load_settings(ident).model, Settings().model)
            path = save_settings(ident, Settings(model="claude-sonnet-5-5", effort="high"))
            self.assertEqual(load_settings(ident).effort, "high")
            with open(path, "w") as f:
                json.dump({"model": "x; rm", "effort": "ultra", "max_steps": -1}, f)
            loaded = load_settings(ident)
            self.assertEqual((loaded.model, loaded.effort, loaded.max_steps),
                             (Settings().model, "medium", Settings().max_steps))
            with open(path, "w") as f:
                f.write("{broken")
            with self.assertRaises(ConfigError):
                load_settings(ident)


if __name__ == "__main__":
    unittest.main()


class OllamaSettingsTest(TempDirTest):
    def test_ollama_urls_must_be_on_this_machine(self):
        for url, expected in (("http://127.0.0.1:11434", "http://127.0.0.1:11434"),
                              ("http://localhost:11434/", "http://localhost:11434"),
                              ("http://[::1]:11434", "http://[::1]:11434"),
                              ("https://127.0.0.2:8443", "https://127.0.0.2:8443")):
            self.assertEqual(check_ollama_url(url), expected)
        for url in ("http://192.168.1.5:11434", "http://ollama.example.com:11434",
                    "http://127.0.0.1.evil.example", "http://localhost.evil.example",
                    "http://user:pw@127.0.0.1:11434", "http://127.0.0.1:11434/v1",
                    "http://127.0.0.1:11434/?next=http://evil.example", "http://127.0.0.1:99999",
                    "file:///tmp/ollama.sock", "ftp://127.0.0.1", "127.0.0.1:11434", ""):
            with self.subTest(url=url), self.assertRaises(ConfigError):
                check_ollama_url(url)

    def test_settings_round_trip_and_a_remote_url_in_the_file_is_refused(self):
        s = Settings(provider="ollama", ollama_model="qwen3:8b")
        again = Settings.from_dict(s.to_dict())
        self.assertEqual((again.provider, again.ollama_model, again.active_model),
                         ("ollama", "qwen3:8b", "qwen3:8b"))
        self.assertEqual(Settings.from_dict({"provider": "openai"}).provider, "anthropic")
        self.assertEqual(Settings.from_dict({"ollama_model": "bad name!"}).ollama_model, "")
        with self.assertRaises(ConfigError):
            Settings.from_dict({"ollama_url": "http://10.0.0.8:11434"})

    def test_model_names_are_checked_per_provider(self):
        s = Settings()
        s.set_model("claude-sonnet-5-5")
        with self.assertRaises(ConfigError):
            s.set_model("hf.co/org/Model:Q4")
        s.set_provider("ollama")
        s.set_model("hf.co/org/Model:Q4")
        self.assertEqual((s.model, s.ollama_model), ("claude-sonnet-5-5", "hf.co/org/Model:Q4"))
        with self.assertRaises(ConfigError):
            s.set_model("x; rm -rf /")
        with self.assertRaises(ConfigError):
            s.set_provider("openai")

    def test_environment_overrides_apply_to_users_but_not_to_root(self):
        env = {"USH_PROVIDER": "ollama", "USH_MODEL": "llama3.2:3b",
               "USH_OLLAMA_URL": "http://localhost:11500"}
        user = Settings()
        apply_environment(user, fake_identity(self.tmp), env)
        self.assertEqual((user.provider, user.ollama_model, user.ollama_url, user.model),
                         ("ollama", "llama3.2:3b", "http://localhost:11500", "claude-opus-5-5"))
        root = Settings()
        apply_environment(root, fake_identity(self.tmp, uid=0), env)
        self.assertEqual(root, Settings())
        with self.assertRaises(ConfigError):
            apply_environment(Settings(), fake_identity(self.tmp),
                              {"USH_PROVIDER": "ollama", "USH_OLLAMA_URL": "http://evil.example"})


class CheckerSettingsTest(TempDirTest):
    def test_checker_is_provider_and_model_or_off(self):
        s = Settings()
        for value, provider, model in (("ollama:qwen3.5:2b", "ollama", "qwen3.5:2b"),
                                       ("anthropic:claude-haiku-5-5", "anthropic",
                                        "claude-haiku-5-5")):
            s.set_checker(value)
            self.assertEqual((s.checker_provider, s.checker_model), (provider, model))
            self.assertEqual(Settings.from_dict(s.to_dict()).checker, value)
        for off in ("off", "", "  "):
            s.set_checker(off)
            self.assertEqual(s.checker, "")
        for bad in ("bad name!", "ollama:bad name!", "anthropic:x; rm", "anthropic:", "ollama:",
                    "claude-haiku 5"):
            with self.subTest(bad=bad), self.assertRaises(ConfigError):
                s.set_checker(bad)
        with self.assertRaises(ConfigError):
            Settings.from_dict({"checker": "anthropic:"})

    def test_bare_checker_names_choose_the_provider_by_name(self):
        # Whatever the main provider: claude-… is Anthropic's, anything else Ollama's (where
        # names have tags, qwen3.5:4b); stored with the provider, so the setting is unambiguous.
        for main in ("anthropic", "ollama"):
            s = Settings(provider=main)
            for value, stored in (("claude-haiku-5-5", "anthropic:claude-haiku-5-5"),
                                  ("qwen3.5:4b", "ollama:qwen3.5:4b"),
                                  ("llama3.2", "ollama:llama3.2"),
                                  ("library/qwen3.5:4b", "ollama:library/qwen3.5:4b")):
                with self.subTest(main=main, value=value):
                    s.set_checker(value)
                    self.assertEqual(s.checker, stored)
                    self.assertEqual(Settings.from_dict(s.to_dict()).checker, stored)

    def test_checker_from_the_environment_but_not_as_root(self):
        env = {"USH_CHECKER": "ollama:qwen3.5:2b"}
        user = Settings()
        apply_environment(user, fake_identity(self.tmp), env)
        self.assertEqual(user.checker, "ollama:qwen3.5:2b")
        root = Settings(checker="anthropic:claude-haiku-5-5")
        apply_environment(root, fake_identity(self.tmp, uid=0), {"USH_CHECKER": "off"})
        self.assertEqual(root.checker, "anthropic:claude-haiku-5-5")
        off = Settings(checker="ollama:qwen3.5:2b")
        apply_environment(off, fake_identity(self.tmp), {"USH_CHECKER": "off"})
        self.assertEqual(off.checker, "")


class EnvTemplateTest(unittest.TestCase):
    """The model menu in .env.template (TODO 2.12): every entry is a name ush accepts, and the
    `just ush` recipe drops the comment after a value when a line is uncommented."""

    ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")

    def strip_comment(self, value: str) -> str:
        with open(os.path.join(self.ROOT, "justfile"), encoding="utf-8") as f:
            (line,) = [x.strip() for x in f if x.strip().startswith("value=$(printf")]
        script = f'value="$1"\n{line}\nprintf "%s" "$value"'
        return subprocess.run(["bash", "-c", script, "bash", value], capture_output=True,
                              text=True, check=True).stdout

    def test_menu_entries_are_valid_after_uncommenting(self):
        with open(os.path.join(self.ROOT, ".env.template"), encoding="utf-8") as f:
            entries = [line[1:].rstrip("\n").split("=", 1) for line in f
                       if line.startswith(("#USH_MODEL=", "#USH_CHECKER="))]
        self.assertGreaterEqual(len(entries), 10)
        for key, raw in entries:
            value = self.strip_comment(raw)
            with self.subTest(entry=value):
                self.assertNotIn("#", value)
                s = Settings()
                if key == "USH_CHECKER":
                    s.set_checker(value)
                    self.assertEqual(s.checker, value)
                else:
                    s.set_provider("anthropic" if value.startswith("claude-") else "ollama")
                    s.set_model(value)
                    self.assertEqual(s.active_model, value)


class AllowSettingsTest(TempDirTest):
    def test_rules_round_trip_and_bad_rules_are_refused(self):
        s = Settings()
        self.assertEqual(s.add_rule(" kubectl  get "), "kubectl get")
        s.add_rule("kubectl get")
        self.assertEqual(s.allow, ["kubectl get"])
        self.assertEqual(Settings.from_dict(s.to_dict()).allow_rules, (("kubectl", "get"),))
        for bad in ("bash", "rm", "find", "git $(x)"):
            with self.subTest(rule=bad), self.assertRaises(ConfigError):
                s.add_rule(bad)
        for data in ({"allow": "git status"}, {"allow": ["git status", "sh"]}, {"allow": [3]}):
            with self.subTest(data=data), self.assertRaises(ConfigError):
                Settings.from_dict(data)
        self.assertTrue(s.remove_rule("kubectl   get"))
        self.assertFalse(s.remove_rule("kubectl get"))

