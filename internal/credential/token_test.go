package credential

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shhac/agent-mongo/internal/config"
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

func fileFlowResolution(t *testing.T, path string) Resolution {
	t.Helper()
	return Resolution{
		Alias:      "eks",
		Kind:       config.KindOIDC,
		Credential: config.Credential{Flow: &config.Flow{Type: config.FlowFile, Path: path}},
	}
}

// AccessToken fetches when asked rather than caching, so a token rotated
// underneath a running process is picked up and a reauth re-reads it.
func TestAccessTokenRereadsTheFile(t *testing.T) {
	path := writeToken(t, "aGVhZGVy.eyJzdWIiOiJhIn0.c2ln")
	res := fileFlowResolution(t, path)

	first, err := res.AccessToken(context.Background(), "c0.abc.mongodb.net")
	if err != nil {
		t.Fatalf("first AccessToken: %v", err)
	}

	rotated := "aGVhZGVy.eyJzdWIiOiJiIn0.c2ln"
	if err := os.WriteFile(path, []byte(rotated), 0o600); err != nil {
		t.Fatalf("rotate token: %v", err)
	}
	second, err := res.AccessToken(context.Background(), "c0.abc.mongodb.net")
	if err != nil {
		t.Fatalf("second AccessToken: %v", err)
	}

	if first == second {
		t.Error("the token was cached; a rotated file would never be picked up")
	}
	if second != rotated {
		t.Errorf("AccessToken = %q, want the rotated token", second)
	}
}

func TestAccessTokenSurfacesAnUnreadableToken(t *testing.T) {
	res := fileFlowResolution(t, filepath.Join(t.TempDir(), "absent"))
	if _, err := res.AccessToken(context.Background(), "c0.abc.mongodb.net"); !errors.Is(err, ErrTokenUnreadable) {
		t.Errorf("error = %v, want ErrTokenUnreadable", err)
	}
}

// The platform-identity flows leave the driver to fetch the token, so asking
// them for one is a programming error and says so rather than returning "".
func TestAccessTokenRefusedForAFlowThatSuppliesNone(t *testing.T) {
	res := Resolution{
		Alias:      "ci",
		Kind:       config.KindOIDC,
		Credential: config.Credential{Flow: &config.Flow{Type: config.FlowEnvironment, Environment: "k8s"}},
	}
	if _, err := res.AccessToken(context.Background(), "c0.abc.mongodb.net"); err == nil {
		t.Error("an environment-flow credential yielded a token")
	}
}

func TestAccessTokenRejectsANonOIDCCredential(t *testing.T) {
	res := Resolution{Alias: "acme", Kind: config.KindSCRAM}
	if _, err := res.AccessToken(context.Background(), "c0.abc.mongodb.net"); err == nil {
		t.Error("a SCRAM credential yielded an OIDC token")
	}
}

// RFC 7519 says exp is a NumericDate, not an integer. Decoding straight to
// int64 failed on a float or exponent, which silently turned the expiry check
// off for exactly the issuers it exists to help.
func TestTokenExpiryAcceptsEveryNumericDateForm(t *testing.T) {
	frozen := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	expired := frozen.Add(-time.Hour).Unix()

	tests := []struct {
		name    string
		payload string
		expired bool
	}{
		{"integer", `{"exp":` + strconv.FormatInt(expired, 10) + `}`, true},
		{"float", `{"exp":` + strconv.FormatInt(expired, 10) + `.0}`, true},
		{"exponent", `{"exp":1.7568e9}`, true},
		{"fractional seconds", `{"exp":` + strconv.FormatInt(expired, 10) + `.75}`, true},
		{"future integer", `{"exp":` + strconv.FormatInt(frozen.Add(time.Hour).Unix(), 10) + `}`, false},
		// exp is optional; absent or unusable means the server decides.
		{"absent", `{"sub":"svc"}`, false},
		{"zero", `{"exp":0}`, false},
		{"not a number", `{"exp":true}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixedClock(t, frozen)
			enc := base64.RawURLEncoding.EncodeToString
			raw := enc([]byte(`{"alg":"RS256"}`)) + "." + enc([]byte(tt.payload)) + ".c2ln"

			_, _, err := ParseToken("the token file \"t\"", raw)
			if tt.expired && !errors.Is(err, ErrTokenExpired) {
				t.Errorf("ParseToken(%s) error = %v, want ErrTokenExpired", tt.payload, err)
			}
			if !tt.expired && err != nil {
				t.Errorf("ParseToken(%s) error = %v, want nil", tt.payload, err)
			}
		})
	}
}

// A JWT's segments are unpadded base64url, but a padded payload should still
// have its expiry read rather than be treated as having none.
func TestTokenExpiryAcceptsAPaddedPayload(t *testing.T) {
	frozen := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	fixedClock(t, frozen)

	payload := []byte(`{"exp":` + strconv.FormatInt(frozen.Add(-time.Hour).Unix(), 10) + `}`)
	raw := "aGVhZGVy." + base64.URLEncoding.EncodeToString(payload) + ".c2ln"

	if _, _, err := ParseToken("the token file \"t\"", raw); !errors.Is(err, ErrTokenExpired) {
		t.Errorf("error = %v, want ErrTokenExpired for a padded payload", err)
	}
}

// ParseToken reports the expiry it read, which is what a refresh decision needs.
func TestParseTokenReportsTheExpiry(t *testing.T) {
	frozen := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	fixedClock(t, frozen)
	want := frozen.Add(time.Hour)

	_, expiry, err := ParseToken("the session for \"corp\"", jwt(t, map[string]any{"exp": want.Unix()}))
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if !expiry.Equal(want) {
		t.Errorf("expiry = %s, want %s", expiry, want)
	}
}

// A payload that is not base64 at all leaves the expiry unknown rather than
// failing: the claim is optional and the server remains the authority.
func TestTokenExpiryIgnoresAnUndecodablePayload(t *testing.T) {
	fixedClock(t, time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))

	token, expiry, err := ParseToken("the token file \"t\"", "aGVhZGVy.!!!not-base64!!!.c2ln")
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if !expiry.IsZero() {
		t.Errorf("expiry = %s, want the zero time when it cannot be read", expiry)
	}
	if token == "" {
		t.Error("the token was discarded; only the expiry is unknown")
	}
}

func TestAccessTokenRejectsAnUnregisteredFlow(t *testing.T) {
	res := Resolution{
		Alias:      "corp",
		Kind:       config.KindOIDC,
		Credential: config.Credential{Flow: &config.Flow{Type: config.FlowType("browser")}},
	}
	if _, err := res.AccessToken(context.Background(), "c0.abc.mongodb.net"); !errors.Is(err, ErrInvalidFlow) {
		t.Errorf("error = %v, want ErrInvalidFlow", err)
	}
}
