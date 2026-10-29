# ush security model

Status: applies to both implementations of ush as of 2026-10-29 (0.2.0-rc1 and the work since): the
local Ollama provider (TODO 2.8), the checker model (2.13), propose mode (2.17) and kept
explanations (2.24). What the Go implementation changes is at the end. Design: [ARCH.md](ARCH.md).
Security tests and open findings: [SECTEST.md](SECTEST.md).

In propose mode (the default) ush runs nothing: an AI model proposes commands, and the user runs
them in their own shell. In agent mode (`--run`) it lets the model run commands on the user's
machine, possibly as root. The protections are the user's approval (pressing Enter on a proposal is
one), a conservative read-only policy, bounded execution, and limited disclosure. ush is not a
sandbox.

## Assets

- The system and the user's data: integrity (no unapproved changes) and
  confidentiality (what reaches the provider, and whether ush says so truthfully).
- The provider API key.
- Root privileges when run under sudo.
- History, which reveals what was investigated.
- Propose mode's saved conversations (`sessions/`, redacted command output) and the plugin's
  handoff directory (the proposal, what ran, its output).
- Kept explanations (`explanations.json`, mode 0600): redacted answers, keyed by a hash of the
  command line, never the command line itself.

## Trust boundaries

| Source | Trust | Consequence |
| --- | --- | --- |
| The user at the terminal | Trusted | Approvals, edits, explicit session commands |
| Model output (commands, scripts, `effect` claims) | Untrusted | Policy plus approval; claims never widen permissions |
| Command output, piped input, file contents | Untrusted data | May carry prompt injection; never grants anything |
| Environment (`USH_BASE_URL`, `USH_PROVIDER`, `USH_MODEL`, `USH_OLLAMA_URL`, `XDG_*`) under root | Untrusted | Ignored as root |
| Ollama endpoint and its model list | Trusted for local models only | Must be a loopback address; cloud models are disclosed |
| Checker model verdicts | Untrusted | A flag or no verdict adds an approval prompt (a warning on a proposal); OK changes nothing |
| `USH_HANDOFF` and the files in it | The user's own, if the directory is theirs and closed to others | Used only then and never as root; files read with `O_NOFOLLOW` as regular files |
| Files ush executes under sudo | Must be root-owned | Root-owned install path documented |

## Threats and mitigations

| Threat | Mitigation | Tests |
| --- | --- | --- |
| Model runs a destructive command (mistake or prompt injection) | Only policy-verified read-only commands run without approval. `effect: "read"` alone is not enough. Without a terminal, non-read-only commands are declined. | `test_agent`: declined/approved/edited, injection via file content, false `read` claim; `test_cli`: no-tty decline |
| Policy bypass through shell syntax | zsh-aware tokenizer; rejects substitutions, brace/`=` expansion, redirections, background, grouping, assignments, comments, multi-line; validates expanded globs/variables | `test_policy` (incl. `find *` with a `-delete` file, `$EVIL`, `a#b; rm`, glob qualifiers) |
| "Read-only" tool with options that write or run code | Per-command validators (`find -exec/-delete`, `sed -i/w/e`, `sort -o`, `awk system()`, `xxd -r`, `uniq IN OUT`, …) | `test_policy` |
| Secrets disclosed to the provider | Content readers of secret paths need approval; recursive reads of home or `/` need approval; output redaction; size bounds | `test_policy` secret-path cases; `test_redact`; agent redaction test |
| API key leakage | Key only in the SDK transport header. Removed from command environments (`ANTHROPIC_*`, `USH_API_KEY`), redacted from output and history, never printed (`ai status` masks it). Keychain writes pass it on stdin, not in argv | `test_execute` env scrub; `test_config` keychain argv; `test_provider` key absent from bodies; `test_cli` status/history |
| Tampered credential file | `O_NOFOLLOW`, regular file, owner is the user or root, mode without group/other bits; otherwise refused with a fix | `test_config` |
| Endpoint redirect under sudo (`sudo -E`) | As root, the API URL is fixed and `XDG_*` and `USH_*` overrides are ignored | `test_config` (XDG and `USH_*` under root); code review for `USH_BASE_URL` |
| Requests sent elsewhere while ush says they stay local (Ollama) | The Ollama URL must be http(s) to a loopback address without credentials or path, from settings or environment; requests ignore proxy variables and do not follow redirects | `test_config` URL cases; `test_provider` bogus proxies; `test_cli` misconfiguration |
| Ollama cloud models (`…:cloud`) forward requests to ollama.com | The banner and `ai status` name the remote host from Ollama's model list; a `cloud` tag counts as remote even if Ollama can't be asked | `test_provider` remote-host cases; `test_cli` cloud disclosure |
| Anthropic key sent to Ollama | The Ollama client uses a fixed placeholder key | `test_provider` (key absent from the request) |
| Prompt injection steers the model to an auto-approved command that leaks or harms (e.g. DNS exfiltration, F2) | Optional checker model (`check.py`) judges each proposal against the user's requests only, never output or piped input; flag, unclear reply, or error → approval needed; its input is redacted and tagged with random markers | `test_check`: flag gates a read-only command, no verdict fails closed, OK never approves, no output/piped input/secrets in its input, marker tags; `test_cli` end to end |
| The checker sends data to a provider the user didn't expect | `provider:model` named in the banner and `ai status` (Anthropic, local, or an Ollama cloud host); as root only the settings file counts | `test_cli` checker disclosure; `test_config` checker settings |
| A user allow rule widens the policy (an injected `read` claim runs `rm`, `curl`, or `sh -c` unasked) | Rules only for commands the table doesn't know; refused for runners, interpreters, and commands that change or send data; plain words only; prefix match on words; the rest of the policy, the model's `change`, `--confirm`, and the checker still apply; settings file only, not used as root | `test_policy` `AllowRuleTest`; `test_agent` `AllowRuleAgentTest`; `test_config` `AllowSettingsTest`; `test_cli` allow |
| Runaway or hanging commands | stdin `/dev/null`, per-command timeout (≤ `max_timeout`), process-group kill, output limit, step limit per request | `test_execute`, `test_agent` step limit |
| Aborted or interrupted requests leave a broken conversation | Every `tool_use` answered; Ctrl-C kills the process group | `test_agent` abort/truncation/refusal |
| Root running user-writable code | `install.sh --prefix /usr/local` for root-owned installs; launcher uses `python -I` (ignores `PYTHON*`, user site, cwd) | Manual; documented |
| A proposal that hides what it does on the command line (escape sequences, `\r`, bidi overrides, zero-width characters) | Refused before display or handoff; the model is asked to propose it again | `test_propose` (control characters) |
| The user runs a harmful proposal by habit | ush runs nothing itself; changes and commands the policy can't verify are marked in yellow, with the checker's objection if one is configured | `test_propose` (notes) |
| Another user or a planted link reads or swaps the handoff files | The plugin's `mktemp -d` directory (0700); ush refuses a directory that isn't the user's, is open to others, or is a link, and reads `ran` and `output` only as regular files without following links | `test_propose` (HandoffTest) |
| Captured output kept on disk | `output` lives in the handoff directory (0700) until `//` takes it, then as `last-output` for notes until the next run replaces it or the shell exits (TODO 2.25); the saved conversation is 0600 in the state directory and expires after an hour; history keeps no output | `test_propose` (sessions, `last_ran`); Go `TestLastRanIsTheNewestRunAndRemovesNothing` |
| Notes (`ush notes`, Ctrl-X n) send a file the user didn't mean to share, or a secret one (TODO 2.25) | Only files named on the line, the plugin's recorded output, or piped input. Before sending, one line names each item and where it goes. Refused, with the reason: secret locations (the policy's list, by the name given and by what a link resolves to), directories, special files (a FIFO is opened without blocking), binary files, anything past 200 KB in all. Redacted, between random-tag markers as data; never as root; the plugin refuses sudo and ush lines | `test_cli` (notes tests, against both implementations); `test_notes`; Go `internal/notes`, `TestNotesNeverAsRoot`; `test_zsh_plugin` |
| Ctrl-X n runs a command the user didn't mean to run | It runs only the line the user typed, as Enter would, recorded like a proposal; on a line that isn't a command it runs `ush notes`, never the line | `test_zsh_plugin` (`ZshProposeTest` notes tests, `ZshNotesTest`) |
| An explanation (`ush explain`, Down) sends more than the user expects | Only the command line, and for a proposal the last request and its purpose; redacted; the command between random-tag markers as data; nothing runs; `-v` shows the destination | `test_cli` (explain tests) |
| The installer changes `~/.zshrc` without consent | It asks, only from a terminal; `--zshrc` or `--no-zshrc` decide explicitly; never as root or twice; only appends one `source` line | `test_install` |
| Text that the terminal acts on: escape sequences, OSC 52, bidi overrides in commands, output, answers, history (F1) | Everything ush writes passes `ui.visible`, which writes them out as `^[`, `^M`, `<U+202E>` | `test_security` `DisplayTest` |
| Expansions the policy misreads: `""~`, `""=cmd` (F4) | Treated as unsupported, so they need approval; the tokenizer is fuzzed against zsh | `test_security` `PolicyTest` |
| A planted temporary file under sudo: root truncates a hard-linked root-owned file (F5) | Private files are written through a random name created with `O_EXCL`, then renamed | `test_security` `PrivilegeTest` |
| Untrusted text that stalls redaction (F3) | Bounded patterns; 300 KB in well under a second | `test_security` `RedactionCorpusTest` |
| A `PATH` entry that lets another program stand in for a listed one | `ai status` lists relative and writable entries; a root session warns about them | `test_security` `test_path_risks` |
| User aliases or rc files changing command meaning | Commands run with `zsh -f` | `test_execute` |

## Residual risks

- **The policy infers, it does not enforce.** A listed command with an unknown side
  effect (or a malicious binary earlier on `PATH`) would run unprompted. Planned:
  OS sandboxing for auto-approved commands (TODO 2.6).
- **Disclosure through read-only commands.** Auto-approved commands can print data
  outside the secret list: logs, `system_profiler` serial numbers, file names,
  process lists. Everything is sent to the provider after redaction, and redaction
  is pattern-based. Use `--confirm` on sensitive machines.
- **Time-of-check/time-of-use.** Globs and paths are checked when the command line
  is classified; files can change before it runs. This affects only which commands
  skip the approval prompt.
- **Approved commands are trusted.** Once the user approves, the command runs with
  the user's (or root's) full privileges. ush shows the purpose and the command but
  cannot judge it.
- **Keychain under sudo** is best effort: ush runs `security` as the invoking user.
  Failure falls back to the credentials file.
- **Network egress** by read-only commands: DNS lookups of names need approval since SECTEST F2
  was fixed; other read-only tools that touch the network (`ping` is not listed, `curl` needs
  approval) stay out of the table.
- **Homoglyphs** in commands (a Cyrillic `а` in a path) are shown as they are; control and
  format characters are written out (SECTEST F1, fixed), lookalike letters are not flagged.
- **An allow rule is the user's judgement.** A rule such as `git` allows every `git` subcommand,
  including ones that write (`git clean -fdx`) if the model calls them `read`. Narrow rules
  (`git status`) are safe; ush can't tell which subcommands of an unknown tool write.
- **The checker is a model too.** It judges the proposal text, which an injection controls, so
  a proposal can be shaped to look harmless or carry text aimed at the checker. Small models miss
  attacks (qwen2.5:1.5b let 3 of 4 pass, SECTEST) and raise false alarms. It narrows what runs
  unasked; it doesn't replace the policy or the approval prompt.
- **Propose mode moves the decision to the Enter key.** A proposal looks like something the user
  typed. An injected proposal (through output of an earlier command) is shown with the same notes as
  any other, but a user who presses Enter without reading runs it. Policy notes and the checker
  are advice there, not a gate.
- **Captured commands run with a pipe as stdout.** Tools that check for a terminal behave
  differently (no colours, no pager); a command the model wrongly marks non-interactive may wait
  for input it can't show properly. Ctrl-C stops it as usual.
- **Notes send what a file holds.** A file the user names goes to the provider whole, and
  redaction is pattern-based: personal data, internal names and secrets in unknown formats pass.
  The secret-location list covers known places, not every secret. The disclosure line names
  every file before the request; a file read for notes can also carry prompt injection, but
  notes run nothing and propose nothing.
- **Small local models** follow injected instructions more readily and call tools less
  reliably. The policy and approval prompt contain them the same way, but answers are
  less trustworthy.

## Security tests before a release

Per [AGENTS.md](../../AGENTS.md), each release candidate reruns `just ushpy-test` on
every supported OS and Python. The policy, injection, redaction, credential, and
history tests listed above are its adversarial suite. Policy changes add negative
cases to `test_policy.py`; credential or storage changes add cases to
`test_config.py`. [SECTEST.md](SECTEST.md) is the full test plan: test levels,
the test classes with their status, open findings, and the release gate.

## What the Go implementation changes

The Go binary has the threat model above unchanged: the same assets, trust boundaries, threats and
controls, and the same findings in [SECTEST](SECTEST.md). ushpy's security tests that run ush pass
against it (`ush/go/scripts/conform.sh`). This section records only what a compiled implementation
changes; until 2026-10-29 it was ushgo's own SEC.

### What gets better

- **Nothing for root to trust but the binary.** ush under `sudo` runs a root-owned Python and a venv
  of installed packages, and has to keep `PYTHON*` variables and user site directories out
  (`python -I`). A static Go binary (`CGO_ENABLED=0`) has no interpreter, no import path and no
  dynamic libraries beyond the system's. A root-owned binary is the whole install.
- **Memory safety** is the same as Python's; the parser and redaction run on untrusted text in a
  memory-safe language either way.

### What is new

| Risk | Control | TODO |
| --- | --- | --- |
| **Behavioural drift.** The two implementations decide differently on a command line, so a command ush asks about runs unasked in ushgo. | `ush/conform/policy.json`: 1545 command lines with ushpy's verdicts and reasons, compared in full by ushgo's `TestConformance`; ushpy's `test_conform` keeps the corpus current, and ushpy's tokenizer is fuzzed against `zsh -f`. ushpy's black-box tests run against the Go binary (`ushgo-conform`). Any difference fails the build. The corpus found F7 in ushpy. | 4.1, 4.10 (done) |
| **Go toolchain and modules.** A dependency or the toolchain is compromised. | ushgo has no dependencies: standard library only, including the Messages API client (ARCH). `govulncheck` before each release covers the standard library and the toolchain; `GOTOOLCHAIN=local` in tests and CI, so no toolchain is fetched. | 5.3 |
| **An HTTP client of our own.** It mishandles TLS, redirects, or a broken stream. | `net/http` with default TLS verification; no redirects followed; for Ollama, no proxy; a reply cut off mid-stream is an error, not a crash (F6); retries only for connection errors and 408/409/429/5xx. Tests against a mock server, a redirecting server and a server that drops the connection. | 4.6 (done) |
| **Patterns on untrusted text.** Redaction or validation stalls on crafted input. | Go's RE2 matches in linear time; no backtracking pattern can stall (ushpy F3). | 4.2 (done) |
| **Release binaries.** A binary differs from the source, or an archive is altered. | Deterministic builds (`tools/release`: `-trimpath`, no build ID, no symbols, fixed archive order, owners and times from the commit), so anyone can rebuild a tag and compare `SHA256SUMS`; the installer writes nothing writable by others. Unsigned on macOS (GAPS Q11). | 5.1 (5.1.1 done), 5.3 |
| **Shared files with ush.** ushgo writes settings, keys or history that ush then trusts. | The same validation as ush on every read (0600 files, owner, no symlinks); the conformance tests cover the file formats. | 4.4, 4.5 |

### Release gate

Every release of the Go binary runs ush's release gate on it: the conformance suite on each
supported OS, the adversarial tests in SECTEST (levels 1 and 2), `govulncheck`, a secret scan,
and the policy corpus against both implementations.
