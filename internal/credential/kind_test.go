package credential

import (
	"strings"
	"testing"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/testutil"
)

// writeRaw puts an entry into config.json verbatim, bypassing Store — the only
// way to stage the shapes a hand-edited or newer-binary config can produce.
func writeRaw(t *testing.T, alias string, cred config.Credential) {
	t.Helper()
	err := config.Update(func(cfg *config.Config) error {
		if cfg.Credentials == nil {
			cfg.Credentials = map[string]config.Credential{}
		}
		cfg.Credentials[alias] = cred
		return nil
	})
	if err != nil {
		t.Fatalf("staging credential %q: %v", alias, err)
	}
}

func TestResolvedKindDefaultsToSCRAM(t *testing.T) {
	tests := []struct {
		name string
		cred config.Credential
		want config.Kind
	}{
		{"absent kind is scram", config.Credential{Username: "u", Password: "p"}, config.KindSCRAM},
		{"explicit scram", config.Credential{Kind: config.KindSCRAM}, config.KindSCRAM},
		{"unknown kind is preserved, not defaulted", config.Credential{Kind: "oidc"}, "oidc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cred.ResolvedKind(); got != tt.want {
				t.Errorf("ResolvedKind() = %q, want %q", got, tt.want)
			}
		})
	}
}

// A config written before kinds existed has no "kind" key at all; it must keep
// resolving as SCRAM with no migration step.
func TestResolveTreatsKindlessEntryAsSCRAM(t *testing.T) {
	testutil.IsolateConfig(t)
	writeRaw(t, "legacy", config.Credential{Username: "deploy", Password: "s3cret"})

	res, err := Resolve("legacy")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Kind != config.KindSCRAM {
		t.Errorf("Kind = %q, want scram", res.Kind)
	}
	if !res.Ready() || res.State != StateReady {
		t.Errorf("State = %q, want ready", res.State)
	}
	if res.Credential.Username != "deploy" || res.Credential.Password != "s3cret" {
		t.Errorf("Credential = %+v, want deploy/s3cret", res.Credential)
	}
}

// Store built the stored entry from scratch before kinds existed, which
// silently dropped every field it did not name.
func TestStorePreservesKindThroughKeychainRoundTrip(t *testing.T) {
	isolateConfig(t)
	swapKeychain(t, newFakeKeychain())

	storage, err := Store("acme", config.Credential{
		Kind: config.KindSCRAM, Username: "deploy", Password: "s3cret",
	})
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if storage != StorageKeychain {
		t.Fatalf("storage = %q, want keychain (the path that rebuilds the entry)", storage)
	}

	entry := config.Read().Credentials["acme"]
	if entry.Kind != config.KindSCRAM {
		t.Errorf("stored kind = %q, want it preserved as scram", entry.Kind)
	}
	if entry.Username != Sentinel || entry.Password != Sentinel {
		t.Errorf("entry = %+v, want both secrets replaced by the sentinel", entry)
	}
}

func TestStoreRejectsUnsupportedKind(t *testing.T) {
	testutil.IsolateConfig(t)

	_, err := Store("future", config.Credential{Kind: "oidc"})
	if err == nil {
		t.Fatal("Store accepted an unsupported kind")
	}
	if !strings.Contains(err.Error(), "scram") {
		t.Errorf("error = %q, want it to list the supported kinds", err)
	}
	if _, ok := config.Read().Credentials["future"]; ok {
		t.Error("an unsupported kind was written to config anyway")
	}
}

// The upgrade path writes username/password into the keychain and stamps
// sentinels over the entry. Running it on a kind whose empty username and
// password are correct rather than missing would destroy the credential.
func TestResolveDoesNotUpgradeUnsupportedKind(t *testing.T) {
	isolateConfig(t)
	fake := newFakeKeychain()
	swapKeychain(t, fake)
	writeRaw(t, "future", config.Credential{Kind: "oidc"})

	res, err := Resolve("future")
	if err == nil {
		t.Fatal("Resolve accepted an unsupported kind")
	}
	if res.State != StateUnsupported {
		t.Errorf("State = %q, want unsupported", res.State)
	}

	entry := config.Read().Credentials["future"]
	if entry.Kind != "oidc" || entry.Username != "" || entry.Password != "" {
		t.Errorf("entry = %+v, want it untouched by the upgrade path", entry)
	}
	if _, ok := fake.Get(usernameAccount("future")); ok {
		t.Error("upgrade wrote an empty username into the keychain")
	}
	if _, ok := fake.Get(passwordAccount("future")); ok {
		t.Error("upgrade wrote an empty password into the keychain")
	}
}

func TestResolveStates(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		testutil.IsolateConfig(t)
		res, err := Resolve("nope")
		if err == nil {
			t.Fatal("Resolve found a credential that was never stored")
		}
		if res.State != StateMissing {
			t.Errorf("State = %q, want missing", res.State)
		}
	})

	t.Run("unresolvable", func(t *testing.T) {
		isolateConfig(t)
		swapKeychain(t, newFakeKeychain())
		writeRaw(t, "ghost", config.Credential{Username: Sentinel, Password: Sentinel})

		res, err := Resolve("ghost")
		if err == nil {
			t.Fatal("Resolve succeeded with no keychain entry behind the sentinel")
		}
		if res.State != StateUnresolvable {
			t.Errorf("State = %q, want unresolvable", res.State)
		}
		// "Not found" would send the caller looking for a different alias; the
		// fix is to re-add this one.
		if !strings.Contains(err.Error(), "credential add ghost") {
			t.Errorf("error = %q, want the re-add hint", err)
		}
	})
}

// connection add/update wire up a reference; they must not demand that the
// credential can authenticate at that moment.
func TestRequireExistsAcceptsUnresolvableCredential(t *testing.T) {
	isolateConfig(t)
	swapKeychain(t, newFakeKeychain())
	writeRaw(t, "ghost", config.Credential{Username: Sentinel, Password: Sentinel})

	if _, err := Resolve("ghost"); err == nil {
		t.Fatal("precondition: expected 'ghost' to be unresolvable")
	}
	if err := RequireExists("ghost"); err != nil {
		t.Errorf("RequireExists = %v, want nil for a stored-but-unresolvable entry", err)
	}
	if err := RequireExists("absent"); err == nil {
		t.Error("RequireExists accepted an alias that is not stored")
	}
}

func TestStorageTypeIgnoresSentinelsOnNonSCRAMKinds(t *testing.T) {
	testutil.IsolateConfig(t)
	writeRaw(t, "future", config.Credential{Kind: "oidc", Username: Sentinel, Password: Sentinel})

	if got := StorageType("future"); got != StorageConfig {
		t.Errorf("StorageType = %q, want config: the sentinels are not this kind's secrets", got)
	}
}
