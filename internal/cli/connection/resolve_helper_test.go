package connection

import (
	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
)

// resolves is the (credential, ok) shape these tests assert against.
func resolves(alias string) (config.Credential, bool) {
	res, err := credential.Resolve(alias)
	return res.Credential, err == nil
}
