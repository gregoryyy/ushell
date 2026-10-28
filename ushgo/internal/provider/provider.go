// Package provider talks to the Anthropic Messages API, and to a local Ollama through its
// Anthropic-compatible API (ushpy provider.py). The rest of ush sees only Turn and Error.
//
// It is a small client of its own on net/http, not the SDK: streaming with server-sent events,
// tool use, thinking, retries. Ollama requests carry no Anthropic options and no real key, use no
// proxy, and follow no redirect (SECTEST §5).
package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	APIURL         = "https://api.anthropic.com"
	apiVersion     = "2026-10-21"
	ollamaKey      = "ollama" // Ollama ignores the key
	MaxTokens      = 64000
	CheckMaxTokens = 2048 // the checker answers one line; local thinking models need room first
	CheckTimeout   = 60 * time.Second
	FallbackBeta   = "server-side-fallback-2026-07-01"
	UpdatesBeta    = "thinking-display-updates-2026-08-18"
)

// Models that accept server-side refusal fallbacks and between-tool progress notes.
var (
	fallbackModels = map[string]bool{"claude-opus-5-5": true, "claude-opus-5": true, "claude-sonnet-5-5": true,
		"claude-fable-5-1": true, "claude-fable-5": true}
	updatesModels = map[string]bool{"claude-opus-5-5": true, "claude-sonnet-5-5": true,
		"claude-fable-5-1": true, "claude-fable-5": true}
)

// Error is a provider problem, with a message the user can act on.
type Error struct{ Msg string }

func (e *Error) Error() string { return e.Msg }

// Block is a content block, kept as JSON values so it can be echoed back unchanged.
type Block = map[string]any

// Turn is one reply of the model.
type Turn struct {
	Content    []Block
	StopReason string
	Refusal    string
	Usage      map[string]any
}

// Provider is what the agent needs.
type Provider interface {
	Name() string
	Model() string
	Turn(ctx context.Context, system string, tools []Block, messages []Block, onText, onProgress func(string)) (Turn, error)
	Complete(ctx context.Context, system, text string) (string, error)
}

// Client is the Anthropic or Ollama provider.
type Client struct {
	name, model, effort, base, key string
	ollama                         bool
	http                           *http.Client
	MaxRetries                     int
}

func dialer() *net.Dialer { return &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second} }

func noRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// NewAnthropic: base "" is the Anthropic API; proxies from the environment are used, as by the SDK.
func NewAnthropic(key, model, effort, base string) *Client {
	if base == "" {
		base = APIURL
	}
	tr := &http.Transport{Proxy: http.ProxyFromEnvironment, DialContext: dialer().DialContext,
		TLSHandshakeTimeout: 30 * time.Second, ForceAttemptHTTP2: true}
	return &Client{name: "Anthropic", model: model, effort: effort, base: strings.TrimRight(base, "/"),
		key: key, http: &http.Client{Transport: tr, CheckRedirect: noRedirect}, MaxRetries: 2}
}

// NewOllama: a local Ollama; no proxy, no redirect, no Anthropic options.
func NewOllama(model, base string) *Client {
	tr := &http.Transport{Proxy: nil, DialContext: dialer().DialContext}
	return &Client{name: "Ollama", model: model, base: strings.TrimRight(base, "/"), key: ollamaKey,
		ollama: true, http: &http.Client{Transport: tr, CheckRedirect: noRedirect}, MaxRetries: 2}
}

func (c *Client) Name() string  { return c.name }
func (c *Client) Model() string { return c.model }

func (c *Client) params(system string, tools []Block, messages []Block) (Block, []string) {
	if c.ollama {
		return Block{"model": c.model, "max_tokens": MaxTokens, "system": system, "tools": nonNil(tools),
			"messages": messages}, nil
	}
	var betas []string
	eager := make([]Block, 0, len(tools))
	for _, t := range tools {
		copied := Block{}
		for k, v := range t {
			copied[k] = v
		}
		copied["eager_input_streaming"] = true
		eager = append(eager, copied)
	}
	p := Block{"model": c.model, "max_tokens": MaxTokens, "system": system, "tools": eager,
		"messages": messages, "cache_control": Block{"type": "ephemeral"},
		"output_config": Block{"effort": c.effort}}
	if updatesModels[c.model] {
		p["thinking"] = Block{"type": "adaptive", "display": "updates"}
		betas = append(betas, UpdatesBeta)
	} else {
		p["thinking"] = Block{"type": "adaptive"}
	}
	if fallbackModels[c.model] {
		p["fallbacks"] = "default"
		betas = append(betas, FallbackBeta)
	}
	return p, betas
}

func nonNil(tools []Block) []Block {
	if tools == nil {
		return []Block{}
	}
	return tools
}

// post sends a request, retrying connection failures and 408/409/429/5xx like the SDK.
func (c *Client) post(ctx context.Context, body Block, betas []string) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", c.base+"/v1/messages", bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		req.Header.Set("content-type", "application/json")
		req.Header.Set("accept", "application/json")
		req.Header.Set("authorization", "Bearer "+c.key)
		req.Header.Set("anthropic-version", apiVersion)
		req.Header.Set("user-agent", "ush-go")
		if len(betas) > 0 {
			req.Header.Set("anthropic-beta", strings.Join(betas, ","))
		}
		resp, err := c.http.Do(req)
		retry := false
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr, retry = c.connectionError(), true
		} else if resp.StatusCode >= 300 {
			retry = resp.StatusCode == 408 || resp.StatusCode == 409 || resp.StatusCode == 429 || resp.StatusCode >= 500
			lastErr = c.statusError(resp)
			resp.Body.Close()
			if !retry {
				return nil, lastErr
			}
		} else {
			return resp, nil
		}
		if !retry || attempt >= c.MaxRetries {
			return nil, lastErr
		}
		delay := time.Duration(math.Min(8, 0.5*math.Pow(2, float64(attempt))) * float64(time.Second))
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *Client) connectionError() error {
	if c.ollama {
		return &Error{fmt.Sprintf("cannot reach Ollama at %s; start it with `ollama serve`", c.base)}
	}
	return &Error{"cannot reach the provider; check the network"}
}

func (c *Client) statusError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var body struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &body)
	message := body.Error.Message
	if message == "" {
		message = fmt.Sprintf("Error code: %d - %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	code := resp.StatusCode
	if c.ollama {
		switch code {
		case 404:
			return &Error{fmt.Sprintf("Ollama has no model %s; run `ollama pull %s` or choose one from `ollama list`", pyRepr(c.model), c.model)}
		case 401:
			return &Error{fmt.Sprintf("Ollama refused %s; cloud models need `ollama signin`", pyRepr(c.model))}
		}
		return &Error{fmt.Sprintf("Ollama error %d: %s", code, message)}
	}
	switch code {
	case 401:
		return &Error{"the API key was rejected; run `ush ai configure`"}
	case 403:
		return &Error{fmt.Sprintf("the API key may not use %s: %s", c.model, message)}
	case 404:
		return &Error{fmt.Sprintf("unknown model %s; run `ush ai configure`", pyRepr(c.model))}
	case 429:
		return &Error{"rate limited by the provider; try again shortly"}
	case 400:
		return &Error{"the provider rejected the request: " + message}
	}
	return &Error{fmt.Sprintf("provider error %d: %s", code, message)}
}

// Turn streams one reply. Tool input that can't be parsed re-issues the turn, up to three times.
func (c *Client) Turn(ctx context.Context, system string, tools []Block, messages []Block, onText, onProgress func(string)) (Turn, error) {
	params, betas := c.params(system, tools, messages)
	params["stream"] = true
	for range 3 {
		turn, err := c.stream(ctx, params, betas, onText, onProgress)
		if errors.Is(err, errUnreadableInput) {
			continue
		}
		return turn, err
	}
	return Turn{}, &Error{"the model produced unreadable tool input three times"}
}

var errUnreadableInput = errors.New("unreadable tool input")

func (c *Client) stream(ctx context.Context, params Block, betas []string, onText, onProgress func(string)) (Turn, error) {
	resp, err := c.post(ctx, params, betas)
	if err != nil {
		return Turn{}, err
	}
	defer resp.Body.Close()
	var turn Turn
	var blocks []Block
	partial := map[int]*strings.Builder{}
	reader := bufio.NewReaderSize(resp.Body, 1<<16)
	var event, data string
	for {
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			if errors.Is(err, io.EOF) {
				break
			}
			if ctx.Err() != nil {
				return Turn{}, ctx.Err()
			}
			return Turn{}, &Error{"the connection to the provider failed while reading its reply"}
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(line[6:])
			continue
		case strings.HasPrefix(line, "data:"):
			data += strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")
			continue
		case line != "":
			continue
		}
		if data == "" {
			continue
		}
		var ev Block
		if json.Unmarshal([]byte(data), &ev) != nil {
			return Turn{}, &Error{"the provider sent an unreadable event"}
		}
		data = ""
		kind, _ := ev["type"].(string)
		if kind == "" {
			kind = event
		}
		switch kind {
		case "message_start":
			if m, ok := ev["message"].(Block); ok {
				turn.Usage, _ = m["usage"].(Block)
			}
		case "content_block_start":
			idx := index(ev)
			for len(blocks) <= idx {
				blocks = append(blocks, nil)
			}
			if cb, ok := ev["content_block"].(Block); ok {
				blocks[idx] = cb
			}
		case "content_block_delta":
			idx := index(ev)
			if idx >= len(blocks) || blocks[idx] == nil {
				continue
			}
			b, d := blocks[idx], ev["delta"].(Block)
			switch d["type"] {
			case "text_delta":
				text, _ := d["text"].(string)
				b["text"] = str(b["text"]) + text
				if onText != nil && text != "" {
					onText(text)
				}
			case "thinking_delta":
				b["thinking"] = str(b["thinking"]) + str(d["thinking"])
			case "signature_delta":
				b["signature"] = str(d["signature"])
			case "input_json_delta":
				if partial[idx] == nil {
					partial[idx] = &strings.Builder{}
				}
				partial[idx].WriteString(str(d["partial_json"]))
			}
		case "content_block_stop":
			idx := index(ev)
			if idx >= len(blocks) || blocks[idx] == nil {
				continue
			}
			b := blocks[idx]
			if p := partial[idx]; p != nil {
				var input any
				raw := strings.TrimSpace(p.String())
				if raw == "" {
					input = Block{}
				} else if json.Unmarshal([]byte(raw), &input) != nil {
					return Turn{}, errUnreadableInput
				}
				b["input"] = input
			}
			if b["type"] == "thinking" && strings.TrimSpace(str(b["thinking"])) != "" && onProgress != nil {
				onProgress(str(b["thinking"]))
			}
		case "message_delta":
			if d, ok := ev["delta"].(Block); ok {
				turn.StopReason = str(d["stop_reason"])
				if details, ok := d["stop_details"].(Block); ok {
					turn.Refusal = str(details["explanation"])
					if turn.Refusal == "" {
						turn.Refusal = str(details["category"])
					}
				}
			}
			if u, ok := ev["usage"].(Block); ok {
				if turn.Usage == nil {
					turn.Usage = Block{}
				}
				for k, v := range u {
					turn.Usage[k] = v
				}
			}
		case "error":
			e, _ := ev["error"].(Block)
			return Turn{}, &Error{"provider error: " + str(e["message"])}
		}
	}
	for _, b := range blocks {
		if b != nil {
			turn.Content = append(turn.Content, b)
		}
	}
	if turn.StopReason == "" {
		return Turn{}, &Error{"the connection to the provider failed while reading its reply"}
	}
	return turn, nil
}

// Complete is one short answer without tools, thinking, or streaming: the checker's request.
func (c *Client) Complete(ctx context.Context, system, text string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, CheckTimeout)
	defer cancel()
	resp, err := c.post(ctx, Block{"model": c.model, "max_tokens": CheckMaxTokens, "system": system,
		"messages": []Block{{"role": "user", "content": text}}}, nil)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", &Error{"the provider did not answer in time"}
		}
		return "", err
	}
	defer resp.Body.Close()
	var message struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&message); err != nil {
		return "", &Error{"the connection to the provider failed while reading its reply"}
	}
	var out strings.Builder
	for _, b := range message.Content {
		if b.Type == "text" {
			out.WriteString(b.Text)
		}
	}
	return out.String(), nil
}

// OllamaRemoteHost is where Ollama sends requests for model: its cloud host, or "" for a local
// model. A cloud tag counts as remote even if the list says otherwise or Ollama can't be asked.
func OllamaRemoteHost(base, model string) string {
	name, tag, _ := strings.Cut(model, ":")
	guess := ""
	if strings.Contains(tag, "cloud") || strings.HasSuffix(name, "-cloud") {
		guess = "https://ollama.com"
	}
	models, err := OllamaTags(base, 2*time.Second)
	if err != nil {
		return guess
	}
	for _, m := range models {
		if m.Name == model || m.Name == model+":latest" {
			if m.RemoteHost != "" {
				return m.RemoteHost
			}
			return guess
		}
	}
	return guess
}

// Tag is one entry of Ollama's model list.
type Tag struct {
	Name       string `json:"name"`
	RemoteHost string `json:"remote_host"`
}

// OllamaTags lists the models of a local Ollama: no proxy, no redirect.
func OllamaTags(base string, timeout time.Duration) ([]Tag, error) {
	client := &http.Client{Timeout: timeout, CheckRedirect: noRedirect,
		Transport: &http.Transport{Proxy: nil, DialContext: dialer().DialContext}}
	resp, err := client.Get(strings.TrimRight(base, "/") + "/api/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var list struct {
		Models []Tag `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}
	return list.Models, nil
}

// EchoContent prepares an assistant turn for the next request: after a mid-output fallback,
// thinking and tool_use blocks before the last fallback block belong to the declined model.
func EchoContent(content []Block) []Block {
	last := -1
	for i, b := range content {
		if b["type"] == "fallback" {
			last = i
		}
	}
	if last < 0 {
		return content
	}
	drop := map[string]bool{"thinking": true, "redacted_thinking": true, "tool_use": true, "server_tool_use": true}
	var out []Block
	for i, b := range content {
		if i >= last || !drop[str(b["type"])] {
			out = append(out, b)
		}
	}
	return out
}

func index(ev Block) int {
	f, _ := ev["index"].(float64)
	return int(f)
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func pyRepr(s string) string {
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		return `"` + s + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `\'`) + "'"
}

var _ = strconv.Itoa
