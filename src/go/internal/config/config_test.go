package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// syntheticKey is assembled at run time, as in ushpy's tests.
var syntheticKey = "sk-ant-test-" + strings.Repeat("0", 28) + "SYNTHETIC"

func noRules(text string) ([]string, error) {
	if strings.Contains(text, "rm") {
		return nil, errors.New("rm changes or sends data")
	}
	return strings.Fields(text), nil
}

func me(home string) Identity {
	uid := os.Getuid()
	return Identity{UID: uid, Name: "tester", Home: home, OwnerUID: uid, OwnerName: "tester", OwnerHome: home}
}

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestRoundTripAndValidation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "cfg"))
	id := me(t.TempDir())
	s, err := Load(id, noRules)
	if err != nil || s.Model != DefaultModel || s.Mode != "propose" {
		t.Fatal(s, err)
	}
	s.Model, s.Effort = "claude-sonnet-5-5", "high"
	path, err := Save(id, s)
	if err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", info.Mode())
	}
	if again, _ := Load(id, noRules); again.Effort != "high" || again.Model != "claude-sonnet-5-5" {
		t.Error(again)
	}
	os.WriteFile(path, []byte(`{"model": "x; rm", "effort": "ultra", "max_steps": -1}`), 0o600)
	loaded, _ := Load(id, noRules)
	if loaded.Model != DefaultModel || loaded.Effort != "medium" || loaded.MaxSteps != 30 {
		t.Error(loaded)
	}
	os.WriteFile(path, []byte("{broken"), 0o600)
	if _, err := Load(id, noRules); err == nil {
		t.Error("broken JSON accepted")
	}
}

// A file ushpy wrote reads the same in Go (GAPS A1: the two can be swapped).
func TestReadsUshpysFile(t *testing.T) {
	var data map[string]any
	json.Unmarshal([]byte(`{"mode": "run", "provider": "ollama", "model": "claude-opus-5-5",
		"effort": "low", "ollama_url": "http://127.0.0.1:11434", "ollama_model": "qwen3.5:9b",
		"checker": "anthropic:claude-haiku-5-5", "allow": ["git status"], "max_steps": 30,
		"command_timeout": 120, "max_timeout": 900, "output_limit": 32000, "show_lines": 12}`), &data)
	s, err := FromMap(data, noRules)
	if err != nil || s.Mode != "run" || s.ActiveModel() != "qwen3.5:9b" || s.CheckerModel() != "claude-haiku-5-5" ||
		len(s.AllowRules()) != 1 || s.Effort != "low" {
		t.Error(s, err)
	}
}

func TestOllamaURLsMustBeOnThisMachine(t *testing.T) {
	for in, want := range map[string]string{"http://127.0.0.1:11434": "http://127.0.0.1:11434",
		"http://localhost:11434/": "http://localhost:11434", "http://[::1]:11434": "http://[::1]:11434",
		"https://127.0.0.2:8443": "https://127.0.0.2:8443"} {
		if got, err := CheckOllamaURL(in); got != want || err != nil {
			t.Errorf("%s: %s %v", in, got, err)
		}
	}
	for _, bad := range []string{"http://192.168.1.5:11434", "http://ollama.example.com:11434",
		"http://127.0.0.1.evil.example", "http://localhost.evil.example", "http://user:pw@127.0.0.1:11434",
		"http://127.0.0.1:11434/v1", "http://127.0.0.1:11434/?next=http://evil.example",
		"http://127.0.0.1:99999", "file:///tmp/ollama.sock", "ftp://127.0.0.1", "127.0.0.1:11434", ""} {
		if _, err := CheckOllamaURL(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if _, err := FromMap(map[string]any{"ollama_url": "http://10.0.0.8:11434"}, noRules); err == nil {
		t.Error("remote URL in the file accepted")
	}
}

func TestModelsAndCheckers(t *testing.T) {
	s := Defaults()
	if s.SetModel("hf.co/org/Model:Q4") == nil {
		t.Error("Ollama name as Anthropic model")
	}
	s.SetProvider("ollama")
	if err := s.SetModel("hf.co/org/Model:Q4"); err != nil || s.Model != DefaultModel {
		t.Error(err, s.Model)
	}
	if s.SetProvider("mistral") == nil || s.SetModel("x; rm -rf /") == nil {
		t.Error("bad values accepted")
	}
	for in, want := range map[string]string{"ollama:qwen3.5:2b": "ollama:qwen3.5:2b",
		"anthropic:claude-haiku-5-5": "anthropic:claude-haiku-5-5", "claude-haiku-5-5": "anthropic:claude-haiku-5-5",
		"qwen3.5:4b": "ollama:qwen3.5:4b", "library/qwen3.5:4b": "ollama:library/qwen3.5:4b", "off": "", " ": ""} {
		if err := s.SetChecker(in); err != nil || s.Checker != want {
			t.Errorf("%q: %q %v", in, s.Checker, err)
		}
	}
	for _, bad := range []string{"bad name!", "ollama:bad name!", "anthropic:x; rm", "anthropic:", "on", "yes"} {
		if s.SetChecker(bad) == nil {
			t.Errorf("checker %q accepted", bad)
		}
	}
}

func TestEnvironmentOverridesApplyToUsersButNotToRoot(t *testing.T) {
	e := env(map[string]string{"USH_PROVIDER": "ollama", "USH_MODEL": "llama3.2:3b",
		"USH_OLLAMA_URL": "http://localhost:11500", "USH_CHECKER": "qwen3.5:2b"})
	user := Defaults()
	if err := ApplyEnvironment(&user, me(t.TempDir()), e); err != nil {
		t.Fatal(err)
	}
	if user.Provider != "ollama" || user.OllamaModel != "llama3.2:3b" || user.OllamaURL != "http://localhost:11500" || user.Checker != "ollama:qwen3.5:2b" {
		t.Error(user)
	}
	root := Defaults()
	rootID := me(t.TempDir())
	rootID.UID = 0
	ApplyEnvironment(&root, rootID, e)
	if root.Provider != "anthropic" || root.Checker != "" {
		t.Error("root used the environment")
	}
	bad := Defaults()
	if ApplyEnvironment(&bad, me(t.TempDir()), env(map[string]string{"USH_OLLAMA_URL": "http://evil.example"})) == nil {
		t.Error("remote URL from the environment accepted")
	}
}

func TestRootIgnoresXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/elsewhere")
	t.Setenv("XDG_STATE_HOME", "/tmp/elsewhere")
	id := Identity{UID: 0, Home: "/var/root", OwnerUID: 501, OwnerHome: "/Users/me"}
	if id.ConfigDir() != "/Users/me/.config/ush" || id.StateDir() != "/var/root/.local/state/ush" {
		t.Error(id.ConfigDir(), id.StateDir())
	}
}

func TestFileStoreIsPrivateAndRoundTrips(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	k := &KeyStore{ID: me(t.TempDir()), Getenv: env(nil), GOOS: "linux"}
	where, err := k.Store(syntheticKey, true, "anthropic")
	if err != nil || !strings.HasPrefix(where, "file (") {
		t.Fatal(where, err)
	}
	if info, _ := os.Stat(k.CredentialsPath()); info.Mode().Perm() != 0o600 {
		t.Error(info.Mode())
	}
	if key, source, err := k.Resolve("anthropic"); key != syntheticKey || !strings.HasPrefix(source, "file") || err != nil {
		t.Error(key, source, err)
	}
	k.Getenv = env(map[string]string{"ANTHROPIC_API_KEY": "from-env"})
	if key, source, _ := k.Resolve("anthropic"); key != "from-env" || source != "environment (ANTHROPIC_API_KEY)" {
		t.Error(key, source)
	}
	if _, err := k.Store("not a key", false, "anthropic"); err == nil {
		t.Error("malformed key stored")
	}
	if got := k.Clear("anthropic"); len(got) != 1 {
		t.Error(got)
	}
}

func TestInsecureOrLinkedCredentialsAreRefused(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	k := &KeyStore{ID: me(t.TempDir()), Getenv: env(nil), GOOS: "linux"}
	os.MkdirAll(filepath.Dir(k.CredentialsPath()), 0o700)
	os.WriteFile(k.CredentialsPath(), []byte(`{"anthropic_api_key": "x"}`), 0o644)
	if _, _, err := k.Resolve("anthropic"); err == nil || !strings.Contains(err.Error(), "accessible by other users") {
		t.Error(err)
	}
	os.Remove(k.CredentialsPath())
	target := filepath.Join(t.TempDir(), "elsewhere")
	os.WriteFile(target, []byte(`{"anthropic_api_key": "x"}`), 0o600)
	os.Symlink(target, k.CredentialsPath())
	if _, _, err := k.Resolve("anthropic"); err == nil || !strings.Contains(err.Error(), "cannot open") {
		t.Error(err)
	}
}

func TestPlantedLinksAreNeverWrittenThrough(t *testing.T) { // SECTEST F5
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	id := me(t.TempDir())
	dir := id.ConfigDir()
	os.MkdirAll(dir, 0o700)
	victim := filepath.Join(t.TempDir(), "victim")
	os.WriteFile(victim, []byte("root-owned content"), 0o644)
	os.Link(victim, filepath.Join(dir, "config.json.tmp"))
	os.Symlink(victim, filepath.Join(dir, "config.json"))
	if _, err := Save(id, Defaults()); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(victim); string(data) != "root-owned content" {
		t.Error("written through a planted link")
	}
	if info, _ := os.Lstat(filepath.Join(dir, "config.json")); info.Mode()&os.ModeSymlink != 0 {
		t.Error("config.json is still a link")
	}
}

func TestMaskAndPathRisks(t *testing.T) {
	if strings.Contains(Mask(syntheticKey), syntheticKey[10:len(syntheticKey)-4]) || Mask("short") != "…" {
		t.Error(Mask(syntheticKey))
	}
	open := filepath.Join(t.TempDir(), "open")
	os.Mkdir(open, 0o777)
	os.Chmod(open, 0o777)
	risks := PathRisks(".::"+open+":/usr/bin:/nonexistent", os.Getuid())
	if strings.Join(risks, "|") != ".: relative|(empty): relative|"+open+": writable by everyone" {
		t.Error(risks)
	}
}

const syntheticOpenAIKey = "sk-proj-test-000000000000000000000000000000SYNTHETIC"

// TODO 2.29.6: the credentials file holds one key per provider; storing or clearing one leaves the
// other; each provider's variable is read for it only.
func TestEachProviderHasItsOwnKey(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	k := &KeyStore{ID: me(t.TempDir()), Getenv: env(map[string]string{"ANTHROPIC_API_KEY": syntheticKey}), GOOS: "linux"}
	if key, source, _ := k.Resolve("openai"); key != "" || source != "none" {
		t.Error(key, source)
	}
	if _, err := k.Store(syntheticOpenAIKey, false, "openai"); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Store(syntheticKey, false, "anthropic"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(k.CredentialsPath())
	var saved map[string]string
	json.Unmarshal(raw, &saved)
	if len(saved) != 2 || saved["openai_api_key"] != syntheticOpenAIKey || saved["anthropic_api_key"] != syntheticKey {
		t.Error(saved)
	}
	if key, _, _ := k.Resolve("openai"); key != syntheticOpenAIKey {
		t.Error(key)
	}
	if got := k.Clear("anthropic"); len(got) != 1 || got[0] != k.CredentialsPath() {
		t.Error(got)
	}
	if key, _, _ := k.Resolve("openai"); key != syntheticOpenAIKey {
		t.Error("clearing one provider's key removed the other's")
	}
	if got := k.Clear("openai"); len(got) != 1 {
		t.Error(got)
	}
	if _, err := os.Stat(k.CredentialsPath()); !os.IsNotExist(err) {
		t.Error("the file stays after its last key")
	}
	variable := &KeyStore{ID: k.ID, Getenv: env(map[string]string{"OPENAI_API_KEY": syntheticOpenAIKey}), GOOS: "linux"}
	if key, source, _ := variable.Resolve("openai"); key != syntheticOpenAIKey || source != "environment (OPENAI_API_KEY)" {
		t.Error(key, source)
	}
	if key, _, _ := variable.Resolve("anthropic"); key != "" {
		t.Error("OPENAI_API_KEY used for Anthropic")
	}
}

func TestTheKeychainAccountIsTheProvider(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	script := filepath.Join(dir, "security")
	os.WriteFile(script, []byte("#!/bin/sh\ncat >> "+log+".in\necho \"$@\" >> "+log+"\nexit 0\n"), 0o755)
	old := Security
	Security = script
	defer func() { Security = old }()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	k := &KeyStore{ID: me(t.TempDir()), Getenv: env(nil), GOOS: "darwin"}
	if where, err := k.Store(syntheticOpenAIKey, true, "openai"); err != nil || where != "macOS keychain" {
		t.Fatal(where, err)
	}
	k.Resolve("openai")
	k.Clear("openai")
	stdin, _ := os.ReadFile(log + ".in")
	if !strings.Contains(string(stdin), "-a openai ") || !strings.Contains(string(stdin), `"ush API key (OpenAI)"`) {
		t.Error(string(stdin))
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "-a openai") != 2 {
		t.Error(string(calls))
	}
}

func TestOpenAIURLs(t *testing.T) {
	for in, want := range map[string]string{"https://gateway.example.com/": "https://gateway.example.com",
		"http://127.0.0.1:8080": "http://127.0.0.1:8080", "https://api.openai.com": "https://api.openai.com"} {
		if got, err := CheckOpenAIURL(in); got != want || err != nil {
			t.Errorf("%s: %s %v", in, got, err)
		}
	}
	for _, bad := range []string{"http://gateway.example.com", "https://user:pw@api.openai.com", "https://api.openai.com/v1",
		"https://api.openai.com/?x=1", "ftp://x", "https://", "https://api.openai.com:99999", ""} {
		if _, err := CheckOpenAIURL(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	_, err := CheckOpenAIURL("http://x.example")
	if err == nil || err.Error() != "OpenAI URL 'http://x.example' must be https://HOST, e.g. https://api.openai.com" {
		t.Error(err)
	}
	if _, err := FromMap(map[string]any{"openai_url": "http://gateway.example.com"}, noRules); err == nil {
		t.Error("remote http URL in the file accepted")
	}
}

func TestOpenAISettings(t *testing.T) {
	s := Defaults()
	if s.OpenAIModel != "gpt-6-astra" || s.OpenAIURL != "https://api.openai.com" {
		t.Error(s)
	}
	s.SetProvider("openai")
	if err := s.SetModel("gpt-6.1-sol"); err != nil || s.ActiveModel() != "gpt-6.1-sol" || s.Model != DefaultModel {
		t.Error(err, s)
	}
	if s.SetModel("a/b") == nil {
		t.Error("bad OpenAI model accepted")
	}
	if err := s.SetChecker("openai:gpt-6-luna"); err != nil || s.Checker != "openai:gpt-6-luna" {
		t.Error(err, s.Checker)
	}
	if err := ApplyEnvironment(&s, me(t.TempDir()), env(map[string]string{"USH_OPENAI_URL": "http://localhost:9"})); err != nil || s.Endpoint("openai") != "http://localhost:9" {
		t.Error(err, s.OpenAIURL)
	}
	if ApplyEnvironment(&s, me(t.TempDir()), env(map[string]string{"USH_OPENAI_URL": "http://evil.example"})) == nil {
		t.Error("remote http URL from the environment accepted")
	}
	got, err := FromMap(map[string]any{"openai_url": "https://gw.example.com", "openai_model": "bad name!"}, noRules)
	if err != nil || got.OpenAIURL != "https://gw.example.com" || got.OpenAIModel != "gpt-6-astra" {
		t.Error(got, err)
	}
}
