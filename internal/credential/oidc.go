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
