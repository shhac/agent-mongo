package connection

import (
	"strings"
	"testing"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/testutil"
)

func seedConnections(t *testing.T, aliases ...string) {
	t.Helper()
	for _, alias := range aliases {
		if err := config.StoreConnection(alias, config.Connection{
			ConnectionString: "mongodb://" + alias + ".example.com/app",
		}); err != nil {
			t.Fatal(err)
		}
	}
}

// A principal bound to one connection must not learn where the others point.
func TestListShowsOnlyThePinnedConnection(t *testing.T) {
	testutil.IsolateConfig(t)
	seedConnections(t, "prod", "staging")

	stdout, err := execute(t, "connection", "list")
	if err != nil || !strings.Contains(stdout, "staging.example.com") {
		t.Fatalf("unpinned list should show everything: err=%v\n%s", err, stdout)
	}

	t.Setenv(config.IdentityEnv, "1")
	stdout, err = execute(t, "connection", "list", "-c", "prod")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "prod.example.com") || strings.Contains(stdout, "staging") {
		t.Errorf("pinned list leaked or lost connections:\n%s", stdout)
	}

	if _, err := execute(t, "connection", "list"); err == nil {
		t.Error("a pinned list with no -c must fail closed")
	}
}
