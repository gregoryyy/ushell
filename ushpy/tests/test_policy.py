import os
import unittest
from unittest import mock

from helpers import TempDirTest
from ush.policy import classify, parse_rule

AUTO = [
    "df -h",
    "du -sh ~/Library/Caches/* 2>/dev/null | sort -h | tail -20",
    "find / -xdev -type f -size +1G 2>/dev/null",
    "find . -name '*.log' -mtime +30",
    "ls -la /var/log && du -sh /var/log",
    "sed -n 1,5p /etc/hosts",
    "sed -E 's/[0-9]+/N/g' /etc/hosts",
    "grep -E '(a|b)' /etc/hosts",
    "tmutil listlocalsnapshots /",
    "diskutil apfs list",
    "docker system df",
    "brew cleanup -n",
    "ps aux",
    "top -l 1 -n 10",
    "log show --last 1h --predicate 'eventMessage contains \"disk\"'",
    "find ~ -maxdepth 1 -mindepth 1 | xargs du -sh",
    "echo \"don't\"",
    "du -sh \"$HOME\"/Library 2>&1 | head",
    "cd /tmp && ls -la",
    "command -v brew",
    "awk '{print $1}' /etc/hosts",
    "ls '#notacomment'",
    "/usr/bin/du -sh /usr",
    "sysctl hw.memsize",
    "journalctl --disk-usage",
]

NEEDS_APPROVAL = {
    "rm -rf /tmp/x": "rm",
    "ls; rm x": "rm",
    "ls a#b; rm -rf x": "rm",
    "echo hi > /tmp/f": "redirect",
    "echo hi >> /tmp/f": "redirect",
    "ls $(pwd)": "substitution",
    "ls `pwd`": "substitution",
    "ls *(e:'touch x':)": "syntax",
    "find {.,-delete}": "brace",
    "find . -name x -delete": "-delete",
    "find . -exec rm {} +": "-exec",
    "sed -i '' s/a/b/ f": "sed",
    "sed 's/a/b/w /tmp/out' f": "sed",
    "sed e f": "sed",
    "awk '{system(\"rm x\")}' f": "awk",
    "awk '{print > \"out\"}' f": "awk",
    "sort -o /etc/hosts /etc/hosts": "option",
    "uniq in out": "written",
    "ps eww": "environment",
    "X=1 ls": "environment",
    "/tmp/ls": "system command",
    "./ls": "system command",
    "ls &": "syntax",
    "sleep 1 & rm x": "syntax",
    "ls\nrm x": "lines",
    "ls ${HOME}": "expansion",
    "ls $'\\x41'": "quoting",
    "cat <(ls)": "substitution",
    "sudo du -sh /": "sudo",
    "brew cleanup": "brew",
    "diskutil eraseDisk JHFS+ x disk9": "subcommands",
    "tmutil deletelocalsnapshots /": "subcommands",
    "find ~ | xargs cat": "xargs",
    "xargs rm": "rm",
    "date 0101000026": "clock",
    "sysctl -w kern.x=1": "kernel",
    "mount -t tmpfs x /mnt": "arguments",
    "tail -f /var/log/system.log": "option",
    "jq env": "environment",
    "eval ls": "eval",
    "zmodload zsh/files": "zmodload",
    "ls 'unterminated": "quote",
    "ls $$": "special",
    "=rm x": "expansion",
    "ls > ": "redirection",
    "": "empty",
}


class PolicyTest(unittest.TestCase):
    def test_read_only_commands_run_without_approval(self):
        for command in AUTO:
            with self.subTest(command=command):
                verdict = classify(command)
                self.assertTrue(verdict.auto, verdict.reason)

    def test_other_commands_need_approval_with_a_reason(self):
        for command, hint in NEEDS_APPROVAL.items():
            with self.subTest(command=command):
                verdict = classify(command)
                self.assertFalse(verdict.auto)
                self.assertTrue(verdict.reason)
                self.assertIn(hint.lower(), verdict.reason.lower() + " " + command.lower())

    def test_sudo_is_reported(self):
        self.assertTrue(classify("df -h && sudo du -sh /").sudo)
        self.assertFalse(classify("rm x").sudo)


class SecretPathTest(TempDirTest):
    def setUp(self):
        super().setUp()
        self.home = os.path.join(self.tmp, "home")
        os.makedirs(os.path.join(self.home, ".ssh"))
        os.makedirs(os.path.join(self.home, "project"))
        for name in (".ssh/id_ed25519", ".ssh/config", "project/notes.txt", "project/.env"):
            with open(os.path.join(self.home, name), "w") as f:
                f.write("x")
        patcher = mock.patch.dict(os.environ, {"HOME": self.home})
        patcher.start()
        self.addCleanup(patcher.stop)

    def test_reading_secret_files_needs_approval(self):
        for command in ("cat ~/.ssh/id_ed25519", "head ~/.ssh/config", "cat < ~/.ssh/config",
                        "cat ~/.s*/id*", "grep x ~/project/.env", "cat \"$HOME/.ssh/config\"",
                        "cat ~/.ssh/id_rsa", "cat /etc/master.passwd", "cat ~/.config/ush/credentials",
                        "grep -r password ~", "grep -R x /", "rg token " + self.home):
            with self.subTest(command=command):
                verdict = classify(command)
                self.assertFalse(verdict.auto)
                self.assertIn("secret", verdict.reason)

    def test_listing_secret_locations_and_ordinary_reads_are_allowed(self):
        for command in ("ls -la ~/.ssh", "du -sh ~/.ssh", "cat ~/project/notes.txt",
                        "grep token ~/project/notes.txt", "grep -r TODO ~/project/notes.txt"):
            with self.subTest(command=command):
                self.assertTrue(classify(command).auto, classify(command).reason)

    def test_glob_cannot_smuggle_options(self):
        # A file named "-delete" would become a find expression after expansion.
        os.chdir(self.tmp)
        self.addCleanup(os.chdir, os.path.dirname(self.tmp))
        open(os.path.join(self.tmp, "-delete"), "w").close()
        self.assertFalse(classify("find *").auto)
        open(os.path.join(self.tmp, "-i"), "w").close()
        self.assertFalse(classify("sed -n p *").auto)

    def test_variables_expand_before_checks(self):
        with mock.patch.dict(os.environ, {"EVIL": "-delete", "TARGET": self.home + "/.ssh/config"}):
            self.assertFalse(classify("find . $EVIL").auto)
            self.assertFalse(classify("cat $TARGET").auto)


if __name__ == "__main__":
    unittest.main()


class AllowRuleTest(TempDirTest):
    """The user's allowlist extends the table for unknown commands and never widens it."""

    RULES = (("git", "status"), ("kubectl", "get"), ("terraform", "plan"))

    def test_rules_let_matching_commands_run(self):
        for command, rule in (("git status", "git status"), ("git status -sb", "git status"),
                              ("kubectl get pods -A | head", "kubectl get"),
                              ("git status && kubectl get ns", "git status, kubectl get"),
                              ("/usr/bin/git status", "git status")):
            with self.subTest(command=command):
                verdict = classify(command, self.RULES)
                self.assertTrue(verdict.auto, verdict.reason)
                self.assertEqual(verdict.rule, rule)
        self.assertEqual(classify("df -h", self.RULES).rule, "")  # the table, not a rule

    def test_rules_are_prefixes_of_words(self):
        for command in ("git push", "git statusx", "git", "kubectl delete pod x",
                        "kubectl --context prod get pods", "terraform apply"):
            with self.subTest(command=command):
                self.assertFalse(classify(command, self.RULES).auto)
        self.assertFalse(classify("git status").auto)  # no rules, no exception

    def test_the_rest_of_the_policy_still_applies(self):
        for command in ("git status > out.txt", "git status $(rm -rf ~)", "git status; rm x",
                        "git status `id`", "GIT_DIR=x git status", "git status &",
                        "/tmp/evil/git status", "git status ~/.ssh/id_ed25519",
                        "sudo git status", "git status | sh"):
            with self.subTest(command=command):
                self.assertFalse(classify(command, self.RULES).auto)

    def test_rules_cannot_name_runners_table_commands_or_syntax(self):
        for text in ("bash", "sh -c", "env", "xargs", "python3.12", "perl -e", "sudo", "find",
                     "sed", "awk", "rm", "ssh", "osascript", "", "-rf", "git $(x)", "git *",
                     "git 'status'", "./git status", "/usr/bin/git status", "a b c d e f g h i"):
            with self.subTest(rule=text), self.assertRaises(ValueError):
                parse_rule(text)
        self.assertEqual(parse_rule("  kubectl   get "), ("kubectl", "get"))

