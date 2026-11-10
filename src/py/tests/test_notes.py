"""What `ush notes` sends and why it refuses a file (TODO 2.25); the command itself is tested in
test_cli, against both implementations."""

import os

from helpers import TempDirTest
from ush import notes


class NotesTest(TempDirTest):
    def test_files_are_words_naming_paths(self):
        os.chdir(self.tmp)
        self.addCleanup(os.chdir, os.path.dirname(os.path.abspath(__file__)))
        with open("app.log", "w") as f:
            f.write("x")
        self.assertEqual(notes.split_words(["why", "app.log", "", "502?"]),
                         (["app.log"], "why 502?"))

    def test_read_file_sends_text_and_says_why_not(self):
        path = os.path.join(self.tmp, "app.log")
        with open(path, "wb") as f:
            f.write(b"ERROR x\n\xff")
        self.assertEqual(notes.read_file(path, notes.LIMIT), ("ERROR x\n�", ""))
        self.assertEqual(notes.read_file(path, 3), ("", "too large (9 bytes; at most 200 KB in all)"))
        self.assertEqual(notes.read_file(os.path.join(self.tmp, "missing"), notes.LIMIT),
                         ("", "can't read it (No such file or directory)"))
        if os.geteuid() != 0:
            os.chmod(path, 0)
            self.assertEqual(notes.read_file(path, notes.LIMIT),
                             ("", "can't read it (Permission denied)"))

    def test_size_text(self):
        for size, text in ((18, "18 bytes"), (1234, "1.2 KB"), (150_000, "150 KB"),
                           (200_001, "200 KB")):
            self.assertEqual(notes.size_text(size), text)
