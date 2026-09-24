package connection

import (
	"testing"
	"time"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/testutil"
)

func seedOIDC(t *testing.T, connAlias, credAlias string) {
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

func TestSessionExpiry(t *testing.T) {
	expiry := time.Now().Add(time.Hour).UTC().Truncate(time.Second)

	t.Run("reported when there is a session", func(t *testing.T) {
		testutil.IsolateConfig(t)
		seedOIDC(t, "prod", "corp")
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
		seedOIDC(t, "prod", "corp")
		if _, _, ok := sessionExpiry("prod"); ok {
			t.Error("an expiry was reported before anyone logged in")
		}

		seedOIDC(t, "plain", "")
		if _, _, ok := sessionExpiry("plain"); ok {
			t.Error("an expiry was reported for a connection with no credential")
		}
		if _, _, ok := sessionExpiry("absent"); ok {
			t.Error("an expiry was reported for a connection that does not exist")
		}
	})
}
