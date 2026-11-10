import os
import unittest

from helpers import TempDirTest
from ush.history import History, Step


class HistoryTest(TempDirTest):
    def test_append_find_and_privacy(self):
        history = History(os.path.join(self.tmp, "state"))
        entry = history.new_entry("why is the disk full", "m", "tester")
        entry.steps.append(Step("command", "df -h", decision="auto", exit_code=0))
        entry.answer = "x" * 10_000
        entry.outcome = "answered"
        history.append(entry)
        self.assertEqual(os.stat(history.path).st_mode & 0o777, 0o600)
        self.assertEqual(os.stat(history.directory).st_mode & 0o777, 0o700)
        found = history.find("last")
        self.assertEqual(found.request, "why is the disk full")
        self.assertEqual(found.steps[0].command, "df -h")
        self.assertLess(len(found.answer), 5000)
        self.assertEqual(history.find(entry.id[:15]).id, entry.id)
        self.assertIsNone(history.find("nope"))

    def test_damaged_lines_are_skipped(self):
        history = History(os.path.join(self.tmp, "state"))
        history.append(history.new_entry("one", "m", "t"))
        with open(history.path, "a") as f:
            f.write("{not json\n")
        history.append(history.new_entry("two", "m", "t"))
        self.assertEqual([e.request for e in history.entries()], ["one", "two"])

    def test_missing_history_is_empty(self):
        self.assertEqual(History(os.path.join(self.tmp, "none")).recent(5), [])


if __name__ == "__main__":
    unittest.main()
