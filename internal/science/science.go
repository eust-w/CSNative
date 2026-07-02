package science

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const ScienceBin = "/Applications/Claude Science.app/Contents/Resources/bin/claude-science"

type Manager struct {
	ScienceBin  string
	SandboxHome string
}

func NewManager(configDir string) *Manager {
	return &Manager{ScienceBin: ScienceBin, SandboxHome: filepath.Join(configDir, "sandbox", "home")}
}

func (m *Manager) DataDir() string { return filepath.Join(m.SandboxHome, ".claude-science") }

func (m *Manager) EnsureRuntimeAssets() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	data := m.DataDir()
	if _, err := os.Stat(filepath.Join(data, "bin")); err == nil {
		return m.ensurePythonCompatibility()
	}
	if err := os.MkdirAll(data, 0o700); err != nil {
		return err
	}
	real := filepath.Join(home, ".claude-science")
	for _, asset := range []string{"bin", "conda", "runtime", "seed-assets"} {
		src := filepath.Join(real, asset)
		if info, err := os.Stat(src); err == nil && info.IsDir() {
			dst := filepath.Join(data, asset)
			_ = exec.Command("cp", "-Rc", src, dst).Run()
		}
	}
	return m.ensurePythonCompatibility()
}

func (m *Manager) EnsureKeychain() {
	kc := filepath.Join(m.SandboxHome, "Library", "Keychains", "login.keychain-db")
	if _, err := os.Stat(kc); err != nil {
		_ = os.MkdirAll(filepath.Dir(kc), 0o700)
		_ = exec.Command("security", "create-keychain", "-p", "", kc).Run()
	}
	for _, args := range [][]string{
		{"list-keychains", "-d", "user", "-s", kc},
		{"default-keychain", "-d", "user", "-s", kc},
		{"unlock-keychain", "-p", "", kc},
		{"set-keychain-settings", kc},
	} {
		cmd := exec.Command("security", args...)
		cmd.Env = append(os.Environ(), "HOME="+m.SandboxHome)
		_ = cmd.Run()
	}
}

func (m *Manager) Start(port uint16, proxyURL string) error {
	if port == 8765 {
		return errors.New("refuse real Science port 8765")
	}
	if _, err := os.Stat(m.bin()); err != nil {
		return fmt.Errorf("Science binary not found: %s", m.bin())
	}
	if err := m.EnsureRuntimeAssets(); err != nil {
		return err
	}
	m.EnsureKeychain()
	authProxyURL := ProxyAuthURL(proxyURL)
	cmd := exec.Command(m.bin(), "serve", "--data-dir", m.DataDir(), "--port", fmt.Sprint(port), "--no-browser", "--no-auto-update", "--detached")
	cmd.Env = m.scienceEnv(
		"ANTHROPIC_BASE_URL="+proxyURL,
		"https_proxy="+authProxyURL,
		"HTTPS_PROXY="+authProxyURL,
		"no_proxy=127.0.0.1,localhost,::1",
		"NO_PROXY=127.0.0.1,localhost,::1",
	)
	return cmd.Run()
}

func ProxyAuthURL(proxyURL string) string {
	u, err := url.Parse(proxyURL)
	if err != nil || u.Host == "" {
		return proxyURL
	}
	secret := strings.Trim(strings.TrimPrefix(u.Path, "/"), "/")
	if i := strings.IndexByte(secret, '/'); i >= 0 {
		secret = secret[:i]
	}
	out := &url.URL{Scheme: "http", Host: u.Host}
	if secret != "" {
		out.User = url.UserPassword("csnative", secret)
	}
	return out.String()
}

func (m *Manager) Stop() error {
	cmd := exec.Command(m.bin(), "stop", "--data-dir", m.DataDir())
	cmd.Env = append(os.Environ(), "HOME="+m.SandboxHome)
	return cmd.Run()
}

func (m *Manager) URL() string {
	cmd := exec.Command(m.bin(), "url", "--data-dir", m.DataDir())
	cmd.Env = append(os.Environ(), "HOME="+m.SandboxHome)
	out, err := cmd.Output()
	if err == nil {
		if u, ok := FirstHTTPURL(string(out)); ok {
			return u
		}
	}
	return ""
}

func (m *Manager) Running(port uint16) bool {
	cmd := exec.Command(m.bin(), "status", "--data-dir", m.DataDir())
	cmd.Env = append(os.Environ(), "HOME="+m.SandboxHome)
	out, err := cmd.Output()
	if err != nil || !bytes.Contains(out, []byte(`"running": true`)) && !bytes.Contains(out, []byte(`"running":true`)) {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/health", port), nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	return res.StatusCode == 200
}

func (m *Manager) bin() string {
	if m.ScienceBin != "" {
		return m.ScienceBin
	}
	return ScienceBin
}

func (m *Manager) ensurePythonCompatibility() error {
	pythonEnv := filepath.Join(m.DataDir(), "conda", "envs", "python")
	info, err := os.Stat(pythonEnv)
	if err != nil || !info.IsDir() {
		return nil
	}
	python3Env := filepath.Join(filepath.Dir(pythonEnv), "python3")
	if _, err := os.Lstat(python3Env); errors.Is(err, os.ErrNotExist) {
		if err := os.Symlink("python", python3Env); err != nil {
			return err
		}
	}
	binDir := filepath.Join(pythonEnv, "bin")
	python := filepath.Join(binDir, "python")
	python3 := filepath.Join(binDir, "python3")
	if _, err := os.Lstat(python); errors.Is(err, os.ErrNotExist) {
		if _, err := os.Stat(python3); err == nil {
			if err := os.Symlink("python3", python); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) scienceEnv(extra ...string) []string {
	env := os.Environ()
	data := m.DataDir()
	pythonEnv := filepath.Join(data, "conda", "envs", "python")
	pythonBin := filepath.Join(pythonEnv, "bin")
	condaBin := filepath.Join(data, "conda", "bin")
	if info, err := os.Stat(pythonBin); err == nil && info.IsDir() {
		pathParts := []string{pythonBin}
		if info, err := os.Stat(condaBin); err == nil && info.IsDir() {
			pathParts = append(pathParts, condaBin)
		}
		if path := os.Getenv("PATH"); path != "" {
			pathParts = append(pathParts, path)
		}
		env = append(env,
			"PATH="+strings.Join(pathParts, string(os.PathListSeparator)),
			"CONDA_DEFAULT_ENV=python",
			"CONDA_PREFIX="+pythonEnv,
		)
	}
	env = append(env, "HOME="+m.SandboxHome)
	env = append(env, extra...)
	return env
}

func FirstHTTPURL(stdout string) (string, bool) {
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 {
			continue
		}
		if strings.HasPrefix(fields[0], "http://") || strings.HasPrefix(fields[0], "https://") {
			return fields[0], true
		}
	}
	return "", false
}

func RewritePublicURL(localURL, proto, host string) (string, error) {
	u, err := url.Parse(localURL)
	if err != nil {
		return "", err
	}
	if proto == "" {
		proto = "https"
	}
	if host == "" {
		return "", errors.New("missing host")
	}
	u.Scheme = proto
	u.Host = host
	return u.String(), nil
}

type PublicEntry struct {
	FreshURL func() (string, error)
	mu       sync.Mutex
	logs     bytes.Buffer
}

func (p *PublicEntry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.Error(w, "route only public root to this helper", http.StatusNotFound)
			return
		}
		fresh, err := p.FreshURL()
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		proto := strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]
		host := r.Header.Get("X-Forwarded-Host")
		if host == "" {
			host = r.Host
		}
		target, err := RewritePublicURL(fresh, proto, host)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		p.logf("%s %s -> 302", r.RemoteAddr, r.URL.Path)
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, target, http.StatusFound)
	})
}

func (p *PublicEntry) LogString() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.logs.String()
}

func (p *PublicEntry) logf(format string, args ...any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, _ = fmt.Fprintf(&p.logs, format+"\n", args...)
}

func StartPublicEntry(addr string, fresh func() (string, error)) (*http.Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	srv := &http.Server{Addr: addr, Handler: (&PublicEntry{FreshURL: fresh}).Handler()}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			_, _ = io.WriteString(os.Stderr, err.Error()+"\n")
		}
	}()
	return srv, nil
}
