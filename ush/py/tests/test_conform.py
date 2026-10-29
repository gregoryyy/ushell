"""ush/conform/policy.json, the policy verdicts both implementations must give (ushgo TODO 1.1), is
regenerated from ushpy and must be current: a policy change in ushpy updates it in the same
change, and ushgo's TestConformance then checks the Go policy against it."""

import os
import subprocess
import sys
import unittest

SCRIPT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "scripts", "policy_corpus.py")


class ConformTest(unittest.TestCase):
    def test_the_policy_corpus_is_current(self):
        result = subprocess.run([sys.executable, SCRIPT, "--check"], capture_output=True,
                                text=True, timeout=300)
        self.assertEqual(result.returncode, 0, result.stderr)


if __name__ == "__main__":
    unittest.main()
