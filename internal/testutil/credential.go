package testutil

import (
	"testing"

	"github.com/shhac/agent-mongo/internal/config"
)

// StageCredential writes an entry into config.json verbatim, bypassing
// credential.Store.
//
// It is the only way to produce the shapes Store will not write but a real
// config can hold: a hand-edited entry, one written by a newer build naming a
// kind this one does not implement, or a keychain sentinel whose secret has
// since disappeared. Those are exactly the inputs the resolution paths have to
// survive, so the bypass is load-bearing rather than a shortcut.
func StageCredential(t *testing.T, alias string, cred config.Credential) {
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
