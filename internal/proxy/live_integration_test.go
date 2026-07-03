package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestLiveQwenCompatiblePing(t *testing.T) {
	key := os.Getenv("CSNATIVE_LIVE_API_KEY")
	if key == "" {
		t.Skip("set CSNATIVE_LIVE_API_KEY to run live upstream test")
	}
	upstream := openAICompletionsURL(os.Getenv("CSNATIVE_LIVE_OPENAI_BASE_URL"))
	srv := NewServer(ServerConfig{Provider: "qwen", Key: key, Secret: "sec", UpstreamOverride: upstream})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := `{"model":"claude-haiku-4-5","max_tokens":8,"messages":[{"role":"user","content":"ping"}]}`
	res, err := http.Post(ts.URL+"/sec/v1/messages", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	if res.StatusCode != http.StatusOK {
		t.Fatalf("live qwen ping status=%d body=%s", res.StatusCode, data)
	}
	var msg map[string]any
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("bad response JSON: %v body=%s", err, data)
	}
	if msg["type"] != "message" || msg["role"] != "assistant" {
		t.Fatalf("unexpected response: %s", data)
	}
}

func openAICompletionsURL(base string) string {
	base = strings.TrimRight(base, "/")
	if base == "" {
		return Providers["qwen"].URL
	}
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	return base + "/chat/completions"
}
