package oauth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"crypto/sha256"
	"golang.org/x/crypto/hkdf"
)

const (
	hkdfInfo = "operon:aes-256-gcm:oauth"
	aad      = "v2:oauth"
)

var keyNames = []string{
	"ANTHROPIC_API_KEY_ENCRYPTION_KEY",
	"OAUTH_ENCRYPTION_KEY",
	"JWT_SIGNING_SECRET",
	"USER_SECRET_ENCRYPTION_KEY",
}

type LoginAction string

const (
	LoginReused   LoginAction = "reused"
	LoginRepaired LoginAction = "repaired"
	LoginCreated  LoginAction = "created"
)

type Result struct {
	AuthDir     string
	AccountUUID string
	OrgUUID     string
	EncFile     string
}

func EncryptTokenV2(plaintext []byte, oauthKeyB64 string) (string, error) {
	key, err := deriveKey(oauthKeyB64)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	iv := randomBytes(12)
	ct := gcm.Seal(nil, iv, plaintext, []byte(aad))
	raw := append(append([]byte{}, iv...), ct...)
	return "v2:" + base64.StdEncoding.EncodeToString(raw), nil
}

func DecryptTokenV2(body, oauthKeyB64 string) ([]byte, error) {
	if !strings.HasPrefix(body, "v2:") {
		return nil, errors.New("missing v2 prefix")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(body, "v2:"))
	if err != nil {
		return nil, err
	}
	if len(raw) < 28 {
		return nil, errors.New("ciphertext too short")
	}
	key, err := deriveKey(oauthKeyB64)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, raw[:12], raw[12:], []byte(aad))
}

func EnsureVirtualLogin(authDir, email, sandboxRoot string) (Result, LoginAction, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Result{}, "", err
	}
	return EnsureVirtualLoginGuarded(authDir, email, sandboxRoot, filepath.Join(home, ".claude-science"))
}

func EnsureVirtualLoginGuarded(authDir, email, sandboxRoot, realCredDir string) (Result, LoginAction, error) {
	resolved, err := resolveGuarded(authDir, email, sandboxRoot, realCredDir)
	if err != nil {
		return Result{}, "", err
	}
	if r, ok := readIntact(resolved, email); ok {
		return r, LoginReused, nil
	}
	priorOrg := readActiveOrg(resolved)
	action := LoginRepaired
	if priorOrg == "" {
		priorOrg = readTokenOrg(resolved)
	}
	if priorOrg == "" {
		orgs := scanOrgDirs(resolved)
		switch len(orgs) {
		case 0:
			action = LoginCreated
		case 1:
			priorOrg = orgs[0]
		default:
			return Result{}, "", fmt.Errorf("检测到 %d 个历史组织，无法确定活动组织", len(orgs))
		}
	}
	account := readPriorAccount(resolved)
	r, err := writeLogin(resolved, email, priorOrg, account)
	return r, action, err
}

func LoginIntact(authDir, email, sandboxRoot string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	resolved, err := resolveGuarded(authDir, email, sandboxRoot, filepath.Join(home, ".claude-science"))
	if err != nil {
		return false
	}
	_, ok := readIntact(resolved, email)
	return ok
}

func writeLogin(authDir, email, preferOrg, preferAccount string) (Result, error) {
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		return Result{}, err
	}
	_ = os.Chmod(authDir, 0o700)
	keys := readKeys(filepath.Join(authDir, "encryption.key"))
	if !validB64Key(keys["OAUTH_ENCRYPTION_KEY"]) {
		delete(keys, "OAUTH_ENCRYPTION_KEY")
	}
	for _, name := range keyNames {
		if keys[name] == "" {
			keys[name] = base64.StdEncoding.EncodeToString(randomBytes(32))
		}
	}
	if err := safeWrite(filepath.Join(authDir, "encryption.key"), []byte(renderKeys(keys)), 0o600); err != nil {
		return Result{}, err
	}
	account := preferAccount
	if account == "" {
		account = uuidV4()
	}
	org := preferOrg
	if org == "" {
		org = uuidV4()
	}
	blob := map[string]any{
		"access_token":            "sk-ant-virtual-" + hex.EncodeToString(randomBytes(24)),
		"refresh_token":           "",
		"api_key":                 nil,
		"token_expires_at":        "2099-01-01T00:00:00.000Z",
		"provider":                "claude_ai",
		"scopes":                  "user:inference user:file_upload user:profile user:mcp_servers user:plugins",
		"email":                   email,
		"account_uuid":            account,
		"subscription_type":       "max",
		"rate_limit_tier":         nil,
		"seat_tier":               nil,
		"org_uuid":                org,
		"billing_type":            nil,
		"has_extra_usage_enabled": false,
	}
	plain, _ := json.Marshal(blob)
	enc, err := EncryptTokenV2(plain, keys["OAUTH_ENCRYPTION_KEY"])
	if err != nil {
		return Result{}, err
	}
	tokDir := filepath.Join(authDir, ".oauth-tokens")
	if err := os.MkdirAll(tokDir, 0o700); err != nil {
		return Result{}, err
	}
	_ = os.Chmod(tokDir, 0o700)
	entries, _ := os.ReadDir(tokDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".enc") {
			if err := os.Remove(filepath.Join(tokDir, e.Name())); err != nil {
				return Result{}, err
			}
		}
	}
	encFile := filepath.Join(tokDir, sanitize(account)+".enc")
	if err := safeWrite(encFile, []byte(enc), 0o600); err != nil {
		return Result{}, err
	}
	active, _ := json.MarshalIndent(map[string]string{"org_uuid": org}, "", "  ")
	active = append(active, '\n')
	if err := safeWrite(filepath.Join(authDir, "active-org.json"), active, 0o600); err != nil {
		return Result{}, err
	}
	return Result{AuthDir: authDir, AccountUUID: account, OrgUUID: org, EncFile: encFile}, nil
}

func readIntact(authDir, email string) (Result, bool) {
	if isSymlink(filepath.Join(authDir, "encryption.key")) || isSymlink(filepath.Join(authDir, ".oauth-tokens")) || isSymlink(filepath.Join(authDir, "active-org.json")) {
		return Result{}, false
	}
	key := readKeys(filepath.Join(authDir, "encryption.key"))["OAUTH_ENCRYPTION_KEY"]
	encFile := singleEnc(authDir)
	if key == "" || encFile == "" || isSymlink(encFile) {
		return Result{}, false
	}
	body, err := os.ReadFile(encFile)
	if err != nil {
		return Result{}, false
	}
	plain, err := DecryptTokenV2(string(body), key)
	if err != nil {
		return Result{}, false
	}
	var blob map[string]any
	if json.Unmarshal(plain, &blob) != nil {
		return Result{}, false
	}
	activeOrg := readActiveOrg(authDir)
	account, _ := blob["account_uuid"].(string)
	org, _ := blob["org_uuid"].(string)
	gotEmail, _ := blob["email"].(string)
	prov, _ := blob["provider"].(string)
	access, _ := blob["access_token"].(string)
	exp, _ := blob["token_expires_at"].(string)
	if activeOrg == "" || activeOrg != org || gotEmail != email || !strings.HasSuffix(gotEmail, "localhost.invalid") || !looksUUID(account) || prov != "claude_ai" || access == "" || !notExpired(exp) {
		return Result{}, false
	}
	return Result{AuthDir: authDir, AccountUUID: account, OrgUUID: org, EncFile: encFile}, true
}

func resolveGuarded(authDir, email, sandboxRoot, realCredDir string) (string, error) {
	resolved := realAncestor(authDir)
	realRoot := realAncestor(realCredDir)
	if hasPathPrefix(resolved, realRoot) {
		return "", fmt.Errorf("refuse real Science credential dir: %s", realRoot)
	}
	root := realAncestor(sandboxRoot)
	if !hasPathPrefix(resolved, root) {
		return "", fmt.Errorf("auth dir outside sandbox root")
	}
	if !strings.HasSuffix(email, "localhost.invalid") {
		return "", fmt.Errorf("email must end with localhost.invalid")
	}
	return resolved, nil
}

func deriveKey(oauthKeyB64 string) ([]byte, error) {
	ikm, err := base64.StdEncoding.DecodeString(strings.TrimSpace(oauthKeyB64))
	if err != nil {
		return nil, err
	}
	r := hkdf.New(sha256.New, ikm, []byte{}, []byte(hkdfInfo))
	out := make([]byte, 32)
	_, err = io.ReadFull(r, out)
	return out, err
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

func uuidV4() string {
	b := randomBytes(16)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func readKeys(path string) map[string]string {
	out := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(k) != "" {
			out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return out
}

func renderKeys(keys map[string]string) string {
	var b strings.Builder
	for _, k := range keyNames {
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(keys[k])
		b.WriteString("\n")
	}
	return b.String()
}

func validB64Key(v string) bool {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(v))
	return err == nil && len(b) >= 16
}

func safeWrite(path string, data []byte, mode os.FileMode) error {
	if isSymlink(path) {
		return fmt.Errorf("refuse symlink: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := filepath.Join(filepath.Dir(path), ".tmp-"+hex.EncodeToString(randomBytes(6)))
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
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
	return os.Chmod(path, mode)
}

func singleEnc(authDir string) string {
	tok := filepath.Join(authDir, ".oauth-tokens")
	entries, err := os.ReadDir(tok)
	if err != nil {
		return ""
	}
	found := ""
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".enc") {
			if found != "" {
				return ""
			}
			found = filepath.Join(tok, e.Name())
		}
	}
	return found
}

func readActiveOrg(authDir string) string {
	var v map[string]string
	if json.Unmarshal(mustRead(filepath.Join(authDir, "active-org.json")), &v) != nil || !looksUUID(v["org_uuid"]) {
		return ""
	}
	return v["org_uuid"]
}

func readPriorAccount(authDir string) string { return readTokenField(authDir, "account_uuid") }
func readTokenOrg(authDir string) string     { return readTokenField(authDir, "org_uuid") }

func readTokenField(authDir, field string) string {
	key := readKeys(filepath.Join(authDir, "encryption.key"))["OAUTH_ENCRYPTION_KEY"]
	encFile := singleEnc(authDir)
	if key == "" || encFile == "" {
		return ""
	}
	plain, err := DecryptTokenV2(string(mustRead(encFile)), key)
	if err != nil {
		return ""
	}
	var v map[string]any
	if json.Unmarshal(plain, &v) != nil {
		return ""
	}
	s, _ := v[field].(string)
	if looksUUID(s) {
		return s
	}
	return ""
}

func scanOrgDirs(authDir string) []string {
	entries, err := os.ReadDir(filepath.Join(authDir, "orgs"))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && looksUUID(e.Name()) {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

func mustRead(path string) []byte {
	b, _ := os.ReadFile(path)
	return b
}

func looksUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

func notExpired(s string) bool {
	if len(s) < 10 {
		return false
	}
	d, err := time.Parse("2006-01-02", s[:10])
	if err != nil {
		return false
	}
	today, _ := time.Parse("2006-01-02", time.Now().UTC().Format("2006-01-02"))
	return !d.Before(today)
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func realAncestor(path string) string {
	cur := filepath.Clean(path)
	var tail []string
	for {
		if _, err := os.Lstat(cur); err == nil {
			break
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		tail = append(tail, filepath.Base(cur))
		cur = parent
	}
	base, err := filepath.EvalSymlinks(cur)
	if err != nil {
		base = cur
	}
	for i := len(tail) - 1; i >= 0; i-- {
		base = filepath.Join(base, tail[i])
	}
	return filepath.Clean(base)
}

func hasPathPrefix(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	return path == root || strings.HasPrefix(path, root+string(os.PathSeparator))
}

func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}
