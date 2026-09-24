package config

import (
	"strings"
	"testing"

	out "github.com/shhac/lib-agent-output"
)

func TestPinnedConnection(t *testing.T) {
	t.Setenv(IdentityEnv, "")
	t.Setenv(BoundConnectionEnv, "prod")
	if _, ok, err := PinnedConnection("staging"); ok || err != nil {
		t.Errorf("without the gate nothing is pinned: ok=%v err=%v", ok, err)
	}

	t.Setenv(IdentityEnv, "1")
	for _, requested := range []string{"", "prod", " prod "} {
		alias, ok, err := PinnedConnection(requested)
		if !ok || err != nil || alias != "prod" {
			t.Errorf("requested %q: alias=%q ok=%v err=%v", requested, alias, ok, err)
		}
	}
	if _, ok, err := PinnedConnection("staging"); !ok || err == nil {
		t.Error("another connection must be refused")
	}
}

// A principal paired without a binding gets nothing — not the default, and not
// whatever it asks for.
func TestPinnedWithoutABindingFailsClosed(t *testing.T) {
	t.Setenv(IdentityEnv, "1")
	t.Setenv(BoundConnectionEnv, "")
	for _, requested := range []string{"", "prod"} {
		_, ok, err := PinnedConnection(requested)
		var contract *out.Error
		if !ok || !out.As(err, &contract) || contract.FixableBy != out.FixableByHuman {
			t.Fatalf("requested %q: ok=%v err=%v, want a refusal for the operator to fix", requested, ok, err)
		}
		if !strings.Contains(contract.Hint, "--bind connection=") {
			t.Errorf("hint should say how to bind: %q", contract.Hint)
		}
	}
}
