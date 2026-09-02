package credential

import (
	"github.com/shhac/agent-mongo/internal/config"
	credstore "github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/output"
)

// addOIDC stores a flow recipe. Nothing secret is written: the environment
// flows read an identity the platform already gave this process, so there is no
// password to keep out of the agent's context and no --form equivalent.
func addOIDC(name string, flags addFlags) error {
	flow := &config.Flow{
		Type:          config.FlowEnvironment,
		Environment:   flags.environment,
		TokenResource: flags.tokenResource,
		ClientID:      flags.clientID,
		AllowedHosts:  flags.allowedHosts,
	}
	// Validated before the write so a bad recipe is rejected while the user is
	// still looking at it, rather than at the next query.
	if err := credstore.ValidateFlow(name, flow); err != nil {
		return err
	}

	storage, err := credstore.Store(name, config.Credential{
		Kind: config.KindOIDC,
		Flow: flow,
	})
	if err != nil {
		return err
	}

	result := map[string]any{
		"ok":         true,
		"credential": name,
		"kind":       string(config.KindOIDC),
		"flow":       string(flow.Type),
		"storage":    storage,
		"hint": "Use with: agent-mongo connection add <alias> <uri> --credential " + name +
			" (the URI must use TLS)",
	}
	if flow.Environment != "" {
		result["environment"] = flow.Environment
	}
	return output.PrintRaw(result)
}
