// Package agent: the request loop (ushpy agent.py). In propose mode (the default) ush runs
// nothing: the model proposes one command at a time, the user runs it in their own shell, and the
// next request carries what ran. In agent mode (--run) ush runs the model's commands itself,
// read-only ones at once, others after approval.
//
// The conversation is append-only. Every tool_use gets a tool_result, including when a command is
// declined, skipped, or interrupted, so follow-up questions remain valid requests.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/gregoryyy/ushell/ush/go/internal/check"
	"github.com/gregoryyy/ushell/ush/go/internal/config"
	"github.com/gregoryyy/ushell/ush/go/internal/execute"
	"github.com/gregoryyy/ushell/ush/go/internal/handoff"
	"github.com/gregoryyy/ushell/ush/go/internal/history"
	"github.com/gregoryyy/ushell/ush/go/internal/policy"
	"github.com/gregoryyy/ushell/ush/go/internal/provider"
	"github.com/gregoryyy/ushell/ush/go/internal/redact"
	"github.com/gregoryyy/ushell/ush/go/internal/ui"
)

// Control: C0 controls but tab and newline, DEL, C1, bidi overrides and isolates, zero-width.
var Control = regexp.MustCompile(`[\x00-\x08\x0b-\x1f\x7f-\x{9f}\x{200b}-\x{200f}\x{202a}-\x{202e}\x{2066}-\x{2069}\x{feff}]`)

// Options of a request.
type Options struct {
	DryRun, ConfirmAll, Verbose, Once, Propose bool
}

// Agent holds the conversation.
type Agent struct {
	Provider provider.Provider
	UI       *ui.Terminal
	Settings config.Settings
	ID       config.Identity
	History  *history.History
	Options  Options
	Secrets  []string
	Checker  *check.Checker

	Messages     []block
	Requests     []string // what the user asked, for the checker
	pending      []block
	pendingSteps []history.Step
	steps        int
	Proposal     block   // propose mode: id, command, purpose, verbose, interactive, cwd
	Unanswered   []block // results due with the next request
	Ran          *handoff.Ran
	system       string
	tools        []block
}

var errAbort = errors.New("the user stopped the request")

// New prepares the system prompt and tools for the mode.
func New(p provider.Provider, t *ui.Terminal, s config.Settings, id config.Identity, h *history.History, o Options, secrets []string, c *check.Checker) *Agent {
	a := &Agent{Provider: p, UI: t, Settings: s, ID: id, History: h, Options: o, Secrets: secrets, Checker: c}
	root := ""
	if id.IsRoot() {
		root = " (root)"
		if id.ViaSudo() {
			root = fmt.Sprintf(" (root, via sudo by %s)", id.OwnerName)
		}
	}
	shell := strings.Join(execute.FindShell()[:len(execute.FindShell())-1], " ")
	cwd, _ := os.Getwd()
	prompt := systemRun
	a.tools = Tools
	if o.Propose {
		prompt, a.tools = systemPropose, []block{ProposeTool}
	}
	a.system = strings.NewReplacer("{os}", OSName(), "{user}", id.Name, "{root}", root, "{cwd}", cwd,
		"{shell}", shell).Replace(prompt)
	return a
}

// System is the system prompt, for tests.
func (a *Agent) System() string { return a.system }

// OSName as the model should see it, e.g. "macOS 26.2 (arm64)".
func OSName() string {
	if runtime.GOOS == "darwin" {
		if out, err := exec.Command("/usr/bin/sw_vers", "-productVersion").Output(); err == nil {
			arch := runtime.GOARCH
			if arch == "amd64" {
				arch = "x86_64"
			}
			return fmt.Sprintf("macOS %s (%s)", strings.TrimSpace(string(out)), arch)
		}
	}
	if out, err := exec.Command("uname", "-sr").Output(); err == nil {
		return strings.ReplaceAll(strings.TrimSpace(string(out)), " ", "-")
	}
	return runtime.GOOS
}

func (a *Agent) clean(text string) string { return redact.Text(text, a.Secrets...) }

// AddContext queues context (piped input, explicit commands) for the next request.
func (a *Agent) AddContext(label, text string) {
	a.pending = append(a.pending, block{"type": "text", "text": label + "\n" + a.clean(text)})
}

// RunExplicit runs a command the user typed in a session; its output becomes context.
func (a *Agent) RunExplicit(ctx context.Context, command string) {
	r, err := execute.RunCommand(ctx, command, execute.Options{Timeout: 24 * time.Hour,
		Limit: a.Settings.OutputLimit, Interactive: true, Echo: func(b []byte) { a.UI.Out.Write(b) }})
	if errors.Is(err, execute.ErrCancelled) {
		a.UI.Warn("interrupted")
		return
	}
	if r.ExitCode != nil && *r.ExitCode != 0 {
		a.UI.Info(fmt.Sprintf("exit %d", *r.ExitCode))
	}
	if a.Proposal != nil {
		// In a session the proposal is pre-filled at the prompt: what runs answers it.
		a.Ran = &handoff.Ran{Command: command, ExitCode: r.ExitCode, Output: r.Output, Captured: true, Truncated: r.Truncated}
		return
	}
	output := r.Output
	if output == "" {
		output = "(no output)"
	}
	a.AddContext(fmt.Sprintf("The user ran this command themselves:\n$ %s\n(exit %s)", command, exitText(r.ExitCode)), output)
	a.pendingSteps = append(a.pendingSteps, history.Step{Kind: "explicit", Command: command,
		Decision: "explicit", ExitCode: r.ExitCode, Duration: r.Duration})
}

func exitText(code *int) string {
	if code == nil {
		return "None"
	}
	return fmt.Sprint(*code)
}

// Ask sends a request, or with an empty one continues after a proposal. ran is what the user ran
// since the proposal; tracked is false when ush can't know (no shell integration).
func (a *Agent) Ask(ctx context.Context, request string, ran *handoff.Ran, tracked bool) *history.Entry {
	label := request
	if label == "" {
		label = "(continue)"
	}
	entry := a.History.NewEntry(label, a.Provider.Model(), a.ID.Name)
	entry.Steps = append(entry.Steps, a.pendingSteps...)
	a.pendingSteps = nil
	var results []block
	if a.Proposal != nil {
		if ran == nil {
			ran = a.Ran
		}
		a.Ran = nil
		results = a.proposalResults(ran, tracked)
		if ran != nil {
			entry.Steps = append(entry.Steps, history.Step{Kind: "command", Command: ran.Command,
				Decision: "user-ran", ExitCode: ran.ExitCode})
		}
	}
	content := append(append([]block{}, results...), a.pending...)
	if request != "" {
		content = append(content, block{"type": "text", "text": request})
		a.Requests = append(a.Requests, request)
	}
	a.Messages = append(a.Messages, block{"role": "user", "content": content})
	a.pending = nil
	a.steps = 0
	outcome, answer, err := a.loop(ctx, entry)
	switch {
	case err == nil:
	case errors.Is(err, context.Canceled), errors.Is(err, errAbort):
		a.UI.Warn("stopped")
		outcome = "aborted"
	default:
		a.UI.Error(err.Error())
		outcome = "error"
	}
	a.UI.FinishAnswer()
	entry.Outcome, entry.Answer = outcome, a.clean(answer)
	if err := a.History.Append(entry); err != nil {
		a.UI.Warn("could not save history: " + err.Error())
	}
	return entry
}

func (a *Agent) loop(ctx context.Context, entry *history.Entry) (string, string, error) {
	for {
		turn, err := a.Provider.Turn(ctx, a.system, a.tools, a.Messages, a.UI.Answer, a.UI.Progress)
		if err != nil {
			if ctx.Err() != nil {
				return "aborted", "", context.Canceled
			}
			return "error", "", err
		}
		content := provider.EchoContent(turn.Content)
		a.Messages = append(a.Messages, block{"role": "assistant", "content": content})
		var text strings.Builder
		var uses []block
		for _, b := range content {
			switch b["type"] {
			case "text":
				text.WriteString(str(b["text"]))
			case "tool_use":
				uses = append(uses, b)
			}
		}
		switch {
		case turn.StopReason == "refusal":
			msg := "the model declined this request"
			if turn.Refusal != "" {
				msg += ": " + turn.Refusal
			}
			a.UI.Warn(msg)
			a.answerUnrun(uses, "Not run: the response was declined.")
			return "refused", text.String(), nil
		case turn.StopReason == "max_tokens" && len(uses) > 0:
			a.UI.Error("the response was cut off; nothing from it was run")
			a.answerUnrun(uses, "Not run: the tool input was cut off.")
			return "error", text.String(), nil
		case turn.StopReason == "pause_turn":
			continue
		case len(uses) == 0:
			if a.Options.DryRun {
				return "dry-run", text.String(), nil
			}
			return "answered", text.String(), nil
		}
		if a.Options.Propose {
			results := a.propose(ctx, uses, entry)
			if a.Proposal != nil {
				a.Unanswered = results
				return "proposed", text.String(), nil
			}
			a.Messages = append(a.Messages, block{"role": "user", "content": results})
			continue
		}
		var results []block
		aborted := false
		for _, b := range uses {
			if aborted {
				results = append(results, result(b, "Not run: the user stopped the request.", false))
				continue
			}
			r, err := a.tool(ctx, b, entry)
			if err != nil {
				aborted = true
				results = append(results, result(b, "Not run or interrupted: the user stopped the request.", false))
				continue
			}
			results = append(results, r)
		}
		a.Messages = append(a.Messages, block{"role": "user", "content": results})
		if aborted {
			return "aborted", text.String(), errAbort
		}
	}
}

func (a *Agent) answerUnrun(uses []block, reason string) {
	if len(uses) == 0 {
		return
	}
	var results []block
	for _, b := range uses {
		results = append(results, result(b, reason, false))
	}
	a.Messages = append(a.Messages, block{"role": "user", "content": results})
}

func (a *Agent) rules() [][]string {
	if a.ID.IsRoot() {
		return nil // a root session uses no allow rules: they were given for the user's own account
	}
	return a.Settings.AllowRules()
}

// propose presents the first valid proposal; the others get results due with the next request.
func (a *Agent) propose(ctx context.Context, uses []block, entry *history.Entry) []block {
	var results []block
	for _, b := range uses {
		name, args := str(b["name"]), b["input"]
		if a.Proposal != nil {
			results = append(results, result(b, "Not proposed: propose one command at a time.", false))
			continue
		}
		problem := validate(name, args, a.tools)
		if problem == "" && a.steps >= a.Settings.MaxSteps {
			problem = fmt.Sprintf("the limit of %d proposals per request is reached; answer with what you have", a.Settings.MaxSteps)
		}
		if problem != "" {
			results = append(results, result(b, invalid(args, problem), true))
			continue
		}
		in := args.(block)
		a.steps++
		command := strings.TrimSpace(str(in["command"]))
		purpose := truncate(strings.Join(strings.Fields(str(in["purpose"])), " "), 200)
		verdict := policy.Classify(command, a.rules()) // advisory only: the user runs it
		var notes []string
		if in["effect"] == "change" {
			notes = append(notes, "changes something; read it before you run it")
		} else if !verdict.Auto {
			notes = append(notes, "not verified as read-only: "+verdict.Reason)
		}
		step := history.Step{Kind: "proposal", Command: command, Purpose: purpose, Decision: "proposed"}
		if a.Checker != nil {
			v := a.Checker.Check(ctx, a.cleanAll(a.Requests), "command", a.clean(command), a.clean(purpose), str(in["effect"]))
			step.Check = v.Summary()
			if !v.OK() {
				what := "gave no verdict"
				if v.Status == "flag" {
					what = "flagged it"
				}
				notes = append(notes, fmt.Sprintf("the checker %s: %s", what, v.Reason))
			}
		}
		a.UI.ShowProposal(command, purpose, notes)
		entry.Steps = append(entry.Steps, step)
		cwd, _ := os.Getwd()
		a.Proposal = block{"id": b["id"], "command": command, "purpose": purpose, "verbose": a.Options.Verbose,
			"interactive": in["interactive"] == true, "cwd": cwd}
	}
	return results
}

// proposalResults: the result of the pending proposal, and the results due with it.
func (a *Agent) proposalResults(ran *handoff.Ran, tracked bool) []block {
	p, due := a.Proposal, a.Unanswered
	a.Proposal, a.Unanswered = nil, nil
	var text string
	if ran == nil {
		text = "The user did not run the proposed command."
		if !tracked {
			text = "ush cannot tell whether the user ran the proposed command (no shell " +
				"integration); if they did, they may describe or paste the result."
		}
	} else {
		lines := []string{"The user ran an edited command instead:\n$ " + a.clean(ran.Command)}
		if ran.Command == str(p["command"]) {
			lines[0] = "The user ran the proposed command."
		}
		if ran.ExitCode != nil {
			lines = append(lines, fmt.Sprintf("Exit status %d.", *ran.ExitCode))
		} else {
			lines = append(lines, "Exit status unknown.")
		}
		if cwd, _ := os.Getwd(); cwd != str(p["cwd"]) {
			lines = append(lines, "The working directory is now "+cwd+".")
		}
		if !ran.Captured {
			lines = append(lines, "Its output was not recorded (interactive).")
		} else {
			if ran.Truncated {
				lines = append(lines, "Output was shortened; the middle part is omitted.")
			}
			out := a.clean(ran.Output)
			if out == "" {
				out = "(no output)"
			}
			lines = append(lines, out)
		}
		text = strings.Join(lines, "\n")
	}
	return append([]block{result(block{"id": p["id"]}, text, false)}, due...)
}

// State is the conversation, to continue it in the next `//` (handoff.Sessions).
func (a *Agent) State() block {
	var proposal any
	if a.Proposal != nil {
		proposal = a.Proposal
	}
	return block{"version": 1, "messages": nonNil(a.Messages), "requests": nonNilS(a.Requests),
		"proposal": proposal, "unanswered": nonNil(a.Unanswered)}
}

// Restore continues a saved conversation; anything unknown is ignored.
func (a *Agent) Restore(data block) {
	if v, _ := data["version"].(float64); v != 1 {
		return
	}
	messages, ok := data["messages"].([]any)
	if !ok {
		return
	}
	a.Messages = toBlocks(messages)
	a.Requests = nil
	if rs, ok := data["requests"].([]any); ok {
		for _, r := range rs {
			if s, ok := r.(string); ok {
				a.Requests = append(a.Requests, s)
			}
		}
	}
	if p, ok := data["proposal"].(block); ok && p["id"] != nil && p["command"] != nil && p["cwd"] != nil {
		a.Proposal = p
		if due, ok := data["unanswered"].([]any); ok {
			a.Unanswered = toBlocks(due)
		}
	}
}

func (a *Agent) cleanAll(texts []string) []string {
	out := make([]string, len(texts))
	for i, t := range texts {
		out[i] = a.clean(t)
	}
	return out
}

// tool handles one tool call in agent mode.
func (a *Agent) tool(ctx context.Context, b block, entry *history.Entry) (block, error) {
	name, args := str(b["name"]), b["input"]
	if problem := validate(name, args, Tools); problem != "" {
		return result(b, invalid(args, problem), true), nil
	}
	in := args.(block)
	if a.steps >= a.Settings.MaxSteps {
		return result(b, fmt.Sprintf("Not run: the limit of %d commands per request is reached. Answer with what you have.", a.Settings.MaxSteps), true), nil
	}
	a.steps++
	timeout := int(in["timeout_seconds"].(float64))
	if timeout == 0 {
		timeout = a.Settings.CommandTimeout
	}
	timeout = min(max(timeout, 1), a.Settings.MaxTimeout)
	purpose := truncate(strings.Join(strings.Fields(str(in["purpose"])), " "), 200)
	var command, reason string
	var auto bool
	var step *history.Step
	if name == "run_command" {
		command = strings.TrimSpace(str(in["command"]))
		verdict := policy.Classify(command, a.rules())
		auto = verdict.Auto && in["effect"] == "read" && !a.Options.ConfirmAll
		switch {
		case verdict.Reason != "":
			reason = verdict.Reason
		case in["effect"] == "change":
			reason = "marked as a change"
		case a.Options.ConfirmAll:
			reason = "--confirm asks for every command"
		}
		rule := ""
		if auto {
			rule = verdict.Rule
		}
		entry.Steps = append(entry.Steps, history.Step{Kind: "command", Command: command, Purpose: purpose, Rule: rule})
		note := ""
		if rule != "" {
			note = "allowed by your rule: " + rule
		}
		a.UI.ShowCommand(command, purpose, note)
	} else {
		command, reason = str(in["source"]), "scripts always need approval"
		entry.Steps = append(entry.Steps, history.Step{Kind: "script", Command: command, Language: str(in["language"]), Purpose: purpose})
		a.UI.ShowScript(str(in["language"]), command, purpose, "")
	}
	step = &entry.Steps[len(entry.Steps)-1]
	if a.Options.DryRun {
		step.Decision = "dry-run"
		return result(b, "Not run: dry-run mode; the user wants to see the plan only.", false), nil
	}
	if a.Checker != nil {
		kind := str(in["language"])
		if kind == "" {
			kind = "command"
		}
		v := a.Checker.Check(ctx, a.cleanAll(a.Requests), kind, a.clean(command), a.clean(purpose), str(in["effect"]))
		step.Check = v.Summary()
		a.UI.ShowCheck(a.Checker.Model, v)
		if !v.OK() {
			auto = false
			flagged := "the checker gave no verdict: " + v.Reason
			if v.Status == "flag" {
				flagged = "the checker flagged it: " + v.Reason
			}
			if reason != "" {
				reason += "; " + flagged
			} else {
				reason = flagged
			}
		}
	}
	edited := ""
	if !auto {
		if !a.UI.CanPrompt() {
			step.Decision = "declined"
			a.UI.Warn(fmt.Sprintf("not run (%s); no terminal to ask for approval", reason))
			return result(b, "Not run: it needs the user's approval and no terminal is available to ask.", false), nil
		}
		choice := a.UI.Confirm(reason, name == "run_command")
		switch choice.Action {
		case "abort":
			step.Decision = "skipped"
			return nil, errAbort
		case "no":
			step.Decision = "declined"
			why := ""
			if choice.Text != "" {
				why = " Reason: " + choice.Text
			}
			return result(b, "The user declined to run this."+why, false), nil
		case "edit":
			edited, command = choice.Text, choice.Text
			step.Command = command
		}
		step.Decision = "approved"
		if edited != "" {
			step.Decision = "edited"
		}
	} else {
		step.Decision = "auto"
	}
	o := execute.Options{Timeout: time.Duration(timeout) * time.Second, Limit: a.Settings.OutputLimit, Interactive: !auto}
	var r execute.Result
	var err error
	if name == "run_command" {
		r, err = execute.RunCommand(ctx, command, o)
	} else {
		r, err = execute.RunScript(ctx, str(in["language"]), command, o)
	}
	step.ExitCode, step.TimedOut, step.Duration = r.ExitCode, r.TimedOut, float64(int(r.Duration*100+0.5))/100
	if err != nil {
		return nil, err
	}
	lines := a.Settings.ShowLines
	if a.Options.Verbose {
		lines = 1_000_000_000
	}
	a.UI.ShowOutput(r, lines)
	return result(b, a.format(r, edited), false), nil
}

func (a *Agent) format(r execute.Result, edited string) string {
	var head []string
	if edited != "" {
		head = append(head, "The user edited the command before running it: "+edited)
	}
	if r.TimedOut {
		head = append(head, fmt.Sprintf("Timed out after %.0fs and was stopped.", r.Duration))
	} else {
		head = append(head, fmt.Sprintf("Exit status %s (%.1fs).", exitText(r.ExitCode), r.Duration))
	}
	if r.Truncated {
		head = append(head, "Output was shortened; the middle part is omitted.")
	}
	out := a.clean(r.Output)
	if out == "" {
		out = "(no output)"
	}
	return strings.Join(head, "\n") + "\n" + out
}

func result(b block, text string, isError bool) block {
	r := block{"type": "tool_result", "tool_use_id": b["id"], "content": text}
	if isError {
		r["is_error"] = true
	}
	return r
}

func invalid(args any, problem string) string {
	raw, _ := json.Marshal(args)
	out, _ := json.Marshal(block{"INVALID_JSON": string(raw), "error": problem})
	return string(out)
}

// validate checks tool input against the schema; eager streaming leaves this to the client.
func validate(name string, args any, tools []block) string {
	var schema block
	for _, t := range tools {
		if t["name"] == name {
			schema = t["input_schema"].(block)
		}
	}
	if schema == nil {
		return fmt.Sprintf("unknown tool %s", pyRepr(name))
	}
	in, ok := args.(block)
	if !ok {
		return "input is not an object"
	}
	for _, key := range schema["required"].([]any) {
		if _, ok := in[key.(string)]; !ok {
			return "missing " + key.(string)
		}
	}
	props := schema["properties"].(block)
	for _, key := range []string{"command", "language", "source", "purpose", "effect", "timeout_seconds", "interactive"} {
		raw, ok := props[key]
		if !ok {
			continue
		}
		spec, value := raw.(block), in[key]
		switch spec["type"] {
		case "string":
			if _, ok := value.(string); !ok {
				return key + " must be a string"
			}
		case "integer":
			f, ok := value.(float64)
			if !ok || f != float64(int64(f)) {
				return key + " must be an integer"
			}
		case "boolean":
			if _, ok := value.(bool); !ok {
				return key + " must be true or false"
			}
		}
		if enum, ok := spec["enum"].([]any); ok {
			found := false
			for _, e := range enum {
				if e == value {
					found = true
				}
			}
			if !found {
				return fmt.Sprintf("%s must be one of %s", key, pyList(enum))
			}
		}
	}
	text := str(in["command"])
	if text == "" {
		text = str(in["source"])
	}
	if strings.TrimSpace(text) == "" {
		return "empty command"
	}
	if len([]rune(text)) > 100_000 {
		return "command or script is too long"
	}
	if name == "propose_command" && Control.MatchString(text) {
		// It goes onto the user's command line: nothing there may hide or rewrite what it shows.
		return "the command contains control characters; propose it without them"
	}
	return ""
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func nonNil(b []block) []block {
	if b == nil {
		return []block{}
	}
	return b
}

func nonNilS(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func toBlocks(items []any) []block {
	var out []block
	for _, it := range items {
		if b, ok := it.(block); ok {
			out = append(out, b)
		}
	}
	return out
}

func pyRepr(s string) string {
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		return `"` + s + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `\'`) + "'"
}

func pyList(items []any) string {
	var parts []string
	for _, it := range items {
		parts = append(parts, pyRepr(fmt.Sprint(it)))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
