package credential

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixedClock pins the package clock for the duration of a test, so an expiry
// check neither sleeps nor depends on wall time.
func fixedClock(t *testing.T, at time.Time) {
	t.Helper()
	prev := now
	now = func() time.Time { return at }
	t.Cleanup(func() { now = prev })
}

// jwt builds a token whose payload carries the given claims. Only the payload
// is meaningful here: nothing in agent-mongo verifies the signature, which is
// the server's job.
func jwt(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"RS256"}`)) + "." + enc(payload) + ".c2ln"
}

func writeToken(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write token: %v", err)
	}
	return path
}

func TestReadTokenFile(t *testing.T) {
	frozen := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)

	t.Run("reads a valid token", func(t *testing.T) {
		fixedClock(t, frozen)
		token := jwt(t, map[string]any{"exp": frozen.Add(time.Hour).Unix()})
		got, err := ReadTokenFile(writeToken(t, token))
		if err != nil {
			t.Fatalf("ReadTokenFile: %v", err)
		}
		if got != token {
			t.Errorf("token = %q, want %q", got, token)
		}
	})

	t.Run("trims the trailing newline a writer leaves", func(t *testing.T) {
		fixedClock(t, frozen)
		token := jwt(t, map[string]any{"exp": frozen.Add(time.Hour).Unix()})
		got, err := ReadTokenFile(writeToken(t, token+"\n"))
		if err != nil {
			t.Fatalf("ReadTokenFile: %v", err)
		}
		if got != token {
			t.Errorf("token = %q, want it trimmed", got)
		}
	})

	t.Run("accepts a token with no exp claim", func(t *testing.T) {
		fixedClock(t, frozen)
		// exp is optional; the server remains the authority.
		if _, err := ReadTokenFile(writeToken(t, jwt(t, map[string]any{"sub": "svc"}))); err != nil {
			t.Errorf("ReadTokenFile: %v", err)
		}
	})

	t.Run("accepts a payload it cannot decode", func(t *testing.T) {
		fixedClock(t, frozen)
		// Shape is right, payload is not JSON: still the server's call.
		if _, err := ReadTokenFile(writeToken(t, "aGVhZGVy.bm90LWpzb24.c2ln")); err != nil {
			t.Errorf("ReadTokenFile: %v", err)
		}
	})

	t.Run("rejects an expired token", func(t *testing.T) {
		fixedClock(t, frozen)
		expiry := frozen.Add(-time.Minute)
		_, err := ReadTokenFile(writeToken(t, jwt(t, map[string]any{"exp": expiry.Unix()})))
		if !errors.Is(err, ErrTokenExpired) {
			t.Fatalf("error = %v, want ErrTokenExpired", err)
		}
		// The point of checking here rather than letting the server refuse is
		// that the message says what to do about it.
		if !strings.Contains(err.Error(), "2026-09-02T11:59:00Z") {
			t.Errorf("error = %q, want it to name the expiry instant", err)
		}
	})

	t.Run("rejects a token expiring exactly now", func(t *testing.T) {
		fixedClock(t, frozen)
		_, err := ReadTokenFile(writeToken(t, jwt(t, map[string]any{"exp": frozen.Unix()})))
		if !errors.Is(err, ErrTokenExpired) {
			t.Errorf("error = %v, want a token expiring now to be expired", err)
		}
	})

	t.Run("rejects a missing file", func(t *testing.T) {
		_, err := ReadTokenFile(filepath.Join(t.TempDir(), "absent"))
		if !errors.Is(err, ErrTokenUnreadable) {
			t.Fatalf("error = %v, want ErrTokenUnreadable", err)
		}
	})

	t.Run("rejects contents that are not a JWT", func(t *testing.T) {
		for _, contents := range []string{"", "not-a-jwt", "only.two", "a.b.c.d", ".b.c", "a..c", "a.b."} {
			if _, err := ReadTokenFile(writeToken(t, contents)); !errors.Is(err, ErrTokenUnreadable) {
				t.Errorf("ReadTokenFile(%q) error = %v, want ErrTokenUnreadable", contents, err)
			}
		}
	})
}
