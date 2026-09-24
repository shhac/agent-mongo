package mongo

import (
	"strings"
	"testing"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/testutil"
)

func TestResolveAliasPrecedence(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seed := config.Config{
		DefaultConnection: "default-conn",
		Connections: map[string]config.Connection{
			"default-conn": {ConnectionString: "mongodb://localhost/db"},
		},
	}
	if err := config.Write(seed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	t.Run("flag beats env", func(t *testing.T) {
		t.Setenv("AGENT_MONGO_CONNECTION", "env-conn")
		alias, err := ResolveAlias("flag-conn")
		if err != nil || alias != "flag-conn" {
			t.Fatalf("got %q, %v", alias, err)
		}
	})

	t.Run("env beats config default", func(t *testing.T) {
		t.Setenv("AGENT_MONGO_CONNECTION", "env-conn")
		alias, err := ResolveAlias("")
		if err != nil || alias != "env-conn" {
			t.Fatalf("got %q, %v", alias, err)
		}
	})

	t.Run("config default when flag and env empty", func(t *testing.T) {
		t.Setenv("AGENT_MONGO_CONNECTION", "")
		alias, err := ResolveAlias("  ")
		if err != nil || alias != "default-conn" {
			t.Fatalf("got %q, %v", alias, err)
		}
	})

	t.Run("error lists available when nothing resolves", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		t.Setenv("AGENT_MONGO_CONNECTION", "")
		_, err := ResolveAlias("")
		if err == nil || !strings.Contains(err.Error(), "No connection specified") ||
			!strings.Contains(err.Error(), "(none)") {
			t.Fatalf("got %v", err)
		}
	})
}

// A pinned process never falls back to the operator's env or default
// connection: those are exactly what a bound principal was not given.
func TestResolveAliasWhenPinned(t *testing.T) {
	testutil.IsolateConfig(t)
	if err := config.StoreConnection("prod", config.Connection{ConnectionString: "mongodb://prod/app"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_MONGO_CONNECTION", "prod")
	t.Setenv(config.IdentityEnv, "1")
	t.Setenv(config.BoundConnectionEnv, "staging")

	if alias, err := ResolveAlias(""); err != nil || alias != "staging" {
		t.Errorf("no -c: alias=%q err=%v, want the bound connection", alias, err)
	}
	if alias, err := ResolveAlias("prod"); err == nil {
		t.Errorf("-c prod resolved to %q; must refuse a connection outside the binding", alias)
	}
}
