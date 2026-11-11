package provider

// OpenAI through its Responses API (TODO 2.29.6), on net/http (ushpy responses.py).
//
// The agent keeps Anthropic-shaped messages (ARCH); this adapter translates at its edge: the system
// prompt becomes `instructions`, tool_use a function_call item, tool_result a function_call_output,
// and the reply's items become content blocks again. Requests are stateless (store: false): the
// reasoning items of a turn are kept in the conversation as `reasoning` blocks, with their
// encrypted content, and sent back with the next request, as OpenAI asks for tool calls. The
// finished reply is read from response.completed; the deltas only stream text to the terminal.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gregoryyy/ushell/src/go/internal/providers"
)

const (
	OpenAIURL    = providers.OpenAIURL
	responseWait = 600 * time.Second // longest wait for response headers
	maxWait      = 20.0              // longest wait for a retry, whatever retry-after says
)

// OpenAI is the Responses API adapter. What a request carries comes from the model's profile.
type OpenAI struct {
	model, effort, base, key string
	profile                  providers.Profile
	http                     *http.Client
	MaxRetries               int
}

// NewOpenAI: base "" is the OpenAI API. Proxies from the environment are used; redirects are not
// followed: a request goes where the settings say or nowhere.
func NewOpenAI(key, model, effort, base string) *OpenAI {
	if base == "" {
		base = OpenAIURL
	}
	return &OpenAI{model: model, effort: effort, base: strings.TrimRight(base, "/"), key: key,
		profile:    providers.ProfileOf("openai", model),
		http:       openAIClient(responseWait),
		MaxRetries: 2}
}

func openAIClient(headerWait time.Duration) *http.Client {
	tr := &http.Transport{Proxy: http.ProxyFromEnvironment, DialContext: dialer().DialContext,
		TLSHandshakeTimeout: 30 * time.Second, ResponseHeaderTimeout: headerWait, ForceAttemptHTTP2: true}
	return &http.Client{Transport: tr, CheckRedirect: noRedirect}
}

func (o *OpenAI) Name() string  { return "OpenAI" }
func (o *OpenAI) Model() string { return o.model }

// toInput is the conversation as Responses input items.
func toInput(messages []Block) []Block {
	items := []Block{}
	for _, m := range messages {
		blocks := blocksOf(m["content"])
		if m["role"] == "user" {
			var texts []any
			flush := func() {
				if len(texts) > 0 {
					items = append(items, Block{"type": "message", "role": "user", "content": texts})
					texts = nil
				}
			}
			for _, b := range blocks {
				switch b["type"] {
				case "text":
					texts = append(texts, Block{"type": "input_text", "text": str(b["text"])})
				case "tool_result":
					flush()
					items = append(items, Block{"type": "function_call_output", "call_id": b["tool_use_id"],
						"output": resultText(b["content"])})
				}
			}
			flush()
			continue
		}
		for _, b := range blocks {
			switch b["type"] {
			case "text":
				if text := str(b["text"]); text != "" {
					item := Block{"type": "message", "role": "assistant",
						"content": []any{Block{"type": "output_text", "text": text}}}
					if id := str(b["openai_id"]); id != "" {
						item["id"] = id
					}
					items = append(items, item)
				}
			case "tool_use":
				input := b["input"]
				if input == nil || isEmpty(input) {
					input = Block{}
				}
				item := Block{"type": "function_call", "call_id": b["id"], "name": b["name"], "arguments": compactJSON(input)}
				if id := str(b["openai_id"]); id != "" {
					item["id"] = id
				}
				items = append(items, item)
			case "reasoning":
				if item, ok := b["item"].(map[string]any); ok {
					items = append(items, item)
				}
			}
			// Blocks of another provider (thinking, fallback) don't go to OpenAI.
		}
	}
	return items
}

// blocksOf: a message's content as blocks; a string is one text block.
func blocksOf(content any) []Block {
	switch c := content.(type) {
	case string:
		return []Block{{"type": "text", "text": c}}
	case []Block:
		return c
	case []any:
		var out []Block
		for _, b := range c {
			if m, ok := b.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

func isEmpty(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		return len(x) == 0
	case []any:
		return len(x) == 0
	case string:
		return x == ""
	case bool:
		return !x
	case float64:
		return x == 0
	}
	return false
}

func compactJSON(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if enc.Encode(v) != nil {
		return "{}"
	}
	return strings.TrimRight(buf.String(), "\n")
}

func resultText(content any) string {
	switch c := content.(type) {
	case string:
		return c
	case []Block, []any:
		var out strings.Builder
		for _, b := range blocksOf(c) {
			out.WriteString(str(b["text"]))
		}
		return out.String()
	}
	return ""
}

var errUnreadableArguments = errors.New("unreadable tool arguments")

// toBlocks is the reply's items as content blocks, its refusal text, and its reasoning summaries.
func toBlocks(output any) ([]Block, string, []string, error) {
	var blocks []Block
	refusal := ""
	var summaries []string
	items, _ := output.([]any)
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		switch str(item["type"]) {
		case "reasoning":
			keep := Block{}
			for k, v := range item {
				if k != "status" {
					keep[k] = v
				}
			}
			blocks = append(blocks, Block{"type": "reasoning", "item": keep})
			parts, _ := item["summary"].([]any)
			for _, p := range parts {
				if m, ok := p.(map[string]any); ok && str(m["text"]) != "" {
					summaries = append(summaries, str(m["text"]))
				}
			}
		case "message":
			parts, _ := item["content"].([]any)
			for _, p := range parts {
				part, _ := p.(map[string]any)
				switch part["type"] {
				case "output_text":
					blocks = append(blocks, Block{"type": "text", "text": str(part["text"]), "openai_id": str(item["id"])})
				case "refusal":
					refusal += str(part["refusal"])
				}
			}
		case "function_call":
			raw := str(item["arguments"])
			if raw == "" {
				raw = "{}"
			}
			var arguments any
			if json.Unmarshal([]byte(raw), &arguments) != nil {
				return nil, "", nil, errUnreadableArguments
			}
			blocks = append(blocks, Block{"type": "tool_use", "id": item["call_id"], "name": item["name"],
				"input": arguments, "openai_id": str(item["id"])})
		}
	}
	return blocks, refusal, summaries, nil
}

func (o *OpenAI) body(system string, tools, messages []Block) Block {
	p := o.profile
	body := Block{"model": o.model, "instructions": system, "input": toInput(messages), "store": false, "stream": true}
	if len(tools) > 0 { // a research request (TODO 2.38) has none
		sent := []Block{}
		for _, t := range tools {
			strict := p.StrictTools && t["strict"] == true
			description := str(t["description"])
			sent = append(sent, Block{"type": "function", "name": t["name"], "description": description,
				"parameters": t["input_schema"], "strict": strict})
		}
		body["tools"] = sent
	}
	if p.Effort {
		body["reasoning"] = Block{"effort": o.effort}
		body["include"] = []string{"reasoning.encrypted_content"}
	}
	if p.OutputLimit > 0 {
		body["max_output_tokens"] = p.OutputLimit
	}
	return body
}

// Turn streams one reply. Tool arguments that can't be parsed re-issue the turn, up to three times.
func (o *OpenAI) Turn(ctx context.Context, system string, tools, messages []Block, onText, onProgress func(string)) (Turn, error) {
	body := o.body(system, tools, messages)
	for range 3 {
		response, err := o.stream(ctx, body, onText)
		if err != nil {
			return Turn{}, err
		}
		blocks, refusal, summaries, err := toBlocks(response["output"])
		if errors.Is(err, errUnreadableArguments) {
			continue
		}
		if onProgress != nil {
			for _, text := range summaries {
				onProgress(text)
			}
		}
		return Turn{Content: blocks, StopReason: stopReason(response, blocks, refusal), Refusal: refusal,
			Usage: usage(response)}, nil
	}
	return Turn{}, &Error{"the model produced unreadable tool input three times"}
}

// Complete is one short answer without tools or streaming: the checker's request.
func (o *OpenAI) Complete(ctx context.Context, system, text string) (string, error) {
	body := Block{"model": o.model, "instructions": system, "store": false,
		"input": []Block{{"type": "message", "role": "user",
			"content": []any{Block{"type": "input_text", "text": text}}}},
		"max_output_tokens": CheckMaxTokens}
	if o.profile.Effort {
		body["reasoning"] = Block{"effort": "low"}
	}
	resp, err := o.post(ctx, body, CheckTimeout)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", &Error{"the connection to the provider failed while reading its reply"}
	}
	response, err := parseJSON(raw)
	if err != nil {
		return "", err
	}
	blocks, _, _, _ := toBlocks(response["output"])
	var out strings.Builder
	for _, b := range blocks {
		if b["type"] == "text" {
			out.WriteString(str(b["text"]))
		}
	}
	return out.String(), nil
}

// post sends a request, retrying connection failures and 408/409/429/5xx, after retry-after
// (at most 20 s) or 0.5 * 2^n seconds. timeout bounds the whole exchange when it is not 0.
func (o *OpenAI) post(ctx context.Context, body Block, timeout time.Duration) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	accept := "application/json"
	if body["stream"] == true {
		accept = "text/event-stream"
	}
	for attempt := 0; ; attempt++ {
		reqCtx, cancel := ctx, context.CancelFunc(func() {})
		if timeout > 0 {
			reqCtx, cancel = context.WithTimeout(ctx, timeout)
		}
		req, err := http.NewRequestWithContext(reqCtx, "POST", o.base+"/v1/responses", bytes.NewReader(data))
		if err != nil {
			cancel()
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+o.key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", accept)
		req.Header.Set("User-Agent", "ush-go")
		resp, err := o.http.Do(req)
		var wait float64
		if err != nil {
			cancel()
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if attempt >= o.MaxRetries {
				return nil, &Error{"cannot reach OpenAI; check the network"}
			}
			wait = retryWait("", attempt)
		} else if resp.StatusCode/100 == 2 {
			resp.Body = &cancelBody{resp.Body, cancel}
			return resp, nil
		} else {
			code := resp.StatusCode
			message := httpMessage(resp)
			header := resp.Header.Get("Retry-After")
			resp.Body.Close()
			cancel()
			if !retryable(code) || attempt >= o.MaxRetries {
				return nil, o.statusError(code, message)
			}
			wait = retryWait(header, attempt)
		}
		select {
		case <-time.After(time.Duration(wait * float64(time.Second))):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

type cancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelBody) Close() error { err := c.ReadCloser.Close(); c.cancel(); return err }

func retryable(code int) bool {
	switch code {
	case 408, 409, 429, 500, 502, 503, 504:
		return true
	}
	return false
}

func retryWait(retryAfter string, attempt int) float64 {
	if v, err := strconv.ParseFloat(strings.TrimSpace(retryAfter), 64); err == nil && !math.IsNaN(v) {
		return math.Min(maxWait, math.Max(0, v))
	}
	return math.Min(maxWait, 0.5*math.Pow(2, float64(attempt)))
}

func httpMessage(resp *http.Response) string {
	reason := strings.TrimSpace(strings.TrimPrefix(resp.Status, strconv.Itoa(resp.StatusCode)))
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var body struct {
		Error any `json:"error"`
	}
	if json.Unmarshal(raw, &body) != nil {
		return reason
	}
	if e, ok := body.Error.(map[string]any); ok {
		if m := str(e["message"]); m != "" {
			return m
		}
	}
	return reason
}

func (o *OpenAI) statusError(code int, message string) error {
	switch code {
	case 401:
		return &Error{"the API key was rejected; run `ush ai configure`"}
	case 403:
		return &Error{"the API key may not use " + o.model + ": " + message}
	case 404:
		return &Error{"unknown model " + pyRepr(o.model) + "; run `ush ai configure`"}
	case 429:
		return &Error{"rate limited by the provider; try again shortly"}
	case 400:
		return &Error{"the provider rejected the request: " + message}
	}
	return &Error{"provider error " + strconv.Itoa(code) + ": " + message}
}

// stream posts the request and reads its events; it returns the finished response object.
func (o *OpenAI) stream(ctx context.Context, body Block, onText func(string)) (Block, error) {
	resp, err := o.post(ctx, body, 0)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	reader := bufio.NewReaderSize(resp.Body, 1<<16)
	var data []string
	handle := func(text string) (Block, bool, error) {
		if text == "[DONE]" {
			return nil, false, nil
		}
		event, err := parseJSON([]byte(text))
		if err != nil {
			return nil, false, err
		}
		switch str(event["type"]) {
		case "response.output_text.delta":
			if onText != nil {
				onText(str(event["delta"]))
			}
		case "response.completed", "response.incomplete":
			response, _ := event["response"].(map[string]any)
			if response == nil {
				response = Block{}
			}
			return response, true, nil
		case "response.failed":
			response, _ := event["response"].(map[string]any)
			e, _ := response["error"].(map[string]any)
			message := str(e["message"])
			if message == "" {
				message = "failed"
			}
			return nil, false, &Error{"OpenAI error: " + message}
		case "error":
			message := str(event["message"])
			if message == "" {
				message = pyDict(event)
			}
			return nil, false, &Error{"OpenAI error: " + message}
		}
		return nil, false, nil
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			if errors.Is(err, io.EOF) {
				break
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, &Error{"the connection to the provider failed while reading its reply"}
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimLeft(line[5:], " \t"))
		case line == "" && len(data) > 0:
			text := strings.Join(data, "\n")
			data = nil
			if response, done, err := handle(text); err != nil || done {
				return response, err
			}
		}
	}
	if len(data) > 0 {
		if response, done, err := handle(strings.Join(data, "\n")); err != nil || done {
			return response, err
		}
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return nil, &Error{"the connection to the provider ended before the reply did"}
}

// pyDict renders an event as JSON for an error event without a message.
func pyDict(event Block) string { return compactJSON(event) }

func parseJSON(raw []byte) (Block, error) {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return nil, &Error{"OpenAI sent a reply that is not JSON"}
	}
	m, _ := value.(map[string]any)
	if m == nil {
		m = Block{}
	}
	return m, nil
}

func stopReason(response Block, blocks []Block, refusal string) string {
	for _, b := range blocks {
		if b["type"] == "tool_use" {
			return "tool_use"
		}
	}
	if refusal != "" {
		return "refusal"
	}
	details, _ := response["incomplete_details"].(map[string]any)
	if response["status"] == "incomplete" {
		if details["reason"] == "content_filter" {
			return "refusal"
		}
		return "max_tokens"
	}
	return "end_turn"
}

func usage(response Block) map[string]any {
	u, _ := response["usage"].(map[string]any)
	out := map[string]any{}
	for _, k := range []string{"input_tokens", "output_tokens"} {
		if v, ok := u[k].(float64); ok && v == math.Trunc(v) {
			out[k] = int(v)
		}
	}
	return out
}

// ListModels is OpenAI's models that answer in text, newest first; nil if they can't be listed.
func ListModels(ctx context.Context, base, key string, timeout time.Duration) []string {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(base, "/")+"/v1/models", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("User-Agent", "ush-go")
	resp, err := openAIClient(timeout).Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil
	}
	var list struct {
		Data []map[string]any `json:"data"`
	}
	if json.NewDecoder(resp.Body).Decode(&list) != nil {
		return nil
	}
	type model struct {
		id      string
		created float64
	}
	var models []model
	for _, m := range list.Data {
		id, ok := m["id"].(string)
		if !ok || !ChatModel(id) {
			continue
		}
		created := 0.0
		if v, ok := m["created"].(float64); ok && v == math.Trunc(v) {
			created = v
		}
		models = append(models, model{id, created})
	}
	sort.SliceStable(models, func(i, j int) bool {
		if models[i].created != models[j].created {
			return models[i].created > models[j].created
		}
		return models[i].id < models[j].id
	})
	var ids []string
	for _, m := range models {
		ids = append(ids, m.id)
	}
	return ids
}

var notChat = []string{"embedding", "tts", "whisper", "dall-e", "image", "audio", "realtime", "transcribe",
	"moderation", "search", "instruct", "babbage", "davinci", "sora", "computer-use"}

// ChatModel is whether a model of OpenAI's list is one ush can talk to: a GPT or o-series text model.
func ChatModel(name string) bool {
	lower := strings.ToLower(name)
	if !(strings.HasPrefix(lower, "gpt-") || (len(lower) > 1 && lower[0] == 'o' && lower[1] >= '0' && lower[1] <= '9')) {
		return false
	}
	for _, word := range notChat {
		if strings.Contains(lower, word) {
			return false
		}
	}
	return true
}
