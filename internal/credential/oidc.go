package credential

import "github.com/shhac/agent-mongo/internal/config"

// validateOIDC is the kinds-table hook: an OIDC credential is exactly its flow
// recipe, so validating the entry is validating the flow.
//
// It runs on every read as well as every write, so a hand-edited config fails
// at the point of use with a self-correcting error naming the valid values,
// rather than somewhere inside the driver.
func validateOIDC(alias string, entry config.Credential) error {
	return ValidateFlow(alias, entry.Flow)
}

// sessionField is the one field an OIDC credential may keep in the keychain.
// Only the device flow fills it; for the others it stays empty and the generic
// storage skips it.
//
// Named rather than declared inline because two paths read it: authentication,
// which fails when it is missing, and the listing, which reports "not logged
// in" instead. Both go through this declaration so they cannot drift.
var sessionField = secretField{
	account: sessionAccount,
	value:   func(c *config.Credential) *string { return &c.Session },
	missing: NotLoggedInError,
}

var oidcFields = []secretField{sessionField}
