package credential

import (
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
	cred, ok := Get("acme")
	if !ok {
		t.Fatal("Get(acme) not found")
	}
	if cred.Username != "deploy" || cred.Password != "secret123" {
		t.Errorf("cred = %+v, want deploy/secret123", cred)
	}
}

func TestStorageTypeReturnsValidType(t *testing.T) {
	testutil.IsolateConfig(t)
	if _, err := Store("acme", config.Credential{Username: "deploy", Password: "secret123"}); err != nil {
		t.Fatalf("Store() error: %v", err)
	}
	if got := StorageType("acme"); got != StorageKeychain && got != StorageConfig {
		t.Errorf("StorageType(acme) = %q, want keychain or config", got)
	}
}

func TestStorageTypeReturnsConfigForUnknownAlias(t *testing.T) {
	testutil.IsolateConfig(t)
	if got := StorageType("nonexistent"); got != StorageConfig {
		t.Errorf("StorageType(nonexistent) = %q, want config", got)
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
	cred, ok := Get("acme")
	if !ok || cred.Password != "new-pass" {
		t.Errorf("cred = %+v,%v, want new-pass", cred, ok)
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
	if c, _ := Get("acme"); c.Username != "deploy" {
		t.Errorf("acme username = %q, want deploy", c.Username)
	}
	if c, _ := Get("globex"); c.Username != "admin" {
		t.Errorf("globex username = %q, want admin", c.Username)
	}
}

func TestGetReturnsFalseForUnknownName(t *testing.T) {
	testutil.IsolateConfig(t)
	if _, ok := Get("nonexistent"); ok {
		t.Error("Get(nonexistent) = ok, want not found")
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
	if _, ok := Get("acme"); ok {
		t.Error("Get(acme) = ok, want removed")
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
	if !strings.Contains(err.Error(), "Unknown credential") {
		t.Errorf("error = %q, want it to mention 'Unknown credential'", err)
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
	if _, ok := Get("acme"); ok {
		t.Error("Get(acme) = ok, want removed")
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
	cred, ok := Get("acme")
	if !ok || cred.Username != "deploy" || cred.Password != "secret" {
		t.Errorf("cred = %+v,%v, want deploy/secret", cred, ok)
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
