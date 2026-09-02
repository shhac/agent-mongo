package mongo

import (
	"context"
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/testutil"
)

func oidcConnection(t *testing.T, uri string, flow *config.Flow) (*optionsAuth, error) {
	t.Helper()
	cred := testutil.OIDCCredential("")
	cred.Flow = flow
	if _, err := credential.Store("corp", cred); err != nil {
		t.Fatalf("Store: %v", err)
	}
	opts, err := clientOptions(config.Connection{
		ConnectionString: uri,
		Credential:       "corp",
	}, 0)
	if err != nil {
		return nil, err
	}
	return &optionsAuth{opts.Auth.AuthMechanism, opts.Auth.AuthMechanismProperties,
		opts.Auth.AuthSource, opts.Auth.Username, opts.Auth.Password}, nil
}

type optionsAuth struct {
	mechanism  string
	props      map[string]string
	authSource string
	username   string
	password   string
}

func TestOIDCCredentialDrivesTheEnvironmentFlow(t *testing.T) {
	tests := []struct {
		name      string
		flow      *config.Flow
		wantProps map[string]string
		wantUser  string
	}{
		{
			name:      "k8s needs only the environment",
			flow:      &config.Flow{Type: config.FlowEnvironment, Environment: config.EnvironmentK8s},
			wantProps: map[string]string{"ENVIRONMENT": "k8s"},
		},
		{
			name: "azure carries the audience and the managed identity",
			flow: &config.Flow{
				Type: config.FlowEnvironment, Environment: config.EnvironmentAzure,
				TokenResource: "api://mongodb-atlas", ClientID: "0oa1b2c3",
			},
			wantProps: map[string]string{"ENVIRONMENT": "azure", "TOKEN_RESOURCE": "api://mongodb-atlas"},
			wantUser:  "0oa1b2c3",
		},
		{
			name: "gcp carries the audience",
			flow: &config.Flow{
				Type: config.FlowEnvironment, Environment: config.EnvironmentGCP,
				TokenResource: "api://mongodb-atlas",
			},
			wantProps: map[string]string{"ENVIRONMENT": "gcp", "TOKEN_RESOURCE": "api://mongodb-atlas"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.IsolateConfig(t)
			auth, err := oidcConnection(t, "mongodb+srv://c0.abc.mongodb.net/app", tt.flow)
			if err != nil {
				t.Fatalf("clientOptions: %v", err)
			}

			if auth.mechanism != "MONGODB-OIDC" {
				t.Errorf("AuthMechanism = %q, want MONGODB-OIDC", auth.mechanism)
			}
			if len(auth.props) != len(tt.wantProps) {
				t.Errorf("properties = %v, want %v", auth.props, tt.wantProps)
			}
			for key, want := range tt.wantProps {
				if auth.props[key] != want {
					t.Errorf("property %s = %q, want %q", key, auth.props[key], want)
				}
			}
			if auth.username != tt.wantUser {
				t.Errorf("Username = %q, want %q", auth.username, tt.wantUser)
			}
			// The driver rejects a password outright for MONGODB-OIDC, and
			// requires authSource to be empty or $external — it fills that in
			// itself, so agent-mongo must leave it alone.
			if auth.password != "" {
				t.Errorf("Password = %q, want empty: the driver rejects one for OIDC", auth.password)
			}
			if auth.authSource != "" {
				t.Errorf("AuthSource = %q, want empty so the driver applies $external", auth.authSource)
			}
		})
	}
}

// The endpoint check runs at connect too, not only when the connection is wired
// up, because the connection string can be edited afterwards.
func TestClientOptionsRefusesOIDCOverPlaintext(t *testing.T) {
	testutil.IsolateConfig(t)
	_, err := oidcConnection(t, "mongodb://localhost:27017/app",
		&config.Flow{Type: config.FlowEnvironment, Environment: config.EnvironmentK8s})
	if !errors.Is(err, credential.ErrInsecureConnection) {
		t.Fatalf("error = %v, want ErrInsecureConnection", err)
	}
}

func TestClientOptionsRefusesOIDCToADisallowedHost(t *testing.T) {
	testutil.IsolateConfig(t)
	_, err := oidcConnection(t, "mongodb+srv://evil.example.com/app",
		&config.Flow{Type: config.FlowEnvironment, Environment: config.EnvironmentK8s})
	if !errors.Is(err, credential.ErrHostNotAllowed) {
		t.Fatalf("error = %v, want ErrHostNotAllowed", err)
	}
}

// A flow this build cannot drive now fails when the driver invokes the
// callback rather than when options are built. Resolve rejects an unregistered
// flow long before this, so the path is unreachable in practice; the callback
// is still where the error has to appear, because that is where every
// token-holding flow reports one.
func TestOIDCCallbackRejectsAFlowItCannotDrive(t *testing.T) {
	opts, err := applyAuth(options.Client(), config.Connection{ConnectionString: "mongodb+srv://c0.abc.mongodb.net/app"}, credential.Resolution{
		Alias:      "corp",
		Kind:       config.KindOIDC,
		Credential: config.Credential{Flow: &config.Flow{Type: config.FlowType("browser")}},
	})
	if err != nil {
		t.Fatalf("applyAuth: %v", err)
	}
	if _, err := opts.Auth.OIDCMachineCallback(context.Background(), &options.OIDCArgs{}); !errors.Is(err, credential.ErrInvalidFlow) {
		t.Fatalf("callback error = %v, want ErrInvalidFlow", err)
	}
}

// A Resolution is a plain struct, so a caller can build one with an OIDC kind
// and no flow. That must be an error, not a nil dereference.
func TestOIDCCredentialRejectsAMissingFlow(t *testing.T) {
	_, err := applyAuth(options.Client(), config.Connection{ConnectionString: "mongodb+srv://c0.abc.mongodb.net/app"}, credential.Resolution{
		Alias: "corp",
		Kind:  config.KindOIDC,
	})
	if !errors.Is(err, credential.ErrInvalidFlow) {
		t.Fatalf("error = %v, want ErrInvalidFlow", err)
	}
}
