//go:build integration

package integration

import (
	"strings"
	"testing"
)

// The process shape the MCP server gives a named principal's call: the gate in
// the environment, and -c from the pairing's binding appended last.
func TestPinnedPrincipalStaysOnItsConnection(t *testing.T) {
	h := home(t)
	if r := runIn(t, h, "connection", "add", "other", "mongodb://127.0.0.1:1/"+testDB); r.exitCode != 0 {
		t.Fatalf("connection add: %s", r.stderr)
	}
	gate := []string{"AGENT_MONGO_REQUIRE_IDENTITY=1"}

	r := runInEnv(t, h, gate, "query", "count", testDB, "orders", "--connection", "it")
	if items, _ := r.records(t); r.exitCode != 0 || len(items) != 1 {
		t.Fatalf("bound call: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	r = runInEnv(t, h, gate, "query", "count", testDB, "orders")
	if r.exitCode != 1 || r.stderrJSON(t)["fixable_by"] != "human" {
		t.Errorf("unbound call used a connection anyway: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	r = runInEnv(t, h, gate, "connection", "test", "other", "--connection", "it")
	if r.exitCode != 1 || !strings.Contains(r.stderr, "not available") {
		t.Errorf("reached a connection outside the binding: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	r = runInEnv(t, h, gate, "connection", "list", "--connection", "it")
	if strings.Contains(r.stdout, "other") {
		t.Errorf("listed a connection outside the binding: %s", r.stdout)
	}
}
