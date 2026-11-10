// Package history: the append-only request history (ushpy history.py), in the same JSONL
// format. It stores commands, scripts, decisions and exit codes, plus a redacted, shortened
// answer; command output is not stored. The file is private (0600).
package history

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// AnswerLimit: longer answers are shortened before they are stored.
const AnswerLimit = 4000

// Step is one command or script of a request.
type Step struct {
	Kind     string  `json:"kind"` // command | script | explicit | proposal
	Command  string  `json:"command"`
	Language string  `json:"language"`
	Purpose  string  `json:"purpose"`
	Decision string  `json:"decision"` // auto | approved | edited | declined | dry-run | explicit | skipped | proposed | user-ran
	Check    string  `json:"check"`    // the checker's verdict
	Rule     string  `json:"rule"`     // the allow rule that let it run without approval
	ExitCode *int    `json:"exit_code"`
	TimedOut bool    `json:"timed_out"`
	Duration float64 `json:"duration"`
}

// Entry is one request.
type Entry struct {
	ID      string  `json:"id"`
	Time    float64 `json:"time"`
	User    string  `json:"user"`
	Cwd     string  `json:"cwd"`
	Request string  `json:"request"`
	Model   string  `json:"model"`
	Steps   []Step  `json:"steps"`
	Answer  string  `json:"answer"`
	Outcome string  `json:"outcome"` // answered | proposed | refused | aborted | error | dry-run
}

// History is history.jsonl in a directory.
type History struct{ Dir, Path string }

// New uses dir/history.jsonl.
func New(dir string) *History { return &History{Dir: dir, Path: filepath.Join(dir, "history.jsonl")} }

// NewEntry starts an entry, with ushpy's id format: 20261028-221500-123.
func (h *History) NewEntry(request, model, user string) *Entry {
	now := time.Now()
	cwd, _ := os.Getwd()
	return &Entry{ID: now.Format("20060102-150405") + fmt.Sprintf("-%03d", now.Nanosecond()/1_000_000),
		Time: float64(now.UnixNano()) / 1e9, User: user, Cwd: cwd, Request: request, Model: model,
		Steps: []Step{}}
}

// Append adds an entry; a symlink at the history path is refused (O_NOFOLLOW).
func (h *History) Append(e *Entry) error {
	if r := []rune(e.Answer); len(r) > AnswerLimit {
		e.Answer = string(r[:AnswerLimit]) + " […]"
	}
	if e.Steps == nil {
		e.Steps = []Step{}
	}
	if err := os.MkdirAll(h.Dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(h.Path, os.O_WRONLY|os.O_APPEND|os.O_CREATE|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	line, err := marshal(e)
	if err != nil {
		return err
	}
	_, err = f.Write(append(line, '\n'))
	return err
}

func marshal(e *Entry) ([]byte, error) {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(e); err != nil {
		return nil, err
	}
	return []byte(strings.TrimRight(b.String(), "\n")), nil
}

// Entries reads all entries; a damaged line does not hide the others.
func (h *History) Entries() []Entry {
	f, err := os.Open(h.Path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []Entry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 64<<20)
	for scanner.Scan() {
		var e Entry
		if json.Unmarshal(scanner.Bytes(), &e) == nil && e.ID != "" {
			out = append(out, e)
		}
	}
	return out
}

// Recent is the last n entries.
func (h *History) Recent(n int) []Entry {
	all := h.Entries()
	if n < len(all) {
		return all[len(all)-n:]
	}
	return all
}

// Find returns the entry with this id or id prefix (the last match), or the last for "last".
func (h *History) Find(id string) *Entry {
	all := h.Entries()
	if (id == "last" || id == "-1") && len(all) > 0 {
		return &all[len(all)-1]
	}
	for i := len(all) - 1; i >= 0; i-- {
		if all[i].ID == id || strings.HasPrefix(all[i].ID, id) {
			return &all[i]
		}
	}
	return nil
}
