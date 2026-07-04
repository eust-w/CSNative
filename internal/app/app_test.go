package app

import (
	"strings"
	"testing"

	"csnative/internal/config"
)

func TestConfigCommandsRoundTripAndMaskKeys(t *testing.T) {
	a := NewWithConfigDir(t.TempDir())
	if _, err := a.SetConfig(UISettings{Provider: "qwen", ProxyPort: 19001, SandboxPort: 9001, PublicBaseURL: "https://csn.example.com/public?ignored=1"}); err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}
	masked, err := a.SaveProviderKey("qwen", "sk-qwen-123456")
	if err != nil {
		t.Fatalf("SaveProviderKey() error = %v", err)
	}
	if masked == "sk-qwen-123456" || masked[len(masked)-4:] != "3456" {
		t.Fatalf("bad mask: %q", masked)
	}
	cfg, err := a.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}
	if cfg.Provider != "qwen" || cfg.ProxyPort != 19001 || cfg.SandboxPort != 9001 || cfg.PublicBaseURL != "https://csn.example.com" || cfg.Keys["qwen"] != masked {
		t.Fatalf("bad config response: %#v", cfg)
	}
}

func TestReadLogsShowsRealAppLogAndClearLogs(t *testing.T) {
	a := NewWithConfigDir(t.TempDir())
	if _, err := a.SetConfig(UISettings{Provider: "qwen", ProxyPort: 19001, SandboxPort: 9001}); err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}
	logs, err := a.ReadLogs()
	if err != nil {
		t.Fatalf("ReadLogs() error = %v", err)
	}
	if !strings.Contains(logs, "app initialized") || !strings.Contains(logs, "config saved provider=qwen") {
		t.Fatalf("ReadLogs() did not include real app actions:\n%s", logs)
	}
	if ok, err := a.ClearLogs(); err != nil || !ok {
		t.Fatalf("ClearLogs() ok=%v err=%v", ok, err)
	}
	logs, err = a.ReadLogs()
	if err != nil {
		t.Fatalf("ReadLogs() after clear error = %v", err)
	}
	if !strings.Contains(logs, "logs cleared") || strings.Contains(logs, "config saved provider=qwen") {
		t.Fatalf("ClearLogs() did not reset log content:\n%s", logs)
	}
}

func TestProviderProfilesCanBeSavedActivatedAndDeleted(t *testing.T) {
	a := NewWithConfigDir(t.TempDir())
	openai := config.ProviderProfile{
		ID:           "custom-openai",
		DisplayName:  "Custom OpenAI",
		Adapter:      "openai_chat_completions",
		BaseURL:      "https://example.test/v1/chat/completions",
		APIKey:       "sk-custom",
		DefaultModel: "model-a",
		Models:       []config.ProviderModel{{ID: "model-a", DisplayName: "Model A"}},
		ModelMap:     map[string]string{"claude-opus-4-8": "model-a"},
		MaxTokensCap: map[string]int{"model-a": 4096},
		Enabled:      true,
	}
	view, err := a.SaveProvider(openai)
	if err != nil {
		t.Fatalf("SaveProvider(openai) error = %v", err)
	}
	if !view.HasKey || view.KeyMasked == "" || view.APIKey != "" {
		t.Fatalf("provider view should be masked: %#v", view)
	}
	anthropic := config.ProviderProfile{
		ID:           "custom-anthropic",
		DisplayName:  "Custom Anthropic",
		Adapter:      "anthropic_messages",
		BaseURL:      "https://anthropic.example/v1/messages",
		APIKey:       "sk-ant",
		DefaultModel: "claude-sonnet-4-6",
		Enabled:      true,
	}
	if _, err := a.SaveProvider(anthropic); err != nil {
		t.Fatalf("SaveProvider(anthropic) error = %v", err)
	}
	if _, err := a.SetActiveProvider("custom-openai"); err != nil {
		t.Fatalf("SetActiveProvider() error = %v", err)
	}
	if _, err := a.DeleteProvider("custom-openai"); err == nil {
		t.Fatal("DeleteProvider() should reject active provider")
	}
	if ok, err := a.DeleteProvider("custom-anthropic"); err != nil || !ok {
		t.Fatalf("DeleteProvider(custom-anthropic) ok=%v err=%v", ok, err)
	}
	qwen := config.BuiltinProviders()["qwen"]
	qwen.Enabled = false
	if _, err := a.SaveProvider(qwen); err != nil {
		t.Fatalf("SaveProvider(disabled builtin) error = %v", err)
	}
	if _, err := a.SetActiveProvider("qwen"); err == nil {
		t.Fatal("SetActiveProvider() should reject disabled builtin provider")
	}
	providers, err := a.ListProviders()
	if err != nil {
		t.Fatal(err)
	}
	qwenDisabled := false
	for _, p := range providers {
		if p.ID == "custom-anthropic" {
			t.Fatalf("custom provider was not deleted: %#v", providers)
		}
		if p.ID == "qwen" && !p.Enabled {
			qwenDisabled = true
		}
	}
	if !qwenDisabled {
		t.Fatalf("disabled builtin provider did not persist: %#v", providers)
	}
}

func TestProviderValidationRejectsMissingKeyBaseURLAndUnknownAdapter(t *testing.T) {
	a := NewWithConfigDir(t.TempDir())
	bad := []config.ProviderProfile{
		{ID: "bad-url", DisplayName: "Bad URL", Adapter: "openai_chat_completions", DefaultModel: "m", Enabled: true},
		{ID: "bad-adapter", DisplayName: "Bad Adapter", Adapter: "custom", BaseURL: "https://example.test/v1/messages", DefaultModel: "m", Enabled: true},
	}
	for _, profile := range bad {
		if _, err := a.SaveProvider(profile); err == nil {
			t.Fatalf("SaveProvider(%s) unexpectedly succeeded", profile.ID)
		}
	}
	if _, err := a.VerifyProvider("qwen"); err == nil || !strings.Contains(err.Error(), "API key") {
		t.Fatalf("VerifyProvider without key should fail clearly, got %v", err)
	}
	cfg, err := config.Load(a.configDir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Providers["qwen"].LastError == "" {
		t.Fatalf("VerifyProvider should persist failure state: %#v", cfg.Providers["qwen"])
	}
}

func TestStartWithProviderSelectsRequestedProvider(t *testing.T) {
	a := NewWithConfigDir(t.TempDir())
	proxyPort := freeTCPPort(t)
	if _, err := a.SetConfig(UISettings{Provider: "qwen", ProxyPort: proxyPort, SandboxPort: freeTCPPort(t), PublicPort: freeTCPPort(t)}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SaveProvider(config.ProviderProfile{
		ID:           "custom-start",
		DisplayName:  "Custom Start",
		Adapter:      "openai_chat_completions",
		BaseURL:      "https://example.test/v1/chat/completions",
		DefaultModel: "model-start",
		Enabled:      true,
	}); err != nil {
		t.Fatal(err)
	}
	_, err := a.StartWithProvider("custom-start")
	if err == nil {
		t.Fatal("StartWithProvider should fail before Science exists in this unit test")
	}
	cfg, cfgErr := a.GetConfig()
	if cfgErr != nil {
		t.Fatal(cfgErr)
	}
	if cfg.Provider != "custom-start" {
		t.Fatalf("StartWithProvider did not select provider: %#v", cfg)
	}
}

func TestSetConfigRejectsUnsafePortsAndUnknownProvider(t *testing.T) {
	a := NewWithConfigDir(t.TempDir())
	for _, cfg := range []UISettings{
		{Provider: "qwen", ProxyPort: 8765, SandboxPort: 9001},
		{Provider: "qwen", ProxyPort: 19001, SandboxPort: 19001},
		{Provider: "qwen", ProxyPort: 19001, SandboxPort: 9001, PublicPort: 8765},
		{Provider: "qwen", ProxyPort: 19001, SandboxPort: 9001, PublicPort: 19001},
		{Provider: "qwen", ProxyPort: 19001, SandboxPort: 9001, PublicPort: 9001},
		{Provider: "unknown", ProxyPort: 19001, SandboxPort: 9001},
		{Provider: "qwen", ProxyPort: 19001, SandboxPort: 9001, PublicBaseURL: "ftp://csn.example.com"},
		{Provider: "qwen", ProxyPort: 19001, SandboxPort: 9001, PublicBaseURL: "csn.example.com"},
	} {
		if _, err := a.SetConfig(cfg); err == nil {
			t.Fatalf("SetConfig(%#v) unexpectedly succeeded", cfg)
		}
	}
}

func TestRestoreProxyStartsSavedProxyWithoutOpeningScience(t *testing.T) {
	a := NewWithConfigDir(t.TempDir())
	t.Cleanup(func() { _ = a.StopAll() })
	if _, err := a.SetConfig(UISettings{Provider: "qwen", ProxyPort: freeTCPPort(t), SandboxPort: freeTCPPort(t), PublicPort: freeTCPPort(t)}); err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}
	if _, err := a.SaveProviderKey("qwen", "sk-test-restore"); err != nil {
		t.Fatalf("SaveProviderKey() error = %v", err)
	}
	if err := a.RestoreProxy(); err != nil {
		t.Fatalf("RestoreProxy() error = %v", err)
	}
	if st := a.Status(); st.Proxy != "green" {
		t.Fatalf("proxy was not restored: %#v", st)
	}
}
