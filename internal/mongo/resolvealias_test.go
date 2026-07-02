package mongo

import (
	"strings"
	"testing"

	"github.com/shhac/agent-mongo/internal/config"
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
