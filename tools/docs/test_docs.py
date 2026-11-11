"""The repository's documents, checked without either implementation: one document set in docs/,
a well-formed manual page, and every follow-up item in TODO saying where it is built. The checks
that the documents cover `ush help` need the Python implementation and are in
src/py/tests/test_help_docs.py. `just links` and the CI job links run these."""

import os
import re
import shutil
import subprocess
import unittest

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
DOCS = os.path.join(ROOT, "docs")
MANUAL_PAGE = os.path.join(ROOT, "src", "man", "ush.1")


class DocsTest(unittest.TestCase):
    @unittest.skipUnless(shutil.which("mandoc"), "mandoc is not installed")
    def test_the_manual_page_is_well_formed(self):
        lint = subprocess.run(["mandoc", "-T", "lint", "-W", "warning", MANUAL_PAGE],
                              capture_output=True, text=True, timeout=30)
        self.assertEqual((lint.returncode, lint.stdout + lint.stderr), (0, ""))

    def test_one_document_set(self):
        for name in ("USER.md", "ADMIN.md", "ARCH.md", "SEC.md", "SECTEST.md", "SMOKE.md",
                     "TODO.md", "ROAD.md", "GAPS.md", "DONE.md"):
            self.assertTrue(os.path.isfile(os.path.join(DOCS, name)), name)
        for implementation in ("py", "go"):
            self.assertFalse(os.path.exists(os.path.join(ROOT, "src", implementation, "docs")),
                             f"src/{implementation}/docs: the documents are shared, in docs/")

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
