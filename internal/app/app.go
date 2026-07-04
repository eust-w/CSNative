package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"csnative/internal/config"
	"csnative/internal/oauth"
	"csnative/internal/proxy"
	"csnative/internal/science"
)

const Version = "0.1.0"
const maxLogBytes = 128 * 1024

type App struct {
	mu         sync.Mutex
	configDir  string
	proxy      *proxy.Server
	public     *http.Server
	provider   string
	keyFP      uint64
	secret     string
	proxyPort  uint16
	sandboxURL string
}

type UISettings struct {
	Provider      string `json:"provider"`
	ProxyPort     uint16 `json:"proxy_port"`
	SandboxPort   uint16 `json:"sandbox_port"`
	PublicPort    uint16 `json:"public_port"`
	PublicBaseURL string `json:"public_base_url"`
}

type ConfigResponse struct {
	Provider      string            `json:"provider"`
	ProxyPort     uint16            `json:"proxy_port"`
	SandboxPort   uint16            `json:"sandbox_port"`
	PublicPort    uint16            `json:"public_port"`
	PublicBaseURL string            `json:"public_base_url"`
	Mode          string            `json:"mode"`
	Keys          map[string]string `json:"keys"`
}

type ProviderView struct {
	config.ProviderProfile
	HasKey    bool   `json:"has_key"`
	KeyMasked string `json:"key_masked"`
	Active    bool   `json:"active"`
}

type StatusResponse struct {
	Proxy    string `json:"proxy"`
	Sandbox  string `json:"sandbox"`
	Upstream string `json:"upstream"`
	Public   string `json:"public"`
}

func New() *App { return NewWithConfigDir(config.DefaultDir()) }

func NewWithConfigDir(dir string) *App {
	app := &App{configDir: dir}
	app.logf("info", "app initialized version=%s", Version)
	return app
}

func (a *App) GetConfig() (ConfigResponse, error) {
	cfg, err := config.Load(a.configDir)
	if err != nil {
		return ConfigResponse{}, err
	}
	keys := map[string]string{}
	for id, profile := range cfg.Providers {
		keys[id] = config.Mask(profile.EffectiveKey())
	}
	return ConfigResponse{Provider: cfg.Provider, ProxyPort: cfg.ProxyPort, SandboxPort: cfg.SandboxPort, PublicPort: cfg.PublicPort, PublicBaseURL: cfg.PublicBaseURL, Mode: cfg.Mode, Keys: keys}, nil
}

func (a *App) SetConfig(cfg UISettings) (bool, error) {
	if cfg.Provider == "" {
		cfg.Provider = "deepseek"
	}
	current, err := config.Load(a.configDir)
	if err != nil {
		return false, err
	}
	if _, ok := current.Providers[cfg.Provider]; !ok {
		return false, fmt.Errorf("未知 provider：%s", cfg.Provider)
	}
	if cfg.PublicPort == 0 {
		cfg.PublicPort = 8992
	}
	if err := config.ValidatePorts(cfg.ProxyPort, cfg.SandboxPort, cfg.PublicPort); err != nil {
		return false, err
	}
	publicBaseURL, err := normalizePublicBaseURL(cfg.PublicBaseURL)
	if err != nil {
		return false, err
	}
	_, err = config.Update(a.configDir, func(c *config.Config) {
		c.Provider = cfg.Provider
		c.ProxyPort = cfg.ProxyPort
		c.SandboxPort = cfg.SandboxPort
		c.PublicPort = cfg.PublicPort
		c.PublicBaseURL = publicBaseURL
	})
	if err == nil {
		a.logf("info", "config saved provider=%s proxy=%d sandbox=%d public=%d public_base_url=%s", cfg.Provider, cfg.ProxyPort, cfg.SandboxPort, cfg.PublicPort, publicBaseURL)
	}
	return err == nil, err
}

func (a *App) SaveProviderKey(provider, key string) (string, error) {
	cfg, err := config.Load(a.configDir)
	if err != nil {
		return "", err
	}
	if _, ok := cfg.Providers[provider]; !ok {
		return "", fmt.Errorf("未知 provider：%s", provider)
	}
	_, err = config.Update(a.configDir, func(c *config.Config) {
		profile := c.Providers[provider]
		profile.APIKey = key
		profile.Key = ""
		profile.Enabled = true
		c.Providers[provider] = profile
	})
	if err != nil {
		a.logf("error", "provider key save failed provider=%s err=%v", provider, err)
		return "", err
	}
	a.logf("info", "provider key saved provider=%s", provider)
	return config.Mask(key), nil
}

func (a *App) ListProviders() ([]ProviderView, error) {
	cfg, err := config.Load(a.configDir)
	if err != nil {
		return nil, err
	}
	out := make([]ProviderView, 0, len(cfg.Providers))
	for id, profile := range cfg.Providers {
		out = append(out, providerView(id, profile, cfg.Provider))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Builtin != out[j].Builtin {
			return out[i].Builtin
		}
		return out[i].DisplayName < out[j].DisplayName
	})
	return out, nil
}

func (a *App) SaveProvider(profile config.ProviderProfile) (ProviderView, error) {
	id := normalizeProviderID(profile.ID)
	if id == "" {
		return ProviderView{}, fmt.Errorf("provider id 不能为空")
	}
	profile.ID = id
	if err := validateProvider(profile, false); err != nil {
		return ProviderView{}, err
	}
	var active string
	var saved config.ProviderProfile
	_, err := config.Update(a.configDir, func(c *config.Config) {
		existing := c.Providers[id]
		if profile.APIKey == "" {
			profile.APIKey = existing.EffectiveKey()
		}
		profile.Key = ""
		profile.Builtin = existing.Builtin
		profile.Disabled = !profile.Enabled
		if profile.Models == nil {
			profile.Models = existing.Models
		}
		if len(profile.ModelMap) == 0 {
			profile.ModelMap = defaultModelMap(profile.DefaultModel)
		}
		if len(profile.MaxTokensCap) == 0 {
			profile.MaxTokensCap = map[string]int{profile.DefaultModel: 8192}
		}
		c.Providers[id] = profile
		active = c.Provider
		saved = profile
	})
	if err != nil {
		a.logf("error", "provider save failed id=%s err=%v", id, err)
		return ProviderView{}, err
	}
	a.logf("info", "provider saved id=%s adapter=%s enabled=%t", id, saved.Adapter, saved.Enabled)
	return providerView(id, saved, active), nil
}

func (a *App) DeleteProvider(id string) (bool, error) {
	id = normalizeProviderID(id)
	if id == "" {
		return false, fmt.Errorf("provider id 不能为空")
	}
	cfg, err := config.Load(a.configDir)
	if err != nil {
		return false, err
	}
	profile, ok := cfg.Providers[id]
	if !ok {
		return false, fmt.Errorf("未知 provider：%s", id)
	}
	if cfg.Provider == id {
		return false, fmt.Errorf("当前启动默认 provider 不能删除：%s", id)
	}
	_, err = config.Update(a.configDir, func(c *config.Config) {
		if profile.Builtin {
			profile.Enabled = false
			profile.Disabled = true
			profile.APIKey = ""
			profile.Key = ""
			profile.Verified = false
			c.Providers[id] = profile
			return
		}
		delete(c.Providers, id)
	})
	if err == nil {
		a.logf("info", "provider deleted id=%s builtin=%t", id, profile.Builtin)
	}
	return err == nil, err
}

func (a *App) SetActiveProvider(id string) (bool, error) {
	id = normalizeProviderID(id)
	cfg, err := config.Load(a.configDir)
	if err != nil {
		return false, err
	}
	profile, ok := cfg.Providers[id]
	if !ok {
		return false, fmt.Errorf("未知 provider：%s", id)
	}
	if !profile.Enabled {
		return false, fmt.Errorf("provider 未启用：%s", id)
	}
	_, err = config.Update(a.configDir, func(c *config.Config) { c.Provider = id })
	if err == nil {
		a.logf("info", "active provider set id=%s", id)
	}
	return err == nil, err
}

func (a *App) VerifyProvider(id string) (map[string]any, error) {
	if _, err := a.SetActiveProvider(id); err != nil {
		return nil, err
	}
	res, err := a.VerifyKey()
	ok, _ := res["ok"].(bool)
	hint, _ := res["hint"].(string)
	if err != nil && hint == "" {
		hint = err.Error()
	}
	_, _ = config.Update(a.configDir, func(c *config.Config) {
		profile := c.Providers[normalizeProviderID(id)]
		profile.Verified = ok
		profile.LastError = ""
		if !ok {
			profile.LastError = hint
		}
		c.Providers[normalizeProviderID(id)] = profile
	})
	if ok {
		a.logf("info", "provider verify ok id=%s", normalizeProviderID(id))
	} else {
		a.logf("warn", "provider verify failed id=%s hint=%s", normalizeProviderID(id), hint)
	}
	return res, err
}

func (a *App) StartWithProvider(id string) (map[string]any, error) {
	a.logf("info", "start requested provider=%s", normalizeProviderID(id))
	if _, err := a.SetActiveProvider(id); err != nil {
		a.logf("error", "start rejected provider=%s err=%v", normalizeProviderID(id), err)
		return nil, err
	}
	res, err := a.OneClickLogin()
	if err != nil {
		a.logf("error", "start failed provider=%s err=%v", normalizeProviderID(id), err)
	} else {
		a.logf("info", "start completed provider=%s action=%v", normalizeProviderID(id), res["action"])
	}
	return res, err
}

func (a *App) SetMode(mode string) (bool, error) {
	if mode != "proxy" && mode != "official" {
		return false, fmt.Errorf("未知模式：%s", mode)
	}
	if mode == "official" {
		if err := a.StopAll(); err != nil {
			return false, err
		}
	}
	_, err := config.Update(a.configDir, func(c *config.Config) { c.Mode = mode })
	if err == nil {
		a.logf("info", "mode set mode=%s", mode)
	}
	return err == nil, err
}

func (a *App) StartProxy() (map[string]any, error) {
	port, _, _, err := a.ensureProxy()
	if err != nil {
		a.logf("error", "proxy start failed err=%v", err)
		return nil, err
	}
	a.logf("info", "proxy start requested port=%d", port)
	return map[string]any{"port": port}, nil
}

func (a *App) RestoreProxy() error {
	cfg, err := config.Load(a.configDir)
	if err != nil {
		return err
	}
	if cfg.Mode == "official" || cfg.ProviderKey(cfg.Provider) == "" {
		a.logf("info", "restore skipped mode=%s provider=%s has_key=%t", cfg.Mode, cfg.Provider, cfg.ProviderKey(cfg.Provider) != "")
		return nil
	}
	if _, _, _, err = a.ensureProxy(); err != nil {
		a.logf("error", "restore proxy failed err=%v", err)
		return err
	}
	mgr := science.NewManager(a.configDir)
	if !mgr.Running(cfg.SandboxPort) {
		return nil
	}
	if err := a.startPublicEntry(cfg.PublicPort, mgr); err != nil {
		return err
	}
	a.logf("info", "restore completed provider=%s sandbox_port=%d", cfg.Provider, cfg.SandboxPort)
	a.mu.Lock()
	a.sandboxURL = mgr.URL()
	a.mu.Unlock()
	return nil
}

func (a *App) VerifyKey() (map[string]any, error) {
	port, secret, _, err := a.ensureProxy()
	if err != nil {
		a.logf("error", "verify key proxy setup failed err=%v", err)
		return nil, err
	}
	body := []byte(`{"model":"claude-opus-4-8","max_tokens":1,"messages":[{"role":"user","content":"ping"}]}`)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/%s/v1/messages", port, secret), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		a.logf("error", "verify key request failed err=%v", err)
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == 200 {
		a.logf("info", "verify key upstream accepted")
		return map[string]any{"ok": true, "hint": "key 有效，上游已接受。"}, nil
	}
	a.logf("warn", "verify key upstream status=%d", res.StatusCode)
	return map[string]any{"ok": false, "hint": fmt.Sprintf("上游返回 %d。", res.StatusCode)}, nil
}

func (a *App) OneClickLogin() (map[string]any, error) {
	pport, secret, _, err := a.ensureProxy()
	if err != nil {
		a.logf("error", "one click proxy setup failed err=%v", err)
		return nil, err
	}
	cfg, err := config.Load(a.configDir)
	if err != nil {
		return nil, err
	}
	mgr := science.NewManager(a.configDir)
	authDir := filepath.Join(mgr.SandboxHome, ".claude-science")
	if mgr.Running(cfg.SandboxPort) && oauth.LoginIntact(authDir, "virtual@localhost.invalid", mgr.SandboxHome) {
		url := mgr.URL()
		if url == "" {
			url = fmt.Sprintf("http://127.0.0.1:%d", cfg.SandboxPort)
		}
		a.mu.Lock()
		a.sandboxURL = url
		a.mu.Unlock()
		_ = openURL(url)
		a.logf("info", "science reopened sandbox_port=%d", cfg.SandboxPort)
		return map[string]any{"url": url, "msg": "已在运行，已重新打开 Science。", "action": "reopened"}, nil
	}
	_, _, err = oauth.EnsureVirtualLogin(authDir, "virtual@localhost.invalid", mgr.SandboxHome)
	if err != nil {
		a.logf("error", "virtual login write failed err=%v", err)
		return nil, fmt.Errorf("写虚拟登录失败：%w", err)
	}
	proxyURL := fmt.Sprintf("http://127.0.0.1:%d/%s", pport, secret)
	if err := mgr.Start(cfg.SandboxPort, proxyURL); err != nil {
		a.logf("error", "science start failed err=%v", err)
		return nil, err
	}
	if !waitHealth(cfg.SandboxPort, "", 8*time.Second) {
		a.logf("error", "science health timeout sandbox_port=%d", cfg.SandboxPort)
		return nil, fmt.Errorf("沙箱起后探活超时")
	}
	url := mgr.URL()
	if url == "" {
		url = fmt.Sprintf("http://127.0.0.1:%d", cfg.SandboxPort)
	}
	if err := a.startPublicEntry(cfg.PublicPort, mgr); err != nil {
		a.logf("error", "public entry start failed err=%v", err)
		return nil, fmt.Errorf("公网入口启动失败：%w", err)
	}
	a.mu.Lock()
	a.sandboxURL = url
	a.mu.Unlock()
	_ = openURL(url)
	a.logf("info", "science started sandbox_port=%d public_port=%d provider=%s", cfg.SandboxPort, cfg.PublicPort, cfg.Provider)
	return map[string]any{"url": url, "msg": "已启动。", "action": "started"}, nil
}

func (a *App) StopAll() error {
	a.mu.Lock()
	proxySrv := a.proxy
	publicSrv := a.public
	a.proxy = nil
	a.public = nil
	a.secret = ""
	a.sandboxURL = ""
	a.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if publicSrv != nil {
		_ = publicSrv.Shutdown(ctx)
	}
	if proxySrv != nil {
		_ = proxySrv.Shutdown(ctx)
	}
	err := science.NewManager(a.configDir).Stop()
	if err != nil {
		a.logf("error", "stop failed err=%v", err)
	} else {
		a.logf("info", "services stopped")
	}
	return err
}

func (a *App) Status() StatusResponse {
	cfg, _ := config.Load(a.configDir)
	a.mu.Lock()
	secret := a.secret
	pport := a.proxyPort
	a.mu.Unlock()
	if pport == 0 {
		pport = cfg.ProxyPort
	}
	proxyState := "amber"
	if secret != "" && waitHealth(pport, secret, 300*time.Millisecond) {
		proxyState = "green"
	}
	sandboxState := "amber"
	if science.NewManager(a.configDir).Running(cfg.SandboxPort) {
		sandboxState = "green"
	}
	publicState := "amber"
	if tcpReachable("127.0.0.1", int(cfg.PublicPort), 300*time.Millisecond) {
		publicState = "green"
	}
	upstream := "amber"
	if profile, err := activeProvider(cfg); err == nil {
		host, port := upstreamAddress(profile)
		if tcpReachable(host, port, 700*time.Millisecond) {
			upstream = "green"
		}
	}
	return StatusResponse{Proxy: proxyState, Sandbox: sandboxState, Upstream: upstream, Public: publicState}
}

func (a *App) OpenURL() error {
	a.mu.Lock()
	url := a.sandboxURL
	a.mu.Unlock()
	if url == "" {
		cfg, _ := config.Load(a.configDir)
		if u := science.NewManager(a.configDir).URL(); u != "" {
			url = u
		} else {
			url = fmt.Sprintf("http://127.0.0.1:%d", cfg.SandboxPort)
		}
	}
	a.logf("info", "open science url")
	return openURL(url)
}

func (a *App) OpenPublicURL() error {
	cfg, err := config.Load(a.configDir)
	if err != nil {
		return err
	}
	a.logf("info", "open public entry url=%s", publicEntryURL(cfg))
	return openURL(publicEntryURL(cfg))
}

func (a *App) RunDoctor() (string, error) {
	a.logf("info", "doctor started")
	cfg, _ := config.Load(a.configDir)
	var b bytes.Buffer
	fmt.Fprintf(&b, "CS Native doctor（只读诊断）\nprovider=%s 代理端口=%d 沙箱端口=%d 公网入口=%d\n", cfg.Provider, cfg.ProxyPort, cfg.SandboxPort, cfg.PublicPort)
	if cfg.PublicBaseURL != "" {
		fmt.Fprintf(&b, "public_base_url=%s\n", cfg.PublicBaseURL)
	}
	if err := config.ValidatePorts(cfg.ProxyPort, cfg.SandboxPort, cfg.PublicPort); err != nil {
		fmt.Fprintf(&b, "⚠ 端口配置异常：%v\n", err)
	} else {
		b.WriteString("✓ 端口配置无冲突\n")
	}
	if _, err := os.Stat(science.ScienceBin); err == nil {
		b.WriteString("✓ Science 二进制存在\n")
	} else {
		b.WriteString("⚠ 未找到 Science 二进制\n")
	}
	if info, err := os.Stat(filepath.Join(a.configDir, "sandbox", "home")); err == nil && info.IsDir() {
		fmt.Fprintf(&b, "✓ 沙箱目录存在（权限 %o）\n", info.Mode().Perm())
	} else {
		b.WriteString("⚠ 沙箱目录尚未创建\n")
	}
	if cfg.ProviderKey(cfg.Provider) != "" {
		b.WriteString("✓ 当前 provider key 已保存（未显示）\n")
	} else {
		b.WriteString("⚠ 当前 provider key 未保存\n")
	}
	a.mu.Lock()
	secret := a.secret
	pport := a.proxyPort
	a.mu.Unlock()
	if pport == 0 {
		pport = cfg.ProxyPort
	}
	if secret != "" && waitHealth(pport, secret, 300*time.Millisecond) {
		b.WriteString("✓ 本地代理 health 正常\n")
	} else if tcpReachable("127.0.0.1", int(cfg.ProxyPort), 300*time.Millisecond) {
		b.WriteString("⚠ 代理端口已被占用或 secret 不匹配\n")
	} else {
		b.WriteString("⚠ 本地代理未运行\n")
	}
	if science.NewManager(a.configDir).Running(cfg.SandboxPort) {
		b.WriteString("✓ Science 沙箱正在运行\n")
	} else if tcpReachable("127.0.0.1", int(cfg.SandboxPort), 300*time.Millisecond) {
		b.WriteString("⚠ 沙箱端口被占用，但当前沙箱状态不可确认\n")
	} else {
		b.WriteString("⚠ Science 沙箱未运行\n")
	}
	if tcpReachable("127.0.0.1", int(cfg.PublicPort), 300*time.Millisecond) {
		b.WriteString("✓ 公网 fresh nonce 入口端口可达\n")
	} else {
		b.WriteString("⚠ 公网 fresh nonce 入口未运行\n")
	}
	out := b.String()
	a.logf("info", "doctor completed bytes=%d", len(out))
	return out, nil
}

func (a *App) OpenOfficial() error {
	cmd := exec.Command("open", "/Applications/Claude Science.app")
	cmd.Env = cleanAnthropicEnv(os.Environ())
	err := cmd.Run()
	if err != nil {
		a.logf("error", "open official failed err=%v", err)
	} else {
		a.logf("info", "open official science")
	}
	return err
}

func (a *App) OpenLogs() error {
	dir := filepath.Join(a.configDir, "logs")
	_ = os.MkdirAll(dir, 0o700)
	a.logf("info", "open logs dir=%s", dir)
	return exec.Command("open", dir).Run()
}

func (a *App) ReadLogs() (string, error) {
	path := logFilePath(a.configDir)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "暂无日志。执行启动、自检、验证或停止后会记录到 ~/.csnative/logs/csnative.log。", nil
	}
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "暂无日志。执行启动、自检、验证或停止后会记录到 ~/.csnative/logs/csnative.log。", nil
	}
	if len(data) > maxLogBytes {
		data = data[len(data)-maxLogBytes:]
		if i := bytes.IndexByte(data, '\n'); i >= 0 && i+1 < len(data) {
			data = data[i+1:]
		}
	}
	return string(data), nil
}

func (a *App) ClearLogs() (bool, error) {
	if err := ensureLogDir(a.configDir); err != nil {
		return false, err
	}
	if err := os.WriteFile(logFilePath(a.configDir), nil, 0o600); err != nil {
		return false, err
	}
	a.logf("info", "logs cleared")
	return true, nil
}

func (a *App) OpenReleasePage() error {
	a.logf("info", "open release page")
	return openURL("https://github.com/eust-w/CSNative/releases/latest")
}
func (a *App) ReportBug() error {
	a.logf("info", "open issue page")
	return openURL("https://github.com/eust-w/CSNative/issues/new")
}
func (a *App) AppVersion() string { return Version }

func (a *App) ensureProxy() (uint16, string, string, error) {
	cfg, err := config.Load(a.configDir)
	if err != nil {
		return 0, "", "", err
	}
	profile, err := activeProvider(cfg)
	if err != nil {
		return 0, "", "", err
	}
	if err := validateProvider(profile, true); err != nil {
		return 0, "", "", err
	}
	key := profile.EffectiveKey()
	if key == "" {
		return 0, "", "", fmt.Errorf("缺少 %s 的 API key，请先保存", cfg.Provider)
	}
	if err := cfg.EnsureSecret(); err != nil {
		return 0, "", "", err
	}
	_ = config.Save(a.configDir, cfg)
	fp := providerFingerprint(profile)
	a.mu.Lock()
	if a.proxy != nil && a.proxyPort == cfg.ProxyPort && a.provider == cfg.Provider && a.keyFP == fp && waitHealth(cfg.ProxyPort, cfg.Secret, 300*time.Millisecond) {
		a.mu.Unlock()
		return cfg.ProxyPort, cfg.Secret, cfg.Provider, nil
	}
	old := a.proxy
	a.proxy = nil
	a.mu.Unlock()
	if old != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = old.Shutdown(ctx)
		cancel()
	}
	srv := proxy.NewServer(proxy.ServerConfig{Provider: cfg.Provider, Profile: profile, Key: key, Secret: cfg.Secret})
	go func() {
		_ = srv.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", cfg.ProxyPort))
	}()
	if !waitHealth(cfg.ProxyPort, cfg.Secret, 4*time.Second) {
		a.logf("error", "proxy health timeout provider=%s port=%d", cfg.Provider, cfg.ProxyPort)
		return 0, "", "", fmt.Errorf("代理起后探活超时")
	}
	a.mu.Lock()
	a.proxy = srv
	a.provider = cfg.Provider
	a.keyFP = fp
	a.secret = cfg.Secret
	a.proxyPort = cfg.ProxyPort
	a.mu.Unlock()
	a.logf("info", "proxy started provider=%s port=%d", cfg.Provider, cfg.ProxyPort)
	return cfg.ProxyPort, cfg.Secret, cfg.Provider, nil
}

func (a *App) startPublicEntry(port uint16, mgr *science.Manager) error {
	a.mu.Lock()
	old := a.public
	a.public = nil
	a.mu.Unlock()
	if old != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = old.Shutdown(ctx)
		cancel()
	}
	srv, err := science.StartPublicEntry(fmt.Sprintf("127.0.0.1:%d", port), func() (string, error) {
		u := mgr.URL()
		if u == "" {
			return "", fmt.Errorf("cannot create fresh Science URL")
		}
		return u, nil
	})
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.public = srv
	a.mu.Unlock()
	a.logf("info", "public entry started port=%d", port)
	return nil
}

func waitHealth(port uint16, secret string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	path := "/health"
	if secret != "" {
		path = "/" + secret + path
	}
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d%s", port, path), nil)
		res, err := http.DefaultClient.Do(req)
		cancel()
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode == 200 {
				return true
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func keyFingerprint(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func upstreamHost(provider string) string {
	if provider == "qwen" {
		return "dashscope.aliyuncs.com"
	}
	return "api.deepseek.com"
}

func upstreamAddress(profile config.ProviderProfile) (string, int) {
	u, err := url.Parse(profile.BaseURL)
	if err != nil {
		return upstreamHost(profile.ID), 443
	}
	port := 443
	if u.Scheme == "http" {
		port = 80
	}
	if u.Port() != "" {
		if p, err := net.LookupPort("tcp", u.Port()); err == nil {
			port = p
		}
	}
	host := u.Hostname()
	if host == "" {
		host = upstreamHost(profile.ID)
	}
	return host, port
}

func activeProvider(cfg config.Config) (config.ProviderProfile, error) {
	profile, ok := cfg.Providers[cfg.Provider]
	if !ok {
		return config.ProviderProfile{}, fmt.Errorf("未知 provider：%s", cfg.Provider)
	}
	return profile, nil
}

func providerView(id string, profile config.ProviderProfile, activeID string) ProviderView {
	key := profile.EffectiveKey()
	profile.APIKey = ""
	profile.Key = ""
	return ProviderView{
		ProviderProfile: profile,
		HasKey:          key != "",
		KeyMasked:       config.Mask(key),
		Active:          id == activeID,
	}
}

func normalizeProviderID(id string) string {
	id = strings.TrimSpace(strings.ToLower(id))
	id = strings.ReplaceAll(id, " ", "-")
	id = strings.ReplaceAll(id, "_", "-")
	return id
}

func validateProvider(profile config.ProviderProfile, requireKey bool) error {
	if profile.ID == "" {
		return fmt.Errorf("provider id 不能为空")
	}
	switch profile.Adapter {
	case "anthropic_messages", "openai_chat_completions":
	default:
		return fmt.Errorf("未知 adapter：%s", profile.Adapter)
	}
	if strings.TrimSpace(profile.BaseURL) == "" {
		return fmt.Errorf("base URL 不能为空")
	}
	u, err := url.Parse(profile.BaseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("base URL 无效：%s", profile.BaseURL)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("base URL 必须使用 http 或 https")
	}
	if strings.TrimSpace(profile.DefaultModel) == "" {
		return fmt.Errorf("默认模型不能为空")
	}
	if requireKey && profile.EffectiveKey() == "" {
		return fmt.Errorf("缺少 %s 的 API key，请先保存", profile.ID)
	}
	return nil
}

func defaultModelMap(model string) map[string]string {
	return map[string]string{
		"claude-opus-4-8":   model,
		"claude-sonnet-5":   model,
		"claude-sonnet-4-6": model,
		"claude-haiku-4-5":  model,
	}
}

func normalizePublicBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("公网域名无效：%s", raw)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("公网域名必须使用 http 或 https")
	}
	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func publicEntryURL(cfg config.Config) string {
	if cfg.PublicBaseURL != "" {
		return cfg.PublicBaseURL
	}
	return fmt.Sprintf("http://127.0.0.1:%d", cfg.PublicPort)
}

func logFilePath(configDir string) string {
	return filepath.Join(configDir, "logs", "csnative.log")
}

func ensureLogDir(configDir string) error {
	return os.MkdirAll(filepath.Join(configDir, "logs"), 0o700)
}

func (a *App) logf(level, format string, args ...any) {
	if a == nil || a.configDir == "" {
		return
	}
	_ = appendLog(a.configDir, level, fmt.Sprintf(format, args...))
}

func appendLog(configDir, level, message string) error {
	if err := ensureLogDir(configDir); err != nil {
		return err
	}
	message = sanitizeLogMessage(message)
	line := fmt.Sprintf("%s [%s] %s\n", time.Now().Format(time.RFC3339), strings.ToUpper(strings.TrimSpace(level)), message)
	f, err := os.OpenFile(logFilePath(configDir), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(line); err != nil {
		return err
	}
	return f.Chmod(0o600)
}

func sanitizeLogMessage(message string) string {
	message = strings.ReplaceAll(message, "\r", " ")
	message = strings.ReplaceAll(message, "\n", " ")
	message = strings.Join(strings.Fields(message), " ")
	if len([]rune(message)) > 1600 {
		r := []rune(message)
		message = string(r[:1600]) + "..."
	}
	return message
}

func providerFingerprint(profile config.ProviderProfile) uint64 {
	plain, _ := json.Marshal(profile)
	return keyFingerprint(string(plain) + profile.EffectiveKey())
}

func tcpReachable(host string, port int, timeout time.Duration) bool {
	conn, err := (&net.Dialer{Timeout: timeout}).Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func openURL(url string) error { return exec.Command("open", url).Run() }

func cleanAnthropicEnv(env []string) []string {
	var out []string
	for _, e := range env {
		if strings.HasPrefix(e, "ANTHROPIC_") {
			continue
		}
		out = append(out, e)
	}
	return out
}
