package credential

import (
	"github.com/shhac/agent-mongo/internal/config"
	credstore "github.com/shhac/agent-mongo/internal/credential"
)

// resolves is the (credential, ok) shape these tests assert against.
func resolves(alias string) (config.Credential, bool) {
	res, err := credstore.Resolve(alias)
	return res.Credential, err == nil
}
