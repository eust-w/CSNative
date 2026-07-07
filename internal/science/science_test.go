package science

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFirstHTTPURLAndPublicRewrite(t *testing.T) {
	raw := "Open this link:\nhttp://localhost:8990/?nonce=abc123\n(single-use)"
	u, ok := FirstHTTPURL(raw)
	if !ok || u != "http://localhost:8990/?nonce=abc123" {
		t.Fatalf("FirstHTTPURL() = %q %v", u, ok)
	}
	out, err := RewritePublicURL(u, "https", "csnative.example")
	if err != nil {
		t.Fatal(err)
	}
	if out != "https://csnative.example/?nonce=abc123" {
		t.Fatalf("rewrite = %q", out)
	}
}

func TestPublicEntryRedirectsWithoutLoggingNonce(t *testing.T) {
	entry := &PublicEntry{FreshURL: func() (string, error) {
		return "http://127.0.0.1:8990/?nonce=secret", nil
	}}
	ts := httptest.NewServer(entry.Handler())
	defer ts.Close()
	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Host = "csnative.example"
	req.Header.Set("X-Forwarded-Proto", "https")
	res, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	loc := res.Header.Get("Location")
	if res.StatusCode != http.StatusFound || loc != "https://csnative.example/?nonce=secret" {
		t.Fatalf("status=%d location=%q", res.StatusCode, loc)
	}
	if strings.Contains(entry.LogString(), "secret") {
		t.Fatalf("nonce leaked to log: %q", entry.LogString())
	}
}

func TestStartPublicEntryReportsBindError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	srv, err := StartPublicEntry(ln.Addr().String(), func() (string, error) {
		return "http://127.0.0.1:8990/?nonce=secret", nil
	})
	if err == nil {
		_ = srv.Close()
		t.Fatal("StartPublicEntry unexpectedly succeeded on an occupied address")
	}
}

func TestProxyAuthURLUsesSecretPathAsBasicAuth(t *testing.T) {
	got := ProxyAuthURL("http://127.0.0.1:18991/sec/v1/messages")
	if got != "http://csnative:sec@127.0.0.1:18991" {
		t.Fatalf("ProxyAuthURL() = %q", got)
	}
	got = ProxyAuthURL("http://127.0.0.1:18991")
	if got != "http://127.0.0.1:18991" {
		t.Fatalf("ProxyAuthURL() without secret = %q", got)
	}
}

func TestDefaultScienceBinUsesEnvironmentOverride(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "custom-claude-science")
	t.Setenv(ScienceBinEnv, "  "+bin+"  ")

	if got := DefaultScienceBin(); got != bin {
		t.Fatalf("DefaultScienceBin() = %q, want %q", got, bin)
	}
	if got := NewManager(t.TempDir()).ScienceBin; got != bin {
		t.Fatalf("NewManager().ScienceBin = %q, want %q", got, bin)
	}
}

func TestDefaultScienceBinFindsClaudeScienceOnLinuxPath(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("PATH discovery is only a Linux default")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "claude-science")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ScienceBinEnv, "")
	t.Setenv("PATH", dir)

	if got := DefaultScienceBin(); got != bin {
		t.Fatalf("DefaultScienceBin() = %q, want %q", got, bin)
	}
}

func TestEnsureRuntimeAssetsAddsPythonCompatibilityAliases(t *testing.T) {
	m := &Manager{SandboxHome: t.TempDir()}
	data := m.DataDir()
	pythonBin := filepath.Join(data, "conda", "envs", "python", "bin")
	if err := os.MkdirAll(filepath.Join(data, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(pythonBin, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pythonBin, "python3"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := m.EnsureRuntimeAssets(); err != nil {
		t.Fatalf("EnsureRuntimeAssets() error = %v", err)
	}
	if target, err := os.Readlink(filepath.Join(data, "conda", "envs", "python3")); err != nil || target != "python" {
		t.Fatalf("python3 env alias target=%q err=%v", target, err)
	}
	if target, err := os.Readlink(filepath.Join(pythonBin, "python")); err != nil || target != "python3" {
		t.Fatalf("python executable alias target=%q err=%v", target, err)
	}
}

func TestScienceEnvIncludesSandboxPythonPath(t *testing.T) {
	m := &Manager{SandboxHome: t.TempDir()}
	pythonBin := filepath.Join(m.DataDir(), "conda", "envs", "python", "bin")
	condaBin := filepath.Join(m.DataDir(), "conda", "bin")
	if err := os.MkdirAll(pythonBin, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(condaBin, 0o700); err != nil {
		t.Fatal(err)
	}
	env := m.scienceEnv("ANTHROPIC_BASE_URL=http://127.0.0.1:1/sec")
	path := envValue(env, "PATH")
	if !strings.HasPrefix(path, pythonBin+string(os.PathListSeparator)+condaBin+string(os.PathListSeparator)) {
		t.Fatalf("PATH does not start with sandbox Python: %q", path)
	}
	if got := envValue(env, "CONDA_DEFAULT_ENV"); got != "python" {
		t.Fatalf("CONDA_DEFAULT_ENV=%q", got)
	}
	if got := envValue(env, "CONDA_PREFIX"); got != filepath.Join(m.DataDir(), "conda", "envs", "python") {
		t.Fatalf("CONDA_PREFIX=%q", got)
	}
	if got := envValue(env, "HOME"); got != m.SandboxHome {
		t.Fatalf("HOME=%q", got)
	}
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], prefix) {
			return strings.TrimPrefix(env[i], prefix)
		}
	}
	return ""
}
