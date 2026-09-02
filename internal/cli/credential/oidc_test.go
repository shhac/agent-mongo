package credential

import (
	"errors"
	"strings"
	"testing"

	out "github.com/shhac/lib-agent-output"

	"github.com/shhac/agent-mongo/internal/config"
	credstore "github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/testutil"
)

func TestAddOIDCStoresTheFlow(t *testing.T) {
	testutil.IsolateConfig(t)

	if err := runAdd(t, "", "corp", "--oidc", "--environment", "k8s"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	entry := credstore.All()["corp"]
	if entry.ResolvedKind() != config.KindOIDC {
		t.Fatalf("kind = %q, want oidc", entry.ResolvedKind())
	}
	if entry.Flow == nil || entry.Flow.Environment != config.EnvironmentK8s {
		t.Errorf("flow = %+v, want the k8s environment", entry.Flow)
	}
	if entry.Username != "" || entry.Password != "" {
		t.Errorf("entry = %+v, want no username or password stored", entry)
	}
}

func TestAddOIDCCarriesAzureOptions(t *testing.T) {
	testutil.IsolateConfig(t)

	err := runAdd(t, "", "azfn", "--oidc", "--environment", "azure",
		"--token-resource", "api://mongodb-atlas", "--client-id", "0oa1b2c3")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	flow := credstore.All()["azfn"].Flow
	if flow.TokenResource != "api://mongodb-atlas" || flow.ClientID != "0oa1b2c3" {
		t.Errorf("flow = %+v, want the audience and client id preserved", flow)
	}
}

func TestAddOIDCCarriesAllowedHosts(t *testing.T) {
	testutil.IsolateConfig(t)

	err := runAdd(t, "", "corp", "--oidc", "--environment", "k8s",
		"--allowed-hosts", "mongo.corp.example.com,*.corp.example.net")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := credstore.All()["corp"].Flow.AllowedHosts
	want := []string{"mongo.corp.example.com", "*.corp.example.net"}
	if len(got) != len(want) {
		t.Fatalf("allowedHosts = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("allowedHosts[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAddOIDCRejectsAnInvalidFlowBeforeWriting(t *testing.T) {
	testutil.IsolateConfig(t)

	err := runAdd(t, "", "corp", "--oidc", "--environment", "aws")
	if !errors.Is(err, credstore.ErrInvalidFlow) {
		t.Fatalf("error = %v, want ErrInvalidFlow", err)
	}
	if !strings.Contains(err.Error(), "k8s, azure, gcp") {
		t.Errorf("error = %q, want it to list the valid environments", err)
	}
	if _, ok := credstore.All()["corp"]; ok {
		t.Error("an invalid credential was written anyway")
	}
}

func TestAddOIDCRequiresATokenResourceForAzure(t *testing.T) {
	testutil.IsolateConfig(t)

	err := runAdd(t, "", "azfn", "--oidc", "--environment", "azure")
	if !errors.Is(err, credstore.ErrInvalidFlow) {
		t.Fatalf("error = %v, want ErrInvalidFlow", err)
	}
	if !strings.Contains(err.Error(), "token resource") {
		t.Errorf("error = %q, want it to name the missing audience", err)
	}
}

// The two kinds share no flags, so mixing them is a mistake rather than a
// precedence question to resolve at runtime.
func TestAddRejectsMixingOIDCWithPasswordFlags(t *testing.T) {
	tests := [][]string{
		{"corp", "--oidc", "--environment", "k8s", "--username", "deploy"},
		{"corp", "--oidc", "--environment", "k8s", "--password", "s3cret"},
		{"corp", "--oidc", "--environment", "k8s", "--form"},
		{"corp", "--username", "deploy", "--environment", "k8s"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			testutil.IsolateConfig(t)
			if err := runAdd(t, "", args...); err == nil {
				t.Fatal("mixed flags were accepted")
			}
			if _, ok := credstore.All()["corp"]; ok {
				t.Error("a credential was written despite the flag conflict")
			}
		})
	}
}

// A SCRAM add with no password should point at the identity-provider path too:
// an agent that cannot supply a secret has somewhere else to go.
func TestSCRAMMissingPasswordMentionsOIDC(t *testing.T) {
	testutil.IsolateConfig(t)

	err := runAdd(t, "", "nopass", "--username", "deploy")
	if err == nil {
		t.Fatal("expected a missing-password error")
	}
	if !strings.Contains(err.Error(), "--oidc") {
		t.Errorf("error = %q, want it to mention the --oidc alternative", err)
	}
}

func TestListRendersOIDCFlow(t *testing.T) {
	testutil.IsolateConfig(t)
	testutil.StageCredential(t, "corp", config.Credential{
		Kind: config.KindOIDC,
		Flow: &config.Flow{
			Type:         config.FlowEnvironment,
			Environment:  config.EnvironmentK8s,
			AllowedHosts: []string{"*.corp.example.com"},
		},
	})

	records := runList(t)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	rec := records[0]
	if rec["kind"] != "oidc" {
		t.Errorf("kind = %v, want oidc", rec["kind"])
	}
	if rec["flow"] != "environment" {
		t.Errorf("flow = %v, want environment", rec["flow"])
	}
	if rec["environment"] != "k8s" {
		t.Errorf("environment = %v, want k8s", rec["environment"])
	}
	if _, ok := rec["password"]; ok {
		t.Error("a password column was rendered for a credential that has none")
	}
}

// An OIDC flag without --oidc used to be accepted and then ignored: the command
// fell through to the SCRAM path and complained about a missing password.
func TestAddRejectsOIDCFlagsWithoutOIDC(t *testing.T) {
	testutil.IsolateConfig(t)

	err := runAdd(t, "", "corp", "--environment", "k8s")
	if err == nil {
		t.Fatal("--environment without --oidc was accepted")
	}
	if !strings.Contains(err.Error(), "--environment") {
		t.Errorf("error = %q, want it to name the offending flag", err)
	}
	// The contract puts the next action in the hint, not the message.
	var oerr *out.Error
	if !out.As(err, &oerr) {
		t.Fatalf("error = %v, want the family error contract", err)
	}
	if !strings.Contains(oerr.Hint, "--oidc") {
		t.Errorf("hint = %q, want it to name --oidc as the fix", oerr.Hint)
	}
	if _, ok := credstore.All()["corp"]; ok {
		t.Error("a credential was written anyway")
	}
}

// The pairwise exclusion rules missed this pair entirely.
func TestAddRejectsPasswordWithOIDCFlags(t *testing.T) {
	testutil.IsolateConfig(t)

	err := runAdd(t, "", "corp", "--password", "s3cret", "--environment", "k8s")
	if err == nil {
		t.Fatal("--password with --environment was accepted")
	}
	if _, ok := credstore.All()["corp"]; ok {
		t.Error("a credential was written anyway")
	}
}
