// Package providers holds the providers ush can use, and what their models take (TODO 2.29.1,
// ARCH; ushpy providers.py).
//
// The provider table says, per provider, where it is, whether it must be on this machine, where
// its key comes from, and how its models are named. A model profile says what a model takes:
// effort, thinking, strict tool schemas, streamed tool input, an output limit, whether it may
// write tool calls as text. Settings stay neutral (effort, model); the adapters in package
// provider read the profile, and the CLI reads the table, so a new provider is a row rather than
// a branch.
package providers

import "regexp"

const (
	APIURL    = "https://api.anthropic.com"
	OllamaURL = "http://127.0.0.1:11434"
	MaxTokens = 64000
)

var (
	AnthropicModel = regexp.MustCompile(`^[a-z0-9.\-]{3,64}$`)
	OllamaModel    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/\-]{0,127}$`)
	KeyEnv         = []string{"USH_API_KEY", "ANTHROPIC_API_KEY"}
	// Models that accept server-side refusal fallbacks and between-tool progress notes.
	FallbackModels = map[string]bool{"claude-opus-5-5": true, "claude-opus-5": true, "claude-sonnet-5-5": true,
		"claude-fable-5-1": true, "claude-fable-5": true}
	UpdatesModels = map[string]bool{"claude-opus-5-5": true, "claude-sonnet-5-5": true,
		"claude-fable-5-1": true, "claude-fable-5": true}
)

// Spec is a row of the provider table.
type Spec struct {
	Name         string         // as in settings and provider:model
	Label        string         // as the disclosure and `ush ai status` name it
	API          string         // "messages": Anthropic's Messages API (Ollama serves it too)
	URL          string         // the default endpoint
	URLSetting   string         // the setting that holds the endpoint, or "" for URL
	Loopback     bool           // must be on this machine: no proxy, no redirect, stays here
	KeyEnv       []string       // variables a key may come from, before the key store
	KeyStore     bool           // needs a key, from the keychain or the credentials file
	ModelPattern *regexp.Regexp // how its models are named
	ModelSetting string         // the setting that holds its model
	ListsModels  bool           // can say which models it has and which are in a cloud
	ModelHint    string         // where to find a model name, for the error when none is set
}

// Table is the provider table, in the order ush offers the providers.
var Table = []Spec{
	{Name: "anthropic", Label: "Anthropic", API: "messages", URL: APIURL, KeyEnv: KeyEnv, KeyStore: true,
		ModelPattern: AnthropicModel, ModelSetting: "model"},
	{Name: "ollama", Label: "Ollama", API: "messages", URL: OllamaURL, URLSetting: "ollama_url", Loopback: true,
		ModelPattern: OllamaModel, ModelSetting: "ollama_model", ListsModels: true, ModelHint: "see `ollama list`"},
}

// Names are the providers' names, in the table's order.
func Names() []string {
	names := make([]string, len(Table))
	for i, s := range Table {
		names[i] = s.Name
	}
	return names
}

// Get returns a provider's row; ok is false for a name not in the table.
func Get(name string) (Spec, bool) {
	for _, s := range Table {
		if s.Name == name {
			return s, true
		}
	}
	return Spec{}, false
}

// MustGet returns a provider's row; the name must be in the table (settings check it).
func MustGet(name string) Spec {
	s, ok := Get(name)
	if !ok {
		panic("unknown provider " + name)
	}
	return s
}

// Profile is what a model takes.
type Profile struct {
	Effort          bool   // takes an effort level (output_config.effort)
	Thinking        string // "", "adaptive", or "adaptive-updates" (progress notes)
	ServerFallbacks bool   // server-side fallbacks after a refusal
	StrictTools     bool   // strict tool schemas are sent as such
	EagerToolInput  bool   // tool input streamed eagerly (eager_input_streaming)
	PromptCache     bool   // cache_control on the request
	OutputLimit     int    // max_tokens of a turn
	TextToolCalls   bool   // may write tool calls as text, read as calls (TODO 2.31)
}

// ProfileOf returns what model of provider takes.
func ProfileOf(provider, model string) Profile {
	if provider == "anthropic" {
		thinking := "adaptive"
		if UpdatesModels[model] {
			thinking = "adaptive-updates"
		}
		return Profile{Effort: true, Thinking: thinking, ServerFallbacks: FallbackModels[model], StrictTools: true,
			EagerToolInput: true, PromptCache: true, OutputLimit: MaxTokens}
	}
	// Ollama: only model, limits, system prompt, tools and messages; small models write calls as text.
	return Profile{StrictTools: true, OutputLimit: MaxTokens, TextToolCalls: true}
}
