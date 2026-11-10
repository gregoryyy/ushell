package providers

import (
	"reflect"
	"testing"
)

func TestTheTable(t *testing.T) {
	if !reflect.DeepEqual(Names(), []string{"anthropic", "ollama"}) {
		t.Fatal(Names())
	}
	for _, s := range Table {
		if got, ok := Get(s.Name); !ok || got.Name != s.Name || s.Label == "" || s.ModelSetting == "" {
			t.Errorf("%+v", s)
		}
		if s.Loopback == s.KeyStore { // on this machine: no key; elsewhere: a key
			t.Errorf("%s: loopback %v, key store %v", s.Name, s.Loopback, s.KeyStore)
		}
	}
	a, o := MustGet("anthropic"), MustGet("ollama")
	if a.URL != "https://api.anthropic.com" || a.Loopback || !reflect.DeepEqual(a.KeyEnv, []string{"USH_API_KEY", "ANTHROPIC_API_KEY"}) {
		t.Error(a)
	}
	if o.URL != "http://127.0.0.1:11434" || o.URLSetting != "ollama_url" || !o.Loopback || len(o.KeyEnv) != 0 || !o.ListsModels {
		t.Error(o)
	}
	if !a.ModelPattern.MatchString("claude-opus-5-5") || a.ModelPattern.MatchString("qwen3.5:9b") || !o.ModelPattern.MatchString("qwen3.5:9b") {
		t.Error("model patterns")
	}
	if _, ok := Get("nope"); ok {
		t.Error("nope")
	}
}

func TestProfiles(t *testing.T) {
	models := []string{"claude-haiku-5-5"}
	for m := range UpdatesModels {
		models = append(models, m)
	}
	for m := range FallbackModels {
		models = append(models, m)
	}
	for _, m := range models {
		p := ProfileOf("anthropic", m)
		thinking := "adaptive"
		if UpdatesModels[m] {
			thinking = "adaptive-updates"
		}
		if !p.Effort || !p.StrictTools || !p.EagerToolInput || !p.PromptCache || p.TextToolCalls ||
			p.Thinking != thinking || p.ServerFallbacks != FallbackModels[m] || p.OutputLimit != 64000 {
			t.Errorf("%s: %+v", m, p)
		}
	}
	if p := ProfileOf("ollama", "llama3.2:3b"); p != (Profile{StrictTools: true, OutputLimit: 64000, TextToolCalls: true}) {
		t.Errorf("%+v", p)
	}
}
