// Command ush is the Go implementation of ush, the local `//` shell for zsh: the same command as
// the Python ush (ush/py/src/ush/cli.py), built from ush/go/. ushpy's black-box tests run against
// this binary too (USH_TEST_BIN), so the two behave the same.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/gregoryyy/ushell/ush/go/internal/agent"
	"github.com/gregoryyy/ushell/ush/go/internal/check"
	"github.com/gregoryyy/ushell/ush/go/internal/config"
	"github.com/gregoryyy/ushell/ush/go/internal/execute"
	"github.com/gregoryyy/ushell/ush/go/internal/policy"
	"github.com/gregoryyy/ushell/ush/go/internal/provider"
	"github.com/gregoryyy/ushell/ush/go/internal/ui"
)

// version is set at build time: go build -ldflags "-X main.version=0.2.0".
var version = "0.0.0-dev"

// implementation tells the two implementations of the same command apart in `ush --version`.
const implementation = "go"

// stdinLimit: piped input beyond this is cut.
const stdinLimit = 200_000

var usage = `ush, the you shell: ask in natural language, get the commands that answer it.

usage:
  ush [OPTIONS] //REQUEST...   propose a command for your command line, or answer
  ush [OPTIONS] //             continue: send what you ran, get the next proposal
  ush explain COMMAND...       explain a command line with the model, without running it
                               (with the plugin: Down on a proposal)
  ush notes [INSTRUCTION] FILE...
                               notes on files or piped input with the model, e.g.
                               ush notes 'any errors?' app.log (with the plugin:
                               Ctrl-N, also on a command's output)
  ush [OPTIONS]                start a session
  ush ai configure [--file]    set the provider (Anthropic or a local Ollama), API key
                               (keychain on macOS), model, and effort
  ush ai status                show provider, model, key source, and paths
  ush ai clear-key             remove the stored API key
  ush allow [add|remove RULE]  list, add, or remove commands that may run without
                               approval, e.g. ush allow add kubectl get
  ush history [-n N]           list recent requests and the commands they ran
  ush replay ID|last [-n]      run those commands again, without AI
  ush help                     show this help

options:
  -r, --run          agent mode: ush runs the commands itself, read-only ones at
                     once, others after your approval; then follow-up questions
  -N, --new          start a new conversation instead of continuing this shell's
  -n, --dry-run      agent mode, but show the commands it would run, run none
  -c, --confirm      ask before every command, including read-only ones
  -1, --once         answer once, without follow-up questions
  -v, --verbose      full explanations; in agent mode, complete command output
  -m, --model MODEL  model for this run (default from ` + "`" + `ush ai configure` + "`" + `)
  -e, --effort LEVEL low, medium, high, xhigh, or max
  -k, --checker MODEL checker for this run: claude-haiku-5-5, qwen3.5:4b, or off

By default ush runs nothing: it proposes one command at a time. With the zsh
plugin the proposal lands on your command line; run it with Enter, edit it, or
ignore it, then ` + "`" + `//` + "`" + ` continues with what you ran, its exit status and output.
` + "`" + `ush ai configure` + "`" + ` sets agent mode as the default instead.

In a session or agent-mode follow-up, ` + "`" + `// TEXT` + "`" + ` asks, any other line runs as a
command, and an empty line (follow-up) or ` + "`" + `exit` + "`" + ` ends it. In agent mode,
commands that ush cannot verify as read-only run only after you approve them.
Requests and command output (with secrets redacted) are sent to the configured
AI provider; with a local Ollama model they stay on this machine. A checker
model, if configured, judges each proposed command against your requests only;
what it flags gets a warning, or needs your approval in agent mode.
USH_PROVIDER, USH_MODEL, USH_CHECKER, and USH_OLLAMA_URL override the settings
(not as root).

With the zsh plugin, ` + "`" + `ush //...` + "`" + ` and ` + "`" + `// ...` + "`" + ` need no quoting:
  source ush/shell/ush.plugin.zsh
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// usageError is a configuration or usage problem: exit status 2.
func usageError(format string, a ...any) error { return config.Errorf(format, a...) }

// run is main without the process: it returns the exit status (2 for a usage or configuration
// error, 130 when stopped).
func run(args []string, stdout, stderr io.Writer) int {
	term := ui.New()
	term.Out, term.Err = stdout, stderr
	status, err := realMain(args, term)
	var cfg *config.Error
	if errors.As(err, &cfg) {
		term.Error(cfg.Msg)
		return 2
	}
	if err != nil {
		term.Error(err.Error())
		return 1
	}
	return status
}

type overrides struct {
	model, effort, checker, mode string
	newConversation              bool
	set                          map[string]bool
}

func parseOptions(argv []string) (agent.Options, overrides, []string, error) {
	var o agent.Options
	ov := overrides{set: map[string]bool{}}
	i := 0
	for i < len(argv) && strings.HasPrefix(argv[i], "-") && argv[i] != "-h" && argv[i] != "--help" && argv[i] != "--version" {
		a := argv[i]
		switch {
		case a == "-n" || a == "--dry-run":
			o.DryRun = true
		case a == "-c" || a == "--confirm":
			o.ConfirmAll = true
		case a == "-v" || a == "--verbose":
			o.Verbose = true
		case a == "-1" || a == "--once":
			o.Once = true
		case a == "-r" || a == "--run":
			ov.mode, ov.set["mode"] = "run", true
		case a == "-N" || a == "--new":
			ov.newConversation = true
		case (a == "-m" || a == "--model" || a == "-e" || a == "--effort" || a == "-k" || a == "--checker") && i+1 < len(argv):
			value := argv[i+1]
			switch a {
			case "-e", "--effort":
				if !contains(config.Efforts, value) {
					return o, ov, nil, usageError("effort must be one of %s", strings.Join(config.Efforts, ", "))
				}
				ov.effort, ov.set["effort"] = value, true
			case "-k", "--checker":
				ov.checker, ov.set["checker"] = value, true
			default:
				ov.model, ov.set["model"] = value, true
			}
			i++
		default:
			return o, ov, nil, usageError("unknown option %s; see `ush help`", a)
		}
		i++
	}
	return o, ov, argv[i:], nil
}

func realMain(argv []string, term *ui.Terminal) (int, error) {
	options, ov, rest, err := parseOptions(argv)
	if err != nil {
		return 2, err
	}
	term.Verbose = options.Verbose
	if len(rest) > 0 && (rest[0] == "help" || rest[0] == "-h" || rest[0] == "--help") {
		fmt.Fprint(term.Out, usage)
		return 0, nil
	}
	if len(rest) > 0 && rest[0] == "--version" {
		fmt.Fprintf(term.Out, "ush %s (%s)\n", version, implementation)
		return 0, nil
	}
	id := config.Current()
	settings, err := config.Load(id, policy.ParseRule)
	if err != nil {
		return 2, err
	}
	if err := config.ApplyEnvironment(&settings, id, os.Getenv); err != nil {
		return 2, err
	}
	if ov.set["model"] {
		if err := settings.SetModel(ov.model); err != nil {
			return 2, err
		}
	}
	if ov.set["effort"] {
		settings.Effort = ov.effort
	}
	if ov.set["checker"] {
		if err := settings.SetChecker(ov.checker); err != nil {
			return 2, err
		}
	}
	if ov.set["mode"] {
		if err := settings.SetMode(ov.mode); err != nil {
			return 2, err
		}
	}
	keys := config.NewKeyStore(id)
	hist := newHistory(id)
	if len(rest) > 0 {
		switch rest[0] {
		case "ai":
			return aiCommand(rest[1:], term, id, settings, keys)
		case "history":
			return historyCommand(rest[1:], term, hist)
		case "allow":
			return allowCommand(rest[1:], term, id)
		case "explain":
			return explainCommand(rest[1:], term, id, settings, keys, options)
		case "notes":
			return notesCommand(rest[1:], term, id, settings, keys, options)
		case "replay":
			return replayCommand(rest[1:], term, id, hist, settings, options)
		}
		if !strings.HasPrefix(rest[0], "//") {
			return 2, usageError("unknown command %s; requests start with //, see `ush help`", pyRepr(rest[0]))
		}
	}
	request := ""
	if len(rest) > 0 {
		request = strings.TrimSpace(strings.Join(rest, " ")[2:])
	}
	continuing := len(rest) > 0 && request == "" // `ush //`: continue after a proposal
	piped := ""
	if !ui.IsTerminal(os.Stdin) && (request != "" || continuing) {
		data, _ := io.ReadAll(io.LimitReader(os.Stdin, stdinLimit+1))
		piped = execute.Decode(data[:min(len(data), stdinLimit)])
		if len(data) > stdinLimit {
			piped += "\n[... input shortened ...]"
		}
	}
	options.Propose = settings.Mode == "propose" && !options.DryRun
	factory := &providerFactory{id: id, settings: settings, keys: keys}
	var secrets []string
	if request != "" || continuing {
		if secrets, err = factory.secrets(); err != nil { // fail early, before any command runs
			return 2, err
		}
	}
	a := agent.New(&lazyProvider{factory: factory}, term, settings, id, hist, options, secrets, factory.checker())
	stop := newInterrupts()
	defer stop.close()
	if options.Propose && (request != "" || continuing) {
		status, done, err := proposeStep(a, term, id, settings, options, ov, request, piped, stop)
		if done || err != nil {
			return status, err
		}
		// `//` with nothing to continue opens a session, as before.
	}
	banner(term, id, settings, options)
	if piped != "" {
		a.AddContext("Input piped to ush on stdin:", piped)
	}
	if request == "" {
		if !term.CanPrompt() {
			return 2, usageError("no request given and no terminal for a session; see `ush help`")
		}
		session(a, term, factory, false, stop)
		return 0, nil
	}
	entry := a.Ask(stop.begin(), request, nil, true)
	stop.end()
	if !options.Once && term.CanPrompt() {
		session(a, term, factory, true, stop)
	}
	switch entry.Outcome {
	case "answered", "dry-run":
		return 0, nil
	case "aborted":
		return 130, nil
	}
	return 1, nil
}

// lazyProvider creates the provider on first use, so commands without AI need no key.
type lazyProvider struct {
	factory  *providerFactory
	provider provider.Provider
}

func (l *lazyProvider) Name() string {
	if l.factory.settings.Provider == "ollama" {
		return "Ollama"
	}
	return "Anthropic"
}
func (l *lazyProvider) Model() string { return l.factory.settings.ActiveModel() }
func (l *lazyProvider) get() (provider.Provider, error) {
	if l.provider == nil {
		p, err := l.factory.create()
		if err != nil {
			return nil, err
		}
		l.provider = p
	}
	return l.provider, nil
}
func (l *lazyProvider) Turn(ctx context.Context, system string, tools, messages []provider.Block, onText, onProgress func(string)) (provider.Turn, error) {
	p, err := l.get()
	if err != nil {
		return provider.Turn{}, err
	}
	return p.Turn(ctx, system, tools, messages, onText, onProgress)
}
func (l *lazyProvider) Complete(ctx context.Context, system, text string) (string, error) {
	p, err := l.get()
	if err != nil {
		return "", err
	}
	return p.Complete(ctx, system, text)
}

type providerFactory struct {
	id       config.Identity
	settings config.Settings
	keys     *config.KeyStore
	key      string
}

func (f *providerFactory) ollama() bool { return f.settings.Provider == "ollama" }

// secrets checks that the provider and the checker can be used and returns the secrets to redact.
func (f *providerFactory) secrets() ([]string, error) {
	if f.ollama() && f.settings.OllamaModel == "" {
		return nil, usageError("no Ollama model; run `ush ai configure` or set USH_MODEL (see `ollama list`)")
	}
	if !f.ollama() || f.settings.CheckerProvider() == "anthropic" {
		key, err := f.resolveKey()
		if err != nil {
			return nil, err
		}
		return []string{key}, nil
	}
	return nil, nil
}

func (f *providerFactory) resolveKey() (string, error) {
	if f.key == "" {
		key, _, err := f.keys.Resolve()
		if err != nil {
			return "", err
		}
		if key == "" {
			who := ""
			if f.id.ViaSudo() {
				who = " for " + f.id.OwnerName
			}
			return "", usageError("no API key%s; run `ush ai configure` or set ANTHROPIC_API_KEY", who)
		}
		f.key = key
	}
	return f.key, nil
}

// baseURL: as root, environment overrides of the endpoint are ignored (sudo -E).
func (f *providerFactory) baseURL() string {
	if f.id.IsRoot() {
		return provider.APIURL
	}
	return os.Getenv("USH_BASE_URL")
}

func (f *providerFactory) create() (provider.Provider, error) {
	if f.ollama() {
		return provider.NewOllama(f.settings.OllamaModel, f.settings.OllamaURL), nil
	}
	key, err := f.resolveKey()
	if err != nil {
		return nil, err
	}
	return provider.NewAnthropic(key, f.settings.Model, f.settings.Effort, f.baseURL()), nil
}

// checker is the checker model, if one is configured; its client is created on first use.
func (f *providerFactory) checker() *check.Checker {
	p, model := f.settings.CheckerProvider(), f.settings.CheckerModel()
	if p == "" {
		return nil
	}
	return check.New(model, func() (check.Completer, error) {
		if p == "ollama" {
			return provider.NewOllama(model, f.settings.OllamaURL), nil
		}
		key, err := f.resolveKey()
		if err != nil {
			return nil, err
		}
		return provider.NewAnthropic(key, model, "", f.baseURL()), nil
	})
}

// interrupts turns Ctrl-C into cancelling the running request, or into a flag while ush reads.
type interrupts struct {
	mu          sync.Mutex
	cancel      context.CancelFunc
	interrupted bool
	signals     chan os.Signal
}

func newInterrupts() *interrupts {
	i := &interrupts{signals: make(chan os.Signal, 4)}
	signal.Notify(i.signals, syscall.SIGINT)
	go func() {
		for range i.signals {
			i.mu.Lock()
			if i.cancel != nil {
				i.cancel()
			}
			i.interrupted = true
			i.mu.Unlock()
		}
	}()
	return i
}

func (i *interrupts) begin() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	i.mu.Lock()
	i.cancel, i.interrupted = cancel, false
	i.mu.Unlock()
	return ctx
}

func (i *interrupts) end() {
	i.mu.Lock()
	if i.cancel != nil {
		i.cancel()
	}
	i.cancel = nil
	i.mu.Unlock()
}

// take returns whether Ctrl-C was pressed since the last call.
func (i *interrupts) take() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	was := i.interrupted
	i.interrupted = false
	return was
}

func (i *interrupts) close() { signal.Stop(i.signals) }

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func pyRepr(s string) string {
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		return "\"" + s + "\""
	}
	return "'" + strings.ReplaceAll(s, "'", "\\'") + "'"
}
