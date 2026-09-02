package credential

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

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

func TestAddOIDCStoresAFileFlow(t *testing.T) {
	testutil.IsolateConfig(t)

	if err := runAdd(t, "", "eks", "--oidc", "--token-file", "/var/run/secrets/token"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	flow := credstore.All()["eks"].Flow
	if flow.Type != config.FlowFile || flow.Path != "/var/run/secrets/token" {
		t.Errorf("flow = %+v, want the file flow with the given path", flow)
	}
}

func TestAddOIDCRejectsTwoFlowSelectors(t *testing.T) {
	testutil.IsolateConfig(t)

	err := runAdd(t, "", "x", "--oidc", "--environment", "k8s", "--token-file", "/var/run/secrets/token")
	if err == nil {
		t.Fatal("two flow selectors were accepted")
	}
	if !strings.Contains(err.Error(), "different OIDC flows") {
		t.Errorf("error = %q, want it to say the selectors conflict", err)
	}
}

func TestAddOIDCRequiresAFlowSelector(t *testing.T) {
	testutil.IsolateConfig(t)

	err := runAdd(t, "", "x", "--oidc")
	if err == nil {
		t.Fatal("--oidc with no flow was accepted")
	}
	var oerr *out.Error
	if !out.As(err, &oerr) {
		t.Fatalf("error = %v, want the family error contract", err)
	}
	// Both ways forward have to be named: an agent cannot guess which applies.
	if !strings.Contains(oerr.Hint, "--environment") || !strings.Contains(oerr.Hint, "--token-file") {
		t.Errorf("hint = %q, want it to name both flow selectors", oerr.Hint)
	}
}

func TestAddOIDCRejectsARelativeTokenPath(t *testing.T) {
	testutil.IsolateConfig(t)

	err := runAdd(t, "", "eks", "--oidc", "--token-file", "token")
	if !errors.Is(err, credstore.ErrInvalidFlow) {
		t.Fatalf("error = %v, want ErrInvalidFlow", err)
	}
	if _, ok := credstore.All()["eks"]; ok {
		t.Error("a credential with a relative path was written anyway")
	}
}

func TestListRendersTheTokenPath(t *testing.T) {
	testutil.IsolateConfig(t)
	testutil.StageCredential(t, "eks", config.Credential{
		Kind: config.KindOIDC,
		Flow: &config.Flow{Type: config.FlowFile, Path: "/var/run/secrets/token"},
	})

	rec := runList(t)[0]
	if rec["flow"] != "file" {
		t.Errorf("flow = %v, want file", rec["flow"])
	}
	if rec["path"] != "/var/run/secrets/token" {
		t.Errorf("path = %v, want the token path", rec["path"])
	}
}

// An empty value for a selector flag is an empty path to complain about, not a
// missing flow: selection is by which flag was given.
func TestAddOIDCTreatsAnEmptySelectorValueAsGiven(t *testing.T) {
	testutil.IsolateConfig(t)

	err := runAdd(t, "", "eks", "--oidc", "--token-file", "")
	if !errors.Is(err, credstore.ErrInvalidFlow) {
		t.Errorf("error = %v, want ErrInvalidFlow about the empty path", err)
	}
}

func TestAddOIDCStoresADeviceFlow(t *testing.T) {
	testutil.IsolateConfig(t)

	if err := runAdd(t, "", "corp", "--oidc", "--device"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	flow := credstore.All()["corp"].Flow
	if flow == nil || flow.Type != config.FlowDevice {
		t.Errorf("flow = %+v, want the device flow", flow)
	}
}

// The device flow's host binding is not overridable, so the flag is refused
// rather than stored and silently ignored.
func TestAddOIDCRejectsAllowedHostsOnTheDeviceFlow(t *testing.T) {
	testutil.IsolateConfig(t)

	err := runAdd(t, "", "corp", "--oidc", "--device", "--allowed-hosts", "evil.example.com")
	if err == nil {
		t.Fatal("--allowed-hosts was accepted for the device flow")
	}
	if !strings.Contains(err.Error(), "not overridable") {
		t.Errorf("error = %q, want it to say the binding is not overridable", err)
	}
	if _, ok := credstore.All()["corp"]; ok {
		t.Error("a credential was written anyway")
	}
}

func TestLogoutClearsTheSession(t *testing.T) {
	testutil.IsolateConfig(t)
	testutil.StageCredential(t, "corp", config.Credential{
		Kind: config.KindOIDC, Flow: &config.Flow{Type: config.FlowDevice},
	})
	if err := credstore.SaveSession("corp", credstore.Session{
		AccessToken: "t", Issuer: "https://idp.example.com", Host: "c0.abc.mongodb.net",
	}); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	root := &cobra.Command{Use: "agent-mongo"}
	Register(root, nil)
	root.SetArgs([]string{"credential", "logout", "corp"})
	root.SetOut(io.Discard)
	buf, restore := testutil.CaptureStdout(t)
	err := root.Execute()
	restore()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(buf.String(), `"loggedOut":true`) {
		t.Errorf("stdout = %q, want it to confirm the logout", buf.String())
	}

	entry, ok := credstore.All()["corp"]
	if !ok {
		t.Fatal("logout removed the credential; it should only end the session")
	}
	if entry.Session != "" {
		t.Errorf("Session = %q, want it cleared", entry.Session)
	}
}

func TestListReportsSessionState(t *testing.T) {
	testutil.IsolateConfig(t)
	testutil.StageCredential(t, "corp", config.Credential{
		Kind: config.KindOIDC, Flow: &config.Flow{Type: config.FlowDevice},
	})

	t.Run("before logging in", func(t *testing.T) {
		rec := runList(t)[0]
		if rec["loggedIn"] != false {
			t.Errorf("loggedIn = %v, want false", rec["loggedIn"])
		}
		if _, ok := rec["expiresAt"]; ok {
			t.Error("an expiry was reported for a credential with no session")
		}
	})

	t.Run("after logging in", func(t *testing.T) {
		expiry := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
		if err := credstore.SaveSession("corp", credstore.Session{
			AccessToken: "secret-token", RefreshToken: "secret-refresh",
			ExpiresAt: expiry, Issuer: "https://idp.example.com", Host: "c0.abc.mongodb.net",
		}); err != nil {
			t.Fatalf("SaveSession: %v", err)
		}

		rec := runList(t)[0]
		if rec["loggedIn"] != true {
			t.Errorf("loggedIn = %v, want true", rec["loggedIn"])
		}
		if rec["boundTo"] != "c0.abc.mongodb.net" {
			t.Errorf("boundTo = %v, want the host the session was obtained for", rec["boundTo"])
		}
		if rec["expiresAt"] != expiry.Format(time.RFC3339) {
			t.Errorf("expiresAt = %v, want %s", rec["expiresAt"], expiry.Format(time.RFC3339))
		}
		if rec["expired"] != false {
			t.Errorf("expired = %v, want false", rec["expired"])
		}

		// The row is what a person reads; the tokens must not be in it.
		encoded, err := json.Marshal(rec)
		if err != nil {
			t.Fatalf("marshal record: %v", err)
		}
		for _, secret := range []string{"secret-token", "secret-refresh"} {
			if strings.Contains(string(encoded), secret) {
				t.Errorf("credential list leaked %q", secret)
			}
		}
	})
}
