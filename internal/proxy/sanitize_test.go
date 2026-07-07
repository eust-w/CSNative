package proxy

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSanitizeDropsSchemalessTools(t *testing.T) {
	req := MessageRequest{
		Tools: []AnthropicTool{
			{Name: "web_search"},
			{Name: "bash", InputSchema: map[string]any{"type": "object"}},
			{Name: "python", InputSchema: map[string]any{"type": "object"}},
		},
	}
	out := (&Server{}).sanitizeAnthropicRequest(req)
	if len(out.Tools) != 2 {
		t.Fatalf("want 2 tools, got %d: %+v", len(out.Tools), out.Tools)
	}
	for _, tool := range out.Tools {
		if tool.Name == "web_search" {
			t.Errorf("web_search should have been dropped")
		}
	}
}

func TestSanitizeReplacesImageBlocks(t *testing.T) {
	req := MessageRequest{
		Messages: []AnthropicMessage{
			{Role: "user", Content: []any{
				map[string]any{"type": "text", "text": "look at this"},
				map[string]any{"type": "image", "source": map[string]any{"type": "base64", "data": "..."}},
				map[string]any{"type": "text", "text": "ok"},
			}},
		},
	}
	out := (&Server{}).sanitizeAnthropicRequest(req)
	blocks := out.Messages[0].Content.([]any)
	if got := blocks[1].(map[string]any); got["type"] != "text" || got["text"] != "[image omitted]" {
		t.Errorf("image block not replaced with placeholder: %v", got)
	}
	if blocks[0].(map[string]any)["text"] != "look at this" {
		t.Errorf("text block changed: %v", blocks[0])
	}
}

func TestCopySSEResponseDedupesMessageStart(t *testing.T) {
	// Simulate a relay that emits message_start twice with the same id.
	input := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"m1"}}`,
		"",
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"m1"}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":0}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`,
		"",
		"event: content_block_stop",
		`data: {"type":"content_block_stop","index":0}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
		"",
	}, "\n")

	resp := &http.Response{
		Header: http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:   io.NopCloser(strings.NewReader(input)),
	}

	var buf bytes.Buffer
	rec := &sseRecorder{w: &buf}
	copySSEResponse(rec, resp)

	counts := map[string]int{}
	for _, line := range strings.Split(buf.String(), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "event:") {
			counts[strings.TrimSpace(strings.TrimPrefix(line, "event:"))]++
		}
	}
	if counts["message_start"] != 1 {
		t.Errorf("message_start count = %d, want 1 (counts=%v)", counts["message_start"], counts)
	}
	if counts["message_stop"] != 1 {
		t.Errorf("message_stop count = %d, want 1", counts["message_stop"])
	}
	if counts["content_block_delta"] != 1 {
		t.Errorf("content_block_delta should pass through, got %d", counts["content_block_delta"])
	}
}

// minimal http.ResponseWriter for copySSEResponse tests
type sseRecorder struct {
	w       *bytes.Buffer
	headers http.Header
	status  int
}

func (r *sseRecorder) Header() http.Header {
	if r.headers == nil {
		r.headers = http.Header{}
	}
	return r.headers
}
func (r *sseRecorder) Write(b []byte) (int, error) { return r.w.Write(b) }
func (r *sseRecorder) WriteHeader(s int)           { r.status = s }
