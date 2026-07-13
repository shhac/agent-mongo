package connection

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
)

// isolate points config at a throwaway dir and forces credential.Store onto
// the config.json fallback so tests never touch a real keychain.
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENT_MONGO_NO_KEYCHAIN", "1")
}

func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := &cobra.Command{Use: "agent-mongo"}
	Register(root, nil)
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	stdout, restore := captureStdout(t)
	err := root.Execute()
	restore()
	return stdout.String(), err
}

func TestAddExtractsEmbeddedCredentials(t *testing.T) {
	isolate(t)

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

func TestAddRejectsEmbeddedCredentialsWithCredentialFlag(t *testing.T) {
	isolate(t)

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
	isolate(t)

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

func TestListRedactsEmbeddedPasswords(t *testing.T) {
	isolate(t)

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

// captureStdout redirects os.Stdout to a pipe and returns a buffer receiving
// everything written to stdout. The returned restore func puts stdout back.
func captureStdout(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	prev := os.Stdout
	os.Stdout = w

	buf := &bytes.Buffer{}
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(buf, r)
		close(done)
	}()

	return buf, func() {
		_ = w.Close()
		<-done
		os.Stdout = prev
		_ = r.Close()
	}
}
