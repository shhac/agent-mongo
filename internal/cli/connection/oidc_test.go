package connection

import (
	"errors"
	"testing"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/testutil"
)

func storeOIDCCredential(t *testing.T, alias string) {
	t.Helper()
	if _, err := credential.Store(alias, testutil.OIDCCredential(config.EnvironmentK8s)); err != nil {
		t.Fatalf("Store: %v", err)
	}
}

// The endpoint check runs while the user is still looking at the command, not
// only later at connect.
func TestAddRefusesOIDCOverPlaintext(t *testing.T) {
	testutil.IsolateConfig(t)
	storeOIDCCredential(t, "corp")

	_, err := execute(t, "connection", "add", "prod",
		"mongodb://localhost:27017/app", "--credential", "corp")
	if !errors.Is(err, credential.ErrInsecureConnection) {
		t.Fatalf("error = %v, want ErrInsecureConnection", err)
	}
	if _, ok := config.GetConnection("prod"); ok {
		t.Error("the connection was stored despite the refusal")
	}
}

func TestAddRefusesOIDCToADisallowedHost(t *testing.T) {
	testutil.IsolateConfig(t)
	storeOIDCCredential(t, "corp")

	_, err := execute(t, "connection", "add", "prod",
		"mongodb+srv://evil.example.com/app", "--credential", "corp")
	if !errors.Is(err, credential.ErrHostNotAllowed) {
		t.Fatalf("error = %v, want ErrHostNotAllowed", err)
	}
	if _, ok := config.GetConnection("prod"); ok {
		t.Error("the connection was stored despite the refusal")
	}
}

func TestAddAcceptsOIDCToAnAllowedHost(t *testing.T) {
	testutil.IsolateConfig(t)
	storeOIDCCredential(t, "corp")

	if _, err := execute(t, "connection", "add", "prod",
		"mongodb+srv://c0.abc.mongodb.net/app", "--credential", "corp"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	conn, ok := config.GetConnection("prod")
	if !ok {
		t.Fatal("connection not stored")
	}
	if conn.Credential != "corp" {
		t.Errorf("credential = %q, want corp", conn.Credential)
	}
}

// Attaching an OIDC credential to a connection that already exists has to run
// the same check, against the connection string already stored.
func TestUpdateRefusesOIDCOnAPlaintextConnection(t *testing.T) {
	testutil.IsolateConfig(t)
	storeOIDCCredential(t, "corp")
	seedConnection(t, "prod")

	_, err := execute(t, "connection", "update", "prod", "--credential", "corp")
	if !errors.Is(err, credential.ErrInsecureConnection) {
		t.Fatalf("error = %v, want ErrInsecureConnection", err)
	}
	conn, _ := config.GetConnection("prod")
	if conn.Credential != "" {
		t.Errorf("credential = %q, want the connection left untouched", conn.Credential)
	}
}

func TestUpdateAcceptsOIDCOnATLSConnection(t *testing.T) {
	testutil.IsolateConfig(t)
	storeOIDCCredential(t, "corp")
	if err := config.StoreConnection("prod", config.Connection{
		ConnectionString: "mongodb+srv://c0.abc.mongodb.net/app",
		Name:             "prod",
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	if _, err := execute(t, "connection", "update", "prod", "--credential", "corp"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	conn, _ := config.GetConnection("prod")
	if conn.Credential != "corp" {
		t.Errorf("credential = %q, want corp", conn.Credential)
	}
}
