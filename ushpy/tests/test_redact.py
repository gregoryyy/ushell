import unittest

from helpers import SYNTHETIC_KEY
from ush.redact import MARK, redact


class RedactTest(unittest.TestCase):
    def test_known_secret_is_removed(self):
        self.assertNotIn(SYNTHETIC_KEY, redact(f"key is {SYNTHETIC_KEY}.", [SYNTHETIC_KEY]))

    def test_token_formats(self):
        samples = [
            "sk-ant-api03-abcdefghijklmnopqrstuvwxyz0123456789",
            "AKIAABCDEFGHIJKLMNOP",
            "ghp_" + "a" * 36,
            "github_pat_" + "B" * 50,
            "xoxb-1234567890-abcdefghij",
            "glpat-abcdefghijklmnopqrstu",
            "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
        ]
        for sample in samples:
            with self.subTest(sample=sample[:12]):
                out = redact(f"value: {sample} end")
                self.assertNotIn(sample, out)
                self.assertIn(MARK, out)

    def test_private_key_block(self):
        block = "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXk\n-----END OPENSSH PRIVATE KEY-----"
        out = redact("before\n" + block + "\nafter")
        self.assertNotIn("b3BlbnNzaC1rZXk", out)
        self.assertIn("before", out)
        self.assertIn("after", out)

    def test_assignments_keep_names(self):
        out = redact('DB_PASSWORD=hunter22 API_KEY: abc123xyz "client_secret": "s3cr3t"')
        for secret in ("hunter22", "abc123xyz", "s3cr3t"):
            self.assertNotIn(secret, out)
        self.assertIn("DB_PASSWORD=", out)
        self.assertIn('"client_secret": ', out)

    def test_url_credentials(self):
        out = redact("remote https://alice:pa55word@example.com/repo.git")
        self.assertNotIn("pa55word", out)
        self.assertIn("example.com/repo.git", out)

    def test_ordinary_output_is_unchanged(self):
        text = ("Filesystem   Size  Used Avail Use% Mounted on\n/dev/disk3s1 460G 440G 20G 96% /\n"
                "auth_required=false password_changed: yes")
        self.assertEqual(redact(text), text)


if __name__ == "__main__":
    unittest.main()
