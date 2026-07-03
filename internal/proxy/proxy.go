package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"csnative/internal/config"
)

type Mode string

const (
	ModeAnthropic Mode = "anthropic"
	ModeOpenAI    Mode = "openai"
)

type Provider struct {
	Name         string
	Mode         Mode
	URL          string
	KeyEnv       string
	Models       []Model
	ModelMap     map[string]string
	ModelCaps    map[string]int
	DefaultCap   int
	DefaultModel string
}

type Model struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

var Providers = map[string]Provider{
	"deepseek": {
		Name:   "deepseek",
		Mode:   ModeAnthropic,
		URL:    "https://api.deepseek.com/anthropic/v1/messages",
		KeyEnv: "DEEPSEEK_API_KEY",
		Models: []Model{
			{ID: "claude-opus-4-8", DisplayName: "DeepSeek V4 Pro"},
			{ID: "claude-haiku-4-5", DisplayName: "DeepSeek V4 Flash"},
		},
		ModelMap: map[string]string{
			"claude-opus-4-8":   "deepseek-v4-pro",
			"claude-sonnet-5":   "deepseek-v4-flash",
			"claude-sonnet-4-6": "deepseek-v4-flash",
			"claude-haiku-4-5":  "deepseek-v4-flash",
		},
		ModelCaps:    map[string]int{"deepseek-v4-pro": 65536, "deepseek-v4-flash": 32768},
		DefaultCap:   8192,
		DefaultModel: "deepseek-v4-flash",
	},
	"qwen": {
		Name:   "qwen",
		Mode:   ModeOpenAI,
		URL:    "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
		KeyEnv: "DASHSCOPE_API_KEY",
		Models: []Model{
			{ID: "claude-opus-4-8", DisplayName: "Qwen 3.7 Max"},
			{ID: "claude-sonnet-4-6", DisplayName: "Qwen Plus Latest"},
			{ID: "claude-haiku-4-5", DisplayName: "Qwen Turbo"},
		},
		ModelMap: map[string]string{
			"claude-opus-4-8":   "qwen3.7-max",
			"claude-sonnet-5":   "qwen-plus-latest",
			"claude-sonnet-4-6": "qwen-plus-latest",
			"claude-haiku-4-5":  "qwen-turbo",
		},
		ModelCaps:    map[string]int{"qwen3.7-max": 8192, "qwen-plus-latest": 8192, "qwen-turbo": 8192},
		DefaultCap:   8192,
		DefaultModel: "qwen-plus-latest",
	},
}

func (p Provider) ResolveModel(name string) string {
	if name == "" {
		return p.DefaultModel
	}
	if v := p.ModelMap[name]; v != "" {
		return v
	}
	for _, m := range p.Models {
		if name == m.ID {
			return name
		}
	}
	stripped := stripDateSuffix(name)
	if v := p.ModelMap[stripped]; v != "" {
		return v
	}
	for k, v := range p.ModelMap {
		if strings.HasPrefix(name, k) || strings.HasPrefix(stripped, k) {
			return v
		}
	}
	return p.DefaultModel
}

func (p Provider) ClampMaxTokens(v int, model string) int {
	if v == 0 {
		return v
	}
	cap := p.ModelCaps[model]
	if cap == 0 {
		cap = p.DefaultCap
	}
	if cap > 0 && v > cap {
		return cap
	}
	return v
}

type ServerConfig struct {
	Provider         string
	Profile          config.ProviderProfile
	Key              string
	Secret           string
	UpstreamOverride string
	Logger           *log.Logger
}

type Server struct {
	cfg      ServerConfig
	provider Provider
	client   *http.Client
	srv      *http.Server
	mu       sync.Mutex
}

func NewServer(cfg ServerConfig) *Server {
	var p Provider
	if cfg.Profile.ID != "" {
		p = ProviderFromProfile(cfg.Profile)
	} else {
		var ok bool
		p, ok = Providers[cfg.Provider]
		if !ok {
			p = Providers["deepseek"]
		}
	}
	if cfg.UpstreamOverride != "" {
		p.URL = cfg.UpstreamOverride
	}
	return &Server{
		cfg:      cfg,
		provider: p,
		client:   &http.Client{Timeout: 5 * time.Minute},
	}
}

func ProviderFromProfile(profile config.ProviderProfile) Provider {
	mode := ModeOpenAI
	if profile.Adapter == "anthropic_messages" {
		mode = ModeAnthropic
	}
	models := make([]Model, 0, len(profile.Models))
	for _, model := range profile.Models {
		models = append(models, Model{ID: model.ID, DisplayName: model.DisplayName})
	}
	return Provider{
		Name:         nonempty(profile.ID, profile.DisplayName),
		Mode:         mode,
		URL:          profile.BaseURL,
		KeyEnv:       "",
		Models:       models,
		ModelMap:     cloneStringMap(profile.ModelMap),
		ModelCaps:    cloneIntMap(profile.MaxTokensCap),
		DefaultCap:   8192,
		DefaultModel: profile.DefaultModel,
	}
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneIntMap(in map[string]int) map[string]int {
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (s *Server) ListenAndServe(addr string) error {
	s.srv = &http.Server{Addr: addr, Handler: s.Handler()}
	return s.srv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			if !s.auth(w, r) {
				return
			}
			s.handleConnect(w, r)
			return
		}
		if !s.auth(w, r) {
			return
		}
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/health"):
			writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "provider": s.provider.Name})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/models"):
			data := make([]map[string]any, 0, len(s.provider.Models))
			for _, m := range s.provider.Models {
				data = append(data, map[string]any{"type": "model", "id": m.ID, "display_name": m.DisplayName, "created_at": "2026-01-01T00:00:00Z"})
			}
			first, last := "", ""
			if len(data) > 0 {
				first, _ = data[0]["id"].(string)
				last, _ = data[len(data)-1]["id"].(string)
			}
			writeJSON(w, http.StatusOK, map[string]any{"data": data, "has_more": false, "first_id": first, "last_id": last})
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/messages"):
			s.handleMessages(w, r)
		default:
			writeJSON(w, http.StatusNotFound, apiErr("not_found_error", r.URL.Path))
		}
	})
}

func (s *Server) auth(w http.ResponseWriter, r *http.Request) bool {
	if s.cfg.Secret == "" {
		return true
	}
	if r.Method == http.MethodConnect && s.proxyAuthorizationOK(r) {
		return true
	}
	prefix := "/" + s.cfg.Secret
	if r.URL.Path == prefix || strings.HasPrefix(r.URL.Path, prefix+"/") {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
		return true
	}
	w.Header().Set("Connection", "close")
	writeJSON(w, http.StatusForbidden, apiErr("permission_error", "forbidden"))
	return false
}

func (s *Server) proxyAuthorizationOK(r *http.Request) bool {
	header := strings.TrimSpace(r.Header.Get("Proxy-Authorization"))
	if header == "" {
		return false
	}
	scheme, encoded, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "basic") {
		return false
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return false
	}
	user, pass, ok := strings.Cut(string(raw), ":")
	if !ok {
		return false
	}
	return user == s.cfg.Secret || pass == s.cfg.Secret
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req MessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiErr("invalid_request_error", err.Error()))
		return
	}
	if req.Messages == nil {
		writeJSON(w, http.StatusBadRequest, apiErr("invalid_request_error", "request body must contain messages"))
		return
	}
	if s.provider.Mode == ModeAnthropic {
		s.handleAnthropic(w, req)
	} else {
		s.handleOpenAI(w, req)
	}
}

func (s *Server) handleAnthropic(w http.ResponseWriter, req MessageRequest) {
	body := req
	target := s.provider.ResolveModel(req.Model)
	body.Model = target
	body.MaxTokens = s.provider.ClampMaxTokens(req.MaxTokens, target)
	data, _ := json.Marshal(body)
	h := map[string]string{"x-api-key": s.cfg.Key, "content-type": "application/json", "anthropic-version": "2023-06-01"}
	resp, err := s.post(data, h)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, apiErr("api_error", err.Error()))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		forwardUpstreamError(w, resp)
		return
	}
	copyResponse(w, resp)
}

func (s *Server) handleOpenAI(w http.ResponseWriter, req MessageRequest) {
	oreq, err := AnthropicToOpenAI(s.provider, req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiErr("invalid_request_error", err.Error()))
		return
	}
	data, _ := json.Marshal(oreq)
	resp, err := s.post(data, map[string]string{"Authorization": "Bearer " + s.cfg.Key, "Content-Type": "application/json"})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, apiErr("api_error", err.Error()))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		forwardUpstreamError(w, resp)
		return
	}
	var ores OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&ores); err != nil {
		writeJSON(w, http.StatusBadGateway, apiErr("api_error", err.Error()))
		return
	}
	aresp := OpenAIToAnthropic(ores, req.Model, allowedToolSet(req.Tools))
	if req.Stream {
		replaySSE(w, aresp)
	} else {
		writeJSON(w, http.StatusOK, aresp)
	}
}

func (s *Server) post(data []byte, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, s.provider.URL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return s.client.Do(req)
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	host, _, _ := net.SplitHostPort(r.Host)
	if host == "" {
		host = strings.Split(r.Host, ":")[0]
	}
	if blockedHost(host) {
		w.Header().Set("Content-Length", "0")
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking unsupported", http.StatusInternalServerError)
		return
	}
	client, rw, err := hj.Hijack()
	if err != nil {
		return
	}
	up, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		_, _ = rw.WriteString("HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n")
		_ = rw.Flush()
		_ = client.Close()
		return
	}
	_, _ = rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
	_ = rw.Flush()
	go func() { _, _ = io.Copy(up, rw); _ = up.Close() }()
	go func() { _, _ = io.Copy(rw, up); _ = client.Close() }()
}

func blockedHost(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	return host == "claude.ai" || host == "api.anthropic.com" || strings.HasSuffix(host, ".anthropic.com") || strings.HasSuffix(host, ".claude.com")
}

func forwardUpstreamError(w http.ResponseWriter, resp *http.Response) {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 400))
	code := http.StatusBadGateway
	if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 429 {
		code = resp.StatusCode
	}
	writeJSON(w, code, apiErr("api_error", fmt.Sprintf("upstream %d: %s", resp.StatusCode, string(b))))
}

func copyResponse(w http.ResponseWriter, resp *http.Response) {
	for k, vals := range resp.Header {
		if strings.EqualFold(k, "Content-Length") {
			continue
		}
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func apiErr(kind, msg string) map[string]any {
	return map[string]any{"type": "error", "error": map[string]any{"type": kind, "message": msg}}
}

func stripDateSuffix(s string) string {
	parts := strings.Split(s, "-")
	if len(parts) < 4 {
		return s
	}
	last := parts[len(parts)-1]
	if len(last) == 8 {
		for _, r := range last {
			if r < '0' || r > '9' {
				return s
			}
		}
		return strings.Join(parts[:len(parts)-1], "-")
	}
	return s
}

type MessageRequest struct {
	Model         string             `json:"model,omitempty"`
	MaxTokens     int                `json:"max_tokens,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
	System        any                `json:"system,omitempty"`
	Messages      []AnthropicMessage `json:"messages"`
	Tools         []AnthropicTool    `json:"tools,omitempty"`
	ToolChoice    map[string]any     `json:"tool_choice,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	Temperature   *float64           `json:"temperature,omitempty"`
	Thinking      any                `json:"thinking,omitempty"`
}

type AnthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type AnthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Stream      bool            `json:"stream"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	Tools       []any           `json:"tools,omitempty"`
	ToolChoice  any             `json:"tool_choice,omitempty"`
	Stop        []string        `json:"stop,omitempty"`
}

type OpenAIMessage struct {
	Role       string `json:"role"`
	Content    any    `json:"content,omitempty"`
	ToolCalls  []any  `json:"tool_calls,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

func AnthropicToOpenAI(p Provider, req MessageRequest) (OpenAIRequest, error) {
	var msgs []OpenAIMessage
	if req.System != nil {
		if s := systemText(req.System); s != "" {
			msgs = append(msgs, OpenAIMessage{Role: "system", Content: s})
		}
	}
	for _, m := range req.Messages {
		msgs = append(msgs, convertMessage(m)...)
	}
	out := OpenAIRequest{Model: p.ResolveModel(req.Model), Messages: msgs, Stream: false}
	out.MaxTokens = p.ClampMaxTokens(req.MaxTokens, out.Model)
	out.Temperature = req.Temperature
	out.TopP = req.TopP
	if len(req.Tools) > 0 {
		for _, t := range req.Tools {
			if t.Name == "" {
				continue
			}
			out.Tools = append(out.Tools, map[string]any{"type": "function", "function": map[string]any{"name": t.Name, "description": t.Description, "parameters": t.InputSchema}})
		}
	}
	out.ToolChoice = mapToolChoice(req.ToolChoice, req.Tools)
	out.Stop = req.StopSequences
	return out, nil
}

func systemText(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		var b strings.Builder
		for _, it := range x {
			if m, ok := it.(map[string]any); ok {
				if t, _ := m["text"].(string); t != "" {
					if b.Len() > 0 {
						b.WriteByte('\n')
					}
					b.WriteString(t)
				}
			}
		}
		return b.String()
	default:
		return ""
	}
}

func convertMessage(m AnthropicMessage) []OpenAIMessage {
	if s, ok := m.Content.(string); ok {
		return []OpenAIMessage{{Role: m.Role, Content: s}}
	}
	arr, ok := m.Content.([]any)
	if !ok {
		return []OpenAIMessage{{Role: m.Role, Content: fmt.Sprint(m.Content)}}
	}
	var text []string
	var toolCalls []any
	var results []OpenAIMessage
	for _, item := range arr {
		blk, ok := item.(map[string]any)
		if !ok {
			continue
		}
		switch blk["type"] {
		case "text":
			if t, _ := blk["text"].(string); t != "" {
				text = append(text, t)
			}
		case "tool_use":
			name, _ := blk["name"].(string)
			id, _ := blk["id"].(string)
			args, _ := json.Marshal(blk["input"])
			toolCalls = append(toolCalls, map[string]any{"id": id, "type": "function", "function": map[string]any{"name": name, "arguments": string(args)}})
		case "tool_result":
			id, _ := blk["tool_use_id"].(string)
			results = append(results, OpenAIMessage{Role: "tool", ToolCallID: id, Content: blockText(blk["content"])})
		}
	}
	if len(results) > 0 {
		if len(text) > 0 {
			results = append(results, OpenAIMessage{Role: m.Role, Content: strings.Join(text, "")})
		}
		return results
	}
	if len(toolCalls) > 0 {
		return []OpenAIMessage{{Role: "assistant", Content: strings.Join(text, ""), ToolCalls: toolCalls}}
	}
	return []OpenAIMessage{{Role: m.Role, Content: strings.Join(text, "")}}
}

func blockText(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		var b strings.Builder
		for _, it := range x {
			if m, ok := it.(map[string]any); ok {
				if t, _ := m["text"].(string); t != "" {
					b.WriteString(t)
				}
			}
		}
		return b.String()
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

func mapToolChoice(tc map[string]any, tools []AnthropicTool) any {
	if tc == nil {
		return nil
	}
	t, _ := tc["type"].(string)
	switch t {
	case "auto", "none":
		return t
	case "tool":
		name, _ := tc["name"].(string)
		if name != "" {
			return map[string]any{"type": "function", "function": map[string]any{"name": name}}
		}
	case "any":
		var names []string
		for _, tool := range tools {
			if tool.Name != "" {
				names = append(names, tool.Name)
			}
		}
		if len(names) == 1 {
			return map[string]any{"type": "function", "function": map[string]any{"name": names[0]}}
		}
		return "required"
	}
	return nil
}

type OpenAIResponse struct {
	ID      string `json:"id"`
	Choices []struct {
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
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func OpenAIToAnthropic(resp OpenAIResponse, model string, allowedTools ...map[string]AnthropicTool) map[string]any {
	var blocks []map[string]any
	var emittedToolCall bool
	var rejectedTools []string
	if len(resp.Choices) > 0 {
		msg := resp.Choices[0].Message
		if msg.Content != "" {
			blocks = append(blocks, map[string]any{"type": "text", "text": msg.Content})
		}
		for _, tc := range msg.ToolCalls {
			if !toolNameAllowed(tc.Function.Name, allowedTools...) {
				rejectedTools = append(rejectedTools, tc.Function.Name)
				continue
			}
			var args map[string]any
			if strings.TrimSpace(tc.Function.Arguments) != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					rejectedTools = append(rejectedTools, fmt.Sprintf("%s: 参数不是有效 JSON", tc.Function.Name))
					continue
				}
			}
			if args == nil {
				args = map[string]any{}
			}
			if issue := validateToolInput(tc.Function.Name, args, allowedTools...); issue != "" {
				rejectedTools = append(rejectedTools, fmt.Sprintf("%s: %s", tc.Function.Name, issue))
				continue
			}
			blocks = append(blocks, map[string]any{"type": "tool_use", "id": tc.ID, "name": tc.Function.Name, "input": args})
			emittedToolCall = true
		}
	}
	if len(rejectedTools) > 0 {
		blocks = append(blocks, map[string]any{"type": "text", "text": fmt.Sprintf("模型尝试调用未注册或参数不符合要求的工具 %s，已被 CS Native 拦截。请改用当前 Science 会话可用工具，或用 Python/OpenAlex 等已授权路径完成。", strings.Join(uniqueNonempty(rejectedTools), ", "))})
	}
	if len(blocks) == 0 {
		blocks = []map[string]any{{"type": "text", "text": ""}}
	}
	stop := "end_turn"
	if len(resp.Choices) > 0 {
		switch resp.Choices[0].FinishReason {
		case "length":
			stop = "max_tokens"
		case "tool_calls":
			if emittedToolCall {
				stop = "tool_use"
			}
		}
	}
	return map[string]any{"id": nonempty(resp.ID, "msg_proxy"), "type": "message", "role": "assistant", "model": model, "content": blocks, "stop_reason": stop, "stop_sequence": nil, "usage": map[string]int{"input_tokens": resp.Usage.PromptTokens, "output_tokens": resp.Usage.CompletionTokens}}
}

func allowedToolSet(tools []AnthropicTool) map[string]AnthropicTool {
	out := make(map[string]AnthropicTool, len(tools))
	for _, tool := range tools {
		if tool.Name != "" {
			out[tool.Name] = tool
		}
	}
	return out
}

func toolNameAllowed(name string, allowedTools ...map[string]AnthropicTool) bool {
	if blockedToolName(name) {
		return false
	}
	if len(allowedTools) == 0 || allowedTools[0] == nil {
		return true
	}
	_, ok := allowedTools[0][name]
	return ok
}

func validateToolInput(name string, args map[string]any, allowedTools ...map[string]AnthropicTool) string {
	if len(allowedTools) == 0 || allowedTools[0] == nil {
		return ""
	}
	tool, ok := allowedTools[0][name]
	if !ok || len(tool.InputSchema) == 0 {
		return ""
	}
	return validateObjectAgainstSchema(args, tool.InputSchema)
}

func validateObjectAgainstSchema(args map[string]any, schema map[string]any) string {
	props, _ := schema["properties"].(map[string]any)
	for _, field := range stringList(schema["required"]) {
		value, ok := args[field]
		if !ok || emptyRequiredValue(value) {
			return fmt.Sprintf("%s required", field)
		}
	}
	for field, rawSchema := range props {
		value, ok := args[field]
		if !ok {
			continue
		}
		propSchema, _ := rawSchema.(map[string]any)
		if issue := validateValueAgainstSchema(field, value, propSchema); issue != "" {
			return issue
		}
	}
	return ""
}

func validateValueAgainstSchema(field string, value any, schema map[string]any) string {
	if minItems, ok := numberValue(schema["minItems"]); ok {
		items, ok := value.([]any)
		if !ok {
			return fmt.Sprintf("%s must be an array", field)
		}
		if float64(len(items)) < minItems {
			return fmt.Sprintf("%s must have at least %.0f item(s)", field, minItems)
		}
	}
	if minLength, ok := numberValue(schema["minLength"]); ok {
		text, ok := value.(string)
		if !ok {
			return fmt.Sprintf("%s must be a string", field)
		}
		if float64(len(strings.TrimSpace(text))) < minLength {
			return fmt.Sprintf("%s must have at least %.0f character(s)", field, minLength)
		}
	}
	if nestedRequired := stringList(schema["required"]); len(nestedRequired) > 0 {
		obj, ok := value.(map[string]any)
		if !ok {
			return fmt.Sprintf("%s must be an object", field)
		}
		if issue := validateObjectAgainstSchema(obj, schema); issue != "" {
			return field + "." + issue
		}
	}
	return ""
}

func stringList(v any) []string {
	switch values := v.(type) {
	case []string:
		return values
	case []any:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if s, ok := value.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func emptyRequiredValue(v any) bool {
	switch value := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(value) == ""
	case []any:
		return len(value) == 0
	case map[string]any:
		return len(value) == 0
	default:
		return false
	}
}

func numberValue(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func blockedToolName(name string) bool {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "web_search":
		return true
	default:
		return false
	}
}

func uniqueNonempty(values []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, value := range values {
		if value == "" {
			value = "<empty>"
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func replaySSE(w http.ResponseWriter, msg map[string]any) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, _ := w.(http.Flusher)
	events := []struct {
		name string
		data any
	}{
		{"message_start", map[string]any{"type": "message_start", "message": map[string]any{"id": msg["id"], "type": "message", "role": "assistant", "model": msg["model"], "content": []any{}, "stop_reason": nil, "stop_sequence": nil, "usage": msg["usage"]}}},
		{"ping", map[string]any{"type": "ping"}},
	}
	for _, e := range events {
		writeSSE(w, e.name, e.data)
	}
	blocks := messageBlocks(msg["content"])
	for i, blk := range blocks {
		switch blk["type"] {
		case "tool_use":
			writeSSE(w, "content_block_start", map[string]any{"type": "content_block_start", "index": i, "content_block": map[string]any{"type": "tool_use", "id": nonemptyString(blk["id"], fmt.Sprintf("toolu_%d", i)), "name": nonemptyString(blk["name"], "tool"), "input": map[string]any{}}})
			input, _ := json.Marshal(nonNil(blk["input"], map[string]any{}))
			writeSSE(w, "content_block_delta", map[string]any{"type": "content_block_delta", "index": i, "delta": map[string]any{"type": "input_json_delta", "partial_json": string(input)}})
		default:
			writeSSE(w, "content_block_start", map[string]any{"type": "content_block_start", "index": i, "content_block": map[string]any{"type": "text", "text": ""}})
			if text, _ := blk["text"].(string); text != "" {
				writeSSE(w, "content_block_delta", map[string]any{"type": "content_block_delta", "index": i, "delta": map[string]any{"type": "text_delta", "text": text}})
			}
		}
		writeSSE(w, "content_block_stop", map[string]any{"type": "content_block_stop", "index": i})
	}
	writeSSE(w, "message_delta", map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": msg["stop_reason"], "stop_sequence": nil}, "usage": map[string]any{"output_tokens": 0}})
	writeSSE(w, "message_stop", map[string]any{"type": "message_stop"})
	if flusher != nil {
		flusher.Flush()
	}
}

func writeSSE(w io.Writer, event string, data any) {
	b, _ := json.Marshal(data)
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
}

func messageBlocks(v any) []map[string]any {
	switch blocks := v.(type) {
	case []map[string]any:
		return blocks
	case []any:
		out := make([]map[string]any, 0, len(blocks))
		for _, block := range blocks {
			if m, ok := block.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

func nonNil(v any, fallback any) any {
	if v == nil {
		return fallback
	}
	return v
}

func nonemptyString(v any, fallback string) string {
	s, _ := v.(string)
	return nonempty(s, fallback)
}

func nonempty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func readLine(r *bufio.Reader) string {
	line, _ := r.ReadString('\n')
	return line
}
