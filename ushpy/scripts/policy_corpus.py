#!/usr/bin/env python3
"""Write conform/policy.json: command lines and ushpy's policy verdicts on them.

The corpus is the contract between the two implementations of ush (ARCH, ushgo TODO 1.1): ushgo's
policy must return the same verdict, reason included, for every line. Lines come from ushpy's
policy tests, an escape-vector sweep, allow rules, and seeded random lines over a fixture tree
(files, a hidden directory, a fake home with secrets), so globs, ~ and secret paths are covered.
Paths in reasons are written relative to the fixture root as $ROOT.

    policy_corpus.py            rewrite the corpus
    policy_corpus.py --check    exit 1 if the corpus is out of date (test_conformance runs this)
"""

from __future__ import annotations

import json
import os
import random
import sys
import tempfile
from unittest import mock

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "src"))
sys.path.insert(0, os.path.join(HERE, "..", "tests"))

from ush.policy import classify, parse_rule  # noqa: E402

CORPUS = os.path.join(HERE, "..", "..", "conform", "policy.json")

# Created under the fixture root; a trailing / makes a directory.
FIXTURE = ["a.txt", "b.log", "notes.md", ".hidden", "dir/", "dir/sub/", "dir/sub/x.txt",
           "dir/y.log", "-delete", "space name.txt", "home/", "home/.ssh/", "home/.ssh/id_rsa",
           "home/.ssh/config", "home/.env", "home/docs/", "home/docs/r.txt", "home/.config/ush/",
           "home/.config/ush/credentials", "home/.netrc", "keys/", "keys/server.pem"]
ENV = {"FOO": "a.txt", "EVIL": "-delete", "TARGET": "$ROOT/home/.ssh/config", "EMPTY": ""}

EXTRA = """
ls *.txt | ls *.log | ls * | ls .* | ls dir/* | ls dir/**/*.txt | ls **/*.log | cat *.txt
cat dir/sub/x.txt | cat ~/.ssh/id_rsa | cat ~/.env | cat ~/docs/r.txt | grep -r x ~ | grep -r x ~/docs
grep -r x dir | grep -R x . | rg x | rg x dir | du -sh ~ | du -sh dir | find . -name '*.txt'
find * -type f | find . -exec du -h {} + | cat $FOO | cat "$FOO" | find . $EVIL | cat $TARGET
cat keys/server.pem | head -c 10 keys/server.pem | cat < a.txt | cat < ~/.ssh/id_rsa | wc -l < b.log
ls 2>/dev/null | ls 2>&1 | ls > out.txt | ls >> out.txt | ls &>/dev/null | ls 1>&2 | ls >
cat 'space name.txt' | cat space\\ name.txt | cat "space name.txt" | ls ~/.config/ush
cat ~/.config/ush/credentials | cat ~/.netrc | cat .env | head .env.local | cat .netrc | cat .bashrc
xargs ls | xargs cat | xargs -n 1 du | xargs | /bin/ls | /usr/bin/du -sh . | ./ls | /tmp/ls
sudo ls | sudo -u root cat a.txt | FOO=1 ls | env ls | command -v ls | command ls | ls $(pwd)
ls `pwd` | ls ${HOME} | ls $'a' | ls $[1] | cat <(ls) | ls >(cat) | ls =(ls) | ls $1 | ls $?
echo {a,b} | echo {} | ls a{}b | echo ""~ | echo ''~/.ssh | cat ""=ls | echo "x"~ | echo ~"" | echo ~
ls # comment | ls; # x | echo a#b | ls && ls || ls | ls | wc -l | ls |& cat | ls ; ls | ls &| ls
ls & | (ls) | { ls; } | ls &&|| ls | ls\\ | 'unterminated | "unterminated | echo \\$HOME
sed -n 1,5p a.txt | sed -n '/x/p' a.txt | sed 's/a/b/g' a.txt | sed 's|a|b|' a.txt | sed s/a/b/w a.txt
sed -e p -e 's/x/y/' a.txt | sed -E 's/[0-9]+/N/g' a.txt | sed y/abc/xyz/ a.txt | sed 's/a/b/;p' a.txt
awk '{print $1}' a.txt | awk 'BEGIN{system("x")}' | jq . a.txt | jq -n env | ps aux | ps eww | ps -E
top -l 1 | top | date +%s | date -u | date 0101 | sysctl -n hw.ncpu | sysctl -w x=1 | ifconfig en0
ifconfig en0 up | ip addr | ip link set up | dig -v | dig x.example | host -t A x.example | uptime
diskutil list | diskutil apfs list | diskutil eraseDisk x | tmutil listlocalsnapshots / | log show
log stream | defaults read x | defaults write x y z | mdutil -s / | mdutil -E / | pmset -g | pmset 1
docker ps | docker system df | docker rm x | brew list | brew cleanup -n | brew cleanup | xcrun simctl list
tail -f a.txt | tail -n 5 a.txt | sort -o x a.txt | sort a.txt | uniq a.txt b.txt | xxd -r a.txt
base64 -o x a.txt | journalctl -f | journalctl -u x | dmesg -c | dmesg | file -C x | file a.txt
networksetup -getinfo Wi-Fi | networksetup -setdnsservers x | scutil --dns | scutil --set x
spctl --status | spctl --master-disable | softwareupdate -l | softwareupdate -i x | nvram -p | nvram x=1
xcode-select -p | xcode-select -s x | plutil -p x | plutil -convert x | csrutil status | csrutil disable
"""
RULES = ["git status", "kubectl get", "git log --oneline", "ollama list"]
RULE_LINES = ["git status", "git status -s", "git statusx", "git -C . status", "git log --oneline -5",
              "git log", "kubectl get pods -A", "kubectl delete pod x", "ollama list",
              "git status > x", "git status ~/.ssh/id_rsa", "git status | cat", "sudo git status",
              "git status && kubectl get ns"]
RULE_TEXTS = ["git status", "kubectl get", "sh -c", "python3", "rm", "curl", "find", "sed",
              "git $(x)", "a/b", "-x", "", "a b c d e f g h i", "git 'status'", "python3.12", "env"]


def fuzz_lines(n: int = 1500) -> list[str]:
    rng = random.Random(20261028)
    pieces = ["ls", "cat", "grep -r x", "find .", "du -sh", "head", "wc -l", "sed -n p", "rm",
              "a.txt", "*.txt", "*", ".*", "dir/*", "**/*.log", "~", "~/.ssh", "~/docs", "$FOO",
              "$EVIL", "\"$TARGET\"", "'a b'", "\"x y\"", "\\ ", "-delete", "-exec", "{}", "{a,b}",
              "|", "&&", "||", ";", "2>/dev/null", ">", "<", "a.txt", "dir", ".env", "=x", "#",
              "''", "\"\"", "keys/*", "home/.ssh/*", "\\*", "'*'", "-r", "-R", "/etc/hosts",
              "/etc/ssh/sshd_config", "/bin/ls", "./x", "sudo", "xargs", "$(x)", "`x`", "${x}"]
    return ["".join(rng.choice(pieces) + rng.choice([" ", " ", ""]) for _ in range(rng.randint(1, 6))).strip()
            for _ in range(n)]


def corpus() -> dict:
    import test_policy  # noqa: the lists of ushpy's own policy tests
    lines = [*test_policy.AUTO, *test_policy.NEEDS_APPROVAL]
    lines += [c.strip() for line in EXTRA.strip().splitlines() for c in line.split(" | ") if c.strip()]
    lines += fuzz_lines()
    with tempfile.TemporaryDirectory(prefix="ush-corpus-") as tmp:
        root = os.path.realpath(tmp)
        etc = os.path.realpath("/etc")  # /private/etc on macOS; the corpus holds /etc
        for path in FIXTURE:
            full = os.path.join(root, path)
            if path.endswith("/"):
                os.makedirs(full, exist_ok=True)
            else:
                os.makedirs(os.path.dirname(full), exist_ok=True)
                with open(full, "w") as f:
                    f.write("x\n")
        env = {k: v.replace("$ROOT", root) for k, v in ENV.items()}
        env["HOME"] = os.path.join(root, "home")
        cwd = os.getcwd()
        os.chdir(root)
        try:
            with mock.patch.dict(os.environ, env):
                rules = tuple(parse_rule(r) for r in RULES)
                cases, seen = [], set()
                for line, rule_set in [*((l, ()) for l in lines), *((l, rules) for l in RULE_LINES)]:
                    if (line, bool(rule_set)) in seen:
                        continue
                    seen.add((line, bool(rule_set)))
                    v = classify(line, rule_set)
                    cases.append({"command": line, "rules": bool(rule_set), "auto": v.auto,
                                  "reason": v.reason.replace(root, "$ROOT").replace(etc + "/", "/etc/"),
                                  "sudo": v.sudo,
                                  "rule": v.rule})
                parsed = []
                for text in RULE_TEXTS:
                    try:
                        parsed.append({"text": text, "words": list(parse_rule(text)), "error": ""})
                    except ValueError as exc:
                        parsed.append({"text": text, "words": [], "error": str(exc)})
        finally:
            os.chdir(cwd)
    return {"about": "ushpy policy verdicts; regenerate with ushpy/scripts/policy_corpus.py",
            "fixture": FIXTURE, "env": ENV, "rules": RULES, "cases": cases, "parse_rule": parsed}


def main() -> int:
    data = json.dumps(corpus(), indent=1, ensure_ascii=False) + "\n"
    if sys.argv[1:] == ["--check"]:
        try:
            with open(CORPUS, encoding="utf-8") as f:
                current = f.read()
        except OSError:
            current = ""
        if current != data:
            print(f"{CORPUS} is out of date; run ushpy/scripts/policy_corpus.py", file=sys.stderr)
            return 1
        return 0
    os.makedirs(os.path.dirname(CORPUS), exist_ok=True)
    with open(CORPUS, "w", encoding="utf-8") as f:
        f.write(data)
    print(f"wrote {CORPUS}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
