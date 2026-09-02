package credential

import "github.com/shhac/agent-mongo/internal/config"

func usernameAccount(alias string) string { return "username:" + alias }
func passwordAccount(alias string) string { return "password:" + alias }

// scramFields is the whole of SCRAM's storage behaviour: two keychain-backed
// fields. Resolution, storage, migration, removal and the storage-type report
// are all derived from this list.
var scramFields = []secretField{
	{
		account: usernameAccount,
		value:   func(c *config.Credential) *string { return &c.Username },
		missing: UnresolvableError,
	},
	{
		account: passwordAccount,
		value:   func(c *config.Credential) *string { return &c.Password },
		missing: UnresolvableError,
	},
}

// scramAccounts is the account list, kept as a named helper because the tests
// assert against exactly the set Remove is expected to erase.
func scramAccounts(alias string) []string {
	return keychainAccountsFor(scramFields, alias)
}
