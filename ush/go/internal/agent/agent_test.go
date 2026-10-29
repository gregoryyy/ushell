package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregoryyy/ushell/ush/go/internal/config"
	"github.com/gregoryyy/ushell/ush/go/internal/handoff"
	"github.com/gregoryyy/ushell/ush/go/internal/history"
	"github.com/gregoryyy/ushell/ush/go/internal/provider"
	"github.com/gregoryyy/ushell/ush/go/internal/ui"
)

var syntheticKey = "sk-ant-test-" + strings.Repeat("0", 28) + "SYNTHETIC"

// fake returns scripted turns and records the messages of each request.
type fake struct {
	turns    [][]block
	requests [][]block
}

func (f *fake) Name() string                                             { return "Fake" }
func (f *fake) Model() string                                            { return "fake-model" }
func (f *fake) Complete(context.Context, string, string) (string, error) { return "OK", nil }
func (f *fake) Turn(_ context.Context, _ string, _ []block, messages []block, onText, _ func(string)) (provider.Turn, error) {
	raw, _ := json.Marshal(messages)
	var copied []block
	json.Unmarshal(raw, &copied)
	f.requests = append(f.requests, copied)
	if len(f.turns) == 0 {
		panic("provider called more often than scripted")
	}
	content := f.turns[0]
	f.turns = f.turns[1:]
	stop := "end_turn"
	for _, b := range content {
		if b["type"] == "tool_use" {
			stop = "tool_use"
		}
		if b["type"] == "text" {
			onText(b["text"].(string))
		}
	}
	return provider.Turn{Content: content, StopReason: stop}, nil
}

func text(s string) block { return block{"type": "text", "text": s} }

func use(id, command, effect string) block {
	return block{"type": "tool_use", "id": id, "name": "run_command",
		"input": block{"command": command, "purpose": "test", "effect": effect, "timeout_seconds": 30.0}}
}

func propose(id, command, effect string) block {
	return block{"type": "tool_use", "id": id, "name": "propose_command",
		"input": block{"command": command, "purpose": "test", "effect": effect, "interactive": false}}
}

func newAgent(t *testing.T, turns [][]block, o Options, answers ...ui.Choice) (*Agent, *fake, *bytes.Buffer, *bytes.Buffer, *history.History) {
	f := &fake{turns: turns}
	var out, errOut bytes.Buffer
	term := &ui.Terminal{Out: &out, Err: &errOut}
	if answers != nil {
		term.Ask = func(string, bool) ui.Choice { c := answers[0]; answers = answers[1:]; return c }
	}
	s := config.Defaults()
	s.ShowLines = 0
	dir := t.TempDir()
	h := history.New(filepath.Join(dir, "state"))
	uid := os.Getuid()
	id := config.Identity{UID: uid, Name: "tester", Home: dir, OwnerUID: uid, OwnerName: "tester", OwnerHome: dir}
	return New(f, term, s, id, h, o, []string{syntheticKey}, nil), f, &out, &errOut, h
}

func lastResults(f *fake) []any {
	last := f.requests[len(f.requests)-1]
	return last[len(last)-1]["content"].([]any)
}

func TestReadOnlyCommandRunsAndIsRecorded(t *testing.T) {
	a, f, out, _, h := newAgent(t, [][]block{{use("t1", "echo disk-report", "read")}, {text("Your disk is fine.")}}, Options{})
	entry := a.Ask(context.Background(), "check my disk", nil, true)
	if entry.Outcome != "answered" || !strings.Contains(out.String(), "Your disk is fine.") {
		t.Fatal(entry.Outcome, out.String())
	}
	r := lastResults(f)[0].(block)
	if !strings.Contains(r["content"].(string), "disk-report") {
		t.Error(r)
	}
	saved := h.Find("last")
	if saved.Steps[0].Decision != "auto" || *saved.Steps[0].ExitCode != 0 {
		t.Error(saved.Steps)
	}
}

func TestChangesNeedApprovalAndDeclinedOnesDontRun(t *testing.T) {
	victim := filepath.Join(t.TempDir(), "victim")
	os.WriteFile(victim, nil, 0o644)
	a, f, _, _, _ := newAgent(t, [][]block{{use("t1", "rm "+victim, "change")}, {text("ok, not removed")}}, Options{},
		ui.Choice{Action: "no", Text: "keep it"})
	entry := a.Ask(context.Background(), "remove the file", nil, true)
	if _, err := os.Stat(victim); err != nil || entry.Steps[0].Decision != "declined" {
		t.Error(err, entry.Steps)
	}
	if r := lastResults(f)[0].(block)["content"].(string); !strings.Contains(r, "declined") || !strings.Contains(r, "keep it") {
		t.Error(r)
	}
}

func TestModelClaimOfReadDoesNotBypassThePolicy(t *testing.T) {
	victim := filepath.Join(t.TempDir(), "victim")
	os.WriteFile(victim, nil, 0o644)
	a, _, _, errOut, _ := newAgent(t, [][]block{{use("t1", "rm "+victim, "read")}, {text("done")}}, Options{})
	a.Ask(context.Background(), "x", nil, true) // no terminal: declined
	if _, err := os.Stat(victim); err != nil || !strings.Contains(errOut.String(), "no terminal to ask") {
		t.Error(err, errOut.String())
	}
}

func TestProposeModeRunsNothingAndContinuesWithWhatRan(t *testing.T) {
	made := filepath.Join(t.TempDir(), "made")
	a, f, _, errOut, _ := newAgent(t, [][]block{{text("This creates it."), propose("p1", "touch "+made, "change"),
		propose("p2", "ls", "read")}, {text("Done.")}}, Options{Propose: true})
	entry := a.Ask(context.Background(), "make a file", nil, true)
	if _, err := os.Stat(made); err == nil || entry.Outcome != "proposed" || a.Proposal["command"] != "touch "+made {
		t.Fatal(entry.Outcome, a.Proposal)
	}
	if !strings.Contains(errOut.String(), "→ touch") || !strings.Contains(errOut.String(), "changes something") {
		t.Error(errOut.String())
	}
	if !strings.Contains(a.System(), "You do not run commands") {
		t.Error("propose prompt")
	}
	// Saved and restored, as between two `//` calls.
	raw, _ := json.Marshal(a.State())
	var state block
	json.Unmarshal(raw, &state)
	again, f2, _, _, _ := newAgent(t, [][]block{{text("Done.")}}, Options{Propose: true})
	again.Restore(state)
	code := 0
	again.Ask(context.Background(), "", &handoff.Ran{Command: "touch " + made, ExitCode: &code,
		Output: "key=" + syntheticKey, Captured: true}, true)
	results := lastResults(f2)
	first := results[0].(block)
	if first["tool_use_id"] != "p1" || !strings.Contains(first["content"].(string), "The user ran the proposed command.\nExit status 0.") ||
		strings.Contains(first["content"].(string), syntheticKey) {
		t.Error(first)
	}
	if second := results[1].(block); second["tool_use_id"] != "p2" || !strings.Contains(second["content"].(string), "one command at a time") {
		t.Error(second)
	}
	_ = f
}

func TestControlCharactersAreNeverProposed(t *testing.T) {
	a, f, _, errOut, _ := newAgent(t, [][]block{{propose("bad", "echo safe\x1b[2K\rrm -rf ~", "read")},
		{propose("good", "echo safe", "read")}}, Options{Propose: true})
	if a.Ask(context.Background(), "x", nil, true).Outcome != "proposed" || a.Proposal["command"] != "echo safe" {
		t.Error(a.Proposal)
	}
	raw, _ := json.Marshal(f.requests[1])
	if !strings.Contains(string(raw), "control characters") || strings.Contains(errOut.String(), "rm -rf") {
		t.Error("control characters")
	}
}

func TestUntrackedAndNotRun(t *testing.T) {
	for _, c := range []struct {
		tracked bool
		want    string
	}{{true, "did not run the proposed command"}, {false, "cannot tell whether the user ran"}} {
		a, f, _, _, _ := newAgent(t, [][]block{{propose("p1", "df -h /", "read")}, {text("ok")}}, Options{Propose: true})
		a.Ask(context.Background(), "space?", nil, true)
		a.Ask(context.Background(), "what now?", nil, c.tracked)
		content := f.requests[1][len(f.requests[1])-1]["content"].([]any)
		if !strings.Contains(content[0].(block)["content"].(string), c.want) || content[len(content)-1].(block)["text"] != "what now?" {
			t.Error(content)
		}
	}
}
