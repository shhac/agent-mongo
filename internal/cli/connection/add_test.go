package connection

import (
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	out "github.com/shhac/lib-agent-output"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/testutil"
)

func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := &cobra.Command{Use: "agent-mongo"}
	Register(root, nil)
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	stdout, restore := testutil.CaptureStdout(t)
	err := root.Execute()
	restore()
	return stdout.String(), err
}

func TestAddExtractsEmbeddedCredentials(t *testing.T) {
	testutil.IsolateConfig(t)

	const canary = "TOPSECRET-CANARY-7A3F"
	stdout, err := execute(t,
		"connection", "add", "staging",
		"mongodb+srv://deploy:"+canary+"@cluster.example.net/app?retryWrites=true")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	conn, ok := config.GetConnection("staging")
	if !ok {
		t.Fatal("connection 'staging' not stored")
	}
	if want := "mongodb+srv://cluster.example.net/app?retryWrites=true"; conn.ConnectionString != want {
		t.Errorf("stored connection string = %q, want stripped %q", conn.ConnectionString, want)
	}
	if conn.Credential != "staging" {
		t.Errorf("connection credential = %q, want matching alias 'staging'", conn.Credential)
	}

	cred, ok := credential.Get("staging")
	if !ok {
		t.Fatal("credential 'staging' not stored")
	}
	if cred.Username != "deploy" || cred.Password != canary {
		t.Errorf("credential = %q/%q, want deploy/%s", cred.Username, cred.Password, canary)
	}

	if strings.Contains(stdout, canary) {
		t.Errorf("password leaked to stdout: %s", stdout)
	}
	if !strings.Contains(stdout, `"credentialCreated":true`) {
		t.Errorf("receipt missing credentialCreated flag: %s", stdout)
	}
}

// hintOf unwraps the structured hint the CLI renders alongside the error.
func hintOf(t *testing.T, err error) string {
	t.Helper()
	var oerr *out.Error
	if !out.As(err, &oerr) {
		t.Fatalf("expected *out.Error with a hint, got %T: %v", err, err)
	}
	return oerr.Hint
}

func TestAddRefusesToClobberDifferentCredential(t *testing.T) {
	testutil.IsolateConfig(t)

	if _, err := credential.Store("staging", config.Credential{
		Username: "old-user",
		Password: "old-pass",
	}); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if err := config.StoreConnection("other", config.Connection{
		ConnectionString: "mongodb://other.example.net/app",
		Credential:       "staging",
	}); err != nil {
		t.Fatalf("StoreConnection: %v", err)
	}

	_, err := execute(t, "connection", "add", "staging", "mongodb://new-user:new-pass@localhost/app")
	if err == nil {
		t.Fatal("expected refusal, got nil")
	}
	if !strings.Contains(err.Error(), "other") {
		t.Errorf("error = %q, want it to name the connections using the credential", err)
	}

	hint := hintOf(t, err)
	if !strings.Contains(hint, "--credential staging") {
		t.Errorf("hint = %q, want the --credential fallback", hint)
	}
	if !strings.Contains(hint, "credential remove staging") {
		t.Errorf("hint = %q, want the credential remove path", hint)
	}

	cred, ok := credential.Get("staging")
	if !ok || cred.Username != "old-user" || cred.Password != "old-pass" {
		t.Errorf("credential = %+v (ok=%v), want untouched old-user/old-pass", cred, ok)
	}
	if _, ok := config.GetConnection("staging"); ok {
		t.Error("connection stored despite refusal")
	}
}

func TestAddClobberHintRecommendsRotationWhenOnlyCredentialChanges(t *testing.T) {
	testutil.IsolateConfig(t)

	if _, err := credential.Store("staging", config.Credential{
		Username: "old-user",
		Password: "old-pass",
	}); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if err := config.StoreConnection("staging", config.Connection{
		ConnectionString: "mongodb://localhost/app",
		Credential:       "staging",
	}); err != nil {
		t.Fatalf("StoreConnection: %v", err)
	}

	_, err := execute(t, "connection", "add", "staging", "mongodb://new-user:new-pass@localhost/app")
	if err == nil {
		t.Fatal("expected refusal, got nil")
	}
	if hint := hintOf(t, err); !strings.Contains(hint, "credential add staging") {
		t.Errorf("hint = %q, want rotation via credential add", hint)
	}
}

func TestAddSameEmbeddedCredentialsIsIdempotent(t *testing.T) {
	testutil.IsolateConfig(t)

	uri := "mongodb://deploy:hunter2@localhost/app"
	if _, err := execute(t, "connection", "add", "staging", uri); err != nil {
		t.Fatalf("first add: %v", err)
	}
	stdout, err := execute(t, "connection", "add", "staging", uri)
	if err != nil {
		t.Fatalf("same-values re-add should succeed, got: %v", err)
	}
	if !strings.Contains(stdout, `"credentialCreated":true`) {
		t.Errorf("re-add receipt = %s, want credentialCreated", stdout)
	}
}

func TestAddRejectsUnknownCredentialFlag(t *testing.T) {
	testutil.IsolateConfig(t)

	_, err := execute(t, "connection", "add", "local", "mongodb://localhost/app", "--credential", "ghost")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want credential-not-found", err)
	}
	if _, ok := config.GetConnection("local"); ok {
		t.Error("connection stored despite unknown credential")
	}
}

func TestAddRejectsEmbeddedCredentialsWithCredentialFlag(t *testing.T) {
	testutil.IsolateConfig(t)

	_, err := execute(t,
		"connection", "add", "staging",
		"mongodb://deploy:secret@localhost/app", "--credential", "acme")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "--credential") {
		t.Errorf("error = %q, want it to mention the --credential conflict", err)
	}
	if _, ok := config.GetConnection("staging"); ok {
		t.Error("connection stored despite conflict error")
	}
}

func TestAddWithoutEmbeddedCredentialsIsUnchanged(t *testing.T) {
	testutil.IsolateConfig(t)

	stdout, err := execute(t, "connection", "add", "local", "mongodb://localhost:27017/myapp")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	conn, ok := config.GetConnection("local")
	if !ok {
		t.Fatal("connection 'local' not stored")
	}
	if conn.ConnectionString != "mongodb://localhost:27017/myapp" {
		t.Errorf("connection string = %q, want unchanged", conn.ConnectionString)
	}
	if conn.Credential != "" {
		t.Errorf("credential = %q, want empty", conn.Credential)
	}
	if len(credential.All()) != 0 {
		t.Errorf("credentials created: %v, want none", credential.All())
	}
	if strings.Contains(stdout, "credentialCreated") {
		t.Errorf("receipt claims credential creation: %s", stdout)
	}
}

func TestLsIsListAlias(t *testing.T) {
	testutil.IsolateConfig(t)

	if _, err := execute(t, "connection", "add", "local", "mongodb://localhost/app"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, err := execute(t, "connection", "ls")
	if err != nil {
		t.Fatalf("connection ls: %v", err)
	}
	if !strings.Contains(stdout, `"alias":"local"`) {
		t.Errorf("ls output = %s, want the saved connection listed", stdout)
	}
}

func TestListRedactsEmbeddedPasswords(t *testing.T) {
	testutil.IsolateConfig(t)

	// A pre-extraction connection stored with the password still in the URI.
	if err := config.StoreConnection("legacy", config.Connection{
		ConnectionString: "mongodb://deploy:supersecret@localhost/app",
	}); err != nil {
		t.Fatalf("StoreConnection: %v", err)
	}

	stdout, err := execute(t, "connection", "list")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if strings.Contains(stdout, "supersecret") {
		t.Errorf("password leaked in list output: %s", stdout)
	}
	if !strings.Contains(stdout, "mongodb://deploy:***@localhost/app") {
		t.Errorf("list output missing redacted connection string: %s", stdout)
	}
}
