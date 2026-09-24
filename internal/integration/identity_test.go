//go:build integration

package integration

import (
	"strings"
	"testing"
)

// The process shape the MCP server gives a named principal's call: the gate and
// the bound connection in the environment, the caller's arguments untouched.
func TestPinnedPrincipalStaysOnItsConnection(t *testing.T) {
	h := home(t)
	if r := runIn(t, h, "connection", "add", "other", "mongodb://127.0.0.1:1/"+testDB); r.exitCode != 0 {
		t.Fatalf("connection add: %s", r.stderr)
	}
	bound := []string{"AGENT_MONGO_REQUIRE_IDENTITY=1", "AGENT_MONGO_BOUND_CONNECTION=it"}

	r := runInEnv(t, h, bound, "query", "count", testDB, "orders")
	if items, _ := r.records(t); r.exitCode != 0 || len(items) != 1 {
		t.Fatalf("bound call: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	for _, args := range [][]string{
		{"query", "count", testDB, "orders", "-c", "other"},
		{"connection", "test", "other"},
	} {
		r = runInEnv(t, h, bound, args...)
		if r.exitCode != 1 || !strings.Contains(r.stderr, "not available") {
			t.Errorf("%v reached a connection outside the binding: exit=%d stderr=%s", args, r.exitCode, r.stderr)
		}
	}

	r = runInEnv(t, h, bound, "connection", "list")
	if strings.Contains(r.stdout, "other") {
		t.Errorf("listed a connection outside the binding: %s", r.stdout)
	}

	unbound := []string{"AGENT_MONGO_REQUIRE_IDENTITY=1", "AGENT_MONGO_BOUND_CONNECTION="}
	r = runInEnv(t, h, unbound, "query", "count", testDB, "orders", "-c", "it")
	if r.exitCode != 1 || r.stderrJSON(t)["fixable_by"] != "human" {
		t.Errorf("an unbound principal chose its own connection: exit=%d stderr=%s", r.exitCode, r.stderr)
	}
}
