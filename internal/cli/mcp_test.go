package cli

import (
	"bytes"
	"encoding/json"
	"testing"
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
		"connection": {destructive: true},
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
	for name := range want {
		if !got[name] {
			t.Errorf("tool %q missing", name)
		}
	}
}
