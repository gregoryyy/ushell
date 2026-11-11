package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

var syntheticOpenAIKey = "sk-proj-test-" + strings.Repeat("0", 30) + "SYNTHETIC"

// A stand-in for OpenAI's Responses API like ushpy's tests/mock_openai.py: replies in order are a
// list of output items, an incomplete response, raw events, or an HTTP error.
type oaReply struct {
	output  []Block
	status  string // "" is completed
	reason  string
	events  []Block
	code    int // an HTTP error with message
	message string
	headers map[string]string
}

type oaRequest struct {
	path    string
	headers http.Header
	body    Block
}

type oaMock struct {
	mu       sync.Mutex
	replies  []oaReply
	requests []oaRequest
	models   []Block // GET /v1/models; nil answers 404
	server   *httptest.Server
}

func oaText(t string) Block {
	return Block{"type": "message", "id": "msg_1", "role": "assistant", "status": "completed",
		"content": []any{Block{"type": "output_text", "text": t, "annotations": []any{}}}}
}

func oaCall(name string, arguments any) Block {
	raw, ok := arguments.(string)
	if !ok {
		data, _ := json.Marshal(arguments)
		raw = string(data)
	}
	return Block{"type": "function_call", "id": "fc_1", "call_id": "call_1", "name": name, "arguments": raw, "status": "completed"}
}

func oaReasoning(id, summary string) Block {
	s := []any{}
	if summary != "" {
		s = append(s, Block{"type": "summary_text", "text": summary})
	}
	return Block{"type": "reasoning", "id": id, "encrypted_content": "gAAAA-opaque-" + id, "summary": s, "status": "completed"}
}

func newOpenAIMock(models []Block, replies ...oaReply) *oaMock {
	m := &oaMock{replies: replies, models: models}
	m.server = httptest.NewServer(http.HandlerFunc(m.handle))
	return m
}

func (m *oaMock) json(w http.ResponseWriter, code int, payload any, headers map[string]string) {
	raw, _ := json.Marshal(payload)
	w.Header().Set("content-type", "application/json")
	for k, v := range headers {
		w.Header().Set(k, v)
	}
	w.WriteHeader(code)
	w.Write(raw)
}

func (m *oaMock) handle(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r.Method == "GET" {
		m.requests = append(m.requests, oaRequest{r.URL.Path, r.Header.Clone(), nil})
		if r.URL.Path != "/v1/models" || m.models == nil {
			m.json(w, 404, Block{"error": Block{"message": "not found"}}, nil)
			return
		}
		data := []Block{}
		for _, model := range m.models {
			data = append(data, model)
		}
		m.json(w, 200, Block{"object": "list", "data": data}, nil)
		return
	}
	var body Block
	raw, _ := io.ReadAll(r.Body)
	json.Unmarshal(raw, &body)
	m.requests = append(m.requests, oaRequest{r.URL.Path, r.Header.Clone(), body})
	rp := oaReply{code: 500, message: "unscripted"}
	if len(m.replies) > 0 {
		rp, m.replies = m.replies[0], m.replies[1:]
	}
	if rp.code != 0 {
		m.json(w, rp.code, Block{"error": Block{"message": rp.message, "type": "error"}}, rp.headers)
		return
	}
	events := rp.events
	if events == nil {
		status := rp.status
		if status == "" {
			status = "completed"
		}
		response := Block{"id": "resp_1", "object": "response", "status": status, "model": body["model"],
			"output": rp.output, "usage": Block{"input_tokens": 11, "output_tokens": 7}}
		if status == "incomplete" {
			response["incomplete_details"] = Block{"reason": rp.reason}
		}
		if body["stream"] != true {
			m.json(w, 200, response, nil)
			return
		}
		created := Block{}
		for k, v := range response {
			created[k] = v
		}
		created["output"], created["status"] = []any{}, "in_progress"
		events = []Block{{"type": "response.created", "response": created}}
		for i, item := range rp.output {
			events = append(events, Block{"type": "response.output_item.added", "output_index": i, "item": item})
			parts, _ := item["content"].([]any)
			for _, p := range parts {
				if part := p.(Block); part["type"] == "output_text" {
					text := part["text"].(string)
					cut := min(5, len(text))
					for _, piece := range []string{text[:cut], text[cut:]} {
						if piece != "" {
							events = append(events, Block{"type": "response.output_text.delta", "item_id": item["id"],
								"output_index": i, "content_index": 0, "delta": piece})
						}
					}
				}
			}
			events = append(events, Block{"type": "response.output_item.done", "output_index": i, "item": item})
		}
		kind := "response.completed"
		if status == "incomplete" {
			kind = "response.incomplete"
		}
		events = append(events, Block{"type": kind, "response": response})
	}
	w.Header().Set("content-type", "text/event-stream")
	w.WriteHeader(200)
	for _, event := range events {
		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "event: %v\ndata: %s\n\n", event["type"], data)
		w.(http.Flusher).Flush()
	}
}

func (m *oaMock) close() { m.server.Close() }

var oaProposal = Block{"command": "df -h", "purpose": "free space", "effect": "read", "interactive": false}

var proposeTool = Block{"name": "propose_command", "description": "Propose a command.", "strict": true,
	"input_schema": Block{"type": "object", "properties": Block{"command": Block{"type": "string"}}}}

func openai(url string, effort string) *OpenAI {
	c := NewOpenAI(syntheticOpenAIKey, "gpt-6-luna", effort, url)
	c.MaxRetries = 0
	return c
}

func ask(t *testing.T, p *OpenAI, messages []Block, tools ...Block) (Turn, string, []string, error) {
	t.Helper()
	if messages == nil {
		messages = []Block{{"role": "user", "content": "space?"}}
	}
	var shown strings.Builder
	var progress []string
	turn, err := p.Turn(context.Background(), "SYSTEM", tools, messages,
		func(s string) { shown.WriteString(s) }, func(s string) { progress = append(progress, s) })
	return turn, shown.String(), progress, err
}

func asJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func TestTheConversationBecomesInputItems(t *testing.T) {
	messages := []Block{
		{"role": "user", "content": "free space?"},
		{"role": "assistant", "content": []Block{
			{"type": "thinking", "thinking": "Anthropic's", "signature": "s"},
			{"type": "reasoning", "item": map[string]any{"type": "reasoning", "id": "rs_1", "encrypted_content": "x", "summary": []any{}}},
			{"type": "text", "text": "Checking.", "openai_id": "msg_1"},
			{"type": "tool_use", "id": "call_1", "name": "propose_command", "input": oaProposal, "openai_id": "fc_1"}}},
		{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": "call_1", "content": "exit 0\n/ 12G"},
			map[string]any{"type": "text", "text": "and now?"}}},
	}
	want := []Block{
		{"type": "message", "role": "user", "content": []any{Block{"type": "input_text", "text": "free space?"}}},
		{"type": "reasoning", "id": "rs_1", "encrypted_content": "x", "summary": []any{}},
		{"type": "message", "role": "assistant", "id": "msg_1", "content": []any{Block{"type": "output_text", "text": "Checking."}}},
		{"type": "function_call", "call_id": "call_1", "name": "propose_command", "arguments": asJSON(oaProposal), "id": "fc_1"},
		{"type": "function_call_output", "call_id": "call_1", "output": "exit 0\n/ 12G"},
		{"type": "message", "role": "user", "content": []any{Block{"type": "input_text", "text": "and now?"}}},
	}
	if got := toInput(messages); asJSON(got) != asJSON(want) { // Anthropic's thinking block is not sent
		t.Errorf("%s\n%s", asJSON(got), asJSON(want))
	}
}

func TestAReplyBecomesContentBlocks(t *testing.T) {
	output := []Block{oaReasoning("rs_1", "Looking at disks."), oaText("Here."), oaCall("propose_command", oaProposal)}
	// The mock's items come back through JSON, as from the wire.
	var wire any
	json.Unmarshal([]byte(asJSON(output)), &wire)
	blocks, refusal, summaries, err := toBlocks(wire)
	if err != nil || len(blocks) != 3 || blocks[0]["type"] != "reasoning" || blocks[1]["type"] != "text" || blocks[2]["type"] != "tool_use" {
		t.Fatal(blocks, err)
	}
	if _, has := blocks[0]["item"].(Block)["status"]; has {
		t.Error("status kept in the reasoning item")
	}
	if blocks[2]["id"] != "call_1" || blocks[2]["name"] != "propose_command" || blocks[2]["openai_id"] != "fc_1" ||
		asJSON(blocks[2]["input"]) != asJSON(oaProposal) || blocks[1]["openai_id"] != "msg_1" {
		t.Error(blocks[2])
	}
	if refusal != "" || !reflect.DeepEqual(summaries, []string{"Looking at disks."}) {
		t.Error(refusal, summaries)
	}
}

func TestChatModelsAreToldFromTheOthers(t *testing.T) {
	for _, name := range []string{"gpt-6-astra", "gpt-6.1-terra", "o5-mini", "gpt-6-luna-2026-10-01"} {
		if !ChatModel(name) {
			t.Error(name)
		}
	}
	for _, name := range []string{"text-embedding-4-large", "gpt-6-tts", "whisper-2", "dall-e-4", "gpt-image-2",
		"gpt-6-realtime", "omni-moderation-latest", "babbage-002", "sora-3"} {
		if ChatModel(name) {
			t.Error(name)
		}
	}
}

func TestAStreamedReplyWithAToolCall(t *testing.T) {
	api := newOpenAIMock(nil, oaReply{output: []Block{oaReasoning("rs_1", ""), oaText("Let me check."), oaCall("propose_command", oaProposal)}})
	defer api.close()
	turn, shown, _, err := ask(t, openai(api.server.URL, "medium"), nil, proposeTool)
	if err != nil {
		t.Fatal(err)
	}
	if shown != "Let me check." || turn.StopReason != "tool_use" || asJSON(turn.Content[len(turn.Content)-1]["input"]) != asJSON(oaProposal) {
		t.Error(shown, turn)
	}
	if !reflect.DeepEqual(turn.Usage, map[string]any{"input_tokens": 11, "output_tokens": 7}) {
		t.Error(turn.Usage)
	}
	req := api.requests[0]
	if req.path != "/v1/responses" || req.headers.Get("Authorization") != "Bearer "+syntheticOpenAIKey {
		t.Error(req.path, req.headers)
	}
	b := req.body
	if b["model"] != "gpt-6-luna" || b["instructions"] != "SYSTEM" || b["store"] != false || b["stream"] != true {
		t.Error(b)
	}
	if asJSON(b["reasoning"]) != `{"effort":"medium"}` || asJSON(b["include"]) != `["reasoning.encrypted_content"]` {
		t.Error(b["reasoning"], b["include"])
	}
	if _, has := b["max_output_tokens"]; has {
		t.Error("an output limit is sent")
	}
	tools := b["tools"].([]any)
	tool := tools[0].(map[string]any)
	if len(tools) != 1 || tool["type"] != "function" || tool["name"] != "propose_command" || tool["strict"] != true ||
		asJSON(tool["parameters"]) != asJSON(proposeTool["input_schema"]) {
		t.Error(tool)
	}
}

func TestTheReasoningOfATurnGoesBackWithItsToolResult(t *testing.T) {
	api := newOpenAIMock(nil, oaReply{output: []Block{oaReasoning("rs_9", ""), oaCall("run_command", Block{"command": "df -h"})}},
		oaReply{output: []Block{oaText("12G free.")}})
	defer api.close()
	p := openai(api.server.URL, "medium")
	first, _, _, err := ask(t, p, nil, proposeTool)
	if err != nil {
		t.Fatal(err)
	}
	messages := []Block{{"role": "user", "content": "space?"}, {"role": "assistant", "content": first.Content},
		{"role": "user", "content": []Block{{"type": "tool_result", "tool_use_id": "call_1", "content": "/ 12G"}}}}
	second, shown, _, err := ask(t, p, messages, proposeTool)
	if err != nil || second.StopReason != "end_turn" || shown != "12G free." {
		t.Fatal(second, shown, err)
	}
	sent := api.requests[1].body["input"].([]any)
	var kinds []string
	for _, i := range sent {
		kinds = append(kinds, i.(map[string]any)["type"].(string))
	}
	if !reflect.DeepEqual(kinds, []string{"message", "reasoning", "function_call", "function_call_output"}) {
		t.Fatal(kinds)
	}
	if sent[1].(map[string]any)["encrypted_content"] != "gAAAA-opaque-rs_9" {
		t.Error(sent[1])
	}
}

func TestResearchSendsNoTools(t *testing.T) {
	api := newOpenAIMock(nil, oaReply{output: []Block{oaText("Compare A and B.")}})
	defer api.close()
	if _, _, _, err := ask(t, openai(api.server.URL, "medium"), nil); err != nil {
		t.Fatal(err)
	}
	if _, has := api.requests[0].body["tools"]; has {
		t.Error("tools sent")
	}
}

func TestACutOffReplyAndARefusal(t *testing.T) {
	refused := Block{"type": "message", "id": "msg_2", "role": "assistant", "content": []any{Block{"type": "refusal", "refusal": "No."}}}
	api := newOpenAIMock(nil, oaReply{output: []Block{oaText("Half")}, status: "incomplete", reason: "max_output_tokens"},
		oaReply{output: []Block{refused}})
	defer api.close()
	p := openai(api.server.URL, "medium")
	cut, _, _, _ := ask(t, p, nil)
	no, _, _, _ := ask(t, p, nil)
	if cut.StopReason != "max_tokens" || no.StopReason != "refusal" || no.Refusal != "No." {
		t.Error(cut, no)
	}
}

func TestUnreadableArgumentsAskAgain(t *testing.T) {
	api := newOpenAIMock(nil, oaReply{output: []Block{oaCall("propose_command", "{not json")}},
		oaReply{output: []Block{oaCall("propose_command", oaProposal)}})
	defer api.close()
	turn, _, _, err := ask(t, openai(api.server.URL, "medium"), nil, proposeTool)
	if err != nil || len(api.requests) != 2 || asJSON(turn.Content[0]["input"]) != asJSON(oaProposal) {
		t.Error(len(api.requests), turn, err)
	}
	api3 := newOpenAIMock(nil, oaReply{output: []Block{oaCall("x", "{")}}, oaReply{output: []Block{oaCall("x", "{")}}, oaReply{output: []Block{oaCall("x", "{")}})
	defer api3.close()
	if _, _, _, err := ask(t, openai(api3.server.URL, "medium"), nil, proposeTool); err == nil || !strings.Contains(err.Error(), "unreadable tool input three times") {
		t.Error(err)
	}
}

func TestErrorsNameTheFix(t *testing.T) {
	cases := []struct {
		code     int
		message  string
		expected string
	}{{401, "Incorrect API key", "API key was rejected"}, {404, "The model does not exist", "unknown model 'gpt-6-luna'"},
		{429, "Rate limit", "rate limited"}, {400, "Unsupported value: 'max'", "rejected the request: Unsupported value"},
		{403, "no access", "may not use gpt-6-luna: no access"}, {503, "overloaded", "provider error 503: overloaded"}}
	for _, c := range cases {
		api := newOpenAIMock(nil, oaReply{code: c.code, message: c.message})
		_, _, _, err := ask(t, openai(api.server.URL, "medium"), nil)
		api.close()
		if err == nil || !strings.Contains(err.Error(), c.expected) {
			t.Errorf("%d: %v", c.code, err)
		}
	}
	api := newOpenAIMock(nil, oaReply{events: []Block{{"type": "error", "message": "server_error"}}})
	_, _, _, err := ask(t, openai(api.server.URL, "medium"), nil)
	api.close()
	if err == nil || !strings.Contains(err.Error(), "OpenAI error: server_error") {
		t.Error(err)
	}
	api = newOpenAIMock(nil, oaReply{events: []Block{{"type": "response.failed", "response": Block{"error": Block{"message": "boom"}}}}})
	_, _, _, err = ask(t, openai(api.server.URL, "medium"), nil)
	api.close()
	if err == nil || !strings.Contains(err.Error(), "OpenAI error: boom") {
		t.Error(err)
	}
	api = newOpenAIMock(nil, oaReply{events: []Block{{"type": "response.created", "response": Block{}}}})
	_, _, _, err = ask(t, openai(api.server.URL, "medium"), nil)
	api.close()
	if err == nil || !strings.Contains(err.Error(), "the connection to the provider ended before the reply did") {
		t.Error(err)
	}
	api = newOpenAIMock(nil)
	closed := api.server.URL
	api.close()
	if _, _, _, err := ask(t, openai(closed, "medium"), nil); err == nil || !strings.Contains(err.Error(), "cannot reach OpenAI; check the network") {
		t.Error(err)
	}
}

func TestARateLimitIsRetriedAfterItsWait(t *testing.T) {
	api := newOpenAIMock(nil, oaReply{code: 429, message: "slow down", headers: map[string]string{"retry-after": "0"}},
		oaReply{output: []Block{oaText("ok")}})
	defer api.close()
	p := openai(api.server.URL, "low")
	p.MaxRetries = 1
	_, shown, _, err := ask(t, p, nil)
	if err != nil || len(api.requests) != 2 || shown != "ok" {
		t.Error(len(api.requests), shown, err)
	}
}

func TestRetryWaits(t *testing.T) {
	for _, c := range []struct {
		header  string
		attempt int
		want    float64
	}{{"3", 0, 3}, {"500", 0, 20}, {"-4", 0, 0}, {"", 0, 0.5}, {"", 2, 2}, {"", 9, 20}, {"Wed, 21 Oct 2026 07:28:00 GMT", 1, 1}} {
		if got := retryWait(c.header, c.attempt); got != c.want {
			t.Errorf("%q %d: %v", c.header, c.attempt, got)
		}
	}
}

func TestTheCheckerGetsOneUnstreamedAnswer(t *testing.T) {
	api := newOpenAIMock(nil, oaReply{output: []Block{oaText("OK")}})
	defer api.close()
	answer, err := openai(api.server.URL, "medium").Complete(context.Background(), "judge", "rm -rf build")
	if err != nil || answer != "OK" {
		t.Fatal(answer, err)
	}
	b := api.requests[0].body
	if _, has := b["stream"]; has || asJSON(b["reasoning"]) != `{"effort":"low"}` || b["max_output_tokens"] != float64(CheckMaxTokens) {
		t.Error(b)
	}
	if api.requests[0].headers.Get("Accept") != "application/json" {
		t.Error(api.requests[0].headers)
	}
}

func TestModelsAreListedNewestFirstWithoutTheOthers(t *testing.T) {
	api := newOpenAIMock([]Block{{"id": "gpt-6-luna", "created": 100}, {"id": "text-embedding-4", "created": 300},
		{"id": "gpt-6-astra", "created": 200}, {"id": "dall-e-4", "created": 400}})
	defer api.close()
	got := ListModels(context.Background(), api.server.URL, syntheticOpenAIKey, 5*time.Second)
	if !reflect.DeepEqual(got, []string{"gpt-6-astra", "gpt-6-luna"}) {
		t.Error(got)
	}
	if api.requests[0].headers.Get("Authorization") != "Bearer "+syntheticOpenAIKey {
		t.Error(api.requests[0].headers)
	}
	none := newOpenAIMock(nil)
	defer none.close()
	if got := ListModels(context.Background(), none.server.URL, syntheticOpenAIKey, 5*time.Second); len(got) != 0 {
		t.Error(got)
	}
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	target := newOpenAIMock(nil, oaReply{output: []Block{oaText("x")}})
	defer target.close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.server.URL+"/v1/responses", http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()
	if _, _, _, err := ask(t, openai(redirector.URL, "medium"), nil); err == nil || len(target.requests) != 0 {
		t.Error(err, len(target.requests))
	}
}

// The Anthropic request carries no OpenAI-only blocks or keys (for_messages_api).
func TestAnthropicDropsOpenAIBlocks(t *testing.T) {
	messages := []Block{{"role": "user", "content": "hi"},
		{"role": "assistant", "content": []any{
			map[string]any{"type": "reasoning", "item": map[string]any{"id": "rs_1"}},
			map[string]any{"type": "text", "text": "ok", "openai_id": "msg_1"},
			map[string]any{"type": "tool_use", "id": "c", "name": "n", "input": map[string]any{}, "openai_id": "fc_1"}}}}
	got := asJSON(forMessagesAPI(messages))
	if strings.Contains(got, "reasoning") || strings.Contains(got, "openai_id") || !strings.Contains(got, `"text":"ok"`) || !strings.Contains(got, `"hi"`) {
		t.Error(got)
	}
	if strings.Contains(asJSON(messages), "openai_id") == false {
		t.Error("the conversation itself was changed")
	}
	plain := []Block{{"role": "user", "content": "hi"}}
	if asJSON(forMessagesAPI(plain)) != asJSON(plain) {
		t.Error("plain conversation changed")
	}
}
