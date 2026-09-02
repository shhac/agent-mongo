package credential

import (
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	credstore "github.com/shhac/agent-mongo/internal/credential"

	"github.com/shhac/agent-mongo/internal/testutil"
)

// runAdd drives the full cobra tree for `credential add` with the given stdin
// and args, so the real RunE (including the stdin fallback) is exercised.
func runAdd(t *testing.T, stdin string, args ...string) error {
	t.Helper()
	root := &cobra.Command{Use: "agent-mongo"}
	Register(root, nil)
	root.SetArgs(append([]string{"credential", "add"}, args...))
	root.SetIn(strings.NewReader(stdin))
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	return root.Execute()
}

func TestAddPasswordFlagWinsOverStdin(t *testing.T) {
	testutil.IsolateConfig(t)

	if err := runAdd(t, "stdin-password", "flagwins", "--username", "deploy", "--password", "flag-password"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	res, err := credstore.Resolve("flagwins")
	if err != nil {
		t.Fatalf("credential not stored: %v", err)
	}
	if cred := res.Credential; cred.Password != "flag-password" {
		t.Errorf("stored password = %q, want flag value 'flag-password' (flag must win over stdin)", cred.Password)
	}
}

func TestAddStdinFillsPassword(t *testing.T) {
	testutil.IsolateConfig(t)

	// A trailing newline is typical of `printf '%s\n'` / here-strings; ReadSecret trims it.
	if err := runAdd(t, "piped-secret\n", "fromstdin", "--username", "deploy"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	res, err := credstore.Resolve("fromstdin")
	if err != nil {
		t.Fatalf("credential not stored: %v", err)
	}
	cred := res.Credential
	if cred.Username != "deploy" {
		t.Errorf("stored username = %q, want 'deploy'", cred.Username)
	}
	if cred.Password != "piped-secret" {
		t.Errorf("stored password = %q, want trimmed 'piped-secret'", cred.Password)
	}
}

func TestAddErrorsWhenPasswordMissing(t *testing.T) {
	testutil.IsolateConfig(t)

	// No --password flag, empty (non-interactive) stdin, no --form: nothing supplies the secret.
	err := runAdd(t, "", "nopass", "--username", "deploy")
	if err == nil {
		t.Fatal("expected a missing-password error, got nil")
	}
	if !strings.Contains(err.Error(), "password") {
		t.Errorf("error = %q, want it to mention the missing password", err.Error())
	}
	if _, err := credstore.Resolve("nopass"); err == nil {
		t.Error("credential should not have been stored when password is missing")
	}
}
