# ush security model

Status: applies to ush 0.1.0 (2026-10-26) and the local Ollama provider (TODO 2.8,
2026-10-27). Design: [DESIGN.md](DESIGN.md). Security tests and open findings:
[SECTEST.md](SECTEST.md).

ush lets an AI model run commands on the user's machine, possibly as root.
The protections are the user's approval, a conservative read-only policy, bounded
execution, and limited disclosure. ush is not a sandbox.

## Assets

- The system and the user's data: integrity (no unapproved changes) and
  confidentiality (what reaches the provider, and whether ush says so truthfully).
- The provider API key.
- Root privileges when run under sudo.
- History, which reveals what was investigated.

## Trust boundaries

| Source | Trust | Consequence |
| --- | --- | --- |
| The user at the terminal | Trusted | Approvals, edits, explicit session commands |
| Model output (commands, scripts, `effect` claims) | Untrusted | Policy plus approval; claims never widen permissions |
| Command output, piped input, file contents | Untrusted data | May carry prompt injection; never grants anything |
| Environment (`USH_BASE_URL`, `USH_PROVIDER`, `USH_MODEL`, `USH_OLLAMA_URL`, `XDG_*`) under root | Untrusted | Ignored as root |
| Ollama endpoint and its model list | Trusted for local models only | Must be a loopback address; cloud models are disclosed |
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
| Runaway or hanging commands | stdin `/dev/null`, per-command timeout (≤ `max_timeout`), process-group kill, output limit, step limit per request | `test_execute`, `test_agent` step limit |
| Aborted or interrupted requests leave a broken conversation | Every `tool_use` answered; Ctrl-C kills the process group | `test_agent` abort/truncation/refusal |
| Root running user-writable code | `install.sh --prefix /usr/local` for root-owned installs; launcher uses `python -I` (ignores `PYTHON*`, user site, cwd) | Manual; documented |
| User aliases or rc files changing command meaning | Commands run with `zsh -f` | `test_execute` |

## Residual risks

- **The policy infers, it does not enforce.** A listed command with an unknown side
  effect (or a malicious binary earlier on `PATH`) would run unprompted. Planned:
  OS sandboxing for auto-approved commands (ush TODO 2.6).
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
- **Network egress** by read-only commands (`dig`, `host`, `nslookup`) is allowed,
  which an injection can use to leak data through DNS names (SECTEST F2).
- **Terminal output is not filtered** (SECTEST F1): escape sequences in a command or its
  output can change what the approval prompt shows. Fix planned in TODO 2.9.
- **Small local models** follow injected instructions more readily and call tools less
  reliably. The policy and approval prompt contain them the same way, but answers are
  less trustworthy.

## Security tests before a release

Per [CLAUDE.md](../../CLAUDE.md), each release candidate reruns `just ush-test` on
every supported OS and Python. The policy, injection, redaction, credential, and
history tests listed above are its adversarial suite. Policy changes add negative
cases to `test_policy.py`; credential or storage changes add cases to
`test_config.py`. [SECTEST.md](SECTEST.md) is the full test plan: test levels,
the test classes with their status, open findings, and the release gate.
