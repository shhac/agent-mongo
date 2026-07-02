package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCorruptFileBehavesLikeEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	configDir := filepath.Join(dir, "agent-mongo")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := Read()
	if cfg.DefaultConnection != "" || cfg.Connections != nil || cfg.Settings != nil {
		t.Fatalf("corrupt config should read as zero Config, got %+v", cfg)
	}

	// And a write over the corrupt file recovers cleanly.
	if err := Write(Config{DefaultConnection: "x", Connections: map[string]Connection{
		"x": {ConnectionString: "mongodb://localhost"},
	}}); err != nil {
		t.Fatalf("Write over corrupt file: %v", err)
	}
	if got := Read().DefaultConnection; got != "x" {
		t.Fatalf("recovery read: got %q", got)
	}
}
