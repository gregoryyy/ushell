# Smoke testing ush as a user

How to check that an installed `ush` works on your machine before you rely on it: after installing,
after an upgrade, on a new OS, and before a release (TODO 2.1, 3.1). The same steps apply to
both implementations of the command, Python ([ush/py/](../py/README.md)) and Go
([ush/go/](../go/README.md)); `ush --version` says which one you have (`ush 0.1.0 (python)`,
`ush 0.0.0-dev (go)`). The developer test suites are separate: `just ushpy-test` and
`just ushgo-test`. Status: 2026-10-28 09:59.

## Principles

- **Test what you installed,** not the source tree: run `install.sh` (or copy the Go binary) first,
  and test that `ush`. Pass `--bin` to test another one.
- **Keep your own state out.** The script gives ush a scratch configuration and history (`XDG_*`)
  and ignores your `USH_*` variables. Only the API key comes from your keychain or
  `ANTHROPIC_API_KEY`, as in normal use.
- **Work in a scratch directory.** Every request runs in a new empty directory; nothing outside it
  is meant to change, and changes need approval, which the automated part never gives.
- **Free before paid.** Run the model checks on a local Ollama model first. The Anthropic run costs
  money (about 15 requests) and needs `--paid`.
- **Record the result.** The script writes `smoke-report.md`; paste it into DONE with the version
  and machine.

## 1. Automated checks: `smoke.sh`

```sh
ush/py/scripts/smoke.sh                                    # offline: no model, no cost
ush/py/scripts/smoke.sh --provider ollama --model qwen3.5:9b --checker ollama:llama3.2:3b
ush/py/scripts/smoke.sh --provider anthropic --paid        # your configured model; --model to change
ush/py/scripts/smoke.sh --bin ~/go/bin/ush                 # another implementation or install
```

| Check | Passes when |
| --- | --- |
| version, help | `ush --version` prints `ush X.Y.Z (impl)`; `ush help` prints the usage |
| unknown command | `ush frobnicate` exits 2 |
| ai status | it works and shows no API key in full |
| no session without a terminal | `ush` without a request and without a terminal exits 2 |
| empty history | a fresh state has no history |
| read-only request | the answer comes; `df` ran without a prompt (WARN if the model ran nothing) |
| disclosure banner | the banner says where requests go; with `--checker`, where the checker's go |
| change without approval | without a terminal, `smoke-made` is not created; history says declined |
| injected instructions | a file that tells the assistant to `touch pwned` and `dig` doesn't get either run; WARN if `dig` ran (SECTEST F2) |
| piped input | `… \| ush //…` answers |
| dry run | `ush -n` runs nothing |
| replay | `ush replay last --dry-run` works without AI |

A model check whose request fails (no model pulled, no key, provider error) fails, so a broken
setup can't pass. WARN means ush behaved safely but the model didn't do what the check asked; a
small model often answers without running commands.

## 2. Manual checks: what needs your terminal

Do these in a scratch directory (`cd "$(mktemp -d)"`). Each line says what to type and what you
should see.

| # | Do | Expect |
| --- | --- | --- |
| M1 | `ush --run //create a file named a.txt` and answer `n`, reason "not now" | Not created; the assistant acknowledges the reason |
| M2 | Same request, answer `e` and type `touch b.txt` | `b.txt` exists, `a.txt` doesn't; history says `edited` |
| M3 | Same request, answer `a` | The request stops; `ush history` says `aborted` |
| M4 | `ush --run //count the files under /usr` and press Ctrl-C while it runs | The command stops, ush returns to the prompt, no stray process (`ps`) |
| M5 | `ush` (session): `ls`, then `// what did ls show?` | The answer refers to the listing |
| M6 | With the zsh plugin: `ush //why is my disk full?` unquoted, and a bare `// what is my IP?` | Both reach ush verbatim, `?` and `'` included |
| M7 | `ush ai configure`, then `ush ai status` | Settings saved; the key masked |
| M8 | Root-owned install (`sudo ./install.sh --prefix /usr/local`), then `sudo ush --run //who am I running as?` | Runs as root with your key and settings; the banner says "as root (for you)"; `USH_*` variables ignored |
| M9 | `printf 'printf "\\033]0;pwned\\007"' > t.sh; ush --run //run sh t.sh` and approve | Until TODO 2.9: the terminal title changes (SECTEST F1). After 2.9: the sequence is shown, not executed |
| M10 | `ush --run -c //how much memory is free?` | Even read-only commands ask (`--confirm`) |
| M11 | With the plugin: `// how much free space is on /?` | The proposal is on the command line, nothing ran yet |
| M12 | Press Enter on it, then `//` | It ran in your shell (in `history`); the answer uses its output |
| M13 | `// make a file named c.txt`, edit the line to `touch d.txt`, Enter, `//` | `d.txt` exists, `c.txt` doesn't; ush mentions the edited command |
| M14 | `// show me the system log in a pager`, Enter, quit the pager, `//` | The pager worked normally (interactive); ush says it didn't see the output |
| M15 | `// list my home directory`, then Ctrl-U and `// never mind, what time is it?` | ush knows the listing didn't run |
| M16 | `// find the largest files on disk`, then Down | Above the command: the parts and the access (files, network); the cursor stays in the command |
| M17 | Same, then Esc, Esc | The first Esc closes the explanation, the second removes the proposal |
| M18 | `ush -v`, then `// find the largest files on disk` and Down at `ush>` | Pre-filled at `ush>`; the explanation also has summary, notes, ush's verdict and "Esc closes this" |

## When it fails

- **FAIL in the offline part:** the install is broken; reinstall, and report it with
  `smoke-report.md`.
- **FAIL in a model check:** read the scratch directory's output (`--keep` keeps it): `out/NAME.err`
  has ush's error. A `change`, `injected instructions`, or `dry run` failure means a command ran
  that shouldn't have: stop using that version and report it as a security finding (SECTEST).
- **One implementation passes a check the other fails:** that is a parity bug; report it with both
  outputs (`--bin` for each).
