package provider

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os/exec"
	"regexp"
	"strings"
	"unicode"
)

// Tool calls written as text (ushpy provider.py text_tool_call). Small models (llama3.2:3b) write
// Llama's call format, {"name": …, "parameters": {…}}, and Ollama turns it into a call only when
// the name is one of the tools: with the command as the name ("ls"), the call arrives as text.

var (
	fence   = regexp.MustCompile("(?s)\\A```(?:json)?\\s*(.*?)\\s*```\\z")
	program = regexp.MustCompile(`\A[A-Za-z0-9][A-Za-z0-9._+-]*\z`)
	slip    = regexp.MustCompile(`"(parameters|arguments)\{`)
)

// TextToolCall returns the name and input of a tool call written as text: one JSON object with a
// string name and an object parameters (or arguments), alone in the text or in one code fence.
//
// Models that write calls as text also get them slightly wrong, so the call is read as meant where
// that is unambiguous; the agent's checks apply afterwards as to any call:
//   - a name that is no tool, with input whose keys fit exactly one tool (all its required keys,
//     none it lacks), is that tool; a name that is a program on the PATH (ls) and isn't the first
//     word of command is put in front of it (-d becomes ls -d);
//   - "true" and "false" in a boolean field are booleans;
//   - `"parameters{`, a slip llama3.2 makes, is `"parameters": {`.
func TextToolCall(text string, tools []Block) (string, Block, bool) {
	text = strings.TrimSpace(text)
	if m := fence.FindStringSubmatch(text); m != nil {
		text = m[1]
	}
	if !strings.HasPrefix(text, "{") {
		return "", nil, false
	}
	var call Block
	if json.Unmarshal([]byte(text), &call) != nil {
		fixed := text
		if loc := slip.FindStringSubmatchIndex(text); loc != nil {
			fixed = text[:loc[0]] + `"` + text[loc[2]:loc[3]] + `": {` + text[loc[1]:]
		}
		if json.Unmarshal([]byte(fixed), &call) != nil {
			return "", nil, false
		}
	}
	name, ok := call["name"].(string)
	if !ok || call == nil || len(call) != 2 {
		return "", nil, false
	}
	raw, ok := call["parameters"]
	if !ok {
		raw, ok = call["arguments"]
	}
	args, isObject := raw.(Block)
	if !ok || !isObject {
		return "", nil, false
	}
	byName := map[string]Block{}
	for _, t := range tools {
		byName[str(t["name"])] = t
	}
	if _, known := byName[name]; !known {
		var fits []Block
		for _, t := range tools {
			if fitsSchema(args, t) {
				fits = append(fits, t)
			}
		}
		if len(fits) != 1 {
			return name, args, true
		}
		if command, ok := args["command"].(string); ok {
			first := strings.Fields(command)
			if (len(first) == 0 || first[0] != name) && program.MatchString(name) {
				if _, err := exec.LookPath(name); err == nil {
					copied := Block{}
					for k, v := range args {
						copied[k] = v
					}
					copied["command"] = strings.TrimSpace(name + " " + strings.TrimSpace(command))
					args = copied
				}
			}
		}
		name = str(fits[0]["name"])
	}
	props, _ := byName[name]["input_schema"].(Block)["properties"].(Block)
	out := Block{}
	for k, v := range args {
		spec, _ := props[k].(Block)
		if s, isString := v.(string); isString && spec["type"] == "boolean" && (s == "true" || s == "false") {
			out[k] = s == "true"
		} else {
			out[k] = v
		}
	}
	return name, out, true
}

// fitsSchema: the input has every required key of the tool and no key the tool lacks.
func fitsSchema(args Block, tool Block) bool {
	schema, _ := tool["input_schema"].(Block)
	props, _ := schema["properties"].(Block)
	required, _ := schema["required"].([]any)
	for _, key := range required {
		if _, ok := args[str(key)]; !ok {
			return false
		}
	}
	for key := range args {
		if _, ok := props[key]; !ok {
			return false
		}
	}
	return true
}

// HeldText streams text through, except while it may still be a tool call written as text (it
// starts with { or a code fence): that is held until Flush, at the end of the turn.
type HeldText struct {
	write   func(string)
	held    string
	passing bool
}

func NewHeldText(write func(string)) *HeldText { return &HeldText{write: write} }

func (h *HeldText) Write(chunk string) {
	if h.passing {
		h.write(chunk)
		return
	}
	h.held += chunk
	start := strings.TrimLeftFunc(h.held, unicode.IsSpace)
	if start != "" && !strings.HasPrefix(start, "{") && !strings.HasPrefix(start, "`") {
		h.Flush()
	}
}

func (h *HeldText) Flush() {
	h.passing = true
	if h.held != "" {
		held := h.held
		h.held = ""
		h.write(held)
	}
}

// fromText makes a turn of Ollama's whose text is a tool call written as text into that call.
func fromText(turn Turn, tools []Block) (Turn, bool) {
	var text strings.Builder
	var others []Block
	for _, b := range turn.Content {
		switch b["type"] {
		case "tool_use":
			return turn, false
		case "text":
			text.WriteString(str(b["text"]))
		default:
			others = append(others, b)
		}
	}
	name, args, ok := TextToolCall(text.String(), tools)
	if !ok {
		return turn, false
	}
	id := make([]byte, 12)
	_, _ = rand.Read(id)
	turn.Content = append(others, Block{"type": "tool_use", "id": "toolu_text_" + hex.EncodeToString(id),
		"name": name, "input": args})
	turn.StopReason = "tool_use"
	return turn, true
}
