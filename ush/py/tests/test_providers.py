"""The provider table and model profiles (TODO 2.29.1): what each provider is, what each model
takes, and how the adapters map each profile setting onto a request."""

import dataclasses
import unittest

import helpers  # noqa: F401  puts ush/py/src on the path
from helpers import SYNTHETIC_KEY

from ush.agent import TOOLS
from ush.config import PROVIDERS, Settings
from ush.providers import FALLBACK_MODELS, PROVIDERS as TABLE, UPDATES_MODELS, profile

try:
    import anthropic  # noqa: F401
    HAVE_SDK = True
except ImportError:
    HAVE_SDK = False


class TableTest(unittest.TestCase):
    def test_every_row_names_itself_and_holds_its_settings(self):
        self.assertEqual(PROVIDERS, tuple(TABLE))
        settings = Settings()
        for name, spec in TABLE.items():
            with self.subTest(provider=name):
                self.assertEqual(spec.name, name)
                self.assertTrue(spec.label)
                self.assertTrue(hasattr(settings, spec.model_setting))
                if spec.url_setting:
                    self.assertEqual(getattr(settings, spec.url_setting), spec.url)
                # A provider on this machine needs no key; one elsewhere does.
                self.assertNotEqual(spec.loopback, spec.key_store)

    def test_anthropic_and_ollama(self):
        anthropic, ollama = TABLE["anthropic"], TABLE["ollama"]
        self.assertEqual((anthropic.url, anthropic.loopback, anthropic.key_env),
                         ("https://api.anthropic.com", False, ("USH_API_KEY", "ANTHROPIC_API_KEY")))
        self.assertTrue(anthropic.model_pattern.match(Settings().model))
        self.assertEqual((ollama.url, ollama.url_setting, ollama.loopback, ollama.key_env),
                         ("http://127.0.0.1:11434", "ollama_url", True, ()))
        self.assertTrue(ollama.model_pattern.match("qwen3.5:9b"))
        self.assertFalse(anthropic.model_pattern.match("qwen3.5:9b"))

    def test_settings_go_through_the_table(self):
        s = Settings()
        s.set_provider("ollama")
        s.set_model("llama3.2:3b")
        self.assertEqual((s.ollama_model, s.active_model, s.model_of("anthropic")),
                         ("llama3.2:3b", "llama3.2:3b", "claude-opus-5-5"))
        with self.assertRaisesRegex(Exception, "not an Ollama model name"):
            s.set_model("bad name!")


class ProfileTest(unittest.TestCase):
    def test_anthropic_models(self):
        for model in sorted(UPDATES_MODELS | FALLBACK_MODELS | {"claude-haiku-5-5"}):
            with self.subTest(model=model):
                p = profile("anthropic", model)
                self.assertTrue(p.effort and p.strict_tools and p.eager_tool_input and p.prompt_cache)
                self.assertFalse(p.text_tool_calls)
                self.assertEqual(p.thinking, "adaptive-updates" if model in UPDATES_MODELS
                                 else "adaptive")
                self.assertEqual(p.server_fallbacks, model in FALLBACK_MODELS)
                self.assertEqual(p.output_limit, 64000)

    def test_ollama_models(self):
        p = profile("ollama", "llama3.2:3b")
        self.assertEqual((p.effort, p.thinking, p.server_fallbacks, p.eager_tool_input,
                          p.prompt_cache, p.strict_tools, p.text_tool_calls),
                         (False, "", False, False, False, True, True))


@unittest.skipUnless(HAVE_SDK, "anthropic SDK not installed (pip install -r requirements.lock)")
class MappingTest(unittest.TestCase):
    """Each profile setting, on and off, as the Messages adapter sends it."""

    def params(self, **changes):
        from ush.provider import AnthropicProvider
        provider = AnthropicProvider(SYNTHETIC_KEY, "claude-opus-5-5", "high",
                                     base_url="http://127.0.0.1:9", max_retries=0)
        provider.profile = dataclasses.replace(provider.profile, **changes)
        return provider._params("sys", TOOLS, [{"role": "user", "content": "x"}])

    def test_each_setting(self):
        full = self.params()
        self.assertEqual(full["output_config"], {"effort": "high"})
        self.assertEqual(full["thinking"], {"type": "adaptive", "display": "updates"})
        self.assertEqual(full["fallbacks"], "default")
        self.assertEqual(len(full["betas"]), 2)
        self.assertEqual(full["cache_control"], {"type": "ephemeral"})
        self.assertTrue(all(t["eager_input_streaming"] and t["strict"] for t in full["tools"]))
        self.assertEqual(full["max_tokens"], 64000)

        self.assertNotIn("output_config", self.params(effort=False))
        self.assertEqual(self.params(thinking="adaptive")["thinking"], {"type": "adaptive"})
        self.assertNotIn("thinking", self.params(thinking=""))
        self.assertNotIn("fallbacks", self.params(server_fallbacks=False))
        self.assertNotIn("cache_control", self.params(prompt_cache=False))
        self.assertTrue(all("eager_input_streaming" not in t
                            for t in self.params(eager_tool_input=False)["tools"]))
        self.assertTrue(all("strict" not in t for t in self.params(strict_tools=False)["tools"]))
        self.assertEqual(self.params(output_limit=2048)["max_tokens"], 2048)
        bare = self.params(effort=False, thinking="", server_fallbacks=False, prompt_cache=False,
                           eager_tool_input=False)
        self.assertEqual(set(bare), {"model", "max_tokens", "system", "tools", "messages"})

    def test_ollama_sends_only_what_its_profile_takes(self):
        from ush.provider import OllamaProvider
        params = OllamaProvider("llama3.2:3b", "http://127.0.0.1:9", max_retries=0)._params(
            "sys", TOOLS, [{"role": "user", "content": "x"}])
        self.assertEqual(set(params), {"model", "max_tokens", "system", "tools", "messages"})
        self.assertEqual(params["tools"], TOOLS)


if __name__ == "__main__":
    unittest.main()
