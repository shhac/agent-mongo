package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/shhac/lib-agent-mcp/oauth"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/testutil"
)

// The annotations are what an MCP host decides auto-approval on, so a library
// bump that changes how they aggregate must not slip through unnoticed.
func TestMCPToolAnnotations(t *testing.T) {
	root := newRootCmd("test")
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"mcp", "schema"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	var manifest struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Annotations *struct {
				ReadOnly    bool `json:"readOnlyHint"`
				Destructive bool `json:"destructiveHint"`
			} `json:"annotations"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &manifest); err != nil {
		t.Fatalf("manifest: %v\n%s", err, stdout.String())
	}

	want := map[string]struct{ readOnly, destructive bool }{
		"database":   {readOnly: true},
		"collection": {readOnly: true},
		"query":      {readOnly: true},
		"connection": {readOnly: true},
	}
	got := map[string]bool{}
	for _, tool := range manifest.Tools {
		got[tool.Name] = true
		expected, ok := want[tool.Name]
		if !ok {
			t.Errorf("unexpected tool %q exposed", tool.Name)
			continue
		}
		if tool.Annotations == nil {
			t.Errorf("%s: no annotations", tool.Name)
			continue
		}
		if tool.Annotations.ReadOnly != expected.readOnly || tool.Annotations.Destructive != expected.destructive {
			t.Errorf("%s: readOnly=%v destructive=%v, want %v/%v", tool.Name,
				tool.Annotations.ReadOnly, tool.Annotations.Destructive, expected.readOnly, expected.destructive)
		}
	}
	for _, tool := range manifest.Tools {
		if tool.Name != "connection" {
			continue
		}
		for _, hidden := range []string{"add", "update", "remove", "set-default"} {
			if strings.Contains(tool.Description, hidden) {
				t.Errorf("connection tool dispatches %q, which writes config: %s", hidden, tool.Description)
			}
		}
	}
	for name := range want {
		if !got[name] {
			t.Errorf("tool %q missing", name)
		}
	}
}

func TestMCPIdentityBindingPinsTheBoundConnection(t *testing.T) {
	argv, env := mcpIdentityBinding(oauth.Verified{PrincipalGrant: oauth.PrincipalGrant{
		Name: "alice", Binding: map[string]string{"connection": "prod"},
	}})
	if len(argv) != 0 {
		t.Errorf("argv = %v; the binding must not ride on arguments the caller shapes", argv)
	}
	for _, want := range []string{config.IdentityEnv + "=1", config.BoundConnectionEnv + "=prod"} {
		if !slices.Contains(env, want) {
			t.Errorf("env = %v, want %s", env, want)
		}
	}

	_, env = mcpIdentityBinding(oauth.Verified{PrincipalGrant: oauth.PrincipalGrant{Name: "bob"}})
	if !slices.Contains(env, config.IdentityEnv+"=1") || !slices.Contains(env, config.BoundConnectionEnv+"=") {
		t.Errorf("unbound principal: env = %v; the gate with no binding must refuse", env)
	}
}

// The call shape lib-agent-mcp builds is the caller's args with the server's
// own suffix after them. Whatever the caller puts there — its own -c, or a
// "--" that turns anything after it positional — the binding holds.
func TestMCPPinSurvivesTheCallersArguments(t *testing.T) {
	testutil.IsolateConfig(t)
	for _, alias := range []string{"prod", "staging"} {
		if err := config.StoreConnection(alias, config.Connection{
			ConnectionString: "mongodb://" + alias + ".example.com/app",
		}); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(config.IdentityEnv, "1")
	t.Setenv(config.BoundConnectionEnv, "prod")

	for _, args := range [][]string{
		{"connection", "list", "-c", "staging"},
		{"connection", "list", "-c", "staging", "--", "--format", "jsonl"},
	} {
		stdout, err := executeRoot(t, args...)
		if strings.Contains(stdout, "staging") {
			t.Errorf("%v reached another connection:\n%s", args, stdout)
		}
		if err == nil {
			t.Errorf("%v: asking for another connection must fail", args)
		}
	}

	stdout, err := executeRoot(t, "connection", "list")
	if err != nil || !strings.Contains(stdout, "prod.example.com") {
		t.Errorf("the bound connection itself: err=%v\n%s", err, stdout)
	}
}

func executeRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd("test")
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	stdout, restore := testutil.CaptureStdout(t)
	err := root.Execute()
	restore()
	return stdout.String(), err
}
