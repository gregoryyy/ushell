"""The user and admin guides exist for both implementations of ush (ushpy/docs and ushgo/docs).
Their shared sections describe the one command and must stay identical."""

import os
import re
import unittest

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
BLOCK = re.compile(r"<!-- shared:begin [^>]*-->\n.*?<!-- shared:end -->\n", re.S)


def shared(project: str, name: str) -> list[str]:
    with open(os.path.join(ROOT, project, "docs", name), encoding="utf-8") as f:
        return BLOCK.findall(f.read())


class SharedGuidesTest(unittest.TestCase):
    def test_shared_sections_are_identical(self):
        for name, count in (("USER.md", 1), ("ADMIN.md", 3)):
            with self.subTest(guide=name):
                python, go = shared("ushpy", name), shared("ushgo", name)
                self.assertEqual(len(go), count, f"ushgo/docs/{name}: shared blocks")
                self.assertEqual(python, go, f"{name}: edit the shared sections in both copies")

    def test_every_option_and_subcommand_is_in_the_user_guide(self):
        from ush.cli import USAGE
        with open(os.path.join(ROOT, "ushgo", "docs", "USER.md"), encoding="utf-8") as f:
            guide = f.read()
        options = re.findall(r"(?m)^  (-\w), (--[\w-]+)", USAGE)
        self.assertGreaterEqual(len(options), 7)
        for option in options:
            for spelling in option:
                self.assertIn(f"`{spelling}", guide)
        for command in ("ai configure", "ai status", "ai clear-key", "allow", "history", "replay"):
            self.assertIn(f"`ush {command}", guide)


if __name__ == "__main__":
    unittest.main()
