package oauth

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncryptDecryptTokenV2RoundTrip(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	body, err := EncryptTokenV2([]byte(`{"email":"virtual@localhost.invalid"}`), key)
	if err != nil {
		t.Fatalf("EncryptTokenV2() error = %v", err)
	}
	if !strings.HasPrefix(body, "v2:") {
		t.Fatalf("ciphertext prefix = %q", body[:3])
	}
	plain, err := DecryptTokenV2(body, key)
	if err != nil {
		t.Fatalf("DecryptTokenV2() error = %v", err)
	}
	if string(plain) != `{"email":"virtual@localhost.invalid"}` {
		t.Fatalf("plain = %s", plain)
	}
}

func TestEnsureVirtualLoginReusesOrgAndRejectsRealDir(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sandbox", "home")
	auth := filepath.Join(root, ".claude-science")
	real := filepath.Join(t.TempDir(), ".claude-science")
	if err := os.MkdirAll(real, 0o700); err != nil {
		t.Fatal(err)
	}

	first, action, err := EnsureVirtualLoginGuarded(auth, "virtual@localhost.invalid", root, real)
	if err != nil {
		t.Fatalf("EnsureVirtualLoginGuarded() error = %v", err)
	}
	if action != LoginCreated {
		t.Fatalf("first action = %v, want created", action)
	}
	second, action, err := EnsureVirtualLoginGuarded(auth, "virtual@localhost.invalid", root, real)
	if err != nil {
		t.Fatalf("second EnsureVirtualLoginGuarded() error = %v", err)
	}
	if action != LoginReused || second.OrgUUID != first.OrgUUID || second.AccountUUID != first.AccountUUID {
		t.Fatalf("expected reuse of org/account; first=%#v second=%#v action=%v", first, second, action)
	}
	if _, _, err := EnsureVirtualLoginGuarded(real, "virtual@localhost.invalid", root, real); err == nil {
		t.Fatal("real credential dir was accepted")
	}
}
