package cli

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/shhac/lib-agent-mcp/oauth"

	"github.com/shhac/agent-mongo/internal/config"
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
	if strings.Join(argv, " ") != "--connection prod" {
		t.Errorf("argv = %v", argv)
	}
	if !slices.Contains(env, config.IdentityEnv+"=1") {
		t.Errorf("env = %v, want the fail-closed gate", env)
	}

	argv, env = mcpIdentityBinding(oauth.Verified{PrincipalGrant: oauth.PrincipalGrant{Name: "bob"}})
	if len(argv) != 0 || !slices.Contains(env, config.IdentityEnv+"=1") {
		t.Errorf("unbound principal: argv=%v env=%v; the gate alone must make it fail closed", argv, env)
	}
}
