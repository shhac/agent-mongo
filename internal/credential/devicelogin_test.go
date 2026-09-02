package credential

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/testutil"
)

func testIDPInfo(issuer string) IDPInfo {
	return IDPInfo{Issuer: issuer, ClientID: "client-1", Scopes: []string{"openid", "offline_access"}}
}

func TestRunDeviceLogin(t *testing.T) {
	idp := useMockIDP(t, frozen)
	idp.PendingPolls = 1

	var prompted DevicePrompt
	session, err := RunDeviceLogin(context.Background(), testIDPInfo(idp.Issuer()), testHost,
		func(p DevicePrompt) { prompted = p })
	if err != nil {
		t.Fatalf("RunDeviceLogin: %v", err)
	}

	if prompted.UserCode != "WDJB-MJHT" {
		t.Errorf("prompted user code = %q", prompted.UserCode)
	}
	if prompted.VerificationURIComplete == "" {
		t.Error("no complete verification URI was offered; it is the link a person can just open")
	}
	if session.AccessToken != "access-token" || session.RefreshToken != "refresh-token" {
		t.Errorf("session = %+v, want the provider's tokens", session)
	}
	if session.Issuer != idp.Issuer() || session.ClientID != "client-1" {
		t.Errorf("session = %+v, want the issuer and client recorded for refresh", session)
	}
	if session.Host != testHost {
		t.Errorf("Host = %q, want the deployment it was obtained for", session.Host)
	}
	if want := frozen.Add(time.Hour); !session.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %s, want %s", session.ExpiresAt, want)
	}
	if got := idp.DeviceForm.Get("scope"); got != "openid offline_access" {
		t.Errorf("scope = %q, want the server's requested scopes forwarded", got)
	}
}

// The prompt has to reach the person before polling starts, or there is nothing
// to act on while the command waits.
func TestRunDeviceLoginPromptsBeforePolling(t *testing.T) {
	idp := useMockIDP(t, frozen)
	idp.PendingPolls = 2

	pollsAtPrompt := -1
	if _, err := RunDeviceLogin(context.Background(), testIDPInfo(idp.Issuer()), testHost,
		func(DevicePrompt) { pollsAtPrompt = idp.Polls }); err != nil {
		t.Fatalf("RunDeviceLogin: %v", err)
	}
	if pollsAtPrompt != 0 {
		t.Errorf("prompted after %d polls, want the code shown before any waiting", pollsAtPrompt)
	}
}

func TestRunDeviceLoginReportsDenial(t *testing.T) {
	idp := useMockIDP(t, frozen)
	idp.TokenError = "access_denied"

	_, err := RunDeviceLogin(context.Background(), testIDPInfo(idp.Issuer()), testHost, func(DevicePrompt) {})
	if err == nil {
		t.Fatal("a declined login succeeded")
	}
	if !strings.Contains(err.Error(), "declined") {
		t.Errorf("error = %q, want it to say the request was declined", err)
	}
	if hint := errHint(t, err); !strings.Contains(hint, "approve") {
		t.Errorf("hint = %q, want it to say to approve the request", hint)
	}
}

func TestRunDeviceLoginReportsAProviderWithoutTheGrant(t *testing.T) {
	idp := useMockIDP(t, frozen)
	idp.OmitDeviceEndpoint = true

	_, err := RunDeviceLogin(context.Background(), testIDPInfo(idp.Issuer()), testHost, func(DevicePrompt) {})
	if err == nil {
		t.Fatal("a provider with no device endpoint was accepted")
	}
}

func TestSaveAndClearSession(t *testing.T) {
	testutil.IsolateConfig(t)
	testutil.StageCredential(t, "corp", config.Credential{
		Kind: config.KindOIDC, Flow: &config.Flow{Type: config.FlowDevice},
	})

	session := liveSession("https://idp.example.com")
	if err := SaveSession("corp", session); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	fixedClock(t, frozen)
	if _, err := Resolve("corp"); err != nil {
		t.Fatalf("Resolve after login: %v", err)
	}

	if err := ClearSession("corp"); err != nil {
		t.Fatalf("ClearSession: %v", err)
	}
	// The credential survives; only the access window ends.
	entry, ok := All()["corp"]
	if !ok {
		t.Fatal("logout removed the credential")
	}
	if entry.Session != "" {
		t.Errorf("Session = %q, want it cleared", entry.Session)
	}
	if _, err := Resolve("corp"); err != nil {
		t.Errorf("Resolve after logout = %v, want the credential still usable to log in again", err)
	}
}

func TestClearSessionRemovesTheKeychainEntry(t *testing.T) {
	isolateConfig(t)
	fake := newFakeKeychain()
	swapKeychain(t, fake)
	testutil.StageCredential(t, "corp", config.Credential{
		Kind: config.KindOIDC, Flow: &config.Flow{Type: config.FlowDevice},
	})

	if err := SaveSession("corp", liveSession("https://idp.example.com")); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	if _, ok := fake.Get(sessionAccount("corp")); !ok {
		t.Fatal("precondition: the session should be in the keychain")
	}

	if err := ClearSession("corp"); err != nil {
		t.Fatalf("ClearSession: %v", err)
	}
	if _, ok := fake.Get(sessionAccount("corp")); ok {
		t.Error("the session survived logout in the keychain — a live bearer token with nothing referencing it")
	}
}

func TestClearSessionRejectsACredentialWithNoSession(t *testing.T) {
	testutil.IsolateConfig(t)
	if _, err := Store("acme", config.Credential{Username: "u", Password: "p"}); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if err := ClearSession("acme"); !errors.Is(err, ErrNotLoggedIn) {
		t.Errorf("error = %v, want it to say there is no session", err)
	}
	if err := ClearSession("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

// A session is the one piece of OIDC material agent-mongo keeps, so it belongs
// in the keychain like any other secret.
func TestSessionIsKeychainBacked(t *testing.T) {
	isolateConfig(t)
	fake := newFakeKeychain()
	swapKeychain(t, fake)
	testutil.StageCredential(t, "corp", config.Credential{
		Kind: config.KindOIDC, Flow: &config.Flow{Type: config.FlowDevice},
	})

	if err := SaveSession("corp", liveSession("https://idp.example.com")); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	entry := All()["corp"]
	if entry.Session != Sentinel {
		t.Errorf("config session = %q, want the keychain sentinel", entry.Session)
	}
	if StorageType(entry) != StorageKeychain {
		t.Errorf("StorageType = %q, want keychain", StorageType(entry))
	}
	stored, ok := fake.Get(sessionAccount("corp"))
	if !ok || !strings.Contains(stored, "live-token") {
		t.Errorf("keychain session = %q, want the token", stored)
	}
}

// A credential on a flow that keeps no session must not have one invented for
// it, nor be reported as keychain-backed.
func TestFlowsWithoutASessionStoreNothing(t *testing.T) {
	isolateConfig(t)
	fake := newFakeKeychain()
	swapKeychain(t, fake)

	if _, err := Store("ci", testutil.OIDCCredential(config.EnvironmentK8s)); err != nil {
		t.Fatalf("Store: %v", err)
	}
	entry := All()["ci"]
	if entry.Session != "" {
		t.Errorf("Session = %q, want it left empty", entry.Session)
	}
	if _, ok := fake.Get(sessionAccount("ci")); ok {
		t.Error("an empty session was written to the keychain")
	}
	if got := StorageType(entry); got != StorageConfig {
		t.Errorf("StorageType = %q, want config", got)
	}
}

// Remove asks the kind for its accounts, so a session goes with the credential
// rather than being stranded in the keychain.
func TestRemoveErasesTheSession(t *testing.T) {
	isolateConfig(t)
	fake := newFakeKeychain()
	swapKeychain(t, fake)
	testutil.StageCredential(t, "corp", config.Credential{
		Kind: config.KindOIDC, Flow: &config.Flow{Type: config.FlowDevice},
	})
	if err := SaveSession("corp", liveSession("https://idp.example.com")); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	if err := Remove("corp"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok := fake.Get(sessionAccount("corp")); ok {
		t.Error("the session outlived the credential in the keychain")
	}
}

// The device flow's binding is not overridable: its refresh token is exactly
// the material an agent cannot otherwise obtain.
func TestDeviceFlowCannotWidenItsAllowedHosts(t *testing.T) {
	got := allowedHostsFor(&config.Flow{
		Type:         config.FlowDevice,
		AllowedHosts: []string{"evil.example.com"},
	})
	if len(got) != len(DefaultAllowedHosts) {
		t.Fatalf("allowedHostsFor = %v, want the default list", got)
	}
	if FlowMayWidenHosts(config.FlowDevice) {
		t.Error("the device flow reports that it may widen its allowed hosts")
	}
}

// "Keeps a session" was asked at three different strengths, so logging out a
// credential on a platform-identity flow reported success and told the person
// to log in again — for a credential that can never be logged in.
func TestClearSessionRejectsFlowsThatKeepNoSession(t *testing.T) {
	tests := []struct {
		name string
		cred config.Credential
	}{
		{"scram", config.Credential{Username: "u", Password: "p"}},
		{"oidc environment flow", testutil.OIDCCredential(config.EnvironmentK8s)},
		{
			name: "oidc file flow",
			cred: config.Credential{
				Kind: config.KindOIDC,
				Flow: &config.Flow{Type: config.FlowFile, Path: "/var/run/token"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.IsolateConfig(t)
			testutil.StageCredential(t, "cred", tt.cred)
			if err := ClearSession("cred"); !errors.Is(err, ErrNotLoggedIn) {
				t.Errorf("ClearSession = %v, want it to say there is no session to clear", err)
			}
		})
	}
}

func TestIsDeviceFlow(t *testing.T) {
	tests := []struct {
		name string
		cred config.Credential
		want bool
	}{
		{"device", config.Credential{Kind: config.KindOIDC, Flow: &config.Flow{Type: config.FlowDevice}}, true},
		{"environment", testutil.OIDCCredential(config.EnvironmentK8s), false},
		{"oidc with no flow", config.Credential{Kind: config.KindOIDC}, false},
		{"scram", config.Credential{Username: "u", Password: "p"}, false},
		{"absent kind", config.Credential{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDeviceFlow(tt.cred); got != tt.want {
				t.Errorf("IsDeviceFlow() = %v, want %v", got, tt.want)
			}
		})
	}
}
