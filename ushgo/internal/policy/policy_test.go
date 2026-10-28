package policy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type corpus struct {
	Fixture []string          `json:"fixture"`
	Env     map[string]string `json:"env"`
	Rules   []string          `json:"rules"`
	Cases   []struct {
		Command string `json:"command"`
		Rules   bool   `json:"rules"`
		Auto    bool   `json:"auto"`
		Reason  string `json:"reason"`
		Sudo    bool   `json:"sudo"`
		Rule    string `json:"rule"`
	} `json:"cases"`
	ParseRule []struct {
		Text  string   `json:"text"`
		Words []string `json:"words"`
		Error string   `json:"error"`
	} `json:"parse_rule"`
}

// TestConformance: every verdict in the shared corpus, written by ushpy (conform/policy.json,
// ushpy/scripts/policy_corpus.py), reason included.
func TestConformance(t *testing.T) {
	data, err := os.ReadFile("../../../conform/policy.json")
	if err != nil {
		t.Fatal(err)
	}
	var c corpus
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range c.Fixture {
		full := filepath.Join(root, path)
		if strings.HasSuffix(path, "/") {
			err = os.MkdirAll(full, 0o755)
		} else if err = os.MkdirAll(filepath.Dir(full), 0o755); err == nil {
			err = os.WriteFile(full, []byte("x\n"), 0o644)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	for k, v := range c.Env {
		t.Setenv(k, strings.ReplaceAll(v, "$ROOT", root))
	}
	t.Setenv("HOME", filepath.Join(root, "home"))
	etc, err := filepath.EvalSymlinks("/etc") // /private/etc on macOS; the corpus holds /etc
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	var rules [][]string
	for _, r := range c.Rules {
		words, err := ParseRule(r)
		if err != nil {
			t.Fatal(err)
		}
		rules = append(rules, words)
	}
	failed := 0
	for _, want := range c.Cases {
		var use [][]string
		if want.Rules {
			use = rules
		}
		got := Classify(want.Command, use)
		got.Reason = strings.ReplaceAll(strings.ReplaceAll(got.Reason, root, "$ROOT"), etc+"/", "/etc/")
		if got.Auto != want.Auto || got.Reason != want.Reason || got.Sudo != want.Sudo || got.Rule != want.Rule {
			failed++
			if failed <= 30 {
				t.Errorf("%q (rules %v):\n got  %+v\n want {Auto:%v Reason:%s Sudo:%v Rule:%s}",
					want.Command, want.Rules, got, want.Auto, want.Reason, want.Sudo, want.Rule)
			}
		}
	}
	if failed > 0 {
		t.Errorf("%d of %d cases differ from ushpy", failed, len(c.Cases))
	}
	if len(c.Cases) < 1000 {
		t.Errorf("only %d cases in the corpus", len(c.Cases))
	}
	t.Logf("%d cases compared", len(c.Cases))
	for _, want := range c.ParseRule {
		words, err := ParseRule(want.Text)
		gotErr := ""
		if err != nil {
			gotErr = err.Error()
		}
		if gotErr != want.Error || strings.Join(words, " ") != strings.Join(want.Words, " ") {
			t.Errorf("ParseRule(%q) = %q, %q; want %q, %q", want.Text, words, gotErr, want.Words, want.Error)
		}
	}
}
