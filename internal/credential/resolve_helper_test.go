package credential

import "github.com/shhac/agent-mongo/internal/config"

// resolves is the (credential, ok) shape these tests assert against, over the
// Resolution API. Tests that care about *which* failure occurred call Resolve
// directly and inspect the State.
func resolves(alias string) (config.Credential, bool) {
	res, err := Resolve(alias)
	return res.Credential, err == nil
}
