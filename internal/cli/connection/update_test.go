package connection

import (
	"errors"
	"strings"
	"testing"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/testutil"
)

func seedConnection(t *testing.T, alias string) {
	t.Helper()
	err := config.StoreConnection(alias, config.Connection{
		ConnectionString: "mongodb://localhost:27017/app",
		Name:             alias,
	})
	if err != nil {
		t.Fatalf("seeding connection %q: %v", alias, err)
	}
}

func TestUpdateSetsCredential(t *testing.T) {
	testutil.IsolateConfig(t)
	seedConnection(t, "prod")
	if _, err := credential.Store("acme", config.Credential{
		Username: "deploy", Password: "s3cret",
	}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if _, err := execute(t, "connection", "update", "prod", "--credential", "acme"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	conn, _ := config.GetConnection("prod")
	if conn.Credential != "acme" {
		t.Errorf("credential = %q, want acme", conn.Credential)
	}
}

func TestUpdateClearsCredential(t *testing.T) {
	testutil.IsolateConfig(t)
	seedConnection(t, "prod")
	if _, err := credential.Store("acme", config.Credential{
		Username: "deploy", Password: "s3cret",
	}); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if _, err := execute(t, "connection", "update", "prod", "--credential", "acme"); err != nil {
		t.Fatalf("seeding the reference: %v", err)
	}

	if _, err := execute(t, "connection", "update", "prod", "--clear-credential"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	conn, _ := config.GetConnection("prod")
	if conn.Credential != "" {
		t.Errorf("credential = %q, want it cleared", conn.Credential)
	}
}

func TestUpdateRejectsUnknownCredential(t *testing.T) {
	testutil.IsolateConfig(t)
	seedConnection(t, "prod")

	_, err := execute(t, "connection", "update", "prod", "--credential", "nope")
	if err == nil {
		t.Fatal("update accepted a credential that is not stored")
	}
	if !errors.Is(err, credential.ErrNotFound) {
		t.Errorf("error = %v, want it to wrap ErrNotFound", err)
	}
	conn, _ := config.GetConnection("prod")
	if conn.Credential != "" {
		t.Errorf("credential = %q, want the connection left untouched", conn.Credential)
	}
}

// RequireExists, not Resolve: wiring up a reference must not demand that the
// credential can authenticate at that moment. A keychain-backed entry whose
// secret is momentarily unreadable is still a valid thing to point a
// connection at, and phases 2-4 add kinds that have no session until someone
// logs in.
func TestUpdateAcceptsCredentialThatCannotResolve(t *testing.T) {
	testutil.IsolateConfig(t)
	seedConnection(t, "prod")
	testutil.StageCredential(t, "ghost", config.Credential{
		Username: credential.Sentinel, Password: credential.Sentinel,
	})
	if _, err := credential.Resolve("ghost"); err == nil {
		t.Fatal("precondition: expected 'ghost' to be unresolvable")
	}

	if _, err := execute(t, "connection", "update", "prod", "--credential", "ghost"); err != nil {
		t.Fatalf("update rejected a stored-but-unresolvable credential: %v", err)
	}
	conn, _ := config.GetConnection("prod")
	if conn.Credential != "ghost" {
		t.Errorf("credential = %q, want ghost", conn.Credential)
	}
}

func TestUpdateRejectsUnknownConnection(t *testing.T) {
	testutil.IsolateConfig(t)

	_, err := execute(t, "connection", "update", "nope", "--database", "other")
	if err == nil {
		t.Fatal("update accepted an alias that is not stored")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error = %q, want it to name the connection", err)
	}
}
