package config

import (
	"strings"
	"testing"
)

// isolate points XDG_CONFIG_HOME at a fresh temp dir so each test reads and
// writes its own config.json. Read() reads the file fresh every call, so no
// cache clearing is needed.
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

func TestReadReturnsEmptyWhenNoConfig(t *testing.T) {
	isolate(t)
	cfg := Read()
	if len(cfg.Connections) != 0 {
		t.Errorf("expected 0 connections, got %d", len(cfg.Connections))
	}
	if cfg.DefaultConnection != "" {
		t.Errorf("default = %q, want empty", cfg.DefaultConnection)
	}
	if cfg.Settings != nil {
		t.Errorf("settings = %+v, want nil", cfg.Settings)
	}
}

func TestWriteCreatesConfigFile(t *testing.T) {
	isolate(t)
	if err := Write(Config{DefaultConnection: "test"}); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if got := Read().DefaultConnection; got != "test" {
		t.Errorf("default = %q, want %q", got, "test")
	}
}

func TestReadRoundTripsWhatWasWritten(t *testing.T) {
	isolate(t)
	err := Write(Config{
		DefaultConnection: "prod",
		Connections: map[string]Connection{
			"prod": {ConnectionString: "mongodb://localhost"},
		},
	})
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	cfg := Read()
	if cfg.DefaultConnection != "prod" {
		t.Errorf("default = %q, want %q", cfg.DefaultConnection, "prod")
	}
	if got := cfg.Connections["prod"].ConnectionString; got != "mongodb://localhost" {
		t.Errorf("connection_string = %q, want %q", got, "mongodb://localhost")
	}
}

func TestConnectionsReturnsEmptyWhenNoneStored(t *testing.T) {
	isolate(t)
	if got := Connections(); len(got) != 0 {
		t.Errorf("Connections() = %v, want empty", got)
	}
}

func TestStoreConnectionStoresAndSetsDefault(t *testing.T) {
	isolate(t)
	if err := StoreConnection("local", Connection{ConnectionString: "mongodb://localhost:27017"}); err != nil {
		t.Fatalf("StoreConnection() error: %v", err)
	}
	conn, ok := GetConnection("local")
	if !ok {
		t.Fatal("GetConnection(local) not found")
	}
	if conn.ConnectionString != "mongodb://localhost:27017" {
		t.Errorf("connection_string = %q", conn.ConnectionString)
	}
	if got := DefaultConnectionAlias(); got != "local" {
		t.Errorf("default = %q, want %q", got, "local")
	}
}

func TestStoreConnectionDoesNotOverwriteExistingDefault(t *testing.T) {
	isolate(t)
	if err := StoreConnection("local", Connection{ConnectionString: "mongodb://localhost:27017"}); err != nil {
		t.Fatalf("StoreConnection(local) error: %v", err)
	}
	err := StoreConnection("prod", Connection{ConnectionString: "mongodb://prod:27017", Name: "Production"})
	if err != nil {
		t.Fatalf("StoreConnection(prod) error: %v", err)
	}
	if got := DefaultConnectionAlias(); got != "local" {
		t.Errorf("default = %q, want %q", got, "local")
	}
	if got := ConnectionAliases(); len(got) != 2 || got[0] != "local" || got[1] != "prod" {
		t.Errorf("aliases = %v, want [local prod]", got)
	}
}

func TestStoreConnectionWithNameAndDatabase(t *testing.T) {
	isolate(t)
	err := StoreConnection("dev", Connection{
		ConnectionString: "mongodb://dev:27017",
		Name:             "Development",
		Database:         "mydb",
	})
	if err != nil {
		t.Fatalf("StoreConnection() error: %v", err)
	}
	conn, ok := GetConnection("dev")
	if !ok {
		t.Fatal("GetConnection(dev) not found")
	}
	if conn.Name != "Development" {
		t.Errorf("name = %q, want %q", conn.Name, "Development")
	}
	if conn.Database != "mydb" {
		t.Errorf("database = %q, want %q", conn.Database, "mydb")
	}
}

func TestGetConnectionReturnsFalseForUnknownAlias(t *testing.T) {
	isolate(t)
	if _, ok := GetConnection("nonexistent"); ok {
		t.Error("GetConnection(nonexistent) = ok, want not found")
	}
}

func TestSetDefaultConnectionSwitchesActive(t *testing.T) {
	isolate(t)
	mustStore(t, "a", "mongodb://a")
	mustStore(t, "b", "mongodb://b")
	if got := DefaultConnectionAlias(); got != "a" {
		t.Fatalf("default = %q, want %q", got, "a")
	}
	if err := SetDefaultConnection("b"); err != nil {
		t.Fatalf("SetDefaultConnection() error: %v", err)
	}
	if got := DefaultConnectionAlias(); got != "b" {
		t.Errorf("default = %q, want %q", got, "b")
	}
}

func TestSetDefaultConnectionErrorsForUnknownAlias(t *testing.T) {
	isolate(t)
	err := SetDefaultConnection("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unknown connection") {
		t.Errorf("error = %q, want it to mention 'Unknown connection'", err)
	}
}

func TestRemoveConnectionRemovesAndReassignsDefault(t *testing.T) {
	isolate(t)
	mustStore(t, "first", "mongodb://first")
	mustStore(t, "second", "mongodb://second")
	if err := RemoveConnection("first"); err != nil {
		t.Fatalf("RemoveConnection() error: %v", err)
	}
	if got := DefaultConnectionAlias(); got != "second" {
		t.Errorf("default = %q, want %q", got, "second")
	}
	if _, ok := GetConnection("first"); ok {
		t.Error("GetConnection(first) = ok, want removed")
	}
}

func TestRemoveConnectionClearsDefaultWhenLastRemoved(t *testing.T) {
	isolate(t)
	mustStore(t, "only", "mongodb://only")
	if err := RemoveConnection("only"); err != nil {
		t.Fatalf("RemoveConnection() error: %v", err)
	}
	if got := DefaultConnectionAlias(); got != "" {
		t.Errorf("default = %q, want empty", got)
	}
	if got := Connections(); len(got) != 0 {
		t.Errorf("Connections() = %v, want empty", got)
	}
}

func TestRemoveConnectionErrorsForUnknownAlias(t *testing.T) {
	isolate(t)
	err := RemoveConnection("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unknown connection") {
		t.Errorf("error = %q, want it to mention 'Unknown connection'", err)
	}
}

func mustStore(t *testing.T, alias, uri string) {
	t.Helper()
	if err := StoreConnection(alias, Connection{ConnectionString: uri}); err != nil {
		t.Fatalf("StoreConnection(%s) error: %v", alias, err)
	}
}
