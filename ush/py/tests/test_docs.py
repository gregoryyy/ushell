"""ush's documents are shared by both implementations, in ush/docs: the user guide covers the
command line, there is one copy of each document, and every follow-up item in TODO says where it
is built."""

import os
import re
import unittest

import helpers  # noqa: F401  puts ush/py/src on the path

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "..")
DOCS = os.path.join(ROOT, "ush", "docs")


class SharedDocsTest(unittest.TestCase):
    def test_every_option_and_subcommand_is_in_the_user_guide(self):
        from ush.cli import USAGE
        with open(os.path.join(DOCS, "USER.md"), encoding="utf-8") as f:
            guide = f.read()
        options = re.findall(r"(?m)^  (-\w), (--[\w-]+)", USAGE)
        self.assertGreaterEqual(len(options), 7)
        for option in options:
            for spelling in option:
                self.assertIn(f"`{spelling}", guide)
        for command in ("ai configure", "ai status", "ai clear-key", "allow", "explain", "notes",
                        "history", "replay"):
            self.assertIn(f"`ush {command}", guide)

    def test_one_document_set(self):
        for name in ("USER.md", "ADMIN.md", "ARCH.md", "SEC.md", "SECTEST.md", "SMOKE.md",
                     "TODO.md", "ROAD.md", "GAPS.md", "DONE.md"):
            self.assertTrue(os.path.isfile(os.path.join(DOCS, name)), name)
        for implementation in ("py", "go"):
            self.assertFalse(os.path.exists(os.path.join(ROOT, "ush", implementation, "docs")),
                             f"ush/{implementation}/docs: the documents are shared, in ush/docs")

    def test_follow_up_items_say_where_they_are_built(self):
        with open(os.path.join(DOCS, "TODO.md"), encoding="utf-8") as f:
            todo = f.read()
        section = todo[todo.index("## 2. "):todo.index("## 3. ")]
        items = re.findall(r"(?m)^- \[([ x])\] (2\.\d+) (.*)$", section)
        self.assertGreaterEqual(len(items), 30)
        for done, ident, rest in items:
            with self.subTest(item=ident):
                status = re.match(r"\*py (✓|–) go (✓|–)\*", rest)
                if status is None:
                    self.fail(f"TODO {ident}: *py ✓ go ✓* or similar")
                if done == "x":
                    self.assertEqual(status.groups(), ("✓", "✓"),
                                     f"TODO {ident}: done only when both have it")


if __name__ == "__main__":
    unittest.main()
