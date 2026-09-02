package mongo

import (
	"errors"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/testutil"
)

// applyAuth is where a resolved secret becomes driver auth material, and the
// point every future kind has to pass through.
func TestClientOptionsAppliesSCRAMCredential(t *testing.T) {
	testutil.IsolateConfig(t)
	if _, err := credential.Store("acme", config.Credential{
		Username: "deploy", Password: "s3cret",
	}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	opts, err := clientOptions(config.Connection{
		ConnectionString: "mongodb://localhost:27017/db",
		Credential:       "acme",
	}, 0)
	if err != nil {
		t.Fatalf("clientOptions: %v", err)
	}
	if opts.Auth == nil {
		t.Fatal("Auth is nil; the stored credential never reached the driver")
	}
	if opts.Auth.Username != "deploy" || opts.Auth.Password != "s3cret" {
		t.Errorf("Auth = %q/%q, want deploy/s3cret", opts.Auth.Username, opts.Auth.Password)
	}
}

func TestClientOptionsPropagatesUnresolvableCredential(t *testing.T) {
	testutil.IsolateConfig(t)
	// A sentinel with no keychain entry behind it: the entry exists, so this
	// is not "not found".
	testutil.StageCredential(t, "ghost", config.Credential{
		Username: credential.Sentinel, Password: credential.Sentinel,
	})

	opts, err := clientOptions(config.Connection{
		ConnectionString: "mongodb://localhost:27017/db",
		Credential:       "ghost",
	}, 0)
	if err == nil {
		t.Fatal("clientOptions accepted a credential whose secret cannot be read")
	}
	if opts != nil {
		t.Error("options returned alongside an error")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error = %q, want it to name the credential", err)
	}
}

func TestClientOptionsRejectsUnsupportedKind(t *testing.T) {
	testutil.IsolateConfig(t)
	testutil.StageCredential(t, "future", config.Credential{Kind: "oidc"})

	_, err := clientOptions(config.Connection{
		ConnectionString: "mongodb://localhost:27017/db",
		Credential:       "future",
	}, 0)
	if err == nil {
		t.Fatal("clientOptions accepted a kind this build cannot drive")
	}
	if !strings.Contains(err.Error(), "scram") {
		t.Errorf("error = %q, want it to list the supported kinds", err)
	}
}

// Characterization test, not an endorsement: a URI carrying ?authSource=app
// loses it as soon as the connection references a stored credential.
// ApplyURI only builds an Auth when HasAuthParameters() is true, and that
// check deliberately excludes authSource, so nothing carries it across and the
// driver falls back to "admin".
//
// Reachable from ordinary use: `connection add prod
// "mongodb://u:p@host/app?authSource=app"` moves the userinfo into a stored
// credential and keeps the query string, so a URI that authenticated inline
// stops working once extracted. Pinned here so the phase-2 OIDC work, which
// needs authSource to be empty or $external, has to make this decision
// deliberately rather than inherit it.
func TestClientOptionsDropsURIAuthSourceWhenCredentialReferenced(t *testing.T) {
	testutil.IsolateConfig(t)
	if _, err := credential.Store("acme", config.Credential{
		Username: "deploy", Password: "s3cret",
	}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	opts, err := clientOptions(config.Connection{
		ConnectionString: "mongodb://localhost:27017/app?authSource=app",
		Credential:       "acme",
	}, 0)
	if err != nil {
		t.Fatalf("clientOptions: %v", err)
	}
	if opts.Auth.AuthSource != "" {
		t.Errorf("AuthSource = %q; this test pins the current lossy behaviour as \"\" — "+
			"if it now survives, the bug was fixed and this test should assert %q instead",
			opts.Auth.AuthSource, "app")
	}
}

// applyAuth's default arm is unreachable through clientOptions, because Resolve
// rejects an unregistered kind before it gets there. It stops being unreachable
// the moment a kind is added to credential's dispatch table without a matching
// arm here — the two-place-registration bug it exists to catch — so it is
// exercised directly rather than left as an untested guard.
func TestApplyAuthRejectsAKindItCannotDrive(t *testing.T) {
	_, err := applyAuth(options.Client(), credential.Resolution{
		Alias: "future",
		Kind:  "oidc",
	})
	if err == nil {
		t.Fatal("applyAuth silently applied no auth for a kind it cannot drive")
	}
	if !errors.Is(err, credential.ErrUnsupportedKind) {
		t.Errorf("error = %v, want it to wrap ErrUnsupportedKind", err)
	}
	if !strings.Contains(err.Error(), "future") {
		t.Errorf("error = %q, want it to name the credential", err)
	}
}
