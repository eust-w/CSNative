package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type ProviderConfig struct {
	ID           string            `json:"id,omitempty"`
	DisplayName  string            `json:"display_name,omitempty"`
	Adapter      string            `json:"adapter,omitempty"`
	BaseURL      string            `json:"base_url,omitempty"`
	APIKey       string            `json:"api_key,omitempty"`
	Key          string            `json:"key,omitempty"`
	DefaultModel string            `json:"default_model,omitempty"`
	Models       []ProviderModel   `json:"models,omitempty"`
	ModelMap     map[string]string `json:"model_map,omitempty"`
	MaxTokensCap map[string]int    `json:"max_tokens_cap,omitempty"`
	Enabled      bool              `json:"enabled"`
	Disabled     bool              `json:"disabled,omitempty"`
	Builtin      bool              `json:"builtin"`
	Verified     bool              `json:"verified,omitempty"`
	LastError    string            `json:"last_error,omitempty"`
}

type ProviderProfile = ProviderConfig

type ProviderModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type Config struct {
	Provider      string                    `json:"provider"`
	ProxyPort     uint16                    `json:"proxy_port"`
	SandboxPort   uint16                    `json:"sandbox_port"`
	PublicPort    uint16                    `json:"public_port"`
	Secret        string                    `json:"secret,omitempty"`
	Mode          string                    `json:"mode"`
	Providers     map[string]ProviderConfig `json:"providers"`
	PublicBaseURL string                    `json:"public_base_url,omitempty"`
}

var writeMu sync.Mutex

func Default() Config {
	cfg := Config{
		Provider:    "deepseek",
		ProxyPort:   18991,
		SandboxPort: 8990,
		PublicPort:  8992,
		Mode:        "proxy",
	}
	cfg.Providers = BuiltinProviders()
	return cfg
}

func DefaultDir() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return filepath.Join(h, ".csnative")
	}
	return ".csnative"
}

func (c Config) ProviderKey(provider string) string {
	if c.Providers == nil {
		return ""
	}
	return c.Providers[provider].EffectiveKey()
}

func (p ProviderProfile) EffectiveKey() string {
	if p.APIKey != "" {
		return p.APIKey
	}
	return p.Key
}

func (c *Config) EnsureSecret() error {
	if c.Secret != "" {
		return nil
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	c.Secret = hex.EncodeToString(b)
	return nil
}

func Load(dir string) (Config, error) {
	if err := rejectSymlink(dir); err != nil {
		return Config{}, err
	}
	path := filepath.Join(dir, "config.json")
	if err := rejectSymlink(path); err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, err
	}
	_ = os.Chmod(path, 0o600)
	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Provider == "" {
		cfg.Provider = "deepseek"
	}
	if cfg.ProxyPort == 0 {
		cfg.ProxyPort = 18991
	}
	if cfg.SandboxPort == 0 {
		cfg.SandboxPort = 8990
	}
	if cfg.PublicPort == 0 {
		cfg.PublicPort = 8992
	}
	if cfg.Mode == "" {
		cfg.Mode = "proxy"
	}
	if cfg.Providers == nil {
		cfg.Providers = map[string]ProviderConfig{}
	}
	cfg.Normalize()
	return cfg, nil
}

func Save(dir string, cfg Config) error {
	writeMu.Lock()
	defer writeMu.Unlock()
	if cfg.Providers == nil {
		cfg.Providers = map[string]ProviderConfig{}
	}
	cfg.Normalize()
	if err := ensureDir(dir); err != nil {
		return err
	}
	path := filepath.Join(dir, "config.json")
	if err := rejectSymlink(path); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	f, err := os.CreateTemp(dir, ".config.json.tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	_, werr := f.Write(data)
	serr := f.Sync()
	cerr := f.Close()
	if werr != nil {
		_ = os.Remove(tmp)
		return werr
	}
	if serr != nil {
		_ = os.Remove(tmp)
		return serr
	}
	if cerr != nil {
		_ = os.Remove(tmp)
		return cerr
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Chmod(path, 0o600)
}

func Update(dir string, fn func(*Config)) (Config, error) {
	writeMu.Lock()
	defer writeMu.Unlock()
	cfg, err := LoadUnlocked(dir)
	if err != nil {
		return Config{}, err
	}
	fn(&cfg)
	cfg.Normalize()
	if err := saveUnlocked(dir, cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func LoadUnlocked(dir string) (Config, error) { return Load(dir) }
func saveUnlocked(dir string, cfg Config) error {
	if cfg.Providers == nil {
		cfg.Providers = map[string]ProviderConfig{}
	}
	cfg.Normalize()
	if err := ensureDir(dir); err != nil {
		return err
	}
	path := filepath.Join(dir, "config.json")
	if err := rejectSymlink(path); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	f, err := os.CreateTemp(dir, ".config.json.tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Chmod(path, 0o600)
}

func (c *Config) Normalize() {
	if c.Provider == "" {
		c.Provider = "deepseek"
	}
	if c.ProxyPort == 0 {
		c.ProxyPort = 18991
	}
	if c.SandboxPort == 0 {
		c.SandboxPort = 8990
	}
	if c.PublicPort == 0 {
		c.PublicPort = 8992
	}
	if c.Mode == "" {
		c.Mode = "proxy"
	}
	builtins := BuiltinProviders()
	if c.Providers == nil {
		c.Providers = map[string]ProviderConfig{}
	}
	for id, builtin := range builtins {
		current, ok := c.Providers[id]
		if !ok {
			c.Providers[id] = builtin
			continue
		}
		merged := mergeProviderDefaults(current, builtin)
		c.Providers[id] = merged
	}
	for id, profile := range c.Providers {
		if profile.ID == "" {
			profile.ID = id
		}
		if profile.APIKey == "" && profile.Key != "" {
			profile.APIKey = profile.Key
		}
		profile.Key = ""
		if profile.DisplayName == "" {
			profile.DisplayName = profile.ID
		}
		if profile.Models == nil {
			profile.Models = []ProviderModel{}
		}
		if profile.ModelMap == nil {
			profile.ModelMap = map[string]string{}
		}
		if profile.MaxTokensCap == nil {
			profile.MaxTokensCap = map[string]int{}
		}
		if profile.Disabled {
			profile.Enabled = false
		}
		c.Providers[id] = profile
	}
	if _, ok := c.Providers[c.Provider]; !ok {
		c.Provider = "deepseek"
	}
}

func mergeProviderDefaults(current, builtin ProviderProfile) ProviderProfile {
	out := current
	if out.ID == "" {
		out.ID = builtin.ID
	}
	if out.DisplayName == "" {
		out.DisplayName = builtin.DisplayName
	}
	if out.Adapter == "" {
		out.Adapter = builtin.Adapter
	}
	if out.BaseURL == "" {
		out.BaseURL = builtin.BaseURL
	}
	if out.DefaultModel == "" {
		out.DefaultModel = builtin.DefaultModel
	}
	if len(out.Models) == 0 {
		out.Models = builtin.Models
	}
	if len(out.ModelMap) == 0 {
		out.ModelMap = builtin.ModelMap
	}
	if len(out.MaxTokensCap) == 0 {
		out.MaxTokensCap = builtin.MaxTokensCap
	}
	out.Builtin = builtin.Builtin
	if out.Disabled {
		out.Enabled = false
	} else if !out.Enabled {
		out.Enabled = builtin.Enabled
	}
	if out.APIKey == "" && out.Key != "" {
		out.APIKey = out.Key
	}
	out.Key = ""
	return out
}

func BuiltinProviders() map[string]ProviderProfile {
	return map[string]ProviderProfile{
		"deepseek": {
			ID:           "deepseek",
			DisplayName:  "DeepSeek",
			Adapter:      "anthropic_messages",
			BaseURL:      "https://api.deepseek.com/anthropic/v1/messages",
			DefaultModel: "deepseek-v4-flash",
			Models: []ProviderModel{
				{ID: "deepseek-v4-pro", DisplayName: "DeepSeek V4 Pro"},
				{ID: "deepseek-v4-flash", DisplayName: "DeepSeek V4 Flash"},
			},
			ModelMap: map[string]string{
				"claude-opus-4-8":   "deepseek-v4-pro",
				"claude-sonnet-5":   "deepseek-v4-flash",
				"claude-sonnet-4-6": "deepseek-v4-flash",
				"claude-haiku-4-5":  "deepseek-v4-flash",
			},
			MaxTokensCap: map[string]int{"deepseek-v4-pro": 65536, "deepseek-v4-flash": 32768},
			Enabled:      true,
			Builtin:      true,
		},
		"qwen": {
			ID:           "qwen",
			DisplayName:  "阿里云 DashScope / Qwen",
			Adapter:      "openai_chat_completions",
			BaseURL:      "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
			DefaultModel: "qwen-plus-latest",
			Models: []ProviderModel{
				{ID: "qwen3.7-max", DisplayName: "Qwen 3.7 Max"},
				{ID: "qwen-plus-latest", DisplayName: "Qwen Plus Latest"},
				{ID: "qwen-turbo", DisplayName: "Qwen Turbo"},
			},
			ModelMap: map[string]string{
				"claude-opus-4-8":   "qwen3.7-max",
				"claude-sonnet-5":   "qwen-plus-latest",
				"claude-sonnet-4-6": "qwen-plus-latest",
				"claude-haiku-4-5":  "qwen-turbo",
			},
			MaxTokensCap: map[string]int{"qwen3.7-max": 8192, "qwen-plus-latest": 8192, "qwen-turbo": 8192},
			Enabled:      true,
			Builtin:      true,
		},
		"openai-compatible": {
			ID:           "openai-compatible",
			DisplayName:  "OpenAI-compatible",
			Adapter:      "openai_chat_completions",
			BaseURL:      "https://api.openai.com/v1/chat/completions",
			DefaultModel: "gpt-4.1",
			Models:       []ProviderModel{{ID: "gpt-4.1", DisplayName: "GPT 4.1"}},
			ModelMap: map[string]string{
				"claude-opus-4-8":   "gpt-4.1",
				"claude-sonnet-5":   "gpt-4.1",
				"claude-sonnet-4-6": "gpt-4.1",
				"claude-haiku-4-5":  "gpt-4.1",
			},
			MaxTokensCap: map[string]int{"gpt-4.1": 8192},
			Enabled:      false,
			Builtin:      true,
		},
		"anthropic-compatible": {
			ID:           "anthropic-compatible",
			DisplayName:  "Anthropic-compatible",
			Adapter:      "anthropic_messages",
			BaseURL:      "https://api.anthropic.com/v1/messages",
			DefaultModel: "claude-sonnet-4-6",
			Models: []ProviderModel{
				{ID: "claude-opus-4-8", DisplayName: "Claude Opus"},
				{ID: "claude-sonnet-4-6", DisplayName: "Claude Sonnet"},
				{ID: "claude-haiku-4-5", DisplayName: "Claude Haiku"},
			},
			ModelMap: map[string]string{
				"claude-opus-4-8":   "claude-opus-4-8",
				"claude-sonnet-5":   "claude-sonnet-4-6",
				"claude-sonnet-4-6": "claude-sonnet-4-6",
				"claude-haiku-4-5":  "claude-haiku-4-5",
			},
			MaxTokensCap: map[string]int{"claude-opus-4-8": 8192, "claude-sonnet-4-6": 8192, "claude-haiku-4-5": 8192},
			Enabled:      false,
			Builtin:      true,
		},
		"openrouter":  openAIProvider("openrouter", "OpenRouter", "https://openrouter.ai/api/v1/chat/completions", "anthropic/claude-3.5-sonnet"),
		"moonshot":    openAIProvider("moonshot", "Moonshot / Kimi", "https://api.moonshot.cn/v1/chat/completions", "kimi-k2.6"),
		"siliconflow": openAIProvider("siliconflow", "SiliconFlow", "https://api.siliconflow.cn/v1/chat/completions", "deepseek-ai/DeepSeek-V3"),
		"ollama":      openAIProvider("ollama", "Ollama", "http://127.0.0.1:11434/v1/chat/completions", "qwen2.5"),
	}
}

func openAIProvider(id, name, baseURL, model string) ProviderProfile {
	return ProviderProfile{
		ID:           id,
		DisplayName:  name,
		Adapter:      "openai_chat_completions",
		BaseURL:      baseURL,
		DefaultModel: model,
		Models:       []ProviderModel{{ID: model, DisplayName: model}},
		ModelMap: map[string]string{
			"claude-opus-4-8":   model,
			"claude-sonnet-5":   model,
			"claude-sonnet-4-6": model,
			"claude-haiku-4-5":  model,
		},
		MaxTokensCap: map[string]int{model: 8192},
		Enabled:      false,
		Builtin:      true,
	}
}

func Mask(key string) string {
	n := len([]rune(key))
	if n == 0 {
		return ""
	}
	if n <= 4 {
		return strings.Repeat("•", n)
	}
	r := []rune(key)
	return strings.Repeat("•", n-4) + string(r[n-4:])
}

func ValidatePorts(proxyPort, sandboxPort uint16, extraPorts ...uint16) error {
	if proxyPort == 0 || sandboxPort == 0 {
		return errors.New("端口不能为 0")
	}
	if proxyPort == 8765 || sandboxPort == 8765 {
		return errors.New("端口 8765 是真实 Science 实例保留端口")
	}
	if proxyPort == sandboxPort {
		return errors.New("代理端口与沙箱端口不能相同")
	}
	seen := map[uint16]string{proxyPort: "代理端口", sandboxPort: "沙箱端口"}
	for _, port := range extraPorts {
		if port == 0 {
			return errors.New("端口不能为 0")
		}
		if port == 8765 {
			return errors.New("端口 8765 是真实 Science 实例保留端口")
		}
		if label, ok := seen[port]; ok {
			return fmt.Errorf("端口冲突：%s 与其他端口不能相同", label)
		}
		seen[port] = "额外端口"
	}
	return nil
}

func ensureDir(dir string) error {
	if err := rejectSymlink(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("配置路径不是目录: %s", dir)
	}
	return nil
}

func rejectSymlink(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("拒绝符号链接: %s", path)
	}
	return nil
}
