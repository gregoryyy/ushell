package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendFindAndPrivacy(t *testing.T) {
	h := New(filepath.Join(t.TempDir(), "state"))
	e := h.NewEntry("why is my disk full", "m", "u")
	code := 0
	e.Steps = append(e.Steps, Step{Kind: "command", Command: "df -h", Decision: "auto", ExitCode: &code})
	e.Answer = strings.Repeat("x", AnswerLimit+10)
	e.Outcome = "answered"
	if err := h.Append(e); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(h.Path); info.Mode().Perm() != 0o600 {
		t.Error(info.Mode())
	}
	got := h.Find("last")
	if got == nil || got.Steps[0].Command != "df -h" || *got.Steps[0].ExitCode != 0 || !strings.HasSuffix(got.Answer, " […]") {
		t.Fatal(got)
	}
	if h.Find(e.ID[:8]) == nil || h.Find("nope") != nil {
		t.Error("find by prefix")
	}
}

// ushpy writes the same format; a damaged line hides nothing else.
func TestReadsUshpysLinesAndSkipsDamage(t *testing.T) {
	dir := t.TempDir()
	h := New(dir)
	os.WriteFile(h.Path, []byte(`{"id": "20261028-101010-001", "time": 1.5, "user": "u", "cwd": "/", "request": "r", "model": "m", "steps": [{"kind": "proposal", "command": "df -h", "language": "", "purpose": "p", "decision": "proposed", "check": "", "rule": "", "exit_code": null, "timed_out": false, "duration": 0.0}], "answer": "a", "outcome": "proposed"}
{broken
`), 0o600)
	all := h.Entries()
	if len(all) != 1 || all[0].Steps[0].Decision != "proposed" || all[0].Steps[0].ExitCode != nil {
		t.Error(all)
	}
}

func TestASymlinkAtTheHistoryPathIsRefused(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(t.TempDir(), "victim")
	os.WriteFile(victim, []byte("x"), 0o644)
	os.Symlink(victim, filepath.Join(dir, "history.jsonl"))
	h := New(dir)
	if err := h.Append(h.NewEntry("r", "m", "u")); err == nil {
		t.Error("appended through a symlink")
	}
	if data, _ := os.ReadFile(victim); string(data) != "x" {
		t.Error("victim changed")
	}
}
