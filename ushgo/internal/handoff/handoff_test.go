package handoff

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func setup(t *testing.T) string {
	dir := filepath.Join(t.TempDir(), "ush.abc123")
	os.Mkdir(dir, 0o700)
	return dir
}

func TestOnlyAPrivateDirectoryOfTheUserIsUsed(t *testing.T) {
	dir := setup(t)
	uid := os.Getuid()
	get := func(v string) func(string) string { return func(string) string { return v } }
	if Dir(uid, false, get(dir)) != dir {
		t.Error("own directory refused")
	}
	if Dir(uid, true, get(dir)) != "" || Dir(uid, false, get("relative")) != "" || Dir(uid, false, get("")) != "" {
		t.Error("root, relative or empty accepted")
	}
	link := filepath.Join(t.TempDir(), "link")
	os.Symlink(dir, link)
	if Dir(uid, false, get(link)) != "" {
		t.Error("symlink accepted")
	}
	os.Chmod(dir, 0o755)
	if Dir(uid, false, get(dir)) != "" {
		t.Error("open directory accepted")
	}
	os.Chmod(dir, 0o700)
	if Dir(uid+1, false, get(dir)) != "" {
		t.Error("another user's directory accepted")
	}
}

func TestProposalReplacesEarlierRecords(t *testing.T) {
	dir := setup(t)
	os.WriteFile(filepath.Join(dir, "ran"), []byte("old"), 0o600)
	os.WriteFile(filepath.Join(dir, "output"), []byte("old"), 0o600)
	if err := WriteProposal(dir, "df -h /", false); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "proposal" {
		t.Error(entries)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "proposal")); string(data) != "capture\ndf -h /" {
		t.Errorf("%q", data)
	}
	WriteProposal(dir, "less log", true)
	if data, _ := os.ReadFile(filepath.Join(dir, "proposal")); string(data) != "interactive\nless log" {
		t.Errorf("%q", data)
	}
}

func TestTakeRanReadsBoundsAndRemoves(t *testing.T) {
	dir := setup(t)
	if TakeRan(dir, 1000) != nil {
		t.Error("nothing ran")
	}
	os.WriteFile(filepath.Join(dir, "ran"), []byte("3\ngrep -r x .\n"), 0o600)
	os.WriteFile(filepath.Join(dir, "output"), []byte(strings.Repeat("a", 5000)+"END"), 0o600)
	ran := TakeRan(dir, 1000)
	if ran.Command != "grep -r x ." || *ran.ExitCode != 3 || !ran.Captured || !ran.Truncated ||
		!strings.HasSuffix(ran.Output, "END") || len(ran.Output) > 1100 {
		t.Errorf("%+v", ran)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Error(entries)
	}
	os.WriteFile(filepath.Join(dir, "ran"), []byte("x\nvim notes"), 0o600)
	if ran := TakeRan(dir, 1000); ran.ExitCode != nil || ran.Captured {
		t.Errorf("%+v", ran)
	}
}

func TestLinksAreNotFollowed(t *testing.T) {
	dir := setup(t)
	secret := filepath.Join(t.TempDir(), "secret")
	os.WriteFile(secret, []byte("0\nTOP SECRET"), 0o600)
	os.Symlink(secret, filepath.Join(dir, "ran"))
	if TakeRan(dir, 1000) != nil {
		t.Error("ran followed a link")
	}
	os.Remove(filepath.Join(dir, "ran"))
	os.WriteFile(filepath.Join(dir, "ran"), []byte("0\ncat x"), 0o600)
	os.Symlink(secret, filepath.Join(dir, "output"))
	if ran := TakeRan(dir, 1000); strings.Contains(ran.Output, "SECRET") || ran.Captured {
		t.Errorf("%+v", ran)
	}
}

func TestSessionsArePrivatePerShellAndExpire(t *testing.T) {
	state := t.TempDir()
	if SessionKey("/tmp/ush.abc123") != "ush.abc123" || !strings.HasPrefix(SessionKey(""), "ppid-") {
		t.Error("keys")
	}
	mine, _ := NewSessions(state, "ush.abc123")
	other, _ := NewSessions(state, "ush.other")
	mine.Save(map[string]any{"version": 1})
	if info, _ := os.Stat(mine.Path); info.Mode().Perm() != 0o600 {
		t.Error(info.Mode())
	}
	if mine.Load()["version"] != float64(1) || other.Load() != nil {
		t.Error("load")
	}
	old := time.Now().Add(-SessionTTL - time.Minute)
	os.Chtimes(mine.Path, old, old)
	if mine.Load() != nil {
		t.Error("expired session loaded")
	}
	other.Save(map[string]any{"a": 1})
	os.Chtimes(other.Path, old, old)
	mine.Save(map[string]any{"b": 2})
	names := []string{}
	entries, _ := os.ReadDir(mine.Dir)
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if strings.Join(names, " ") != "ush.abc123.json" {
		t.Error(names)
	}
	for _, bad := range []string{"../x", "", "a/b", strings.Repeat("x", 65)} {
		if _, err := NewSessions(state, bad); err == nil {
			t.Errorf("key %q accepted", bad)
		}
	}
}
