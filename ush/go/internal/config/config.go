// Package config: identity, settings, and API key storage (ushpy config.py). The files are
// ushpy's, in the same formats, so either implementation reads what the other wrote.
//
// Under sudo, ush runs as root but uses the invoking user's key and settings (found through
// SUDO_USER), while history is written to root's own state directory. Environment overrides of
// the provider, model, checker, and endpoints are ignored as root. An Ollama endpoint must be on
// this machine.
package config

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/gregoryyy/ushell/ush/go/internal/providers"
)

const (
	DefaultModel = "claude-opus-5-5"
	OllamaURL    = providers.OllamaURL
)

var (
	Efforts        = []string{"low", "medium", "high", "xhigh", "max"}
	Providers      = providers.Names()          // the provider table
	Modes          = []string{"propose", "run"} // ARCH: propose mode (default) and agent mode
	anthropicModel = providers.AnthropicModel
	ollamaModel    = providers.OllamaModel
)

// Error is a configuration problem: ush reports it and exits with status 2.
type Error struct{ Msg string }

func (e *Error) Error() string { return e.Msg }

// Errorf makes a configuration Error.
func Errorf(format string, a ...any) error { return &Error{fmt.Sprintf(format, a...)} }

// Identity: who runs ush, and whose key and settings it uses.
type Identity struct {
	UID       int
	Name      string
	Home      string
	OwnerUID  int // user whose key and settings are used
	OwnerName string
	OwnerHome string
}

func (i Identity) IsRoot() bool  { return i.UID == 0 }
func (i Identity) ViaSudo() bool { return i.UID != i.OwnerUID }

// ConfigDir is the owner's ~/.config/ush, or $XDG_CONFIG_HOME/ush (not as root).
func (i Identity) ConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" && !i.IsRoot() {
		return filepath.Join(xdg, "ush")
	}
	return filepath.Join(i.OwnerHome, ".config", "ush")
}

// StateDir is ~/.local/state/ush of the user running ush, or $XDG_STATE_HOME/ush (not as root).
func (i Identity) StateDir() string {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" && !i.IsRoot() {
		return filepath.Join(xdg, "ush")
	}
	return filepath.Join(i.Home, ".local", "state", "ush")
}

// Current returns the identity of this process.
func Current() Identity {
	uid := os.Geteuid()
	me, err := user.LookupId(strconv.Itoa(uid))
	name, home := "", os.Getenv("HOME")
	if err == nil {
		name, home = me.Username, me.HomeDir
	}
	id := Identity{UID: uid, Name: name, Home: home, OwnerUID: uid, OwnerName: name, OwnerHome: home}
	if sudoUser := os.Getenv("SUDO_USER"); uid == 0 && sudoUser != "" && sudoUser != "root" {
		if owner, err := user.Lookup(sudoUser); err == nil {
			if ouid, err := strconv.Atoi(owner.Uid); err == nil {
				id.OwnerUID, id.OwnerName, id.OwnerHome = ouid, owner.Username, owner.HomeDir
			}
		}
	}
	return id
}

// ---------------------------------------------------------------------------
// Settings

// Settings, stored as config.json with ushpy's keys.
type Settings struct {
	Mode           string   `json:"mode"` // propose: ush proposes, the user runs; run: agent mode
	Provider       string   `json:"provider"`
	Model          string   `json:"model"` // the Anthropic model
	Effort         string   `json:"effort"`
	OllamaURL      string   `json:"ollama_url"`
	OllamaModel    string   `json:"ollama_model"` // no default: a model the user has pulled
	Checker        string   `json:"checker"`      // "provider:model" of the checker; "" is off
	Allow          []string `json:"allow"`        // the user's allow rules
	MaxSteps       int      `json:"max_steps"`
	CommandTimeout int      `json:"command_timeout"` // default seconds per command
	MaxTimeout     int      `json:"max_timeout"`     // upper bound a request may ask for
	OutputLimit    int      `json:"output_limit"`    // bytes of each output sent to the model
	ShowLines      int      `json:"show_lines"`      // output lines shown per command
}

// Defaults are ushpy's.
func Defaults() Settings {
	return Settings{Mode: "propose", Provider: "anthropic", Model: DefaultModel, Effort: "medium",
		OllamaURL: OllamaURL, Allow: []string{}, MaxSteps: 30, CommandTimeout: 120,
		MaxTimeout: 900, OutputLimit: 32_000, ShowLines: 12}
}

// FromMap reads settings leniently, as ushpy's from_dict: unknown or invalid values keep the
// default, except an Ollama URL off this machine, a bad checker, or bad allow rules.
func FromMap(data map[string]any, parseRule func(string) ([]string, error)) (Settings, error) {
	s := Defaults()
	str := func(k string) (string, bool) { v, ok := data[k].(string); return v, ok }
	if v, ok := str("mode"); ok && contains(Modes, v) {
		s.Mode = v
	}
	if v, ok := str("provider"); ok && contains(Providers, v) {
		s.Provider = v
	}
	if v, ok := str("model"); ok && anthropicModel.MatchString(v) {
		s.Model = v
	}
	if v, ok := str("effort"); ok && contains(Efforts, v) {
		s.Effort = v
	}
	if v, ok := str("ollama_url"); ok {
		u, err := CheckOllamaURL(v)
		if err != nil {
			return s, err
		}
		s.OllamaURL = u
	}
	if v, ok := str("ollama_model"); ok && ollamaModel.MatchString(v) {
		s.OllamaModel = v
	}
	if v, ok := str("checker"); ok {
		if err := s.SetChecker(v); err != nil {
			return s, err
		}
	}
	if raw, present := data["allow"]; present && raw != nil {
		list, ok := raw.([]any)
		if !ok {
			return s, Errorf("allow must be a list of rules such as \"git status\"")
		}
		for _, r := range list {
			text, _ := r.(string)
			if _, err := s.AddRule(text, parseRule); err != nil {
				return s, err
			}
		}
	}
	for _, f := range []struct {
		key       string
		field     *int
		low, high int
	}{{"max_steps", &s.MaxSteps, 1, 200}, {"command_timeout", &s.CommandTimeout, 1, 3600},
		{"max_timeout", &s.MaxTimeout, 1, 7200}, {"output_limit", &s.OutputLimit, 1000, 500_000},
		{"show_lines", &s.ShowLines, 0, 10_000}} {
		if v, ok := data[f.key].(float64); ok && v == float64(int(v)) && int(v) >= f.low && int(v) <= f.high {
			*f.field = int(v)
		}
	}
	return s, nil
}

// ActiveModel is the model of the selected provider.
func (s *Settings) ActiveModel() string { return s.ModelOf(s.Provider) }

// ModelOf is the model of a provider, from the setting its row names.
func (s *Settings) ModelOf(provider string) string { return *s.modelField(providers.MustGet(provider)) }

func (s *Settings) modelField(spec providers.Spec) *string {
	if spec.ModelSetting == "ollama_model" {
		return &s.OllamaModel
	}
	return &s.Model
}

// Endpoint is the URL of a provider: the setting its row names, or the row's own.
func (s *Settings) Endpoint(provider string) string {
	spec := providers.MustGet(provider)
	if spec.URLSetting == "ollama_url" {
		return s.OllamaURL
	}
	return spec.URL
}

func (s *Settings) SetMode(mode string) error {
	if !contains(Modes, mode) {
		return Errorf("mode must be one of %s", strings.Join(Modes, ", "))
	}
	s.Mode = mode
	return nil
}

func (s *Settings) SetProvider(provider string) error {
	if !contains(Providers, provider) {
		return Errorf("provider must be one of %s", strings.Join(Providers, ", "))
	}
	s.Provider = provider
	return nil
}

// SetModel sets the model of the active provider.
func (s *Settings) SetModel(model string) error {
	spec := providers.MustGet(s.Provider)
	if !spec.ModelPattern.MatchString(model) {
		return Errorf("%s is not an %s model name", pyRepr(model), spec.Label)
	}
	*s.modelField(spec) = model
	return nil
}

// SetChecker sets the checker as provider:model or a bare model name, Anthropic's if it starts
// with claude- and Ollama's otherwise; empty or off turns it off. Stored as provider:model.
func (s *Settings) SetChecker(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || value == "off" {
		s.Checker = ""
		return nil
	}
	switch strings.ToLower(value) {
	case "on", "yes", "true", "enable", "enabled":
		return Errorf("give the checker's model name, e.g. claude-haiku-5-5 or qwen3.5:4b, or off")
	}
	provider, model, _ := strings.Cut(value, ":")
	if !contains(Providers, provider) {
		provider, model = "ollama", value
		if strings.HasPrefix(value, "claude-") {
			provider = "anthropic"
		}
	}
	if !providers.MustGet(provider).ModelPattern.MatchString(model) {
		return Errorf("checker %s is not a model name, e.g. claude-haiku-5-5, qwen3.5:4b, or provider:model", pyRepr(value))
	}
	s.Checker = provider + ":" + model
	return nil
}

// AddRule adds an allow rule and returns it normalised.
func (s *Settings) AddRule(text string, parseRule func(string) ([]string, error)) (string, error) {
	words, err := parseRule(text)
	if err != nil {
		return "", Errorf("allow rule %s: %v", pyRepr(text), err)
	}
	rule := strings.Join(words, " ")
	if !contains(s.Allow, rule) {
		s.Allow = append(s.Allow, rule)
	}
	return rule, nil
}

// RemoveRule removes a rule; false if there was none.
func (s *Settings) RemoveRule(text string) bool {
	rule := strings.Join(strings.Fields(text), " ")
	for i, r := range s.Allow {
		if r == rule {
			s.Allow = append(s.Allow[:i], s.Allow[i+1:]...)
			return true
		}
	}
	return false
}

// AllowRules are the rules split into words.
func (s *Settings) AllowRules() [][]string {
	var rules [][]string
	for _, r := range s.Allow {
		rules = append(rules, strings.Fields(r))
	}
	return rules
}

func (s *Settings) CheckerProvider() string { p, _, _ := strings.Cut(s.Checker, ":"); return p }
func (s *Settings) CheckerModel() string    { _, m, _ := strings.Cut(s.Checker, ":"); return m }

// CheckOllamaURL: the Ollama endpoint must be on this machine, http(s) to a loopback address.
// Anything else would send requests and command output elsewhere while ush says they stay local.
func CheckOllamaURL(raw string) (string, error) {
	bad := Errorf("Ollama URL %s is not on this machine; ush only uses a local Ollama, e.g. %s", pyRepr(raw), OllamaURL)
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" ||
		strings.Contains(strings.TrimSpace(raw), "?") || strings.Contains(strings.TrimSpace(raw), "#") {
		return "", bad
	}
	host := u.Hostname()
	if port := u.Port(); port != "" {
		if n, err := strconv.Atoi(port); err != nil || n < 0 || n > 65535 {
			return "", bad
		}
	} else if strings.HasSuffix(u.Host, ":") {
		return "", bad
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return "", bad
	}
	return u.Scheme + "://" + u.Host, nil
}

// ApplyEnvironment: USH_PROVIDER, USH_OLLAMA_URL, USH_MODEL and USH_CHECKER override the
// settings file, except as root.
func ApplyEnvironment(s *Settings, id Identity, getenv func(string) string) error {
	if id.IsRoot() {
		return nil
	}
	if v := getenv("USH_PROVIDER"); v != "" {
		if err := s.SetProvider(strings.TrimSpace(v)); err != nil {
			return err
		}
	}
	if v := getenv("USH_OLLAMA_URL"); v != "" {
		u, err := CheckOllamaURL(v)
		if err != nil {
			return err
		}
		s.OllamaURL = u
	}
	if v := getenv("USH_MODEL"); v != "" {
		if err := s.SetModel(strings.TrimSpace(v)); err != nil {
			return err
		}
	}
	if v := getenv("USH_CHECKER"); v != "" {
		if err := s.SetChecker(v); err != nil {
			return err
		}
	}
	return nil
}

// Load reads config.json; defaults if there is none.
func Load(id Identity, parseRule func(string) ([]string, error)) (Settings, error) {
	path := filepath.Join(id.ConfigDir(), "config.json")
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Defaults(), nil
	}
	if err != nil {
		return Defaults(), Errorf("cannot read %s: %v", path, err)
	}
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return Defaults(), Errorf("cannot read %s: %v", path, err)
	}
	m, _ := data.(map[string]any)
	if m == nil {
		m = map[string]any{}
	}
	return FromMap(m, parseRule)
}

// Save writes config.json privately and returns its path.
func Save(id Identity, s Settings) (string, error) {
	dir, err := privateDir(id.ConfigDir(), id)
	if err != nil {
		return "", err
	}
	if s.Allow == nil {
		s.Allow = []string{}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "config.json")
	return path, WritePrivate(path, buf.Bytes(), id)
}

// ---------------------------------------------------------------------------
// Private files

func privateDir(path string, id Identity) (string, error) {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return "", err
	}
	if id.IsRoot() && id.ViaSudo() {
		_ = os.Chown(path, id.OwnerUID, -1)
	}
	return path, nil
}

// WritePrivate writes via a new temporary file and renames it. The temporary name is random and
// created with O_EXCL: under sudo, root writes into a directory the invoking user controls, where
// a planted name, a symlink or a hard link to a root-owned file, must never be opened (SECTEST F5).
func WritePrivate(path string, data []byte, id Identity) error {
	var f *os.File
	var tmp string
	for range 100 {
		b := make([]byte, 6)
		_, _ = rand.Read(b)
		tmp = path + "." + hex.EncodeToString(b) + ".tmp"
		var err error
		f, err = os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
	}
	if f == nil {
		return Errorf("cannot create a temporary file next to %s", path)
	}
	_, err := f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && id.IsRoot() && id.ViaSudo() {
		_ = os.Chown(tmp, id.OwnerUID, -1)
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// ReadPrivate reads a file that must be a regular, non-symlink file owned by one of owners and
// readable only by its owner; "" with nil error if it doesn't exist.
func ReadPrivate(path string, owners ...int) (string, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		var pe *os.PathError
		if errors.As(err, &pe) {
			return "", Errorf("cannot open %s: %s", path, strerror(pe.Err))
		}
		return "", Errorf("cannot open %s: %v", path, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", Errorf("%s is not a regular file", path)
	}
	st := info.Sys().(*syscall.Stat_t)
	if !containsInt(owners, int(st.Uid)) {
		return "", Errorf("%s is owned by uid %d; refusing to use it", path, st.Uid)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return "", Errorf("%s is accessible by other users; run: chmod 600 %s", path, path)
	}
	data, err := os.ReadFile(path)
	return string(data), err
}

// strerror is the C library's message, as Python's exc.strerror ("Too many levels of symbolic
// links" for a link opened with O_NOFOLLOW).
func strerror(err error) string {
	if errno, ok := err.(syscall.Errno); ok {
		msg := errno.Error()
		return strings.ToUpper(msg[:1]) + msg[1:]
	}
	return err.Error()
}

// PathRisks lists PATH entries through which another program could stand in for a listed one
// (SECTEST §2): relative or empty entries and world-writable directories; as root, also
// group-writable directories and directories not owned by root. Missing directories are skipped.
func PathRisks(path string, uid int) []string {
	var risks []string
	seen := map[string]bool{}
	for _, entry := range strings.Split(path, ":") {
		if seen[entry] {
			continue
		}
		seen[entry] = true
		if entry == "" || !filepath.IsAbs(entry) {
			name := entry
			if name == "" {
				name = "(empty)"
			}
			risks = append(risks, name+": relative")
			continue
		}
		info, err := os.Stat(entry)
		if err != nil {
			continue
		}
		st := info.Sys().(*syscall.Stat_t)
		mode := info.Mode().Perm()
		switch {
		case mode&0o002 != 0:
			risks = append(risks, entry+": writable by everyone")
		case uid == 0 && mode&0o020 != 0:
			risks = append(risks, entry+": writable by its group")
		case uid == 0 && st.Uid != 0:
			risks = append(risks, fmt.Sprintf("%s: owned by uid %d", entry, st.Uid))
		}
	}
	return risks
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func containsInt(list []int, n int) bool {
	for _, x := range list {
		if x == n {
			return true
		}
	}
	return false
}

// pyRepr quotes like Python's repr() of a string.
func pyRepr(s string) string {
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		return `"` + s + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `\'`) + "'"
}
