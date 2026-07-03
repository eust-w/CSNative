package proxy

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"csnative/internal/config"
)

func TestQwenModelMappingAndAnthropicToOpenAI(t *testing.T) {
	p := Providers["qwen"]
	if got := p.ResolveModel("claude-opus-4-8"); got != "qwen3.7-max" {
		t.Fatalf("opus mapped to %q", got)
	}
	if got := p.ResolveModel("claude-sonnet-5-20260101"); got != "qwen-plus-latest" {
		t.Fatalf("sonnet mapped to %q", got)
	}
	out, err := AnthropicToOpenAI(p, MessageRequest{
		Model:      "claude-haiku-4-5",
		Messages:   []AnthropicMessage{{Role: "user", Content: "hi"}},
		Tools:      []AnthropicTool{{Name: "grade", InputSchema: map[string]any{"type": "object"}}},
		ToolChoice: map[string]any{"type": "tool", "name": "grade"},
		MaxTokens:  100000,
	})
	if err != nil {
		t.Fatalf("AnthropicToOpenAI() error = %v", err)
	}
	if out.Model != "qwen-turbo" || out.MaxTokens != 8192 {
		t.Fatalf("bad qwen request: %#v", out)
	}
	if got := out.ToolChoice.(map[string]any)["function"].(map[string]any)["name"]; got != "grade" {
		t.Fatalf("tool choice function = %v", got)
	}
}

func TestProviderFromProfileBuildsAdapterModes(t *testing.T) {
	deepseek := config.BuiltinProviders()["deepseek"]
	if got := ProviderFromProfile(deepseek); got.Mode != ModeAnthropic || got.URL == "" {
		t.Fatalf("deepseek profile did not build anthropic provider: %#v", got)
	}
	openrouter := config.BuiltinProviders()["openrouter"]
	got := ProviderFromProfile(openrouter)
	if got.Mode != ModeOpenAI || got.ResolveModel("claude-sonnet-4-6") != openrouter.DefaultModel {
		t.Fatalf("openrouter profile did not build openai provider: %#v", got)
	}
}

func TestProxyAuthAndUpstream401Passthrough(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"nope"}`, http.StatusUnauthorized)
	}))
	defer up.Close()
	srv := NewServer(ServerConfig{Provider: "deepseek", Key: "fake", Secret: "sec", UpstreamOverride: up.URL})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Post(ts.URL+"/v1/messages", "application/json", strings.NewReader(`{"messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("without secret status = %d", res.StatusCode)
	}
	res.Body.Close()

	res, err = http.Post(ts.URL+"/sec/v1/messages", "application/json", strings.NewReader(`{"model":"claude-opus-4-8","max_tokens":1,"messages":[{"role":"user","content":"ping"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("upstream 401 became %d body=%s", res.StatusCode, body)
	}
}

func TestOpenAIStreamReplayAndBlockedConnect(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"choices": []map[string]any{{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": "ok"}}},
			"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer up.Close()
	srv := NewServer(ServerConfig{Provider: "qwen", Key: "fake", Secret: "sec", UpstreamOverride: up.URL})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Post(ts.URL+"/sec/v1/messages", "application/json", strings.NewReader(`{"model":"claude-opus-4-8","stream":true,"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("event: message_start")) || !bytes.Contains(body, []byte("event: message_stop")) {
		t.Fatalf("bad SSE replay status=%d body=%s", res.StatusCode, body)
	}

	conn, err := net.Dial("tcp", strings.TrimPrefix(ts.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = conn.Write([]byte("CONNECT claude.ai:443 HTTP/1.1\r\nHost: claude.ai:443\r\n\r\n"))
	buf := make([]byte, 256)
	n, _ := conn.Read(buf)
	if !bytes.Contains(buf[:n], []byte("403")) {
		t.Fatalf("unauthorized CONNECT did not fast-fail 403: %q", buf[:n])
	}

	conn, err = net.Dial("tcp", strings.TrimPrefix(ts.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	auth := base64.StdEncoding.EncodeToString([]byte("csnative:sec"))
	_, _ = conn.Write([]byte("CONNECT claude.ai:443 HTTP/1.1\r\nHost: claude.ai:443\r\nProxy-Authorization: Basic " + auth + "\r\n\r\n"))
	n, _ = conn.Read(buf)
	if !bytes.Contains(buf[:n], []byte("401")) {
		t.Fatalf("blocked CONNECT did not fast-fail 401: %q", buf[:n])
	}
}

func TestConnectRequiresProxyAuthorizationSecret(t *testing.T) {
	up, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer up.Close()
	go func() {
		conn, err := up.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()
	srv := NewServer(ServerConfig{Provider: "deepseek", Key: "fake", Secret: "sec"})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	conn, err := net.Dial("tcp", strings.TrimPrefix(ts.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = conn.Write([]byte("CONNECT " + up.Addr().String() + " HTTP/1.1\r\nHost: " + up.Addr().String() + "\r\n\r\n"))
	buf := make([]byte, 256)
	n, _ := conn.Read(buf)
	_ = conn.Close()
	if !bytes.Contains(buf[:n], []byte("403")) {
		t.Fatalf("unauthorized CONNECT status = %q", buf[:n])
	}

	conn, err = net.Dial("tcp", strings.TrimPrefix(ts.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	auth := base64.StdEncoding.EncodeToString([]byte("csnative:sec"))
	_, _ = conn.Write([]byte("CONNECT " + up.Addr().String() + " HTTP/1.1\r\nHost: " + up.Addr().String() + "\r\nProxy-Authorization: Basic " + auth + "\r\n\r\n"))
	n, _ = conn.Read(buf)
	_ = conn.Close()
	if !bytes.Contains(buf[:n], []byte("200 Connection Established")) {
		t.Fatalf("authorized CONNECT status = %q", buf[:n])
	}
}

func TestOpenAIStreamReplayToolCallsDoNotEmitNullText(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-tools",
			"choices": []map[string]any{{
				"finish_reason": "tool_calls",
				"message": map[string]any{
					"role":    "assistant",
					"content": "",
					"tool_calls": []map[string]any{
						{"id": "call_a", "type": "function", "function": map[string]any{"name": "search", "arguments": `{"query":"a"}`}},
						{"id": "call_b", "type": "function", "function": map[string]any{"name": "search", "arguments": `{"query":"b"}`}},
					},
				},
			}},
			"usage": map[string]any{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer up.Close()
	srv := NewServer(ServerConfig{Provider: "qwen", Key: "fake", Secret: "sec", UpstreamOverride: up.URL})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Post(ts.URL+"/sec/v1/messages", "application/json", strings.NewReader(`{"model":"claude-opus-4-8","stream":true,"messages":[{"role":"user","content":"use tools"}],"tools":[{"name":"search","input_schema":{"type":"object"}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("bad status=%d body=%s", res.StatusCode, body)
	}
	if bytes.Contains(body, []byte(`"text":null`)) || bytes.Contains(body, []byte(`text_delta`)) {
		t.Fatalf("tool call SSE leaked null text delta: %s", body)
	}
	if !bytes.Contains(body, []byte(`"type":"tool_use"`)) || !bytes.Contains(body, []byte(`input_json_delta`)) {
		t.Fatalf("tool call SSE missing tool_use events: %s", body)
	}
}

func TestOpenAIUnknownToolCallIsDowngradedToText(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-unknown-tool",
			"choices": []map[string]any{{
				"finish_reason": "tool_calls",
				"message": map[string]any{
					"role":    "assistant",
					"content": "",
					"tool_calls": []map[string]any{
						{"id": "call_web", "type": "function", "function": map[string]any{"name": "web_search", "arguments": `{"query":"protein folding"}`}},
					},
				},
			}},
			"usage": map[string]any{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer up.Close()
	srv := NewServer(ServerConfig{Provider: "qwen", Key: "fake", Secret: "sec", UpstreamOverride: up.URL})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := `{"model":"claude-opus-4-8","stream":true,"messages":[{"role":"user","content":"search"}],"tools":[{"name":"search_openalex","input_schema":{"type":"object"}}]}`
	res, err := http.Post(ts.URL+"/sec/v1/messages", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("bad status=%d body=%s", res.StatusCode, data)
	}
	if bytes.Contains(data, []byte(`"type":"tool_use"`)) || bytes.Contains(data, []byte(`input_json_delta`)) {
		t.Fatalf("unknown tool call reached Science as tool_use: %s", data)
	}
	if !bytes.Contains(data, []byte(`web_search`)) || !bytes.Contains(data, []byte(`未注册`)) {
		t.Fatalf("unknown tool call was not explained as text: %s", data)
	}
}

func TestOpenAIWebSearchToolCallIsBlockedEvenWhenAdvertised(t *testing.T) {
	resp := OpenAIResponse{ID: "chatcmpl-web-search"}
	resp.Choices = append(resp.Choices, struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	}{FinishReason: "tool_calls"})
	resp.Choices[0].Message.ToolCalls = append(resp.Choices[0].Message.ToolCalls, struct {
		ID       string `json:"id"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}{ID: "call_web"})
	resp.Choices[0].Message.ToolCalls[0].Function.Name = "web_search"
	resp.Choices[0].Message.ToolCalls[0].Function.Arguments = `{"query":"protein folding"}`

	msg := OpenAIToAnthropic(resp, "claude-opus-4-8", map[string]AnthropicTool{"web_search": {Name: "web_search"}})
	if got := msg["stop_reason"]; got != "end_turn" {
		t.Fatalf("blocked tool call should end turn, got %v", got)
	}
	blocks, ok := msg["content"].([]map[string]any)
	if !ok || len(blocks) != 1 {
		t.Fatalf("unexpected content blocks: %#v", msg["content"])
	}
	if got := blocks[0]["type"]; got != "text" {
		t.Fatalf("blocked tool call reached Science as %v", got)
	}
	text, _ := blocks[0]["text"].(string)
	if !strings.Contains(text, "web_search") || !strings.Contains(text, "拦截") {
		t.Fatalf("blocked tool call was not explained: %q", text)
	}
}

func TestOpenAIToolCallWithInvalidSchemaArgsIsDowngradedToText(t *testing.T) {
	resp := OpenAIResponse{ID: "chatcmpl-invalid-args"}
	resp.Choices = append(resp.Choices, struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	}{FinishReason: "tool_calls"})
	resp.Choices[0].Message.ToolCalls = append(resp.Choices[0].Message.ToolCalls, struct {
		ID       string `json:"id"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}{ID: "call_plan"})
	resp.Choices[0].Message.ToolCalls[0].Function.Name = "generate_review_plan"
	resp.Choices[0].Message.ToolCalls[0].Function.Arguments = `{"task_summary":"enzyme review","phases":[]}`

	tools := map[string]AnthropicTool{
		"generate_review_plan": {
			Name: "generate_review_plan",
			InputSchema: map[string]any{
				"type":     "object",
				"required": []any{"task_summary", "phases"},
				"properties": map[string]any{
					"task_summary": map[string]any{"type": "string", "minLength": 1},
					"phases":       map[string]any{"type": "array", "minItems": 1},
				},
			},
		},
	}
	msg := OpenAIToAnthropic(resp, "claude-opus-4-8", tools)
	if got := msg["stop_reason"]; got != "end_turn" {
		t.Fatalf("invalid tool args should end turn, got %v", got)
	}
	blocks := msg["content"].([]map[string]any)
	if got := blocks[0]["type"]; got != "text" {
		t.Fatalf("invalid tool args reached Science as %v", got)
	}
	text, _ := blocks[0]["text"].(string)
	if !strings.Contains(text, "generate_review_plan") || !strings.Contains(text, "phases") {
		t.Fatalf("invalid tool args were not explained: %q", text)
	}
}
