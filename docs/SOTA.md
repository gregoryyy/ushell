# State of the art

What exists next to this repository's two projects, and what sets them apart: **ush**, the `//`
shell for zsh, and **wish**, the browser workspace shell. Status: 2026-10-29 01:29.

- **ush:** this file. Sources were checked on 2026-10-29: maintainer documentation,
  repositories and papers. License, latest release and activity come from the GitHub API on that
  day.
- **wish:** a summary of [wish/docs/SOTA.md](../wish/docs/SOTA.md), researched 2026-10-13/14 and
  not rechecked here.

Products change quickly; recheck a fact before relying on it in a decision. "Unverified" means no
primary source confirmed it. The comparisons describe documented behaviour, not measured quality.

## ush

ush turns a request typed at the zsh prompt (`// why is my disk full?`) into one proposed command
on the user's own command line. The user reads it, edits it, runs it with Enter, and `//`
continues with what actually ran and its output. Down explains a command. Agent mode (`--run`,
behind a flag) runs commands that ush's policy verifies as read-only itself and asks before
anything else. Requests go to Anthropic, or to a local Ollama model and stay on the machine.

### Differentiators

1. **Small and focused.** ush is meant to give maximum productivity in a manual workflow, not
   agentic automation. It proposes one command at a time into the shell the user already works in.
   Planning, multi-step autonomy and file editing are left to coding agents (Claude Code, Codex
   CLI, Gemini CLI), which ush doesn't try to be (ush GAPS Q8).
2. **The user's shell, and what really ran.** The proposal is ordinary text on the zsh command
   line: `cd` and variables persist, and it lands in the shell's history. The plugin records the
   command line the user actually ran, edited or not, with its exit status and output, and `//`
   continues from there.
   - shell_gpt, llm-cmd, aichat, ShellOracle and zsh_codex pre-fill or insert a command, but
     don't continue from its result.
   - Butterfish and Warp do see output, but they wrap the shell or replace the terminal.
   - Atuin AI keeps the shell, and by default sends only the OS and shell.
3. **Deterministic read-only verification.** Agent mode runs a command unasked only if ush's own
   parser verifies it as read-only: no shell tricks, no secret files, no network lookups of
   chosen names. The model's own claim never counts.
   - Kiro CLI and Claude Code use prefix rules plus a built-in read-only set.
   - Codex CLI's `auto_review` and Copilot CLI's "assisted" mode let a model approve actions.
   - ush deliberately keeps a model out of approving. Its optional checker model can only add a
     question, never approve (the "dual LLM" pattern below).
4. **Explanations of access.** Down shows, above the command, what each part does and what it
   reads and writes on disk and on the network, and whether secrets end up exposed. Each command
   line is explained once (ush TODO 2.24). shell_gpt's `--describe-shell` and explainshell
   explain syntax, not access.
5. **Privacy by construction.**
   - Known secret formats are redacted before anything is sent.
   - A banner says where requests go.
   - Ollama is accepted only on loopback, without proxies or redirects, so "stays on this
     machine" holds.
   - History stores no command output.
6. **Two implementations held to one behaviour.**
   - ushpy (Python) is where features are tried; ushgo (Go) is the static binary that releases
     ship.
   - A corpus of 1545 command lines pins the policy verdicts.
   - ushpy's black-box tests run against the Go binary.
   - Releases are reproducible: the same checksums on macOS and Linux.
7. **Injection tested at release.** A model-in-the-loop eval plants instructions in READMEs,
   logs, commit messages, HTML, file names and DNS answers. Its results are part of the release
   gate (ush SECTEST), including the open finding F8.

### Comparison

| Tool | Proposes or runs | Shell integration | Context sent | Safety | Local models | Licence, status |
| --- | --- | --- | --- | --- | --- | --- |
| **ush** | Proposes one command; agent mode runs verified read-only ones | zsh plugin: `//`, pre-filled line, Down explains, Esc removes | Request, OS, working directory, output of what ran (redacted) | Read-only parser, approval, checker model, redaction | Ollama, loopback only | MIT; 0.2.0-rc1 |
| [Atuin AI](https://blog.atuin.sh/atuin-v18-13) | Proposes; Enter runs, Tab inserts for editing | `?` on an empty prompt in zsh, bash, fish (a PTY proxy draws the popup) | OS and shell by default | Static checks plus an LLM guardrail; dangerous commands need Enter twice | Unverified; uses hosted models | Atuin's client is MIT, the AI service's terms unverified; v18.24.2, 2026-10-22 |
| [shell_gpt](https://github.com/TheR1D/shell_gpt) | Proposes: Execute, Describe, Abort | Ctrl-L in zsh and bash replaces the line | The request | The prompt only | Through LiteLLM, "not optimized" | MIT; v1.5.1, 2026-05 |
| [llm-cmd](https://github.com/simonw/llm-cmd) | Proposes in an editable prompt | `llm cmd …` | The request | A README warning | Any `llm` model | Apache-2.0; alpha, dormant since 2025-05 |
| [aichat](https://github.com/sigoden/aichat) | Proposes (`-e`) | Alt-E in zsh replaces the line | The request, OS and shell | Unverified | Ollama | MIT or Apache-2.0; v0.30.0, 2025-07 |
| [ShellOracle](https://github.com/djcopley/ShellOracle) | Proposes, inserted at the prompt | Ctrl-F in zsh, bash, fish | Text left of the cursor | None documented | Ollama, LocalAI, LM Studio | GPL-3.0; v1.12.0, 2026-09 |
| [zsh_codex](https://github.com/tom-doerr/zsh_codex) | Replaces a comment with a command | Ctrl-X in zsh | The line | None | Ollama via base URL | MIT; dormant since 2025-03 |
| [Butterfish](https://github.com/bakks/butterfish) | `@` proposes one command; `!` is an agent that confirms each step | Wraps the shell | History with command output (1024 tokens per block) | Confirmation per step | Any OpenAI-compatible endpoint | MIT; v0.4.3, 2026-06 |
| [Kiro CLI](https://kiro.dev/docs/reference/cli-commands/) (formerly Amazon Q Developer CLI, renamed 2025-11-17) | `translate` proposes; chat runs | Inline ghost-text suggestions | Unverified | `permissions.yaml` allow/ask/deny by prefix; compound commands split; read-only presets; workspace trust; a sandbox | Unverified | Unverified (the Q CLI repository was Apache-2.0) |
| [GitHub Copilot CLI](https://docs.github.com/en/copilot/concepts/agents/about-copilot-cli) (`gh copilot` retired 2025-10-25) | Runs, as an agent | Its own prompt | Repository and session | Approval per tool, allow/deny patterns, `/sandbox`, an LLM auto-approver | BYOK, e.g. Ollama | Proprietary; GA 2026-02-25 |
| [Warp](https://github.com/warpdotdev/warp) | Agent mode runs; "Next Command" suggests | Replaces the terminal | Terminal blocks, history, exit codes | Regex allow and deny lists | Unverified | AGPL-3.0 core since 2026-04-28 |
| [Claude Code](https://code.claude.com/docs/en/permissions) | Runs, as an agent | Its own prompt | Repository and session | Permission modes and rules, a built-in read-only set, an optional OS sandbox (Seatbelt; bubblewrap and seccomp) with a network allowlist | Unverified | Proprietary |
| [Codex CLI](https://learn.chatgpt.com/docs/sandboxing) | Runs, as an agent | Its own prompt | Repository and session | Sandbox modes `read-only`, `workspace-write`, full access; Seatbelt, bubblewrap; approval policies; `auto_review` | Unverified (Open Interpreter's fork targets them) | Apache-2.0; rust-v0.165.0 |
| [Gemini CLI](https://geminicli.com/docs/cli/sandbox/) | Runs, as an agent | Its own prompt; `!` shell mode | Repository and session | Approval modes, policy engine, opt-in sandbox (Seatbelt, containers, gVisor) | Unverified | Apache-2.0; v0.66.0 |
| [goose](https://goose-docs.ai/docs/guides/managing-tools/goose-permissions) | Runs; autonomous by default | Its own prompt | Session | Manual, smart (read-only auto) or chat-only modes; no sandbox documented | Ollama | Apache-2.0; Linux Foundation's Agentic AI Foundation |
| [explainshell](https://github.com/idank/explainshell) | Explains only | Web | The command | Not applicable | Optional LLM extractor that cites man-page lines | GPL-3.0; active |
| [tldr-pages](https://github.com/tldr-pages/tldr) | Examples only | Clients | None | Not applicable | Not applicable | CC BY 4.0; very active |

Open Interpreter was rewritten in Rust as a Codex fork for low-cost and open models (2026-06,
Apache-2.0); the Python original continues as a community fork (AGPL-3.0).

### Where ush is behind, and what is planned

| Gap | Who has it | ush |
| --- | --- | --- |
| Enforced read-only: an OS sandbox | Claude Code, Codex CLI, Gemini CLI, Copilot CLI, Kiro CLI | ush TODO 2.6, first of the agent-mode work: no writes and no network for what agent mode runs unasked |
| Untrusted output choosing what runs | CaMeL's data-flow policies (research) | ush TODO 2.23: a command whose arguments come only from earlier output asks first (finding F8) |
| History as context | Butterfish, Warp | ush TODO 2.26: recent history, filtered, off by default |
| bash and fish | Atuin AI, ShellOracle, shell_gpt (bash) | ush TODO 2.27 (bash), 2.28 (fish) |
| Notes on output and files | Butterfish, Warp (blocks as context) | Built 2026-10-29 (ush TODO 2.25): Ctrl-X n and `ush notes` |
| Many model platforms | aichat, ShellOracle, shell_gpt (through LiteLLM), Copilot CLI (BYOK) | ush TODO 2.29: OpenAI-compatible platforms and local servers; 2.30: Bedrock, Vertex AI |
| Explanations grounded in man pages | explainshell's extractor | Not planned; a possible check on Down's explanations |

### Research

- **Command generation:**
  - NL2Bash (Lin et al., LREC 2018) has about 9,300 English-to-Bash pairs over 102 utilities:
    [arXiv:1802.08979](https://arxiv.org/abs/1802.08979).
  - The NLC2CMD competition (Agarwal et al., NeurIPS 2020) built on it:
    [arXiv:2103.02523](https://arxiv.org/abs/2103.02523).
  - InterCode (Yang et al., NeurIPS 2023) makes Bash interactive with execution feedback:
    [arXiv:2306.14898](https://arxiv.org/abs/2306.14898).
  - Terminal-Bench 2.0 (2026-01) has 89 hard CLI tasks; frontier agents solve under 65%:
    [arXiv:2601.11868](https://arxiv.org/abs/2601.11868).
- **Injection through what the model reads:**
  - Greshake et al. (AISec 2023) showed that retrieved content becomes instructions:
    [arXiv:2302.12173](https://arxiv.org/abs/2302.12173). For ush, that content is command output.
  - AgentDojo (NeurIPS 2024) benchmarks this for tool-using agents:
    [arXiv:2406.13352](https://arxiv.org/abs/2406.13352).
- **Defences ush follows:**
  - Simon Willison's [dual LLM pattern](https://simonwillison.net/2023/Apr/25/dual-llm-pattern/)
    keeps untrusted text away from the privileged model. ush's checker sees only the user's
    requests and the proposal, never output.
  - CaMeL (Google DeepMind, 2025) separates control flow from data flow and enforces capability
    policies at tool calls: [arXiv:2503.18813](https://arxiv.org/abs/2503.18813). TODO 2.23 is
    a small, deterministic instance of that idea.

**Unverified:** Warp's explain feature, autonomy levels and local models; local models in Gemini
CLI, Atuin AI, Claude Code and Codex CLI; aichat's current menu; the NLC2CMD metric; an end-of-support date for Amazon
Q.

## wish

Summary of [wish/docs/SOTA.md](../wish/docs/SOTA.md), which has the full comparison, maturity and
licence review.

- **Closest precedents:**
  - WebVM, for a browser shell with AI command execution.
  - JupyterLite and marimo, for browser Python notebooks.
  - Puter, for browser applications with user-owned cloud resources.
  - Browser Use, Stagehand and agent-browser, for natural-language browser control.
  - WebMCP (experimental), for pages that expose structured tools.
- **First candidates to reuse:** just-bash (Apache-2.0, beta) as the shell library; Pyodide
  (MPL-2.0) with one notebook UI for Python; DuckDB-Wasm, SQLite Wasm or PGlite when a workload
  needs them.
- **Licence boundaries that shape the design:**
  - WebVM's CheerpX engine has custom terms.
  - The Wasmer SDK is Modified MIT.
  - The Puter platform is AGPL-3.0.
  - WebContainers has commercial runtime terms.

### Differentiators

These are design intentions, not demonstrated advantages: wish is not built yet.

1. **One connected workflow.** A shell edit, a Python cell, a query and a live tool work on the
   same workspace files, with versions and conflict checks. That holds in place of separate
   tools with copies of the data.
2. **`//` programs that stay useful.** A request produces a displayed, validated program for the
   same executor as typed commands. It can be inspected, edited and rerun without another model
   call, under the same grants whether a person or a model wrote it. This is the trait wish
   shares with ush: the model proposes, the user decides and keeps the result.
3. **Explicit placement and authority.** Storage, compute and the AI provider are separate,
   visible choices. Permission to read a file stays separate from permission to disclose it to a
   provider.
4. **Lean first.** Phase 1 is just-bash, Pyodide and one notebook UI over checked files. It
   adds no new shell dialect or type system, and reuses runtimes rather than rebuilding them.

What would show the distinction: import a CSV, change it from the shell, plot it in a notebook,
save and reload, and replay an AI-generated program. All of it should run with little glue and
under enforced grants.
