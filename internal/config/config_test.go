package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigRoundTripSetsPrivatePermissionsAndMasksKeys(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".csnative")
	cfg := Default()
	cfg.Provider = "qwen"
	profile := cfg.Providers["qwen"]
	profile.APIKey = "sk-test-abcdef"
	cfg.Providers["qwen"] = profile

	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Provider != "qwen" || got.ProviderKey("qwen") != "sk-test-abcdef" {
		t.Fatalf("round trip mismatch: %#v", got)
	}
	if got.Providers["qwen"].BaseURL == "" || got.Providers["qwen"].Adapter != "openai_chat_completions" {
		t.Fatalf("builtin profile was not normalized: %#v", got.Providers["qwen"])
	}
	if mode := fileMode(t, dir); mode != 0o700 {
		t.Fatalf("config dir mode = %o, want 700", mode)
	}
	if mode := fileMode(t, filepath.Join(dir, "config.json")); mode != 0o600 {
		t.Fatalf("config file mode = %o, want 600", mode)
	}
	if masked := Mask("sk-test-abcdef"); masked == "sk-test-abcdef" || masked[len(masked)-4:] != "cdef" {
		t.Fatalf("bad mask: %q", masked)
	}
}

func TestLoadMigratesLegacyProviderKeysIntoProfiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".csnative")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{"provider":"qwen","providers":{"qwen":{"key":"sk-legacy"}}}`)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProviderKey("qwen") != "sk-legacy" {
		t.Fatalf("legacy key not migrated: %#v", got.Providers["qwen"])
	}
	if got.Providers["qwen"].Key != "" || got.Providers["qwen"].APIKey != "sk-legacy" {
		t.Fatalf("legacy key should move to api_key: %#v", got.Providers["qwen"])
	}
	if got.Providers["deepseek"].Adapter != "anthropic_messages" || got.Providers["openrouter"].Adapter != "openai_chat_completions" {
		t.Fatalf("builtin catalog incomplete: %#v", got.Providers)
	}
}

func TestLoadRejectsSymlinkedConfig(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".csnative")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target.json")
	if err := os.WriteFile(target, []byte(`{"provider":"leak"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "config.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("Load() accepted symlinked config")
	}
}

func TestSaveIgnoresStaleTempFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".csnative")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".config.json.tmp-stale"), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := Default()
	cfg.Provider = "qwen"
	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save() with stale temp file error = %v", err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != "qwen" {
		t.Fatalf("provider = %q", got.Provider)
	}
}

func TestValidatePortsRejectsPublicPortConflicts(t *testing.T) {
	cases := [][]uint16{
		{19001, 9001, 8765},
		{19001, 9001, 19001},
		{19001, 9001, 9001},
		{19001, 9001, 0},
	}
	for _, ports := range cases {
		if err := ValidatePorts(ports[0], ports[1], ports[2]); err == nil {
			t.Fatalf("ValidatePorts(%v) unexpectedly succeeded", ports)
		}
	}
	if err := ValidatePorts(19001, 9001, 8992); err != nil {
		t.Fatalf("ValidatePorts valid ports error = %v", err)
	}
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}
