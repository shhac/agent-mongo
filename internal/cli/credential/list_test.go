package credential

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/testutil"
)

// runList drives the full cobra tree for `credential list` and returns the
// NDJSON records it printed.
func runList(t *testing.T) []map[string]any {
	t.Helper()
	buf, restore := testutil.CaptureStdout(t)
	root := &cobra.Command{Use: "agent-mongo"}
	Register(root)
	root.SetArgs([]string{"credential", "list"})
	err := root.Execute()
	restore()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" || strings.HasPrefix(line, `{"@`) {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("unmarshal %q: %v", line, err)
		}
		records = append(records, rec)
	}
	return records
}

func TestListReportsKind(t *testing.T) {
	testutil.IsolateConfig(t)
	testutil.StageCredential(t, "legacy", config.Credential{Username: "deploy", Password: "s3cret"})

	records := runList(t)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if got := records[0]["kind"]; got != "scram" {
		t.Errorf("kind = %v, want scram for a credential stored without one", got)
	}
	if got := records[0]["username"]; got != "deploy" {
		t.Errorf("username = %v, want deploy", got)
	}
	if got := records[0]["password"]; got != "***" {
		t.Errorf("password = %v, want it redacted", got)
	}
}

// A row must not imply a credential holds material it does not: printing
// username/password for a kind that has neither invites the reader to go
// looking for a password that was never set.
func TestListOmitsSCRAMFieldsForOtherKinds(t *testing.T) {
	testutil.IsolateConfig(t)
	testutil.StageCredential(t, "future", config.Credential{Kind: "oidc"})

	records := runList(t)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if got := records[0]["kind"]; got != "oidc" {
		t.Errorf("kind = %v, want oidc", got)
	}
	if _, ok := records[0]["username"]; ok {
		t.Error("username rendered for a kind that has none")
	}
	if _, ok := records[0]["password"]; ok {
		t.Error("password rendered for a kind that has none")
	}
}

func TestListRendersKeychainBackedUsername(t *testing.T) {
	testutil.IsolateConfig(t)
	testutil.StageCredential(t, "acme", config.Credential{Username: "__KEYCHAIN__", Password: "__KEYCHAIN__"})

	records := runList(t)
	if got := records[0]["username"]; got != "(keychain)" {
		t.Errorf("username = %v, want (keychain)", got)
	}
}
