package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

var syntheticKey = "sk-ant-test-" + strings.Repeat("0", 28) + "SYNTHETIC"

type reply struct {
	blocks []Block
	stop   string
	status int
	errMsg Block
}

// mock streams scripted replies like ushpy's tests/mock_api.py.
type mock struct {
	mu       sync.Mutex
	replies  []reply
	requests []struct {
		path    string
		headers http.Header
		body    Block
	}
	server *httptest.Server
}

func newMock(replies ...reply) *mock {
	m := &mock{replies: replies}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body Block
		json.NewDecoder(r.Body).Decode(&body)
		m.mu.Lock()
		m.requests = append(m.requests, struct {
			path    string
			headers http.Header
			body    Block
		}{r.URL.Path, r.Header.Clone(), body})
		var rp reply
		if len(m.replies) > 0 {
			rp, m.replies = m.replies[0], m.replies[1:]
		} else {
			rp = reply{status: 500, errMsg: Block{"message": "unscripted"}}
		}
		m.mu.Unlock()
		if rp.status != 0 {
			w.WriteHeader(rp.status)
			json.NewEncoder(w).Encode(Block{"type": "error", "error": rp.errMsg})
			return
		}
		if body["stream"] != true {
			json.NewEncoder(w).Encode(Block{"type": "message", "content": rp.blocks, "stop_reason": rp.stop})
			return
		}
		w.Header().Set("content-type", "text/event-stream")
		ev := func(name string, data Block) {
			raw, _ := json.Marshal(data)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, raw)
		}
		ev("message_start", Block{"type": "message_start", "message": Block{"usage": Block{"input_tokens": 5}}})
		for i, b := range rp.blocks {
			start := func(cb Block) {
				ev("content_block_start", Block{"type": "content_block_start", "index": i, "content_block": cb})
			}
			delta := func(d Block) { ev("content_block_delta", Block{"type": "content_block_delta", "index": i, "delta": d}) }
			switch b["type"] {
			case "text":
				start(Block{"type": "text", "text": ""})
				delta(Block{"type": "text_delta", "text": b["text"]})
			case "thinking":
				start(Block{"type": "thinking", "thinking": "", "signature": ""})
				delta(Block{"type": "thinking_delta", "thinking": b["thinking"]})
				delta(Block{"type": "signature_delta", "signature": "sig-test"})
			case "tool_use":
				start(Block{"type": "tool_use", "id": b["id"], "name": b["name"], "input": Block{}})
				raw, _ := json.Marshal(b["input"])
				if b["broken"] == true {
					raw = []byte(`{"command": `)
				}
				delta(Block{"type": "input_json_delta", "partial_json": string(raw[:min(10, len(raw))])})
				delta(Block{"type": "input_json_delta", "partial_json": string(raw[min(10, len(raw)):])})
			}
			ev("content_block_stop", Block{"type": "content_block_stop", "index": i})
		}
		ev("message_delta", Block{"type": "message_delta", "delta": Block{"stop_reason": rp.stop}, "usage": Block{"output_tokens": 10}})
		ev("message_stop", Block{"type": "message_stop"})
	}))
	return m
}

var command = Block{"command": "df -h /", "purpose": "free space", "effect": "read", "timeout_seconds": 30.0}

func tools() []Block {
	return []Block{{"name": "run_command", "strict": true, "input_schema": Block{"type": "object"}}}
}

func TestToolRoundTripAndAnthropicOptions(t *testing.T) {
	m := newMock(reply{blocks: []Block{{"type": "thinking", "thinking": "Checking free space"},
		{"type": "tool_use", "id": "toolu_1", "name": "run_command", "input": command}}, stop: "tool_use"})
	defer m.server.Close()
	c := NewAnthropic(syntheticKey, "claude-opus-5-5", "medium", m.server.URL)
	var progress []string
	turn, err := c.Turn(context.Background(), "sys", tools(), []Block{{"role": "user", "content": "x"}}, nil,
		func(s string) { progress = append(progress, s) })
	if err != nil {
		t.Fatal(err)
	}
	if turn.StopReason != "tool_use" || turn.Content[0]["signature"] != "sig-test" ||
		turn.Content[1]["input"].(Block)["command"] != "df -h /" || len(progress) != 1 {
		t.Fatalf("%+v %v", turn, progress)
	}
	req := m.requests[0]
	if req.path != "/v1/messages" || req.headers.Get("authorization") != "Bearer "+syntheticKey ||
		!strings.Contains(req.headers.Get("anthropic-beta"), FallbackBeta) || !strings.Contains(req.headers.Get("anthropic-beta"), UpdatesBeta) {
		t.Error(req.path, req.headers)
	}
	b := req.body
	if b["stream"] != true || b["fallbacks"] != "default" || b["output_config"].(Block)["effort"] != "medium" ||
		b["cache_control"].(Block)["type"] != "ephemeral" || b["thinking"].(Block)["display"] != "updates" {
		t.Error(b)
	}
	if tool := b["tools"].([]any)[0].(Block); tool["eager_input_streaming"] != true || tool["strict"] != true {
		t.Error(tool)
	}
	raw, _ := json.Marshal(b)
	if strings.Contains(string(raw), syntheticKey) {
		t.Error("key in the body")
	}
}

func TestOtherModelsGetNoUnsupportedOptions(t *testing.T) {
	m := newMock(reply{blocks: []Block{{"type": "text", "text": "hi"}}, stop: "end_turn"})
	defer m.server.Close()
	var text strings.Builder
	NewAnthropic(syntheticKey, "claude-haiku-5-5", "low", m.server.URL).Turn(context.Background(), "s", nil,
		[]Block{{"role": "user", "content": "x"}}, func(s string) { text.WriteString(s) }, nil)
	b := m.requests[0].body
	if _, ok := b["fallbacks"]; ok || b["thinking"].(Block)["type"] != "adaptive" || m.requests[0].headers.Get("anthropic-beta") != "" {
		t.Error(b)
	}
	if text.String() != "hi" {
		t.Error(text.String())
	}
}

func TestErrorsBecomeActionableMessages(t *testing.T) {
	for status, want := range map[int]string{401: "rejected", 404: "unknown model", 429: "rate limited", 400: "rejected"} {
		m := newMock(reply{status: status, errMsg: Block{"type": "x", "message": "m"}})
		c := NewAnthropic(syntheticKey, "claude-opus-5-5", "medium", m.server.URL)
		c.MaxRetries = 0
		_, err := c.Turn(context.Background(), "s", nil, []Block{{"role": "user", "content": "x"}}, nil, nil)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%d: %v", status, err)
		}
		m.server.Close()
	}
}

func TestOllamaRequestsAreBareAndErrorsNameTheFix(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:9")
	t.Setenv("ALL_PROXY", "http://127.0.0.1:9")
	m := newMock(reply{blocks: []Block{{"type": "tool_use", "id": "call_1", "name": "run_command", "input": command}}, stop: "tool_use"})
	c := NewOllama("qwen3:8b", m.server.URL)
	turn, err := c.Turn(context.Background(), "sys", tools(), []Block{{"role": "user", "content": "x"}}, nil, nil)
	m.server.Close()
	if err != nil || turn.Content[0]["input"].(Block)["command"] != "df -h /" {
		t.Fatal(turn, err)
	}
	b := m.requests[0].body
	for _, option := range []string{"fallbacks", "thinking", "output_config", "cache_control"} {
		if _, ok := b[option]; ok {
			t.Error(option)
		}
	}
	if m.requests[0].headers.Get("anthropic-beta") != "" || strings.Contains(m.requests[0].headers.Get("authorization"), "sk-ant") {
		t.Error(m.requests[0].headers)
	}
	for _, c := range []struct {
		r    reply
		want string
	}{{reply{status: 404, errMsg: Block{"type": "not_found_error"}}, "ollama pull qwen3:8b"},
		{reply{status: 410, errMsg: Block{"message": "model retired"}}, "Ollama error 410: model retired"},
		{reply{status: 401, errMsg: Block{"type": "authentication_error"}}, "ollama signin"}} {
		m := newMock(c.r)
		o := NewOllama("qwen3:8b", m.server.URL)
		o.MaxRetries = 0
		_, err := o.Turn(context.Background(), "s", nil, []Block{{"role": "user", "content": "x"}}, nil, nil)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v, want %s", err, c.want)
		}
		m.server.Close()
	}
	closed := newMock()
	url := closed.server.URL
	closed.server.Close()
	o := NewOllama("qwen3:8b", url)
	o.MaxRetries = 0
	if _, err := o.Turn(context.Background(), "s", nil, nil, nil, nil); err == nil || !strings.Contains(err.Error(), "ollama serve") {
		t.Error(err)
	}
}

func TestUnreadableToolInputIsReissuedAndRefusalReported(t *testing.T) {
	broken := reply{blocks: []Block{{"type": "tool_use", "id": "t", "name": "run_command", "input": command, "broken": true}}, stop: "tool_use"}
	m := newMock(broken, reply{blocks: []Block{{"type": "text", "text": ""}}, stop: "refusal"})
	defer m.server.Close()
	turn, err := NewAnthropic(syntheticKey, "claude-opus-5-5", "medium", m.server.URL).Turn(context.Background(), "s", nil, nil, nil, nil)
	if err != nil || turn.StopReason != "refusal" || len(m.requests) != 2 {
		t.Error(turn, err, len(m.requests))
	}
}

func TestRedirectsAndDroppedConnections(t *testing.T) { // SECTEST §5, F6
	var hits int
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	defer elsewhere.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+"/stolen", http.StatusFound)
	}))
	defer redirect.Close()
	o := NewOllama("qwen3.5:4b", redirect.URL)
	o.MaxRetries = 0
	if _, err := o.Turn(context.Background(), "s", nil, nil, nil, nil); err == nil {
		t.Error("redirect followed")
	}
	if OllamaRemoteHost(redirect.URL, "qwen3.5:4b") != "" {
		t.Error("remote host from a redirect")
	}
	if hits != 0 {
		t.Error("the other host was contacted")
	}
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			io.ReadAtLeast(conn, make([]byte, 100), 1)
			conn.Write([]byte("HTTP/1.1 200 OK\r\ncontent-type: text/event-stream\r\ncontent-length: 100000\r\n\r\nevent: message_start\ndata: {\"type\": \"message_start\", "))
			conn.Close()
		}
	}()
	defer ln.Close()
	for _, c := range []*Client{NewOllama("q", "http://"+ln.Addr().String()), NewAnthropic(syntheticKey, "claude-haiku-5-5", "low", "http://"+ln.Addr().String())} {
		c.MaxRetries = 0
		_, err := c.Turn(context.Background(), "s", nil, nil, nil, nil)
		if _, ok := err.(*Error); !ok {
			t.Errorf("%s: %T %v", c.Name(), err, err)
		}
	}
}

func TestCompleteAndRemoteHost(t *testing.T) {
	m := newMock(reply{blocks: []Block{{"type": "text", "text": "OK"}}, stop: "end_turn"})
	defer m.server.Close()
	if text, err := NewOllama("q", m.server.URL).Complete(context.Background(), "s", "t"); text != "OK" || err != nil {
		t.Error(text, err)
	}
	if m.requests[0].body["max_tokens"] != float64(CheckMaxTokens) {
		t.Error(m.requests[0].body)
	}
	tags := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"models": [{"name": "llama3.2:latest"}, {"name": "gemma4:31b-cloud", "remote_host": "https://ollama.com:443"}]}`))
	}))
	defer tags.Close()
	if OllamaRemoteHost(tags.URL, "llama3.2") != "" || OllamaRemoteHost(tags.URL, "gemma4:31b-cloud") != "https://ollama.com:443" ||
		OllamaRemoteHost("http://127.0.0.1:9", "x:cloud") != "https://ollama.com" {
		t.Error("remote hosts")
	}
}

func TestEchoContentDropsTheDeclinedModelsBlocks(t *testing.T) {
	content := []Block{{"type": "thinking"}, {"type": "tool_use"}, {"type": "text", "text": "a"}, {"type": "fallback"}, {"type": "tool_use"}}
	got := EchoContent(content)
	if len(got) != 3 || got[0]["type"] != "text" || got[2]["type"] != "tool_use" {
		t.Error(got)
	}
}
