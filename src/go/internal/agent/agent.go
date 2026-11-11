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

	"github.com/gregoryyy/ushell/src/go/internal/check"
	"github.com/gregoryyy/ushell/src/go/internal/config"
	"github.com/gregoryyy/ushell/src/go/internal/execute"
	"github.com/gregoryyy/ushell/src/go/internal/handoff"
	"github.com/gregoryyy/ushell/src/go/internal/history"
	"github.com/gregoryyy/ushell/src/go/internal/policy"
	"github.com/gregoryyy/ushell/src/go/internal/provider"
	"github.com/gregoryyy/ushell/src/go/internal/redact"
	"github.com/gregoryyy/ushell/src/go/internal/ui"
)

// Control: C0 controls but tab and newline, DEL, C1, bidi overrides and isolates, zero-width.
var Control = regexp.MustCompile(`[\x00-\x08\x0b-\x1f\x7f-\x{9f}\x{200b}-\x{200f}\x{202a}-\x{202e}\x{2066}-\x{2069}\x{feff}]`)

// Options of a request.
type Options struct {
	DryRun, ConfirmAll, Verbose, Once, Propose bool
	Research                                   bool // `//? TEXT`, `ush research`: the model only answers (TODO 2.38)
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
	Question     block   // the model asked the user (TODO 2.35): id, question
	Next         block   // next steps the model offered (TODO 2.41): id, steps
	Ran          *handoff.Ran
	system       string
	research     string // the system prompt of a research request
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
	a.tools = append(append([]block{}, Tools...), AskTool, NextTool)
	if o.Propose {
		prompt, a.tools = systemPropose, []block{ProposeTool, AskTool, NextTool}
	}
	replacer := strings.NewReplacer("{os}", OSName(), "{user}", id.Name, "{root}", root, "{cwd}", cwd,
		"{shell}", shell)
	a.system, a.research = replacer.Replace(prompt), replacer.Replace(systemResearch)
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

// RunExplicit runs a command the user typed in a session; its output becomes context. A `cd` in
// it moves the session, as in the user's shell.
func (a *Agent) RunExplicit(ctx context.Context, command string) {
	r, err := execute.RunSessionCommand(ctx, command, execute.Options{Timeout: 24 * time.Hour,
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
	return a.ask(ctx, request, ran, tracked, false)
}

// Research sends a research request (TODO 2.38): the model only answers; ush runs and proposes
// nothing for it. A pending proposal is answered as for any request.
func (a *Agent) Research(ctx context.Context, request string, ran *handoff.Ran, tracked bool) *history.Entry {
	return a.ask(ctx, request, ran, tracked, true)
}

func (a *Agent) ask(ctx context.Context, request string, ran *handoff.Ran, tracked, research bool) *history.Entry {
	label := request
	if research {
		label = "? " + request
	} else if label == "" {
		label = "(continue)"
	}
	entry := a.History.NewEntry(label, a.Provider.Model(), a.ID.Name)
	entry.Steps = append(entry.Steps, a.pendingSteps...)
	a.pendingSteps = nil
	var results []block
	answering := a.Question != nil // the request answers the model's question
	if answering {
		results = a.questionResults(request)
	} else if a.Next != nil {
		// `//N` picks an offered step and `//` goes on: both answer the offer, without text.
		answering = request == "" || a.HasStep(request)
		results = a.nextResults(request)
	} else if a.Proposal != nil {
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
		if !answering {
			content = append(content, block{"type": "text", "text": request})
		}
		a.Requests = append(a.Requests, request)
	}
	if len(content) == 0 {
		content = append(content, block{"type": "text", "text": "Go on."}) // `//` after an answer (2.41)
	}
	a.Messages = append(a.Messages, block{"role": "user", "content": content})
	a.pending = nil
	a.steps = 0
	outcome, answer, err := a.loop(ctx, entry, research)
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

// maxRefusedTurns is how many turns in a row may have all their tool calls refused (invalid, or
// past the step limit) before the request stops: a small model that keeps writing a wrong call
// would otherwise loop.
const maxRefusedTurns = 3

func (a *Agent) loop(ctx context.Context, entry *history.Entry, research bool) (string, string, error) {
	refused := 0
	system := a.system
	if research {
		system = a.research
	}
	for {
		tools := a.tools
		if research && !hasToolUse(a.Messages) {
			// No tools, unless the conversation has tool calls: a request with tool_use blocks
			// must define them. Any call the model makes is refused below.
			tools = []block{}
		}
		turn, err := a.Provider.Turn(ctx, system, tools, a.Messages, a.UI.Answer, a.UI.Progress)
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
		if research {
			var results []block
			for _, b := range uses {
				results = append(results, result(b, notResearch, true))
			}
			a.Messages = append(a.Messages, block{"role": "user", "content": results})
			if refused++; refused >= maxRefusedTurns {
				return a.stopRefused(), text.String(), nil
			}
			continue
		}
		if due, asked := a.askBack(uses); asked {
			a.Unanswered = due
			return "asked", text.String(), nil
		}
		if a.offerNext(uses) {
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
			if refused = countRefused(refused, results); refused >= maxRefusedTurns {
				return a.stopRefused(), text.String(), nil
			}
			continue
		}
		var results []block
		aborted := false
		for _, b := range uses {
			if aborted {
				results = append(results, result(b, "Not run: the user stopped the request.", false))
				continue
			}
			if str(b["name"]) == str(NextTool["name"]) {
				results = append(results, a.notShown(b))
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
		if refused = countRefused(refused, results); refused >= maxRefusedTurns {
			return a.stopRefused(), text.String(), nil
		}
	}
}

// hasToolUse: whether a conversation has a tool call.
func hasToolUse(messages []block) bool {
	for _, m := range messages {
		content, _ := m["content"].([]any)
		blocks, _ := m["content"].([]block)
		for _, c := range content {
			if b, ok := c.(block); ok {
				blocks = append(blocks, b)
			}
		}
		for _, b := range blocks {
			if b["type"] == "tool_use" {
				return true
			}
		}
	}
	return false
}

// countRefused continues the count of turns whose tool results were all errors, or restarts it.
func countRefused(count int, results []block) int {
	for _, r := range results {
		if r["is_error"] != true {
			return 0
		}
	}
	return count + 1
}

func (a *Agent) stopRefused() string {
	a.UI.Error(fmt.Sprintf("the model's tool calls were refused %d times in a row; nothing more was run (a larger model may do better)", maxRefusedTurns))
	return "error"
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

// askBack: if the model asks the user a valid question, show it as `// question` and return the
// results of its other calls, due with the answer (TODO 2.35).
func (a *Agent) askBack(uses []block) ([]block, bool) {
	var ask block
	for _, b := range uses {
		if str(b["name"]) == str(AskTool["name"]) && validate(str(b["name"]), b["input"], a.tools) == "" {
			ask = b
			break
		}
	}
	if ask == nil {
		return nil, false
	}
	question := truncate(strings.Join(strings.Fields(str(ask["input"].(block)["question"])), " "), 1000)
	a.UI.ShowQuestion(question)
	a.Question = block{"id": ask["id"], "question": question}
	due := []block{}
	for _, b := range uses {
		if b["id"] != ask["id"] {
			due = append(due, result(b, "Not done: you asked the user a question first; wait for the answer.", false))
		}
	}
	return due, true
}

// questionResults: the user's answer to the model's question, and the results due with it.
func (a *Agent) questionResults(request string) []block {
	q, due := a.Question, a.Unanswered
	a.Question, a.Unanswered = nil, nil
	text := "The user did not answer; continue with what you have, or ask again."
	if request != "" {
		text = "The user answers: " + a.clean(request)
	}
	return append([]block{result(block{"id": q["id"]}, text, false)}, due...)
}

// HasStep: whether request (`//2`) picks a step the model offered.
func (a *Agent) HasStep(request string) bool {
	return Step.MatchString(request) && a.Next != nil && int(request[0]-'0') <= len(steps(a.Next))
}

// Offered is how many next steps are on offer.
func (a *Agent) Offered() int { return len(steps(a.Next)) }

func steps(offer block) []string {
	var out []string
	raw, _ := offer["steps"].([]any)
	for _, s := range raw {
		out = append(out, str(s))
	}
	if typed, ok := offer["steps"].([]string); ok {
		out = typed
	}
	return out
}

// offerNext shows the next steps the model offers with its answer, if valid and alone with it;
// their result is due with the next request.
func (a *Agent) offerNext(uses []block) bool {
	if len(uses) != 1 || str(uses[0]["name"]) != str(NextTool["name"]) ||
		validate(str(uses[0]["name"]), uses[0]["input"], a.tools) != "" {
		return false
	}
	var shown []string
	for _, s := range uses[0]["input"].(block)["steps"].([]any) {
		shown = append(shown, truncate(strings.Join(strings.Fields(str(s)), " "), 200))
	}
	a.UI.ShowNext(shown)
	a.Next = block{"id": uses[0]["id"], "steps": shown}
	return true
}

// notShown answers an offer of next steps that ush didn't show: invalid, or not alone.
func (a *Agent) notShown(b block) block {
	if problem := validate(str(b["name"]), b["input"], a.tools); problem != "" {
		return result(b, invalid(b["input"], problem), true)
	}
	return result(b, notShown, false)
}

// nextResults: what the user did with the offered steps: picked one, went on, or asked otherwise.
func (a *Agent) nextResults(request string) []block {
	offer, offered := a.Next, steps(a.Next)
	a.Next = nil
	text := "The user asked something else instead."
	switch {
	case Step.MatchString(request) && int(request[0]-'0') <= len(offered):
		n := int(request[0] - '0')
		text = fmt.Sprintf("The user chose step %d: %s", n, offered[n-1])
	case request == "":
		text = "The user says: go on with the next step."
	}
	return []block{result(block{"id": offer["id"]}, text, false)}
}

// propose presents the first valid proposal; the others get results due with the next request.
func (a *Agent) propose(ctx context.Context, uses []block, entry *history.Entry) []block {
	var results []block
	for _, b := range uses {
		name, args := str(b["name"]), b["input"]
		if name == str(NextTool["name"]) {
			results = append(results, a.notShown(b))
			continue
		}
		if a.Proposal != nil {
			results = append(results, result(b, "Not proposed: propose one command at a time.", false))
			continue
		}
		// An invalid question gets its own error; any other tool is not a proposal.
		tools := []block{ProposeTool}
		if name == str(AskTool["name"]) {
			tools = a.tools
		}
		problem := validate(name, args, tools)
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
	var question, next any
	if a.Question != nil {
		question = a.Question
	}
	if a.Next != nil {
		next = a.Next
	}
	return block{"version": 1, "messages": nonNil(a.Messages), "requests": nonNilS(a.Requests),
		"proposal": proposal, "unanswered": nonNil(a.Unanswered), "question": question, "next": next}
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
	} else if q, ok := data["question"].(block); ok {
		if _, ok := q["id"].(string); ok {
			a.Question = q
			if due, ok := data["unanswered"].([]any); ok {
				a.Unanswered = toBlocks(due)
			}
		}
	}
	if offer, ok := data["next"].(block); ok && a.Proposal == nil && a.Question == nil {
		_, hasID := offer["id"].(string)
		list, isList := offer["steps"].([]any)
		allText := isList
		for _, s := range list {
			if _, ok := s.(string); !ok {
				allText = false
			}
		}
		if hasID && allText {
			a.Next = offer
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
	tools := Tools
	if name == str(AskTool["name"]) {
		tools = a.tools // an invalid question gets its own error
	}
	if problem := validate(name, args, tools); problem != "" {
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
		names := make([]string, 0, len(tools))
		for _, t := range tools {
			names = append(names, str(t["name"]))
		}
		return fmt.Sprintf("unknown tool %s; the tools are %s", pyRepr(name), strings.Join(names, ", "))
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
	for _, key := range []string{"command", "language", "source", "purpose", "effect", "timeout_seconds", "interactive", "question"} {
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
	if name == str(NextTool["name"]) {
		list, ok := in["steps"].([]any)
		if !ok {
			return "steps must be a list"
		}
		for _, s := range list {
			if text, ok := s.(string); !ok || strings.TrimSpace(text) == "" {
				return "steps must be a list of short phrases"
			}
		}
		if len(list) < 1 || len(list) > MaxNext {
			return fmt.Sprintf("offer one to %d steps", MaxNext)
		}
		for _, s := range list {
			if Control.MatchString(s.(string)) {
				return "a step contains control characters; offer it without them"
			}
		}
		return ""
	}
	if name == str(AskTool["name"]) {
		if strings.TrimSpace(str(in["question"])) == "" {
			return "empty question"
		}
		if Control.MatchString(str(in["question"])) {
			return "the question contains control characters; ask it without them"
		}
		return ""
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
