package credential

import (
	"errors"
	"strings"
	"testing"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/testutil"
)

func TestAllReturnsEmptyWhenNoneStored(t *testing.T) {
	testutil.IsolateConfig(t)
	if got := All(); len(got) != 0 {
		t.Errorf("All() = %v, want empty", got)
	}
}

func TestStoreCredentialStores(t *testing.T) {
	testutil.IsolateConfig(t)
	storage, err := Store("acme", config.Credential{Username: "deploy", Password: "secret123"})
	if err != nil {
		t.Fatalf("Store() error: %v", err)
	}
	if storage != StorageKeychain && storage != StorageConfig {
		t.Errorf("storage = %q, want keychain or config", storage)
	}
	res, err := Resolve("acme")
	if err != nil {
		t.Fatalf("Resolve(acme): %v", err)
	}
	if cred := res.Credential; cred.Username != "deploy" || cred.Password != "secret123" {
		t.Errorf("cred = %+v, want deploy/secret123", cred)
	}
}

func TestStorageTypeReturnsValidType(t *testing.T) {
	testutil.IsolateConfig(t)
	if _, err := Store("acme", config.Credential{Username: "deploy", Password: "secret123"}); err != nil {
		t.Fatalf("Store() error: %v", err)
	}
	if got := StorageType(All()["acme"]); got != StorageKeychain && got != StorageConfig {
		t.Errorf("StorageType(acme) = %q, want keychain or config", got)
	}
}

func TestStorageTypeReturnsConfigForUnknownAlias(t *testing.T) {
	testutil.IsolateConfig(t)
	if got := StorageType(All()["nonexistent"]); got != StorageConfig {
		t.Errorf("StorageType of an absent entry = %q, want config", got)
	}
}

func TestStoreCredentialUpsertsExisting(t *testing.T) {
	testutil.IsolateConfig(t)
	if _, err := Store("acme", config.Credential{Username: "deploy", Password: "old-pass"}); err != nil {
		t.Fatalf("Store(old) error: %v", err)
	}
	if _, err := Store("acme", config.Credential{Username: "deploy", Password: "new-pass"}); err != nil {
		t.Fatalf("Store(new) error: %v", err)
	}
	res, err := Resolve("acme")
	if err != nil || res.Credential.Password != "new-pass" {
		t.Errorf("cred = %+v, err = %v, want new-pass", res.Credential, err)
	}
}

func TestMultipleCredentialsStoredIndependently(t *testing.T) {
	testutil.IsolateConfig(t)
	if _, err := Store("acme", config.Credential{Username: "deploy", Password: "acme-pass"}); err != nil {
		t.Fatalf("Store(acme) error: %v", err)
	}
	if _, err := Store("globex", config.Credential{Username: "admin", Password: "globex-pass"}); err != nil {
		t.Fatalf("Store(globex) error: %v", err)
	}
	aliases := Aliases()
	if len(aliases) != 2 || aliases[0] != "acme" || aliases[1] != "globex" {
		t.Errorf("Aliases() = %v, want [acme globex]", aliases)
	}
	if res, _ := Resolve("acme"); res.Credential.Username != "deploy" {
		t.Errorf("acme username = %q, want deploy", res.Credential.Username)
	}
	if res, _ := Resolve("globex"); res.Credential.Username != "admin" {
		t.Errorf("globex username = %q, want admin", res.Credential.Username)
	}
}

func TestGetReturnsFalseForUnknownName(t *testing.T) {
	testutil.IsolateConfig(t)
	if _, err := Resolve("nonexistent"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Resolve(nonexistent) error = %v, want ErrNotFound", err)
	}
}

func TestRemoveCredentialRemoves(t *testing.T) {
	testutil.IsolateConfig(t)
	if _, err := Store("acme", config.Credential{Username: "deploy", Password: "secret"}); err != nil {
		t.Fatalf("Store() error: %v", err)
	}
	if err := Remove("acme"); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}
	if _, err := Resolve("acme"); err == nil {
		t.Error("Resolve(acme) succeeded, want removed")
	}
	if got := All(); len(got) != 0 {
		t.Errorf("All() = %v, want empty", got)
	}
}

func TestRemoveCredentialErrorsForUnknownName(t *testing.T) {
	testutil.IsolateConfig(t)
	err := Remove("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Remove speaks the same vocabulary as every other missing-credential
	// failure; an agent that learned to recover from one should not meet a
	// second wording for the identical fault.
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want it to wrap ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want the shared not-found message", err)
	}
}

func TestRemoveCredentialErrorsWhenInUse(t *testing.T) {
	testutil.IsolateConfig(t)
	if _, err := Store("acme", config.Credential{Username: "deploy", Password: "secret"}); err != nil {
		t.Fatalf("Store() error: %v", err)
	}
	err := config.StoreConnection("prod", config.Connection{
		ConnectionString: "mongodb://prod:27017",
		Credential:       "acme",
	})
	if err != nil {
		t.Fatalf("StoreConnection() error: %v", err)
	}
	rmErr := Remove("acme")
	if rmErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(rmErr.Error(), "used by connections: prod") {
		t.Errorf("error = %q, want it to mention 'used by connections: prod'", rmErr)
	}
}

func TestRemoveCredentialSucceedsAfterClearingReferences(t *testing.T) {
	testutil.IsolateConfig(t)
	if _, err := Store("acme", config.Credential{Username: "deploy", Password: "secret"}); err != nil {
		t.Fatalf("Store() error: %v", err)
	}
	err := config.StoreConnection("prod", config.Connection{
		ConnectionString: "mongodb://prod:27017",
		Credential:       "acme",
	})
	if err != nil {
		t.Fatalf("StoreConnection() error: %v", err)
	}
	empty := ""
	if err := config.UpdateConnection("prod", config.ConnectionUpdates{Credential: &empty}); err != nil {
		t.Fatalf("UpdateConnection() error: %v", err)
	}
	if err := Remove("acme"); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}
	if _, err := Resolve("acme"); err == nil {
		t.Error("Resolve(acme) succeeded, want removed")
	}
}

func TestStoreCredentialDoesNotTouchConnectionData(t *testing.T) {
	testutil.IsolateConfig(t)
	err := config.StoreConnection("local", config.Connection{ConnectionString: "mongodb://localhost"})
	if err != nil {
		t.Fatalf("StoreConnection() error: %v", err)
	}
	if _, err := Store("acme", config.Credential{Username: "deploy", Password: "secret"}); err != nil {
		t.Fatalf("Store() error: %v", err)
	}
	res, err := Resolve("acme")
	if err != nil || res.Credential.Username != "deploy" || res.Credential.Password != "secret" {
		t.Errorf("cred = %+v, err = %v, want deploy/secret", res.Credential, err)
	}
	conn, ok := config.GetConnection("local")
	if !ok || conn.ConnectionString != "mongodb://localhost" {
		t.Errorf("connection = %+v,%v, want intact", conn, ok)
	}
}

func TestConnectionsUsingReturnsEmptyWhenNoneReference(t *testing.T) {
	testutil.IsolateConfig(t)
	if _, err := Store("acme", config.Credential{Username: "deploy", Password: "secret"}); err != nil {
		t.Fatalf("Store() error: %v", err)
	}
	err := config.StoreConnection("local", config.Connection{ConnectionString: "mongodb://localhost"})
	if err != nil {
		t.Fatalf("StoreConnection() error: %v", err)
	}
	if got := ConnectionsUsing("acme"); len(got) != 0 {
		t.Errorf("ConnectionsUsing(acme) = %v, want empty", got)
	}
}

func TestConnectionsUsingReturnsReferencingConnections(t *testing.T) {
	testutil.IsolateConfig(t)
	if _, err := Store("acme", config.Credential{Username: "deploy", Password: "secret"}); err != nil {
		t.Fatalf("Store() error: %v", err)
	}
	for alias, conn := range map[string]config.Connection{
		"staging": {ConnectionString: "mongodb://staging:27017", Credential: "acme"},
		"prod":    {ConnectionString: "mongodb://prod:27017", Credential: "acme"},
		"local":   {ConnectionString: "mongodb://localhost"},
	} {
		if err := config.StoreConnection(alias, conn); err != nil {
			t.Fatalf("StoreConnection(%s) error: %v", alias, err)
		}
	}
	got := ConnectionsUsing("acme")
	if len(got) != 2 || got[0] != "prod" || got[1] != "staging" {
		t.Errorf("ConnectionsUsing(acme) = %v, want [prod staging]", got)
	}
}
