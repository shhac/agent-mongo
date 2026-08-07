package credential

import (
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/shhac/agent-mongo/internal/config"
)

// fakeKeychain is an in-memory keychainStore for exercising the sentinel and
// upgrade paths without touching a real OS keychain.
//
// The mutex is what keeps the concurrency tests honest: without it the race
// detector would trip on these maps and report the test double instead of the
// code under test.
type fakeKeychain struct {
	mu        sync.Mutex
	entries   map[string]string
	failSet   map[string]bool // account → force Set error
	deletes   []string
	available bool
}

func newFakeKeychain() *fakeKeychain {
	return &fakeKeychain{entries: map[string]string{}, failSet: map[string]bool{}, available: true}
}

func (f *fakeKeychain) Available() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.available
}

func (f *fakeKeychain) Get(account string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.entries[account]
	return v, ok
}

func (f *fakeKeychain) Set(account, secret string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failSet[account] {
		return errors.New("keychain write denied")
	}
	f.entries[account] = secret
	return nil
}

func (f *fakeKeychain) Delete(account string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletes = append(f.deletes, account)
	delete(f.entries, account)
	return nil
}

func swapKeychain(t *testing.T, fake *fakeKeychain) {
	t.Helper()
	prev := keychain
	keychain = fake
	t.Cleanup(func() { keychain = prev })
}

func isolateConfig(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

func captureStderr(t *testing.T) func() string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	return func() string {
		_ = w.Close()
		os.Stderr = orig
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		return string(buf[:n])
	}
}

func TestStoreKeychainRoundTrip(t *testing.T) {
	isolateConfig(t)
	fake := newFakeKeychain()
	swapKeychain(t, fake)

	storage, err := Store("acme", config.Credential{Username: "deploy", Password: "s3cret"})
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if storage != StorageKeychain {
		t.Fatalf("storage: got %q, want keychain", storage)
	}

	entry := config.Read().Credentials["acme"]
	if entry.Username != "__KEYCHAIN__" || entry.Password != "__KEYCHAIN__" {
		t.Fatalf("config entry should hold sentinels, got %+v", entry)
	}
	if StorageType("acme") != StorageKeychain {
		t.Errorf("StorageType: got %q", StorageType("acme"))
	}

	cred, ok := Get("acme")
	if !ok || cred.Username != "deploy" || cred.Password != "s3cret" {
		t.Fatalf("Get round-trip failed: %+v ok=%v", cred, ok)
	}
}

func TestGetUpgradesPlaintextToKeychain(t *testing.T) {
	isolateConfig(t)
	fake := newFakeKeychain()
	swapKeychain(t, fake)

	cfg := config.Read()
	cfg.Credentials = map[string]config.Credential{
		"legacy": {Username: "user", Password: "pass"},
	}
	if err := config.Write(cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	read := captureStderr(t)
	cred, ok := Get("legacy")
	stderr := read()

	if !ok || cred.Username != "user" || cred.Password != "pass" {
		t.Fatalf("Get: %+v ok=%v", cred, ok)
	}
	if !strings.Contains(stderr, `"notice"`) || !strings.Contains(stderr, "upgraded") {
		t.Errorf("expected upgrade notice on stderr, got %q", stderr)
	}
	entry := config.Read().Credentials["legacy"]
	if entry.Username != "__KEYCHAIN__" || entry.Password != "__KEYCHAIN__" {
		t.Fatalf("config should now hold sentinels, got %+v", entry)
	}
	if v, _ := fake.Get("password:legacy"); v != "pass" {
		t.Errorf("keychain should hold the secret, got %q", v)
	}

	// Second read resolves via keychain silently.
	read = captureStderr(t)
	if _, ok := Get("legacy"); !ok {
		t.Fatal("second Get failed")
	}
	if second := read(); second != "" {
		t.Errorf("second Get should be silent, got %q", second)
	}
}

func TestGetSentinelWithoutKeychainEntry(t *testing.T) {
	isolateConfig(t)
	fake := newFakeKeychain()
	swapKeychain(t, fake)

	cfg := config.Read()
	cfg.Credentials = map[string]config.Credential{
		"ghost": {Username: "__KEYCHAIN__", Password: "__KEYCHAIN__"},
	}
	if err := config.Write(cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	if _, ok := Get("ghost"); ok {
		t.Fatal("Get should fail when sentinel has no keychain entry")
	}
}

func TestStorePartialWriteFallsBackToPlaintext(t *testing.T) {
	isolateConfig(t)
	fake := newFakeKeychain()
	fake.failSet["password:flaky"] = true
	swapKeychain(t, fake)

	storage, err := Store("flaky", config.Credential{Username: "user", Password: "pass"})
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if storage != StorageConfig {
		t.Fatalf("storage: got %q, want config fallback", storage)
	}

	entry := config.Read().Credentials["flaky"]
	if entry.Username != "user" || entry.Password != "pass" {
		t.Fatalf("plaintext fallback entry wrong: %+v", entry)
	}
	// The partial username entry must have been cleaned up.
	if _, ok := fake.Get("username:flaky"); ok {
		t.Error("partial keychain entry should have been deleted")
	}
	wantDeletes := map[string]bool{"username:flaky": true, "password:flaky": true}
	for _, account := range fake.deletes {
		delete(wantDeletes, account)
	}
	if len(wantDeletes) > 0 {
		t.Errorf("missing cleanup deletes: %v", wantDeletes)
	}
}

func TestUpgradeSkippedWhenKeychainUnavailable(t *testing.T) {
	isolateConfig(t)
	fake := newFakeKeychain()
	fake.available = false
	swapKeychain(t, fake)

	cfg := config.Read()
	cfg.Credentials = map[string]config.Credential{
		"plain": {Username: "user", Password: "pass"},
	}
	if err := config.Write(cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	cred, ok := Get("plain")
	if !ok || cred.Password != "pass" {
		t.Fatalf("Get: %+v ok=%v", cred, ok)
	}
	entry := config.Read().Credentials["plain"]
	if entry.Username != "user" {
		t.Fatalf("plaintext entry must stay untouched, got %+v", entry)
	}
}
