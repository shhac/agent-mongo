package cli

import (
	"io"
	"strings"
	"testing"

	"github.com/shhac/agent-mongo/internal/config"
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
