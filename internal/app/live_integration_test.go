package app

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveAppQwenVerifyKeyStatusAndStop(t *testing.T) {
	key := os.Getenv("CSNATIVE_LIVE_API_KEY")
	if key == "" {
		t.Skip("set CSNATIVE_LIVE_API_KEY to run live app test")
	}
	proxyPort := freeTCPPort(t)
	sandboxPort := freeTCPPort(t)
	a := NewWithConfigDir(t.TempDir())
	if _, err := a.SetConfig(UISettings{Provider: "qwen", ProxyPort: proxyPort, SandboxPort: sandboxPort, PublicPort: freeTCPPort(t)}); err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}
	if _, err := a.SaveProviderKey("qwen", key); err != nil {
		t.Fatalf("SaveProviderKey() error = %v", err)
	}
	res, err := a.VerifyKey()
	if err != nil {
		t.Fatalf("VerifyKey() error = %v", err)
	}
	if ok, _ := res["ok"].(bool); !ok {
		t.Fatalf("VerifyKey() did not pass: %#v", res)
	}
	st := a.Status()
	if st.Proxy != "green" || st.Upstream != "green" {
		t.Fatalf("bad status after VerifyKey(): %#v", st)
	}
	if err := a.StopAll(); err != nil {
		t.Fatalf("StopAll() error = %v", err)
	}
	if st := a.Status(); st.Proxy == "green" {
		t.Fatalf("proxy still green after StopAll(): %#v", st)
	}
}

func TestLiveAppOneClickStartsSandboxAndPublicEntry(t *testing.T) {
	key := os.Getenv("CSNATIVE_LIVE_API_KEY")
	if key == "" || os.Getenv("CSNATIVE_LIVE_SCIENCE") != "1" {
		t.Skip("set CSNATIVE_LIVE_API_KEY and CSNATIVE_LIVE_SCIENCE=1 to run Science sandbox test")
	}
	proxyPort := freeTCPPort(t)
	sandboxPort := freeTCPPort(t)
	publicPort := preferredTCPPort(t, 8992, 18992, 19092, 29092)
	a := NewWithConfigDir(t.TempDir())
	t.Cleanup(func() { _ = a.StopAll() })
	if _, err := a.SetConfig(UISettings{Provider: "qwen", ProxyPort: proxyPort, SandboxPort: sandboxPort, PublicPort: publicPort}); err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}
	if _, err := a.SaveProviderKey("qwen", key); err != nil {
		t.Fatalf("SaveProviderKey() error = %v", err)
	}
	res, err := a.OneClickLogin()
	if err != nil {
		t.Fatalf("OneClickLogin() error = %v", err)
	}
	if action, _ := res["action"].(string); action != "started" && action != "reopened" {
		t.Fatalf("unexpected one-click result: %#v", res)
	}
	if !httpOK(fmt.Sprintf("http://127.0.0.1:%d/health", sandboxPort), 5*time.Second) {
		t.Fatalf("sandbox health did not become ready on %d", sandboxPort)
	}
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/", publicPort), nil)
	req.Host = "csnative.example"
	req.Header.Set("X-Forwarded-Proto", "https")
	client := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	redir, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer redir.Body.Close()
	loc := redir.Header.Get("Location")
	if redir.StatusCode != http.StatusFound || !strings.HasPrefix(loc, "https://csnative.example/") || !strings.Contains(loc, "nonce=") {
		t.Fatalf("bad public entry redirect status=%d location=%q", redir.StatusCode, loc)
	}
}

func freeTCPPort(t *testing.T) uint16 {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return uint16(ln.Addr().(*net.TCPAddr).Port)
}

func preferredTCPPort(t *testing.T, ports ...uint16) uint16 {
	t.Helper()
	for _, port := range ports {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			_ = ln.Close()
			return port
		}
	}
	return freeTCPPort(t)
}

func httpOK(raw string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		res, err := http.Get(raw)
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
