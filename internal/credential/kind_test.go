package credential

import (
	"errors"
	"strings"
	"testing"

	out "github.com/shhac/lib-agent-output"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/testutil"
)

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
	testutil.StageCredential(t, "legacy", config.Credential{Username: "deploy", Password: "s3cret"})

	res, err := Resolve("legacy")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Kind != config.KindSCRAM {
		t.Errorf("Kind = %q, want scram", res.Kind)
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
	testutil.StageCredential(t, "future", config.Credential{Kind: "oidc"})

	_, err := Resolve("future")
	if err == nil {
		t.Fatal("Resolve accepted an unsupported kind")
	}
	if !errors.Is(err, ErrUnsupportedKind) {
		t.Errorf("error = %v, want it to wrap ErrUnsupportedKind", err)
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
		if _, err := Resolve("nope"); !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want it to wrap ErrNotFound", err)
		}
	})

	t.Run("unresolvable", func(t *testing.T) {
		isolateConfig(t)
		swapKeychain(t, newFakeKeychain())
		testutil.StageCredential(t, "ghost", config.Credential{Username: Sentinel, Password: Sentinel})

		_, err := Resolve("ghost")
		if err == nil {
			t.Fatal("Resolve succeeded with no keychain entry behind the sentinel")
		}
		// "Not found" would send the caller looking for a different alias; the
		// fix is to re-add this one.
		if !errors.Is(err, ErrUnresolvable) {
			t.Errorf("error = %v, want it to wrap ErrUnresolvable", err)
		}
		if errors.Is(err, ErrNotFound) {
			t.Error("an unreadable secret must not present as a missing credential")
		}
		var oerr *out.Error
		if !out.As(err, &oerr) {
			t.Fatalf("error = %v, want the family error contract", err)
		}
		if oerr.FixableBy != out.FixableByHuman {
			t.Errorf("fixable_by = %q, want human: --form opens an OS dialog", oerr.FixableBy)
		}
		if !strings.Contains(oerr.Hint, "credential add ghost") {
			t.Errorf("hint = %q, want the re-add command", oerr.Hint)
		}
	})
}

// connection add/update wire up a reference; they must not demand that the
// credential can authenticate at that moment.
func TestRequireExistsAcceptsUnresolvableCredential(t *testing.T) {
	isolateConfig(t)
	swapKeychain(t, newFakeKeychain())
	testutil.StageCredential(t, "ghost", config.Credential{Username: Sentinel, Password: Sentinel})

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
	testutil.StageCredential(t, "future", config.Credential{Kind: "oidc", Username: Sentinel, Password: Sentinel})

	if got := StorageType(All()["future"]); got != StorageConfig {
		t.Errorf("StorageType = %q, want config: the sentinels are not this kind's secrets", got)
	}
}

// Inspect exists so a caller can compare what is stored without rewriting it.
// The plaintext-to-keychain migration used to fire from the read path, so a
// caller merely checking a credential mutated config.json and the keychain.
func TestInspectResolvesWithoutMigrating(t *testing.T) {
	isolateConfig(t)
	fake := newFakeKeychain()
	swapKeychain(t, fake)
	testutil.StageCredential(t, "legacy", config.Credential{Username: "deploy", Password: "s3cret"})

	res, err := Inspect("legacy")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if res.Credential.Username != "deploy" || res.Credential.Password != "s3cret" {
		t.Errorf("Credential = %+v, want the stored values resolved", res.Credential)
	}

	entry := config.Read().Credentials["legacy"]
	if entry.Username != "deploy" || entry.Password != "s3cret" {
		t.Errorf("entry = %+v, want it still plaintext: Inspect must not migrate", entry)
	}
	if _, ok := fake.Get(usernameAccount("legacy")); ok {
		t.Error("Inspect wrote the username into the keychain")
	}

	// The authenticate path still migrates: this is a split of responsibility,
	// not a removal of the upgrade.
	if _, err := Resolve("legacy"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := config.Read().Credentials["legacy"].Username; got != Sentinel {
		t.Errorf("entry username = %q after Resolve, want the sentinel", got)
	}
}

// Remove asks the kind which accounts it owns. Naming SCRAM's pair directly
// would leave a future kind's secret in the keychain with nothing referencing
// it — the config entry is gone, so the CLI can no longer show or erase it.
func TestRemoveErasesTheKindsKeychainAccounts(t *testing.T) {
	isolateConfig(t)
	fake := newFakeKeychain()
	swapKeychain(t, fake)

	if _, err := Store("acme", config.Credential{Username: "deploy", Password: "s3cret"}); err != nil {
		t.Fatalf("Store: %v", err)
	}
	for _, account := range scramAccounts("acme") {
		if _, ok := fake.Get(account); !ok {
			t.Fatalf("precondition: %q should hold a secret", account)
		}
	}

	if err := Remove("acme"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	for _, account := range scramAccounts("acme") {
		if _, ok := fake.Get(account); ok {
			t.Errorf("%q survived Remove — a stranded secret", account)
		}
	}
}

// An entry from a newer build must stay removable; refusing would strand it in
// config.json with no way to delete it from this binary.
func TestRemoveDropsUnsupportedKindEntry(t *testing.T) {
	isolateConfig(t)
	swapKeychain(t, newFakeKeychain())
	testutil.StageCredential(t, "future", config.Credential{Kind: "oidc"})

	if err := Remove("future"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok := config.Read().Credentials["future"]; ok {
		t.Error("entry survived Remove")
	}
}

func TestSupportedKindsComesFromTheDispatchTable(t *testing.T) {
	got := SupportedKinds()
	if len(got) != len(kinds) {
		t.Fatalf("SupportedKinds() = %v (%d), want one entry per dispatch-table kind (%d)",
			got, len(got), len(kinds))
	}
	for _, name := range got {
		if _, ok := kinds[config.Kind(name)]; !ok {
			t.Errorf("SupportedKinds() advertises %q, which nothing in the table drives", name)
		}
	}
}
