// Package testutil holds test helpers shared across packages' tests.
package testutil

import (
	"bytes"
	"io"
	"os"
	"testing"
)

// IsolateConfig points config I/O at a throwaway directory and forces
// credential storage onto the config.json fallback so tests never touch a
// real keychain.
func IsolateConfig(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENT_MONGO_NO_KEYCHAIN", "1")
}

// CaptureStdout redirects os.Stdout to a pipe and returns a buffer receiving
// everything written to stdout. The returned restore func puts stdout back.
func CaptureStdout(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	prev := os.Stdout
	os.Stdout = w

	buf := &bytes.Buffer{}
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(buf, r)
		close(done)
	}()

	return buf, func() {
		_ = w.Close()
		<-done
		os.Stdout = prev
		_ = r.Close()
	}
}
