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

// Moving a URI's credentials into the credential store must not change how
// that URI authenticates. ApplyURI does not build an Auth from authSource
// alone, so before this was fixed the option was dropped and the driver fell
// back to "admin" — a connection that worked inline stopped working once
// `connection add` extracted its userinfo.
func TestClientOptionsAuthSourceFollowsTheURI(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{
			name: "explicit authSource wins",
			uri:  "mongodb://localhost:27017/app?authSource=admin",
			want: "admin",
		},
		{
			name: "case-insensitive option key",
			uri:  "mongodb://localhost:27017/app?authsource=admin",
			want: "admin",
		},
		{
			name: "falls back to the URI database",
			uri:  "mongodb://localhost:27017/app",
			want: "app",
		},
		{
			name: "no database leaves the driver default",
			uri:  "mongodb://localhost:27017",
			want: "",
		},
		{
			name: "survives a multi-host URI",
			uri:  "mongodb://a:27017,b:27017/app?authSource=admin",
			want: "admin",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.IsolateConfig(t)
			if _, err := credential.Store("acme", config.Credential{
				Username: "deploy", Password: "s3cret",
			}); err != nil {
				t.Fatalf("Store: %v", err)
			}

			opts, err := clientOptions(config.Connection{
				ConnectionString: tt.uri,
				Credential:       "acme",
			}, 0)
			if err != nil {
				t.Fatalf("clientOptions: %v", err)
			}
			if opts.Auth.AuthSource != tt.want {
				t.Errorf("AuthSource = %q, want %q", opts.Auth.AuthSource, tt.want)
			}
			if opts.Auth.Username != "deploy" || opts.Auth.Password != "s3cret" {
				t.Errorf("Auth = %q/%q, want the stored credential",
					opts.Auth.Username, opts.Auth.Password)
			}
		})
	}
}

// The URI may also name a mechanism. SetAuth replaces the whole Credential, so
// the stored username and password are overlaid onto what ApplyURI derived
// rather than substituted for it.
func TestClientOptionsKeepsURIAuthMechanism(t *testing.T) {
	testutil.IsolateConfig(t)
	if _, err := credential.Store("acme", config.Credential{
		Username: "deploy", Password: "s3cret",
	}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	opts, err := clientOptions(config.Connection{
		ConnectionString: "mongodb://localhost:27017/app?authMechanism=SCRAM-SHA-1",
		Credential:       "acme",
	}, 0)
	if err != nil {
		t.Fatalf("clientOptions: %v", err)
	}
	// ApplyURI leaves Auth nil here: a URI naming a mechanism with no username
	// fails the driver's validation, which is exactly the shape `connection
	// add` produces after it lifts the userinfo out.
	if opts.Auth.AuthMechanism != "SCRAM-SHA-1" {
		t.Errorf("AuthMechanism = %q, want the mechanism the URI asked for", opts.Auth.AuthMechanism)
	}
	if opts.Auth.Username != "deploy" || opts.Auth.Password != "s3cret" {
		t.Errorf("Auth = %q/%q, want the stored credential overlaid",
			opts.Auth.Username, opts.Auth.Password)
	}
}

// applyAuth's default arm is unreachable through clientOptions, because Resolve
// rejects an unregistered kind before it gets there. It stops being unreachable
// the moment a kind is added to credential's dispatch table without a matching
// arm here — the two-place-registration bug it exists to catch — so it is
// exercised directly rather than left as an untested guard.
func TestApplyAuthRejectsAKindItCannotDrive(t *testing.T) {
	_, err := applyAuth(options.Client(), config.Connection{}, credential.Resolution{
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
