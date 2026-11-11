"""The shared documents cover ushpy's command line: every option and subcommand of `ush help` is in
the user guide and the manual page. The checks of the documents that need no implementation are in
tools/docs/test_docs.py."""

import os
import re
import unittest

import helpers  # noqa: F401  puts src/py/src on the path

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "..")
DOCS = os.path.join(ROOT, "docs")
MANUAL_PAGE = os.path.join(ROOT, "src", "man", "ush.1")


def manual_page_text() -> str:
    """The page's source as words: font changes, quotes and comments removed, roff minus signs as
    hyphens, so `.BR \\-r ", " \\-\\-run` reads `-r, --run`."""
    with open(MANUAL_PAGE, encoding="utf-8") as f:
        text = "".join(line for line in f if not line.startswith('.\\"'))
    text = re.sub(r"\\f[BIRP]", "", text).replace("\\-", "-").replace('"', "")
    return re.sub(r" +", " ", re.sub(r" +,", ",", text))


class HelpIsDocumentedTest(unittest.TestCase):
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

    def test_every_option_and_subcommand_is_in_the_manual_page(self):
        from ush.cli import USAGE
        page = manual_page_text()
        options = re.findall(r"(?m)^  (-\w), (--[\w-]+)", USAGE)
        self.assertGreaterEqual(len(options), 7)
        words = [f"{short}, {long}" for short, long in options]
        words += [f"ush {command}" for command in ("ai configure", "ai status", "ai clear-key",
                                                  "allow", "explain", "notes", "history",
                                                  "replay", "help")]
        words += ["USH_PROVIDER", "USH_MODEL", "USH_CHECKER", "USH_OLLAMA_URL", "USH_NOTES_KEY",
                  "USH_BARE_SLASH"]
        for word in words:
            self.assertTrue(word in page, f"{word} is not in src/man/ush.1")


if __name__ == "__main__":
    unittest.main()
