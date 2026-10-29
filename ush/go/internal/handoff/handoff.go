// Package handoff: propose mode between two `//` calls (ushpy handoff.py, same files).
//
// ush proposes one command; the zsh plugin puts it on the user's command line, and the user runs
// it in their own shell, edited or not, or doesn't. The plugin records what ran, and the next `//`
// continues the conversation with it.
//
// The handoff directory is the plugin's (USH_HANDOFF, mktemp -d): ush writes `proposal` (a mode
// line, capture or interactive, then the command); the plugin writes `ran` (exit status, then the
// command line) and `output`. ush uses the directory only if it is the user's own, closed to
// others, and never as root. When `//` takes `ran` and `output`, they stay as `last-ran` and
// `last-output` for notes (`ush notes --last`, Ctrl-X n, TODO 2.25) until the next run replaces
// them or the shell exits. Conversations are saved per shell in the state directory,
// sessions/KEY.json, mode 0600, and expire after an hour unused. Explanations are kept in
// explanations.json, mode 0600: the model's redacted answer under the SHA-256 of the command line,
// never the command line itself.
package handoff

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gregoryyy/ushell/ush/go/internal/execute"
)

// SessionTTL: a conversation unused for longer expires.
const SessionTTL = time.Hour

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// Ran is what the user ran after a proposal.
type Ran struct {
	Command   string
	ExitCode  *int
	Output    string
	Captured  bool // false: interactive, its output was not recorded
	Truncated bool
}

// Dir returns the plugin's handoff directory if usable: never as root, only the user's own, 0700.
func Dir(uid int, root bool, getenv func(string) string) string {
	path := getenv("USH_HANDOFF")
	if root || path == "" || !filepath.IsAbs(path) {
		return ""
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ""
	}
	st := info.Sys().(*syscall.Stat_t)
	if int(st.Uid) != uid || info.Mode().Perm()&0o077 != 0 {
		return ""
	}
	return path
}

// WriteProposal leaves the proposal for the plugin's next prompt, replacing an earlier one.
func WriteProposal(dir, command string, interactive bool) error {
	for _, name := range []string{"ran", "output"} {
		removeIfExists(filepath.Join(dir, name))
	}
	tmp := filepath.Join(dir, fmt.Sprintf(".proposal.%d", os.Getpid()))
	removeIfExists(tmp)
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return err
	}
	mode := "capture"
	if interactive {
		mode = "interactive"
	}
	_, err = io.WriteString(f, mode+"\n"+command)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, "proposal"))
}

// TakeRan returns what the plugin recorded since the proposal, then keeps it only for notes; nil
// if nothing ran.
func TakeRan(dir string, limit int) *Ran {
	ran := loadRan(dir, "", limit)
	if ran == nil {
		return nil
	}
	for _, name := range []string{"ran", "output"} {
		if _, err := os.Lstat(filepath.Join(dir, name)); err == nil {
			_ = os.Rename(filepath.Join(dir, name), filepath.Join(dir, "last-"+name))
		} else {
			removeIfExists(filepath.Join(dir, "last-"+name))
		}
	}
	removeIfExists(filepath.Join(dir, "proposal"))
	return ran
}

// LastRan returns the last run the plugin recorded, whether or not `//` has taken it; it removes
// nothing.
func LastRan(dir string, limit int) *Ran {
	if ran := loadRan(dir, "", limit); ran != nil {
		return ran
	}
	return loadRan(dir, "last-", limit)
}

func loadRan(dir, prefix string, limit int) *Ran {
	text, ok := readRegular(filepath.Join(dir, prefix+"ran"))
	if !ok {
		return nil
	}
	status, command, _ := strings.Cut(text, "\n")
	ran := &Ran{Command: strings.TrimRight(command, "\n")}
	if code, err := strconv.Atoi(strings.TrimSpace(status)); err == nil {
		ran.ExitCode = &code
	}
	collector := execute.NewCollector(limit, nil)
	if f, err := os.OpenFile(filepath.Join(dir, prefix+"output"), os.O_RDONLY|syscall.O_NOFOLLOW, 0); err == nil {
		if info, err := f.Stat(); err == nil && info.Mode().IsRegular() {
			ran.Captured = true
			buf := make([]byte, 65536)
			for {
				n, err := f.Read(buf)
				if n > 0 {
					collector.Feed(append([]byte(nil), buf[:n]...))
				}
				if err != nil {
					break
				}
			}
		}
		f.Close()
	}
	ran.Output, ran.Truncated = collector.Text()
	return ran
}

// SessionKey: one conversation per shell, the plugin's directory or the parent process without it.
func SessionKey(dir string) string {
	if dir != "" && keyPattern.MatchString(filepath.Base(dir)) {
		return filepath.Base(dir)
	}
	return fmt.Sprintf("ppid-%d", os.Getppid())
}

// Sessions are the saved conversations of propose mode, one per shell.
type Sessions struct{ Dir, Path string }

// NewSessions: stateDir/sessions/key.json.
func NewSessions(stateDir, key string) (*Sessions, error) {
	if !keyPattern.MatchString(key) {
		return nil, fmt.Errorf("bad session key %q", key)
	}
	dir := filepath.Join(stateDir, "sessions")
	return &Sessions{Dir: dir, Path: filepath.Join(dir, key+".json")}, nil
}

// Load returns the saved conversation, or nil if there is none or it expired.
func (s *Sessions) Load() map[string]any {
	info, err := os.Lstat(s.Path)
	if err != nil {
		return nil
	}
	if time.Since(info.ModTime()) > SessionTTL {
		s.Clear()
		return nil
	}
	text, ok := readRegular(s.Path)
	if !ok {
		return nil
	}
	var data map[string]any
	if json.Unmarshal([]byte(text), &data) != nil {
		return nil
	}
	return data
}

// Save writes the conversation (0600), pruning other shells' expired ones.
func (s *Sessions) Save(data any) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	s.prune()
	tmp := fmt.Sprintf("%s.%d", s.Path, os.Getpid())
	removeIfExists(tmp)
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	err = enc.Encode(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, s.Path)
}

// Clear removes the conversation.
func (s *Sessions) Clear() { removeIfExists(s.Path) }

func (s *Sessions) prune() {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		path := filepath.Join(s.Dir, e.Name())
		if info, err := os.Lstat(path); err == nil && strings.HasSuffix(e.Name(), ".json") &&
			time.Since(info.ModTime()) > SessionTTL {
			os.Remove(path)
		}
	}
}

// ExplanationsKept: how many explanations are kept, the newest.
const ExplanationsKept = 500

// Explanations are the explanations given before, by command line (ushpy handoff.Explanations,
// the same file).
type Explanations struct{ Dir, Path string }

// NewExplanations: stateDir/explanations.json.
func NewExplanations(stateDir string) *Explanations {
	return &Explanations{Dir: stateDir, Path: filepath.Join(stateDir, "explanations.json")}
}

type explanation struct {
	Answer string `json:"answer"`
	Time   int64  `json:"time"`
}

// explanationKey is the SHA-256 of the trimmed command line, in hex.
func explanationKey(command string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(command)))
	return hex.EncodeToString(sum[:])
}

// load returns the kept explanations in file order, oldest first.
func (e *Explanations) load() ([]string, map[string]explanation) {
	text, ok := readRegular(e.Path)
	if !ok {
		return nil, map[string]explanation{}
	}
	dec := json.NewDecoder(strings.NewReader(text))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, map[string]explanation{}
	}
	var order []string
	data := map[string]explanation{}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, map[string]explanation{}
		}
		key, _ := t.(string)
		var raw json.RawMessage
		if dec.Decode(&raw) != nil {
			return nil, map[string]explanation{}
		}
		var entry explanation
		if json.Unmarshal(raw, &entry) != nil {
			continue
		}
		if _, seen := data[key]; !seen {
			order = append(order, key)
		}
		data[key] = entry
	}
	return order, data
}

// Get returns the kept answer for the command line, or "".
func (e *Explanations) Get(command string) string {
	_, data := e.load()
	return data[explanationKey(command)].Answer
}

// Put keeps an answer; a failure to write only means it is asked for again.
func (e *Explanations) Put(command, answer string) {
	order, data := e.load()
	key := explanationKey(command)
	kept := order[:0]
	for _, k := range order {
		if k != key {
			kept = append(kept, k)
		}
	}
	order = append(kept, key)
	data[key] = explanation{Answer: answer, Time: time.Now().Unix()}
	if len(order) > ExplanationsKept {
		order = order[len(order)-ExplanationsKept:]
	}
	var b strings.Builder
	b.WriteString("{")
	for i, k := range order {
		if i > 0 {
			b.WriteString(", ")
		}
		name, _ := json.Marshal(k)
		value, err := marshalNoEscape(data[k])
		if err != nil {
			return
		}
		b.Write(name)
		b.WriteString(": ")
		b.Write(value)
	}
	b.WriteString("}")
	if os.MkdirAll(e.Dir, 0o700) != nil {
		return
	}
	var f *os.File
	var tmp string
	for i := 0; i < 100; i++ {
		r := make([]byte, 6)
		_, _ = rand.Read(r)
		tmp = e.Path + "." + hex.EncodeToString(r) + ".tmp"
		var err error
		f, err = os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return
		}
		f = nil
	}
	if f == nil {
		return
	}
	_, err := f.WriteString(b.String())
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil || os.Rename(tmp, e.Path) != nil {
		os.Remove(tmp)
	}
}

func marshalNoEscape(v any) ([]byte, error) {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return []byte(strings.TrimSuffix(b.String(), "\n")), nil
}

func readRegular(path string) (string, bool) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return "", false
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return "", false
	}
	return execute.Decode(data), true
}

func removeIfExists(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = err
	}
}
