package cli

import (
	"io"
	"strings"
	"testing"
	"time"

	out "github.com/shhac/lib-agent-output"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/testutil"
)

func seed(t *testing.T, connAlias, credAlias string) {
	t.Helper()
	if credAlias != "" {
		testutil.StageCredential(t, credAlias, config.Credential{
			Kind: config.KindOIDC, Flow: &config.Flow{Type: config.FlowDevice},
		})
	}
	err := config.StoreConnection(connAlias, config.Connection{
		ConnectionString: "mongodb+srv://" + connAlias + ".abc.mongodb.net/app",
		Name:             connAlias,
		Credential:       credAlias,
	})
	if err != nil {
		t.Fatalf("seeding %q: %v", connAlias, err)
	}
}

// Which deployment to log in against decides where the session is bound, so
// guessing wrong is not a cosmetic mistake.
func TestConnectionForLogin(t *testing.T) {
	t.Run("the sole user needs no flag", func(t *testing.T) {
		testutil.IsolateConfig(t)
		seed(t, "prod", "corp")

		conn, alias, err := connectionForLogin("corp", "")
		if err != nil {
			t.Fatalf("connectionForLogin: %v", err)
		}
		if alias != "prod" || !strings.Contains(conn.ConnectionString, "prod") {
			t.Errorf("chose %q (%q), want prod", alias, conn.ConnectionString)
		}
	})

	t.Run("an explicit flag wins", func(t *testing.T) {
		testutil.IsolateConfig(t)
		seed(t, "prod", "corp")
		seed(t, "staging", "")

		_, alias, err := connectionForLogin("corp", "staging")
		if err != nil {
			t.Fatalf("connectionForLogin: %v", err)
		}
		if alias != "staging" {
			t.Errorf("chose %q, want the named connection", alias)
		}
	})

	t.Run("an unknown flag value is refused", func(t *testing.T) {
		testutil.IsolateConfig(t)
		seed(t, "prod", "corp")

		if _, _, err := connectionForLogin("corp", "nope"); err == nil {
			t.Error("an unknown connection was accepted")
		}
	})

	t.Run("no connection at all", func(t *testing.T) {
		testutil.IsolateConfig(t)
		testutil.StageCredential(t, "corp", config.Credential{
			Kind: config.KindOIDC, Flow: &config.Flow{Type: config.FlowDevice},
		})

		_, _, err := connectionForLogin("corp", "")
		if err == nil {
			t.Fatal("a login with nothing to log in against succeeded")
		}
		if !strings.Contains(err.Error(), "No connection uses") {
			t.Errorf("error = %q", err)
		}
	})

	// Guessing here would bind the session to the wrong host, so the choice is
	// handed back with the list.
	t.Run("several connections are ambiguous", func(t *testing.T) {
		testutil.IsolateConfig(t)
		seed(t, "prod", "corp")
		seed(t, "staging", "corp")

		_, _, err := connectionForLogin("corp", "")
		if err == nil {
			t.Fatal("an ambiguous login was resolved by guessing")
		}
		hint := hintOf(t, err)
		for _, want := range []string{"prod", "staging", "--connection"} {
			if !strings.Contains(hint, want) {
				t.Errorf("hint = %q, want it to name %q", hint, want)
			}
		}
	})
}

// Every other command takes -c; a local --connection flag here shadowed the
// root's persistent one, and cobra drops a persistent flag whose name is
// already taken, so "credential login corp -c prod" failed outright.
func TestLoginAcceptsTheGlobalConnectionFlag(t *testing.T) {
	testutil.IsolateConfig(t)
	seed(t, "prod", "corp")

	root := newRootCmd("test")
	root.SetArgs([]string{"credential", "login", "corp", "-c", "prod"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	// It gets as far as connecting, which is as far as it can go here. What
	// matters is that flag parsing did not reject -c.
	err := root.Execute()
	if err != nil && strings.Contains(err.Error(), "unknown shorthand flag") {
		t.Fatalf("-c was rejected: %v", err)
	}
	if err != nil && strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("flag parsing failed: %v", err)
	}
}

func TestSessionExpiry(t *testing.T) {
	expiry := time.Now().Add(time.Hour).UTC().Truncate(time.Second)

	t.Run("reported when there is a session", func(t *testing.T) {
		testutil.IsolateConfig(t)
		seed(t, "prod", "corp")
		if err := credential.SaveSession("corp", credential.Session{
			AccessToken: "t", ExpiresAt: expiry, Host: "prod.abc.mongodb.net",
		}); err != nil {
			t.Fatalf("SaveSession: %v", err)
		}

		credAlias, got, ok := sessionExpiry("prod")
		if !ok {
			t.Fatal("no expiry reported for a logged-in credential")
		}
		if credAlias != "corp" || !got.Equal(expiry) {
			t.Errorf("got %q/%s, want corp/%s", credAlias, got, expiry)
		}
	})

	t.Run("silent for credentials with no session", func(t *testing.T) {
		testutil.IsolateConfig(t)
		seed(t, "prod", "corp")
		if _, _, ok := sessionExpiry("prod"); ok {
			t.Error("an expiry was reported before anyone logged in")
		}

		seed(t, "plain", "")
		if _, _, ok := sessionExpiry("plain"); ok {
			t.Error("an expiry was reported for a connection with no credential")
		}
		if _, _, ok := sessionExpiry("absent"); ok {
			t.Error("an expiry was reported for a connection that does not exist")
		}
	})
}

// The receipt is what an agent reads back; it must carry the binding and never
// a token.
func TestLoginReceipt(t *testing.T) {
	testutil.IsolateConfig(t)
	seed(t, "prod", "corp")
	expiry := time.Date(2026, 9, 2, 13, 0, 0, 0, time.UTC)

	receipt := loginReceipt("corp", "prod", credential.Session{
		AccessToken: "secret-token", RefreshToken: "secret-refresh",
		ExpiresAt: expiry, Issuer: "https://idp.example.com", Host: "prod.abc.mongodb.net",
	})

	if receipt["host"] != "prod.abc.mongodb.net" {
		t.Errorf("host = %v, want the deployment the session is bound to", receipt["host"])
	}
	if receipt["expiresAt"] != "2026-09-02T13:00:00Z" {
		t.Errorf("expiresAt = %v", receipt["expiresAt"])
	}
	for key, value := range receipt {
		if text, ok := value.(string); ok && strings.Contains(text, "secret-") {
			t.Errorf("receipt leaked a token in %q: %q", key, text)
		}
	}
}

func TestPromptTextPrefersTheCompleteURI(t *testing.T) {
	withComplete := promptText(credential.DevicePrompt{
		UserCode:                "WDJB-MJHT",
		VerificationURI:         "https://idp/activate",
		VerificationURIComplete: "https://idp/activate?user_code=WDJB-MJHT",
	})
	if !strings.Contains(withComplete, "user_code=WDJB-MJHT") {
		t.Errorf("prompt = %q, want the link that carries the code", withComplete)
	}

	bare := promptText(credential.DevicePrompt{
		UserCode: "WDJB-MJHT", VerificationURI: "https://idp/activate",
	})
	if !strings.Contains(bare, "https://idp/activate") || !strings.Contains(bare, "WDJB-MJHT") {
		t.Errorf("prompt = %q, want both the URL and the code", bare)
	}
}

// hintOf reads the actionable half of the family error contract, which is
// where the next command lives rather than in the message.
func hintOf(t *testing.T, err error) string {
	t.Helper()
	var oerr *out.Error
	if !out.As(err, &oerr) {
		t.Fatalf("error = %v, want the family error contract", err)
	}
	return oerr.Hint
}
