package mongo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

// The flow switch's default arm: a flow registered for validation before this
// switch learns to drive it must fail here rather than silently authenticate
// with the wrong shape.
func TestOIDCCredentialRejectsAFlowItCannotDrive(t *testing.T) {
	_, err := applyAuth(options.Client(), config.Connection{}, credential.Resolution{
		Alias:      "corp",
		Kind:       config.KindOIDC,
		Credential: config.Credential{Flow: &config.Flow{Type: config.FlowType("device")}},
	})
	if !errors.Is(err, credential.ErrInvalidFlow) {
		t.Fatalf("error = %v, want ErrInvalidFlow", err)
	}
}

// A Resolution is a plain struct, so a caller can build one with an OIDC kind
// and no flow. That must be an error, not a nil dereference.
func TestOIDCCredentialRejectsAMissingFlow(t *testing.T) {
	_, err := applyAuth(options.Client(), config.Connection{}, credential.Resolution{
		Alias: "corp",
		Kind:  config.KindOIDC,
	})
	if !errors.Is(err, credential.ErrInvalidFlow) {
		t.Fatalf("error = %v, want ErrInvalidFlow", err)
	}
}

// The file flow hands the driver a callback rather than a token, so the file is
// read when authentication happens. This exercises the callback the way the
// driver would.
func TestFileFlowSuppliesAMachineCallback(t *testing.T) {
	testutil.IsolateConfig(t)

	token := "aGVhZGVy.eyJzdWIiOiJzdmMifQ.c2ln"
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		t.Fatalf("write token: %v", err)
	}

	if _, err := credential.Store("eks", config.Credential{
		Kind: config.KindOIDC,
		Flow: &config.Flow{Type: config.FlowFile, Path: path},
	}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	opts, err := clientOptions(config.Connection{
		ConnectionString: "mongodb+srv://c0.abc.mongodb.net/app",
		Credential:       "eks",
	}, 0)
	if err != nil {
		t.Fatalf("clientOptions: %v", err)
	}
	if opts.Auth.AuthMechanism != "MONGODB-OIDC" {
		t.Errorf("AuthMechanism = %q, want MONGODB-OIDC", opts.Auth.AuthMechanism)
	}
	if opts.Auth.OIDCMachineCallback == nil {
		t.Fatal("no machine callback; the driver has no way to obtain a token")
	}
	if opts.Auth.OIDCHumanCallback != nil {
		t.Error("a human callback was set; the driver rejects both being present")
	}

	cred, err := opts.Auth.OIDCMachineCallback(context.Background(), &options.OIDCArgs{})
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	if cred.AccessToken != token {
		t.Errorf("AccessToken = %q, want the trimmed file contents %q", cred.AccessToken, token)
	}
}

// The callback reads at authentication time, so a token rotated underneath the
// process is picked up rather than cached from when options were built.
func TestFileFlowCallbackRereadsTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("aGVhZGVy.eyJzdWIiOiJhIn0.c2ln"), 0o600); err != nil {
		t.Fatalf("write token: %v", err)
	}
	callback := tokenFileCallback(path)

	first, err := callback(context.Background(), &options.OIDCArgs{})
	if err != nil {
		t.Fatalf("first callback: %v", err)
	}

	rotated := "aGVhZGVy.eyJzdWIiOiJiIn0.c2ln"
	if err := os.WriteFile(path, []byte(rotated), 0o600); err != nil {
		t.Fatalf("rotate token: %v", err)
	}
	second, err := callback(context.Background(), &options.OIDCArgs{})
	if err != nil {
		t.Fatalf("second callback: %v", err)
	}

	if first.AccessToken == second.AccessToken {
		t.Error("the callback cached the token; a rotated file would never be picked up")
	}
	if second.AccessToken != rotated {
		t.Errorf("AccessToken = %q, want the rotated token", second.AccessToken)
	}
}

func TestFileFlowCallbackSurfacesAnUnreadableToken(t *testing.T) {
	callback := tokenFileCallback(filepath.Join(t.TempDir(), "absent"))
	if _, err := callback(context.Background(), &options.OIDCArgs{}); !errors.Is(err, credential.ErrTokenUnreadable) {
		t.Errorf("error = %v, want ErrTokenUnreadable", err)
	}
}
