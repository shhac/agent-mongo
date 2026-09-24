package config

import (
	"strings"
	"testing"

	out "github.com/shhac/lib-agent-output"
)

func TestPinnedConnection(t *testing.T) {
	t.Setenv(IdentityEnv, "")
	if _, ok, err := PinnedConnection(""); ok || err != nil {
		t.Errorf("unpinned: ok=%v err=%v", ok, err)
	}

	t.Setenv(IdentityEnv, "1")
	alias, ok, err := PinnedConnection(" prod ")
	if !ok || err != nil || alias != "prod" {
		t.Errorf("pinned with -c: alias=%q ok=%v err=%v", alias, ok, err)
	}

	_, ok, err = PinnedConnection("")
	var contract *out.Error
	if !ok || !out.As(err, &contract) || contract.FixableBy != out.FixableByHuman {
		t.Errorf("pinned without -c must fail closed for the operator to fix: ok=%v err=%v", ok, err)
	}
	if !strings.Contains(contract.Hint, "--bind connection=") {
		t.Errorf("hint should say how to bind: %q", contract.Hint)
	}
}

func TestCheckPinnedTo(t *testing.T) {
	t.Setenv(IdentityEnv, "")
	if err := CheckPinnedTo("", "staging"); err != nil {
		t.Errorf("unpinned processes may name any connection: %v", err)
	}

	t.Setenv(IdentityEnv, "true")
	if err := CheckPinnedTo("prod", "prod"); err != nil {
		t.Errorf("the pinned connection itself: %v", err)
	}
	if err := CheckPinnedTo("prod", "staging"); err == nil {
		t.Error("another connection must be refused")
	}
}
