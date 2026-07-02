package credential

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/shhac/lib-agent-cli/dialog"
	"github.com/shhac/lib-agent-cli/dialog/dialogtest"
	out "github.com/shhac/lib-agent-output"
)

func TestPromptMissingReturnsSuppliedValuesWithoutPrompting(t *testing.T) {
	rec := &dialogtest.Recorder{
		PromptResults: []dialog.Result{{ID: "username", Value: "should not be used"}},
	}
	defer dialog.SetDefault(rec)()

	user, pass, err := promptMissingViaDialog(context.Background(), "acme", "u", "p")
	if err != nil {
		t.Fatalf("promptMissingViaDialog() error = %v", err)
	}
	if user != "u" || pass != "p" {
		t.Errorf("user/pass = %q/%q, want u/p", user, pass)
	}
	if len(rec.Calls) != 0 {
		t.Errorf("Prompt called %d times, want 0", len(rec.Calls))
	}
}

func TestPromptMissingPromptsOnlyMissingPassword(t *testing.T) {
	rec := &dialogtest.Recorder{
		PromptResults: []dialog.Result{{ID: "password", Value: "from-dialog"}},
	}
	defer dialog.SetDefault(rec)()

	user, pass, err := promptMissingViaDialog(context.Background(), "acme", "preset", "")
	if err != nil {
		t.Fatalf("promptMissingViaDialog() error = %v", err)
	}
	if user != "preset" {
		t.Errorf("username = %q, want unchanged 'preset'", user)
	}
	if pass != "from-dialog" {
		t.Errorf("password = %q, want 'from-dialog'", pass)
	}
	if len(rec.Calls) != 1 {
		t.Fatalf("Prompt called %d times, want 1", len(rec.Calls))
	}
	spec := rec.Calls[0]
	if len(spec.Items) != 1 || spec.Items[0].ID != "password" {
		t.Fatalf("spec.Items = %+v, want single 'password' item", spec.Items)
	}
	if spec.Items[0].InputType != dialog.Password {
		t.Errorf("password field InputType = %v, want Password", spec.Items[0].InputType)
	}
	if spec.Title != "agent-mongo credential: acme" {
		t.Errorf("spec.Title = %q, want 'agent-mongo credential: acme'", spec.Title)
	}
}

func TestPromptMissingPromptsBothWhenBothBlank(t *testing.T) {
	rec := &dialogtest.Recorder{
		PromptResults: []dialog.Result{
			{ID: "username", Value: "deploy"},
			{ID: "password", Value: "secret-from-dialog"},
		},
	}
	defer dialog.SetDefault(rec)()

	user, pass, err := promptMissingViaDialog(context.Background(), "acme", "", "")
	if err != nil {
		t.Fatalf("promptMissingViaDialog() error = %v", err)
	}
	if user != "deploy" || pass != "secret-from-dialog" {
		t.Errorf("user/pass = %q/%q, want deploy/secret-from-dialog", user, pass)
	}
	spec := rec.Calls[0]
	if len(spec.Items) != 2 {
		t.Fatalf("spec.Items = %+v, want 2 items", spec.Items)
	}
	if spec.Items[0].ID != "username" || spec.Items[0].InputType != dialog.Text {
		t.Errorf("first field = %+v, want username/Text", spec.Items[0])
	}
	if spec.Items[1].ID != "password" || spec.Items[1].InputType != dialog.Password {
		t.Errorf("second field = %+v, want password/Password", spec.Items[1])
	}
}

func TestPromptMissingIgnoresExtraneousResultIDs(t *testing.T) {
	rec := &dialogtest.Recorder{
		PromptResults: []dialog.Result{
			{ID: "username", Value: "u"},
			{ID: "password", Value: "p"},
			{ID: "rogue", Value: "should-be-ignored"},
		},
	}
	defer dialog.SetDefault(rec)()

	user, pass, err := promptMissingViaDialog(context.Background(), "acme", "", "")
	if err != nil {
		t.Fatalf("promptMissingViaDialog() error = %v", err)
	}
	if user != "u" || pass != "p" {
		t.Errorf("user/pass = %q/%q, want u/p", user, pass)
	}
}

func TestPromptMissingReturnsRetryErrorOnCancel(t *testing.T) {
	rec := &dialogtest.Recorder{
		PromptErr: fmt.Errorf("%w (Database password)", dialog.ErrCancelled),
	}
	defer dialog.SetDefault(rec)()

	_, _, err := promptMissingViaDialog(context.Background(), "acme", "", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var oerr *out.Error
	if !out.As(err, &oerr) {
		t.Fatalf("expected *out.Error, got %T", err)
	}
	if oerr.FixableBy != out.FixableByRetry {
		t.Errorf("FixableBy = %q, want retry", oerr.FixableBy)
	}
	if !strings.Contains(oerr.Hint, "Re-run") {
		t.Errorf("hint = %q, want it to mention re-running", oerr.Hint)
	}
	if !errors.Is(err, dialog.ErrCancelled) {
		t.Error("errors.Is(err, ErrCancelled) = false, want true (sentinel chain broken)")
	}
}

func TestPromptMissingReturnsHumanErrorWhenNoGUI(t *testing.T) {
	rec := &dialogtest.Recorder{
		AvailableErr: fmt.Errorf("%w: no $DISPLAY", dialog.ErrNoGUI),
	}
	defer dialog.SetDefault(rec)()

	_, _, err := promptMissingViaDialog(context.Background(), "acme", "", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var oerr *out.Error
	if !out.As(err, &oerr) {
		t.Fatalf("expected *out.Error, got %T", err)
	}
	if oerr.FixableBy != out.FixableByHuman {
		t.Errorf("FixableBy = %q, want human", oerr.FixableBy)
	}
	if !strings.Contains(oerr.Hint, "agent-mongo credential add --form") {
		t.Errorf("hint = %q, want the agent-mongo-specific --form guidance", oerr.Hint)
	}
	if !strings.Contains(oerr.Hint, "--username <u> --password <secret>") {
		t.Errorf("hint = %q, want the non-interactive fallback", oerr.Hint)
	}
	if !errors.Is(err, dialog.ErrNoGUI) {
		t.Error("errors.Is(err, ErrNoGUI) = false, want true (sentinel chain broken)")
	}
}

// TestCredentialAddFormDoesNotLeakSecretToStdout is the load-bearing test for
// this package's headline claim: the LLM driving the CLI must never see the
// secret the user types into the OS dialog. We run the full cobra tree so the
// real on-success PrintRaw receipt path is exercised, feed a canary through the
// Recorder, and assert it never reaches stdout.
//
// AGENT_MONGO_NO_KEYCHAIN forces credential.Store onto the config.json fallback
// so no keychain backend is touched (and macOS never prompts).
func TestCredentialAddFormDoesNotLeakSecretToStdout(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENT_MONGO_NO_KEYCHAIN", "1")

	const canary = "TOPSECRET-CANARY-7A3F"
	rec := &dialogtest.Recorder{
		PromptResults: []dialog.Result{{ID: "password", Value: canary}},
	}
	defer dialog.SetDefault(rec)()

	stdout, restore := captureStdout(t)

	root := &cobra.Command{Use: "agent-mongo"}
	Register(root)
	root.SetArgs([]string{"credential", "add", "leak-test", "--username", "deploy", "--form"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	err := root.Execute()
	restore()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	captured := stdout.String()

	if strings.Contains(captured, canary) {
		t.Fatalf("canary %q leaked to stdout: %s", canary, captured)
	}
	// Sanity: the receipt is still emitted, just without the secret.
	if !strings.Contains(captured, "leak-test") {
		t.Errorf("expected receipt to include credential name, got: %s", captured)
	}
	if !strings.Contains(captured, "deploy") {
		t.Errorf("expected receipt to include username, got: %s", captured)
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
