package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gregoryyy/ushell/ushgo/internal/agent"
	"github.com/gregoryyy/ushell/ushgo/internal/config"
	"github.com/gregoryyy/ushell/ushgo/internal/handoff"
	"github.com/gregoryyy/ushell/ushgo/internal/history"
	"github.com/gregoryyy/ushell/ushgo/internal/provider"
	"github.com/gregoryyy/ushell/ushgo/internal/ui"
)

func newHistory(id config.Identity) *history.History { return history.New(id.StateDir()) }

// proposeStep is one step of propose mode: send the request (or what ran), then hand over the
// proposal. done is false when `//` has nothing to continue, which opens a session instead.
func proposeStep(a *agent.Agent, term *ui.Terminal, id config.Identity, settings config.Settings,
	options agent.Options, ov overrides, request, piped string, stop *interrupts) (int, bool, error) {
	dir := handoff.Dir(id.UID, id.IsRoot(), os.Getenv)
	sessions, err := handoff.NewSessions(id.StateDir(), handoff.SessionKey(dir))
	if err != nil {
		return 1, true, err
	}
	if ov.newConversation {
		sessions.Clear()
	} else if saved := sessions.Load(); saved != nil {
		a.Restore(saved)
	}
	if request == "" && piped == "" && a.Proposal == nil {
		return 0, false, nil
	}
	banner(term, id, settings, options)
	if piped != "" {
		a.AddContext("Input piped to ush on stdin:", piped)
	}
	var ran *handoff.Ran
	if dir != "" && a.Proposal != nil {
		ran = handoff.TakeRan(dir, settings.OutputLimit)
	}
	entry := a.Ask(stop.begin(), request, ran, dir != "")
	stop.end()
	if err := sessions.Save(a.State()); err != nil {
		term.Warn("could not save the conversation: " + err.Error())
	}
	if entry.Outcome != "proposed" || a.Proposal == nil {
		switch entry.Outcome {
		case "answered":
			return 0, true, nil
		case "aborted":
			return 130, true, nil
		}
		return 1, true, nil
	}
	if dir != "" {
		interactive, _ := a.Proposal["interactive"].(bool)
		command, _ := a.Proposal["command"].(string)
		if err := handoff.WriteProposal(dir, command, interactive); err == nil {
			term.Info("on your command line: Enter runs it, or edit it first · then // continues")
			return 0, true, nil
		} else {
			term.Warn("could not put it on your command line: " + err.Error())
		}
	}
	if id.IsRoot() {
		term.Info("ush ran nothing: copy the command to run it as root · then `ush //` continues")
		return 0, true, nil
	}
	term.Info("ush ran nothing: copy the command to run it · then `ush //` continues")
	where := " (see `ush help`)"
	if p := pluginPath(); p != "" {
		where = ":\n  source '" + p + "'"
	}
	term.Info("to get it on your command line, add the zsh plugin to ~/.zshrc and open a new terminal" + where)
	return 0, true, nil
}

// pluginPath is the installed zsh plugin: PREFIX/lib/ush, or ~/.local/share/ush for a user install.
func pluginPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	prefix := filepath.Dir(filepath.Dir(exe))
	for _, p := range []string{filepath.Join(prefix, "lib", "ush", "ush.plugin.zsh"),
		filepath.Join(prefix, "share", "ush", "ush.plugin.zsh")} {
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() && !strings.Contains(p, "'") {
			return p
		}
	}
	return ""
}

func banner(term *ui.Terminal, id config.Identity, settings config.Settings, options agent.Options) {
	parts := []string{"ush · " + settings.ActiveModel()}
	if options.Propose {
		parts = append(parts, "proposes, you run")
	}
	if id.IsRoot() {
		root := "as root"
		if id.ViaSudo() {
			root += " (for " + id.OwnerName + ")"
		}
		parts = append(parts, root)
	}
	if options.DryRun {
		parts = append(parts, "dry run")
	}
	parts = append(parts, disclosure(settings))
	term.Info(strings.Join(parts, " · "))
	if id.IsRoot() {
		for _, risk := range config.PathRisks(os.Getenv("PATH"), 0) {
			term.Warn("PATH entry " + risk + ": as root, another program could run instead of a listed one")
		}
	}
	if settings.Checker != "" {
		term.Info(checkerDisclosure(settings))
	}
}

// disclosure: where requests and command output go, as shown before every session.
func disclosure(s config.Settings) string {
	if s.Provider != "ollama" {
		return "requests and command output go to Anthropic, secrets redacted"
	}
	if remote := provider.OllamaRemoteHost(s.OllamaURL, s.OllamaModel); remote != "" {
		return "cloud model: requests and command output go to " + remote + " via Ollama, secrets redacted"
	}
	return "local Ollama (" + s.OllamaURL + "): requests and command output stay on this machine"
}

// checkerDisclosure: where the checker sends requests and proposals (never command output).
func checkerDisclosure(s config.Settings) string {
	model := s.CheckerModel()
	where := "stay on this machine"
	if s.CheckerProvider() == "anthropic" {
		where = "go to Anthropic"
	} else if remote := provider.OllamaRemoteHost(s.OllamaURL, model); remote != "" {
		where = "go to " + remote + " via Ollama"
	}
	return "checker " + model + ": your requests and the proposed commands " + where +
		", secrets redacted; command output is not sent to it"
}

// session is the `ush>` loop: a session, or the follow-up after an agent-mode answer.
func session(a *agent.Agent, term *ui.Terminal, factory *providerFactory, followUp bool, stop *interrupts) {
	switch {
	case followUp:
		term.Info("follow up with // QUESTION, run a command, or press Enter to finish")
	case a.Options.Propose:
		term.Info("// REQUEST asks · proposals appear at the prompt: Enter runs, // continues · exit or Ctrl-D ends")
	default:
		term.Info("// REQUEST asks · other lines run as commands · exit or Ctrl-D ends")
	}
	offer := ""
	for {
		stop.take()
		line, ok := term.ReadLine("ush> ", offer, true)
		offer = ""
		if !ok {
			if stop.take() { // Ctrl-C: a new line, not the end
				term.FinishAnswer()
				term.Err.Write([]byte("\n"))
				continue
			}
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			if followUp {
				break
			}
			continue
		}
		if line == "exit" || line == "quit" {
			break
		}
		if strings.HasPrefix(line, "//") {
			request := strings.TrimSpace(line[2:])
			if request == "" && a.Proposal == nil {
				continue
			}
			secrets, err := factory.secrets()
			if err != nil {
				term.Error(err.Error())
				continue
			}
			a.Secrets = secrets
			a.Ask(stop.begin(), request, nil, true)
			stop.end()
			if a.Proposal != nil {
				offer, _ = a.Proposal["command"].(string)
			}
		} else {
			a.RunExplicit(stop.begin(), line)
			stop.end()
		}
	}
}
