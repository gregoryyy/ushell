package provider

import (
	"context"
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// The schemas of ush's tools, as the agent sends them (agent.go), with their keys and types.
func schemas(t *testing.T, names ...string) []Block {
	all := map[string]string{
		"propose_command": `{"name": "propose_command", "input_schema": {"type": "object",
			"properties": {"command": {"type": "string"}, "purpose": {"type": "string"},
				"effect": {"type": "string"}, "interactive": {"type": "boolean"}},
			"required": ["command", "purpose", "effect", "interactive"]}}`,
		"run_command": `{"name": "run_command", "input_schema": {"type": "object",
			"properties": {"command": {"type": "string"}, "purpose": {"type": "string"},
				"effect": {"type": "string"}, "timeout_seconds": {"type": "integer"}},
			"required": ["command", "purpose", "effect", "timeout_seconds"]}}`,
		"run_script": `{"name": "run_script", "input_schema": {"type": "object",
			"properties": {"language": {"type": "string"}, "source": {"type": "string"},
				"purpose": {"type": "string"}, "effect": {"type": "string"},
				"timeout_seconds": {"type": "integer"}},
			"required": ["language", "source", "purpose", "effect", "timeout_seconds"]}}`,
	}
	var out []Block
	for _, n := range names {
		var b Block
		if err := json.Unmarshal([]byte(all[n]), &b); err != nil {
			t.Fatal(err)
		}
		out = append(out, b)
	}
	return out
}

var proposal = Block{"command": "ls -bA", "purpose": "list", "effect": "read", "interactive": false}

func written(name string, key string, args Block) string {
	raw, _ := json.Marshal(Block{"name": name, key: args})
	return string(raw)
}

func with(b Block, k string, v any) Block {
	out := Block{}
	for key, val := range b {
		out[key] = val
	}
	out[k] = v
	return out
}

// Tool calls small models write as text (llama3.2:3b, 2026-10-29), as ushpy's TextToolCallTest.
func TestTextToolCall(t *testing.T) {
	propose := schemas(t, "propose_command")
	agent := schemas(t, "run_command", "run_script")
	runArgs := Block{"command": "echo mock-output", "purpose": "check", "effect": "read", "timeout_seconds": 20.0}
	slip := strings.Replace(written("propose_command", "parameters", proposal), `"parameters":{`, `"parameters{`, 1)
	for _, c := range []struct {
		text  string
		tools []Block
		name  string
		args  Block
	}{
		// A named tool is taken as written.
		{written("propose_command", "parameters", proposal), propose, "propose_command", proposal},
		{written("run_command", "arguments", runArgs), agent, "run_command", runArgs},
		// A program as the name is the tool its input fits, in front of the command if missing.
		{`{"name": "ls", "parameters": {"command": "-d", "purpose": "list", "effect": "read", "interactive": "false"}}`,
			propose, "propose_command", Block{"command": "ls -d", "purpose": "list", "effect": "read", "interactive": false}},
		{written("ls", "parameters", proposal), propose, "propose_command", proposal},
		{written("wc", "parameters", with(runArgs, "command", "-l x")), agent, "run_command", with(runArgs, "command", "wc -l x")},
		// A name that is no program is not put in front.
		{written("no-such-program-ush-test", "parameters", proposal), propose, "propose_command", proposal},
		{written("list files", "parameters", proposal), propose, "propose_command", proposal},
		// Input that fits no tool keeps its name, for the agent to refuse.
		{written("ls", "parameters", Block{"command": "-d", "purpose": "list"}), propose, "ls", Block{"command": "-d", "purpose": "list"}},
		{written("ls", "parameters", with(proposal, "shell", "zsh")), propose, "ls", with(proposal, "shell", "zsh")},
		// Fences and the parameters slip.
		{"```json\n" + written("propose_command", "parameters", proposal) + "\n```", propose, "propose_command", proposal},
		{"  ```\n" + written("propose_command", "parameters", proposal) + "```\n", propose, "propose_command", proposal},
		{slip, propose, "propose_command", proposal},
	} {
		name, args, ok := TextToolCall(c.text, c.tools)
		if !ok || name != c.name || !reflect.DeepEqual(args, c.args) {
			t.Errorf("%s: %v %s %v, want %s %v", c.text, ok, name, args, c.name, c.args)
		}
	}
	for _, text := range []string{"Run `ls -d` to list it.", `{"command": "ls"}`, `{"name": "ls"}`,
		`{"name": "ls", "parameters": "ls -d"}`, `{"name": "ls", "parameters": {}, "extra": 1}`, "[1, 2]",
		"{not json", `Here: {"name": "ls", "parameters": {}}`, `{"name": 3, "parameters": {}}`} {
		if name, args, ok := TextToolCall(text, propose); ok {
			t.Errorf("%s: %s %v", text, name, args)
		}
	}
}

func TestHeldTextStreamsProseAndHoldsAPossibleCall(t *testing.T) {
	var out []string
	h := NewHeldText(func(s string) { out = append(out, s) })
	for _, chunk := range []string{"  ", "Run ", "`ls`."} {
		h.Write(chunk)
	}
	if !reflect.DeepEqual(out, []string{"  Run ", "`ls`."}) {
		t.Fatal(out)
	}
	for _, opening := range []string{"{", "`"} {
		out = nil
		h := NewHeldText(func(s string) { out = append(out, s) })
		h.Write(" " + opening)
		h.Write(`"name"`)
		if out != nil {
			t.Fatal(out)
		}
		h.Flush()
		if !reflect.DeepEqual(out, []string{" " + opening + `"name"`}) {
			t.Fatal(out)
		}
	}
}

const writtenCall = `{"name": "ls", "parameters": {"command": "-d", "purpose": "list", "effect": "read", "interactive": "false"}}`

func TestOllamaTurnsAWrittenCallIntoAToolUse(t *testing.T) {
	m := newMock(reply{blocks: []Block{{"type": "text", "text": writtenCall}}, stop: "end_turn"})
	defer m.server.Close()
	var shown strings.Builder
	turn, err := NewOllama("llama3.2:3b", m.server.URL).Turn(context.Background(), "sys", schemas(t, "propose_command"),
		[]Block{{"role": "user", "content": "x"}}, func(s string) { shown.WriteString(s) }, nil)
	if err != nil || shown.String() != "" || turn.StopReason != "tool_use" || len(turn.Content) != 1 {
		t.Fatal(turn, err, shown.String())
	}
	b := turn.Content[0]
	in := b["input"].(Block)
	if b["type"] != "tool_use" || b["name"] != "propose_command" || in["command"] != "ls -d" || in["interactive"] != false {
		t.Fatal(b)
	}
	if !regexp.MustCompile(`^toolu_text_[0-9a-f]{24}$`).MatchString(str(b["id"])) {
		t.Error(b["id"])
	}
}

func TestProseARealCallAndAnthropicTextAreUnchanged(t *testing.T) {
	use := Block{"type": "tool_use", "id": "call_1", "name": "run_command", "input": command}
	m := newMock(reply{blocks: []Block{{"type": "text", "text": "It is empty."}}, stop: "end_turn"},
		reply{blocks: []Block{{"type": "text", "text": writtenCall}, use}, stop: "tool_use"},
		reply{blocks: []Block{{"type": "text", "text": writtenCall}}, stop: "end_turn"})
	defer m.server.Close()
	ask := func(c *Client, tools []Block) (Turn, string) {
		var shown strings.Builder
		turn, err := c.Turn(context.Background(), "sys", tools, []Block{{"role": "user", "content": "x"}},
			func(s string) { shown.WriteString(s) }, nil)
		if err != nil {
			t.Fatal(err)
		}
		return turn, shown.String()
	}
	o := NewOllama("llama3.2:3b", m.server.URL)
	if turn, shown := ask(o, schemas(t, "propose_command")); shown != "It is empty." || turn.Content[0]["text"] != "It is empty." {
		t.Error(turn, shown)
	}
	if turn, shown := ask(o, schemas(t, "run_command", "run_script")); shown != writtenCall || len(turn.Content) != 2 || turn.Content[1]["type"] != "tool_use" {
		t.Error(turn, shown)
	}
	a := NewAnthropic(syntheticKey, "claude-opus-5-5", "medium", m.server.URL)
	a.MaxRetries = 0
	if turn, shown := ask(a, schemas(t, "propose_command")); shown != writtenCall || turn.StopReason != "end_turn" {
		t.Error(turn, shown)
	}
}
