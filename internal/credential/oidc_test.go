package credential

import (
	"errors"
	"strings"
	"testing"

	out "github.com/shhac/lib-agent-output"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/testutil"
)

func envFlow(environment string) *config.Flow {
	return &config.Flow{Type: config.FlowEnvironment, Environment: environment}
}

func TestValidateFlow(t *testing.T) {
	tests := []struct {
		name    string
		flow    *config.Flow
		wantErr bool
		wantIn  string
	}{
		{
			name: "k8s needs nothing else",
			flow: envFlow(config.EnvironmentK8s),
		},
		{
			name: "azure with an audience",
			flow: &config.Flow{
				Type: config.FlowEnvironment, Environment: config.EnvironmentAzure,
				TokenResource: "api://mongodb-atlas",
			},
		},
		{
			name: "gcp with an audience",
			flow: &config.Flow{
				Type: config.FlowEnvironment, Environment: config.EnvironmentGCP,
				TokenResource: "api://mongodb-atlas",
			},
		},
		{
			name:    "no flow at all",
			flow:    nil,
			wantErr: true,
			wantIn:  "no flow",
		},
		{
			name:    "unknown flow type",
			flow:    &config.Flow{Type: "device"},
			wantErr: true,
			wantIn:  "environment",
		},
		{
			name:    "unknown environment",
			flow:    envFlow("aws"),
			wantErr: true,
			wantIn:  "k8s, azure, gcp",
		},
		{
			name:    "empty environment",
			flow:    envFlow(""),
			wantErr: true,
			wantIn:  "k8s, azure, gcp",
		},
		{
			name:    "azure without an audience",
			flow:    envFlow(config.EnvironmentAzure),
			wantErr: true,
			wantIn:  "token resource",
		},
		{
			name:    "gcp without an audience",
			flow:    envFlow(config.EnvironmentGCP),
			wantErr: true,
			wantIn:  "token resource",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFlow("corp", tt.flow)
			if tt.wantErr == (err == nil) {
				t.Fatalf("ValidateFlow() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				return
			}
			if !errors.Is(err, ErrInvalidFlow) {
				t.Errorf("error = %v, want it to wrap ErrInvalidFlow", err)
			}
			if !strings.Contains(err.Error(), tt.wantIn) {
				t.Errorf("error = %q, want it to mention %q", err, tt.wantIn)
			}
			var oerr *out.Error
			if out.As(err, &oerr) && oerr.Hint == "" {
				t.Error("flow errors must name the command that fixes them")
			}
		})
	}
}

// A hand-edited config is validated when it is used, not only when written, so
// a bad recipe fails with agent-mongo's own error rather than inside the driver.
func TestResolveValidatesTheStoredFlow(t *testing.T) {
	testutil.IsolateConfig(t)
	testutil.StageCredential(t, "corp", config.Credential{
		Kind: config.KindOIDC, Flow: envFlow("aws"),
	})

	if _, err := Resolve("corp"); !errors.Is(err, ErrInvalidFlow) {
		t.Errorf("error = %v, want ErrInvalidFlow", err)
	}
}

func TestStoreOIDCRoundTrip(t *testing.T) {
	testutil.IsolateConfig(t)

	storage, err := Store("corp", config.Credential{
		Kind: config.KindOIDC, Flow: envFlow(config.EnvironmentK8s),
	})
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if storage != StorageConfig {
		t.Errorf("storage = %q, want config: an environment flow holds no secret", storage)
	}

	res, err := Resolve("corp")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Kind != config.KindOIDC {
		t.Errorf("Kind = %q, want oidc", res.Kind)
	}
	if res.Credential.Flow == nil || res.Credential.Flow.Environment != config.EnvironmentK8s {
		t.Errorf("Flow = %+v, want the k8s environment preserved", res.Credential.Flow)
	}
	if res.Credential.Username != "" || res.Credential.Password != "" {
		t.Errorf("Credential = %+v, want no username or password", res.Credential)
	}
}

func TestStoreOIDCRejectsAnInvalidFlow(t *testing.T) {
	testutil.IsolateConfig(t)

	if _, err := Store("corp", config.Credential{
		Kind: config.KindOIDC, Flow: envFlow("aws"),
	}); !errors.Is(err, ErrInvalidFlow) {
		t.Fatalf("Store error = %v, want ErrInvalidFlow", err)
	}
	if _, ok := config.Read().Credentials["corp"]; ok {
		t.Error("an invalid flow was written to config anyway")
	}
}

// The OIDC read path must never reach the SCRAM plaintext migration: an OIDC
// credential's empty username and password are correct, not missing.
func TestResolveOIDCNeverTouchesTheKeychain(t *testing.T) {
	isolateConfig(t)
	fake := newFakeKeychain()
	swapKeychain(t, fake)
	testutil.StageCredential(t, "corp", config.Credential{
		Kind: config.KindOIDC, Flow: envFlow(config.EnvironmentK8s),
	})

	if _, err := Resolve("corp"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if _, ok := fake.Get(usernameAccount("corp")); ok {
		t.Error("the OIDC read path wrote a username into the keychain")
	}
	entry := config.Read().Credentials["corp"]
	if entry.Username != "" || entry.Password != "" {
		t.Errorf("entry = %+v, want it untouched by the upgrade path", entry)
	}
}

func TestCheckConnectionRequiresTLS(t *testing.T) {
	testutil.IsolateConfig(t)
	testutil.StageCredential(t, "corp", config.Credential{
		Kind: config.KindOIDC, Flow: envFlow(config.EnvironmentK8s),
	})

	err := CheckConnection("corp", "mongodb://localhost:27017/app")
	if !errors.Is(err, ErrInsecureConnection) {
		t.Fatalf("error = %v, want ErrInsecureConnection", err)
	}
	if err := CheckConnection("corp", "mongodb://localhost:27017/app?tls=true"); err != nil {
		t.Errorf("tls=true was rejected: %v", err)
	}
	if err := CheckConnection("corp", "mongodb+srv://c0.abc.mongodb.net/app"); err != nil {
		t.Errorf("srv URI was rejected: %v", err)
	}
}

// The driver applies its allowed-hosts list only to the human flow, so a
// machine flow will hand a platform identity token to whatever host the URI
// names. An agent can add a connection, so agent-mongo applies the check itself.
func TestCheckConnectionEnforcesAllowedHosts(t *testing.T) {
	tests := []struct {
		name    string
		hosts   []string
		uri     string
		allowed bool
	}{
		{"atlas by default", nil, "mongodb+srv://c0.abc.mongodb.net/app", true},
		{"gov cloud by default", nil, "mongodb+srv://c0.abc.mongodbgov.net/app", true},
		{"loopback by default", nil, "mongodb://localhost:27017/app?tls=true", true},
		{"ipv4 loopback by default", nil, "mongodb://127.0.0.1:27017/app?tls=true", true},
		{"ipv6 loopback by default", nil, "mongodb://[::1]:27017/app?tls=true", true},
		{"a stranger is refused", nil, "mongodb+srv://evil.example.com/app", false},
		{
			name: "a bare domain does not match its own wildcard",
			// "*.mongodb.net" matches a subdomain, not mongodb.net itself.
			uri: "mongodb+srv://mongodb.net/app", allowed: false,
		},
		{
			name:  "an explicit allowlist admits a self-hosted deployment",
			hosts: []string{"mongo.corp.example.com"},
			uri:   "mongodb://mongo.corp.example.com:27017/app?tls=true", allowed: true,
		},
		{
			name:  "an explicit allowlist replaces the default rather than adding to it",
			hosts: []string{"mongo.corp.example.com"},
			uri:   "mongodb+srv://c0.abc.mongodb.net/app", allowed: false,
		},
		{
			name:  "wildcards work in an explicit allowlist",
			hosts: []string{"*.corp.example.com"},
			uri:   "mongodb://mongo.corp.example.com:27017/app?tls=true", allowed: true,
		},
		{
			// DNS is case-insensitive, so a URI pasted with any capitalisation
			// has to match. The wildcard arm used to be case-sensitive while
			// the literal arm was not, so this was denied and localhost was not.
			name: "an uppercase host still matches",
			uri:  "mongodb+srv://C0.ABC.MONGODB.NET/app", allowed: true,
		},
		{
			name:  "an uppercase pattern still matches",
			hosts: []string{"*.CORP.EXAMPLE.COM"},
			uri:   "mongodb://mongo.corp.example.com:27017/app?tls=true", allowed: true,
		},
		{
			name: "a trailing-dot FQDN names the same host",
			uri:  "mongodb+srv://c0.abc.mongodb.net./app", allowed: true,
		},
		{
			// The driver compiles arbitrary globs, and --allowed-hosts promises
			// the same; only a leading "*." used to work.
			name:  "a wildcard in the middle of a label",
			hosts: []string{"db-*.corp.example.com"},
			uri:   "mongodb://db-01.corp.example.com:27017/app?tls=true", allowed: true,
		},
		{
			name:  "a bare star disables the guard, explicitly",
			hosts: []string{"*"},
			uri:   "mongodb+srv://anything.example.com/app", allowed: true,
		},
		{
			name:  "a pattern is a literal dot, not a regexp wildcard",
			hosts: []string{"a.example.com"},
			uri:   "mongodb+srv://axexample.com/app", allowed: false,
		},
		{
			// ParseHostFromURI cannot read a host out of this, and a URI whose
			// host is unknown is exactly the one a token must not go to.
			name: "a schemeless URI has no host and is denied",
			uri:  "evil.example.com:27017?tls=true", allowed: false,
		},
		{
			// IsTLS and ParseHostFromURI disagree about where the query starts
			// here; both currently fail closed, and this pins that.
			name: "a host smuggled into the userinfo is denied",
			uri:  "mongodb://user:x?tls=true&@evil.example.com/app", allowed: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.IsolateConfig(t)
			testutil.StageCredential(t, "corp", config.Credential{
				Kind: config.KindOIDC,
				Flow: &config.Flow{
					Type:         config.FlowEnvironment,
					Environment:  config.EnvironmentK8s,
					AllowedHosts: tt.hosts,
				},
			})

			err := CheckConnection("corp", tt.uri)
			if tt.allowed && err != nil {
				t.Fatalf("CheckConnection(%q) = %v, want it allowed", tt.uri, err)
			}
			if !tt.allowed {
				if !errors.Is(err, ErrHostNotAllowed) {
					t.Fatalf("CheckConnection(%q) = %v, want ErrHostNotAllowed", tt.uri, err)
				}
				if !strings.Contains(err.Error(), "allowed hosts") {
					t.Errorf("error = %q, want it to name the allowlist", err)
				}
			}
		})
	}
}

// SCRAM places no constraint on the endpoint: there is no bearer token to leak,
// and requiring TLS of every existing connection would be a breaking change.
func TestCheckConnectionIgnoresSCRAM(t *testing.T) {
	testutil.IsolateConfig(t)
	if _, err := Store("acme", config.Credential{Username: "u", Password: "p"}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if err := CheckConnection("acme", "mongodb://evil.example.com:27017/app"); err != nil {
		t.Errorf("CheckConnection = %v, want nil for a SCRAM credential", err)
	}
}

func TestCheckConnectionReportsAMissingCredential(t *testing.T) {
	testutil.IsolateConfig(t)
	if err := CheckConnection("nope", "mongodb+srv://c0.abc.mongodb.net/app"); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

// Remove asks the kind for its accounts; OIDC owns none until a session exists,
// and must not have SCRAM's pair deleted on its behalf.
func TestRemoveOIDCLeavesSCRAMAccountsAlone(t *testing.T) {
	isolateConfig(t)
	fake := newFakeKeychain()
	swapKeychain(t, fake)

	if _, err := Store("acme", config.Credential{Username: "u", Password: "p"}); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if _, err := Store("corp", config.Credential{
		Kind: config.KindOIDC, Flow: envFlow(config.EnvironmentK8s),
	}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if err := Remove("corp"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	for _, account := range scramAccounts("acme") {
		if _, ok := fake.Get(account); !ok {
			t.Errorf("removing the OIDC credential erased %q, which belongs to another credential", account)
		}
	}
}

func TestHostAllowedDeniesAnUnknownHost(t *testing.T) {
	if hostAllowed("", DefaultAllowedHosts) {
		t.Error("an empty host was allowed; a URI whose host cannot be read must be denied")
	}
}

func TestOIDCStorageTypeIsAlwaysConfig(t *testing.T) {
	// The environment flow holds no secret, so there is nothing in the keychain
	// to report even on a host that has one.
	got := StorageType(config.Credential{Kind: config.KindOIDC, Flow: envFlow(config.EnvironmentK8s)})
	if got != StorageConfig {
		t.Errorf("StorageType = %q, want config", got)
	}
}

func TestValidateFileFlow(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		wantIn  string
	}{
		{name: "absolute path", path: "/var/run/secrets/token"},
		{name: "no path", wantErr: true, wantIn: "no token path"},
		{name: "relative path", path: "token", wantErr: true, wantIn: "relative"},
		{name: "dot-relative path", path: "./token", wantErr: true, wantIn: "relative"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFlow("eks", &config.Flow{Type: config.FlowFile, Path: tt.path})
			if tt.wantErr == (err == nil) {
				t.Fatalf("ValidateFlow() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				return
			}
			if !errors.Is(err, ErrInvalidFlow) {
				t.Errorf("error = %v, want ErrInvalidFlow", err)
			}
			if !strings.Contains(err.Error(), tt.wantIn) {
				t.Errorf("error = %q, want it to mention %q", err, tt.wantIn)
			}
		})
	}
}

// The path is not read when the credential is stored: a token file that does
// not exist yet, or has been rotated away, is an authentication-time failure,
// not a reason to refuse to save the recipe.
func TestStoreFileFlowDoesNotReadTheToken(t *testing.T) {
	testutil.IsolateConfig(t)

	if _, err := Store("eks", config.Credential{
		Kind: config.KindOIDC,
		Flow: &config.Flow{Type: config.FlowFile, Path: "/definitely/not/here"},
	}); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if _, err := Resolve("eks"); err != nil {
		t.Errorf("Resolve: %v", err)
	}
}

func TestFileFlowMayWidenItsAllowedHosts(t *testing.T) {
	widened := []string{"mongo.corp.example.com"}
	got := allowedHostsFor(&config.Flow{Type: config.FlowFile, AllowedHosts: widened})
	if len(got) != 1 || got[0] != widened[0] {
		t.Errorf("allowedHostsFor = %v, want the override %v", got, widened)
	}
}
