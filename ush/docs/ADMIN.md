# ush admin guide

How to install, verify, configure, run under `sudo`, upgrade, and remove `ush` on machines you look
after, and what it touches on the system and the network. For daily use, see the
[user guide](USER.md). Status: 2026-10-29 02:01.

`ush` has two implementations with the same behaviour, files, settings and plugin: the Go one in
`ush/go/`, the deliverable, one static binary per platform, and the Python one in `ush/py/`, where
features are tried first. Switching between them is a reinstall. Sections below say where the two
differ.

## Requirements

| | Go implementation | Python implementation |
| --- | --- | --- |
| Platforms | macOS and Linux, arm64 and amd64 | macOS and Linux |
| Runtime | none: a static binary (on macOS it links the system's `libSystem` only) | Python 3.10 or newer with `venv`; pinned packages in a private venv |
| Shell | zsh 5.8 or newer for the plugin | the same |
| Size | about 1.6 MB today | about 70 MB (venv) |
| Model | an Anthropic API key per user, or Ollama on the same machine | the same |

## Install

### Go: with Homebrew

```sh
brew tap gregoryyy/ush
brew trust gregoryyy/ush                            # Homebrew loads third-party taps only once trusted
brew install ush                                    # macOS or Linux: /opt/homebrew/bin/ush
```

The tap [gregoryyy/homebrew-ush](https://github.com/gregoryyy/homebrew-ush) builds `ush` from the
tagged source with Homebrew's Go, as the release build does (`-trimpath`, no cgo, the version
stamped in), and Homebrew needs current Command Line Tools to build it. The plugin goes to `$(brew --prefix)/share/ush/ush.plugin.zsh`; `brew info ush` prints
the line for `~/.zshrc`; the manual page is `man ush`. A Homebrew prefix is owned by the user, so a Homebrew `ush` is not for
`sudo`: see [For sudo](#for-sudo-a-root-owned-install).

### Go: from a release archive

Each release has one `ush-VERSION-OS-ARCH.tar.gz` per platform and a `SHA256SUMS` file. Verify,
unpack, install:

```sh
shasum -a 256 -c SHA256SUMS --ignore-missing      # or: sha256sum -c SHA256SUMS --ignore-missing
tar -xzf ush-0.2.0-darwin-arm64.tar.gz
ush-0.2.0-darwin-arm64/install.sh                 # for you: ~/.local/bin/ush
```

`install.sh` copies the binary to `PREFIX/bin/ush` (replacing it atomically) and the zsh plugin to
`~/.local/share/ush/` (per user) or `PREFIX/lib/ush/` (with `--prefix`), and the manual page to
`PREFIX/share/man/man1/ush.1` (`man ush` finds it next to the binary on the `PATH`). Run from a terminal, it
offers to add the plugin line to `~/.zshrc` (`--zshrc` adds it without asking, `--no-zshrc` only
prints it); it never adds it twice or as root. These are the Python installer's places, so the
plugin line stays valid when you switch.

**macOS:** the binary is not signed. Archives downloaded with a browser get a quarantine attribute,
and macOS then refuses to run the binary; download with `curl`, or remove the attribute after
checking the checksum (`xattr -d com.apple.quarantine ush`). Homebrew builds from source, so its
binary is not quarantined. Signing and notarization are not planned for now ([GAPS](GAPS.md) Q11).

### Go: from source

```sh
cd ush/go
scripts/release.sh 0.2.0     # all four platforms into dist/, with SHA256SUMS (Go 1.27)
```

The build is deterministic: the same commit and version give the same checksums on any machine, so
you can rebuild a published release and compare `SHA256SUMS` before trusting it. `go build -o ush
./cmd/ush` gives a quick local binary.

### Python

```sh
cd ush/py && ./install.sh                          # for you: ~/.local/bin/ush, ~/.local/share/ush/
```

`install.sh` takes the newest Python 3.10 or newer it finds (or `$PYTHON`), creates a private venv
in `~/.local/share/ush/venv`, installs the hash-pinned packages from `requirements.lock` into it, and
copies the `ush` package in instead of building it, so it fetches no unpinned build tools. The
launcher `~/.local/bin/ush` runs that venv's Python with `-I`, which ignores `PYTHON*` variables,
the user site directory and the current directory. The plugin and `~/.zshrc` are handled as by the
Go installer.

### For sudo: a root-owned install

`sudo ush` runs whatever `ush` it finds, as root. If that file, or anything it loads, is writable by a
user, that user can run code as root. Install a root-owned copy for `sudo` use:

```sh
sudo ush-0.2.0-darwin-arm64/install.sh --prefix /usr/local # Go: /usr/local/bin/ush
sudo ./install.sh --prefix /usr/local                          # Python: plus /usr/local/lib/ush
```

`sudo`'s `secure_path` usually doesn't include `~/.local/bin`; the zsh plugin substitutes the
absolute path of `ush` for `sudo ush //…`. The Go binary is the whole install, which is the main
reason it is the deliverable: no interpreter, venv or `PYTHON*` variables for root to trust.

## Configure

ush has no system-wide configuration; each user configures their own with `ush ai configure`.

| What | Where | Notes |
| --- | --- | --- |
| Settings | `~/.config/ush/config.json` (or `$XDG_CONFIG_HOME/ush/`) | written mode 0600 |
| API key | macOS keychain (service `ush`, account `anthropic`), else `~/.config/ush/credentials` | the file must be a regular file owned by the user (or root), mode 0600, not a symlink, or ush refuses it |
| History | `~/.local/state/ush/history.jsonl` (or `$XDG_STATE_HOME/ush/`) | append-only, mode 0600, no command output |
| Conversations | `~/.local/state/ush/sessions/KEY.json` | propose mode, one per shell, mode 0600, with redacted output; expire after an hour |
| Explanations | `~/.local/state/ush/explanations.json` | the newest 500 answers of `ush explain`, mode 0600, keyed by a hash of the command line (no command lines) |
| Handoff | a `mktemp -d` directory in `$TMPDIR`, as `USH_HANDOFF` | made by the zsh plugin, mode 0700: the proposal, what ran, its output; removed when the shell exits |

Settings in `config.json`:

| Key | Default | Range or values |
| --- | --- | --- |
| `mode` | `propose` | `propose` (ush proposes, the user runs) or `run` (agent mode) |
| `provider` | `anthropic` | `anthropic`, `ollama` |
| `model` | `claude-opus-5-5` | Anthropic model name |
| `effort` | `medium` | `low` … `max` |
| `ollama_url` | `http://127.0.0.1:11434` | loopback only (`127.0.0.1`, `localhost`, `::1`) |
| `ollama_model` | none | a pulled model with tool support |
| `checker` | off | `provider:model`, e.g. `anthropic:claude-haiku-5-5` |
| `allow` | none | the user's allow rules ([USER](USER.md#your-allowlist)) |
| `max_steps` | 30 | 1–200 commands per request |
| `command_timeout` | 120 | 1–3600 s, default per command |
| `max_timeout` | 900 | 1–7200 s, the most a request may ask for |
| `output_limit` | 32000 | 1000–500000 bytes of each output sent to the model |
| `show_lines` | 12 | 0–10000 output lines shown per command |

Environment variables (ignored as root, so `sudo -E` can't redirect a root session):

| Variable | Effect |
| --- | --- |
| `ANTHROPIC_API_KEY`, `USH_API_KEY` | the API key, before the keychain and the file |
| `USH_PROVIDER`, `USH_MODEL`, `USH_OLLAMA_URL`, `USH_CHECKER` | override the settings for one run |
| `USH_BASE_URL` | another Anthropic endpoint, e.g. a company gateway |
| `XDG_CONFIG_HOME`, `XDG_STATE_HOME` | where settings and history live |
| `USH_BARE_SLASH=0` | before sourcing the plugin: no bare `// …` form |
| `USH_HANDOFF` | set by the plugin; ush uses it only if it is a 0700 directory of the user's, never as root |
| `NO_COLOR` | no colours |

ush never reads `.env` files, and it removes `ANTHROPIC_*` and `USH_API_KEY` from the environment of
the commands it runs.

## Under sudo

As root via `sudo`, ush:
- uses the invoking user's key and settings (from `SUDO_USER`); on macOS it reads the keychain as
  that user, which can fail when the keychain is locked: then `ush ai configure --file`;
- writes history to root's state directory;
- ignores `USH_*`, `XDG_*` and `USH_BASE_URL`, and uses none of the user's allow rules;
- in propose mode, prints proposals without putting them on a command line, and doesn't continue a
  conversation across `sudo` calls;
- in agent mode, still asks before every command the policy can't verify.

## Network

| Destination | When | Port |
| --- | --- | --- |
| `api.anthropic.com` | provider or checker `anthropic` | 443 |
| the `USH_BASE_URL` host | if set (not as root) | as given |
| Ollama on loopback | provider or checker `ollama` | 11434 by default |
| `ollama.com`, through Ollama | Ollama *cloud* models (`…:cloud`) | Ollama's own connection |

For Ollama, ush ignores proxy variables and doesn't follow redirects, so "stays on this machine" is
true by construction. The commands ush runs use the network as they would from the user's shell; a
DNS lookup (`dig`, `host`, `nslookup`) of a name needs approval in agent mode, so injected text
can't leak data through DNS names unasked ([SECTEST](SECTEST.md) F2, fixed 2026-10-28).

## Verify an install

```sh
ush --version                                       # ush X.Y.Z (go) or (python)
ush ai status                                       # provider, model, key source, paths
ush/py/scripts/smoke.sh --bin "$(command -v ush)"    # offline checks, from a checkout
ush/py/scripts/smoke.sh --provider ollama --model qwen3.5:9b   # with a local model, no cost
```

The smoke test keeps the user's settings and history out and writes `smoke-report.md`
([SMOKE](SMOKE.md)). Run it after every install and upgrade.

## Upgrade, roll back, remove

- **Homebrew:** `brew upgrade ush`; `brew uninstall ush` and `brew untap gregoryyy/ush` remove it.
- **Upgrade:** install the new release over the old one (Python: check out the new version and run
  `install.sh` again with the same `--prefix`; it recreates the venv). Settings, keys and history
  stay; their formats are the same in both implementations.
- **Roll back:** install the previous release archive the same way (Python: check out the previous
  tag and run `install.sh` again).
- **Switch implementation:** install the other one into the same prefix; nothing else changes.
  From Python to Go, remove the venv afterwards: `PREFIX/lib/ush/venv` or `~/.local/share/ush/venv`.
- **Remove:** delete `PREFIX/bin/ush`, `PREFIX/share/man/man1/ush.1` and `PREFIX/lib/ush/` (or `~/.local/share/ush/`), the plugin
  line in `~/.zshrc`, and per user, if wanted: `ush ai clear-key`, `~/.config/ush/`,
  `~/.local/state/ush/`.

## Security notes

- The approval prompt is the main control. ush shows commands and output with control and escape
  sequences neutralised and refuses proposals that contain them, so what you approve is what runs
  ([SECTEST](SECTEST.md) F1, fixed 2026-10-28). On sensitive machines, use `--confirm` and read
  commands carefully anyway.
- The read-only policy infers, it doesn't sandbox: a "read-only" tool with an unknown side effect, or
  a malicious binary earlier on `PATH`, runs unasked. Keep `PATH` free of writable or relative
  entries for users of ush, especially root.
- Threat model: [SEC](SEC.md), with what the Go build changes in its last section; findings and
  security tests in [SECTEST](SECTEST.md).

