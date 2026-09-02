package credential

import (
	"errors"
	"testing"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/testutil"
)

// The allowlist policy, separated from the check that applies it so the
// decision is testable without a config or a URI.
func TestAllowedHostsFor(t *testing.T) {
	widened := []string{"mongo.corp.example.com"}

	tests := []struct {
		name string
		flow *config.Flow
		want []string
	}{
		{
			name: "no flow falls back to the default",
			flow: nil,
			want: DefaultAllowedHosts,
		},
		{
			name: "an environment flow with no override uses the default",
			flow: &config.Flow{Type: config.FlowEnvironment, Environment: config.EnvironmentK8s},
			want: DefaultAllowedHosts,
		},
		{
			name: "an environment flow may widen",
			flow: &config.Flow{Type: config.FlowEnvironment, AllowedHosts: widened},
			want: widened,
		},
		{
			// The device flow's refresh token lives in the keychain and is
			// exactly what an agent cannot otherwise obtain, so its binding is
			// not overridable. This pins that before the flow exists: adding it
			// to flowsThatMayWidenHosts has to be a deliberate act that breaks
			// this test.
			name: "a flow that may not widen ignores its own allowlist",
			flow: &config.Flow{Type: config.FlowType("device"), AllowedHosts: widened},
			want: DefaultAllowedHosts,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := allowedHostsFor(tt.flow)
			if len(got) != len(tt.want) {
				t.Fatalf("allowedHostsFor() = %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("allowedHostsFor()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestOIDCFlowStatesTheInvariant(t *testing.T) {
	t.Run("wrong kind", func(t *testing.T) {
		res := Resolution{Alias: "acme", Kind: config.KindSCRAM}
		if _, err := res.OIDCFlow(); err == nil {
			t.Error("a SCRAM resolution yielded an OIDC flow")
		}
	})

	t.Run("nil flow", func(t *testing.T) {
		// A hand-built Resolution can carry an OIDC kind with no flow; the
		// accessor exists so that is an error rather than a nil dereference.
		res := Resolution{Alias: "corp", Kind: config.KindOIDC}
		if _, err := res.OIDCFlow(); err == nil {
			t.Error("a nil flow was returned as a valid one")
		}
	})

	t.Run("resolved", func(t *testing.T) {
		res := Resolution{
			Alias:      "corp",
			Kind:       config.KindOIDC,
			Credential: config.Credential{Flow: &config.Flow{Type: config.FlowEnvironment, Environment: "k8s"}},
		}
		flow, err := res.OIDCFlow()
		if err != nil {
			t.Fatalf("OIDCFlow: %v", err)
		}
		if flow.Environment != "k8s" {
			t.Errorf("Environment = %q, want k8s", flow.Environment)
		}
	})
}

func TestSupportedFlowTypesComesFromTheValidatorTable(t *testing.T) {
	got := SupportedFlowTypes()
	if len(got) != len(flows) {
		t.Fatalf("SupportedFlowTypes() = %v, want one entry per registered flow (%d)",
			got, len(flows))
	}
	for _, name := range got {
		if _, ok := flows[config.FlowType(name)]; !ok {
			t.Errorf("SupportedFlowTypes() advertises %q, which nothing in the flows table drives", name)
		}
	}
}

// A pattern that will not compile must match nothing rather than panic or match
// everything: "[" survives the glob escaping and is an invalid character class.
func TestHostAllowedIgnoresAnUncompilablePattern(t *testing.T) {
	if hostAllowed("anything.example.com", []string{"a[b"}) {
		t.Error("an uncompilable pattern matched")
	}
	// A good pattern alongside a bad one still works.
	if !hostAllowed("mongo.corp.example.com", []string{"a[b", "*.corp.example.com"}) {
		t.Error("a valid pattern was skipped because another one was invalid")
	}
}

// The resolution-carrying form is what clientOptions uses; the alias-taking one
// is for callers with no resolution yet.
func TestResolutionCheckConnection(t *testing.T) {
	oidc := Resolution{
		Alias:      "corp",
		Kind:       config.KindOIDC,
		Credential: config.Credential{Flow: &config.Flow{Type: config.FlowEnvironment}},
	}
	if err := oidc.CheckConnection("mongodb://localhost:27017/app"); err == nil {
		t.Error("a plaintext endpoint was accepted")
	}
	if err := oidc.CheckConnection("mongodb+srv://c0.abc.mongodb.net/app"); err != nil {
		t.Errorf("an allowed endpoint was refused: %v", err)
	}

	// SCRAM declares no endpoint policy, so every endpoint passes.
	scram := Resolution{Alias: "acme", Kind: config.KindSCRAM}
	if err := scram.CheckConnection("mongodb://anywhere.example.com/app"); err != nil {
		t.Errorf("SCRAM endpoint check = %v, want nil", err)
	}

	unknown := Resolution{Alias: "future", Kind: "x509"}
	if err := unknown.CheckConnection("mongodb+srv://c0.abc.mongodb.net/app"); err == nil {
		t.Error("a kind this build cannot drive was accepted")
	}
}

func TestCheckConnectionByAliasRejectsAnUnsupportedKind(t *testing.T) {
	testutil.IsolateConfig(t)
	testutil.StageCredential(t, "future", config.Credential{Kind: "x509"})

	if err := CheckConnection("future", "mongodb+srv://c0.abc.mongodb.net/app"); !errors.Is(err, ErrUnsupportedKind) {
		t.Errorf("error = %v, want ErrUnsupportedKind", err)
	}
}
