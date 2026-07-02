package mongo

import (
	"testing"
	"time"

	"github.com/shhac/agent-mongo/internal/config"
)

// TestClientOptionsPoolIsCLISized pins the pool shape for a short-lived CLI
// process: exactly one pooled connection, no warm minimum, short idle
// lifetime, bounded server selection.
func TestClientOptionsPoolIsCLISized(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	conn := config.Connection{ConnectionString: "mongodb://localhost:27017/db"}
	opts, err := clientOptions(conn, 30*time.Second)
	if err != nil {
		t.Fatalf("clientOptions: %v", err)
	}

	if opts.MaxPoolSize == nil || *opts.MaxPoolSize != 1 {
		t.Errorf("MaxPoolSize: got %v, want 1", opts.MaxPoolSize)
	}
	if opts.MinPoolSize == nil || *opts.MinPoolSize != 0 {
		t.Errorf("MinPoolSize: got %v, want 0", opts.MinPoolSize)
	}
	if opts.MaxConnIdleTime == nil || *opts.MaxConnIdleTime != 5*time.Second {
		t.Errorf("MaxConnIdleTime: got %v, want 5s", opts.MaxConnIdleTime)
	}
	if opts.ServerSelectionTimeout == nil || *opts.ServerSelectionTimeout != 10*time.Second {
		t.Errorf("ServerSelectionTimeout: got %v, want 10s", opts.ServerSelectionTimeout)
	}
	if opts.Timeout == nil || *opts.Timeout != 30*time.Second {
		t.Errorf("Timeout: got %v, want 30s", opts.Timeout)
	}
}

func TestClientOptionsZeroTimeoutLeavesDriverDefault(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	opts, err := clientOptions(config.Connection{ConnectionString: "mongodb://localhost:27017"}, 0)
	if err != nil {
		t.Fatalf("clientOptions: %v", err)
	}
	if opts.Timeout != nil {
		t.Errorf("Timeout: got %v, want unset", opts.Timeout)
	}
}

func TestClientOptionsUnknownCredential(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENT_MONGO_NO_KEYCHAIN", "1")

	_, err := clientOptions(config.Connection{
		ConnectionString: "mongodb://localhost:27017",
		Credential:       "ghost",
	}, 0)
	if err == nil {
		t.Fatal("expected error for unknown credential")
	}
}
