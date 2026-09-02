package credential

import (
	"github.com/shhac/agent-mongo/internal/config"
	credstore "github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/output"
)

// flowFromFlags builds the recipe the flags describe. Pure, and the single
// place the flow-type decision grows: --oidc alone means the environment flow
// today, and a later flow adds its arm here rather than inside addOIDC.
func flowFromFlags(flags addFlags) *config.Flow {
	return &config.Flow{
		Type:          config.FlowEnvironment,
		Environment:   flags.environment,
		TokenResource: flags.tokenResource,
		ClientID:      flags.clientID,
		AllowedHosts:  flags.allowedHosts,
	}
}

// addOIDC stores a flow recipe. Nothing secret is written: the environment
// flows read an identity the platform already gave this process, so there is no
// password to keep out of the agent's context and no --form equivalent.
//
// The recipe is validated by Store before anything is written, so there is no
// pre-check here to keep in step with it.
func addOIDC(name string, flags addFlags) error {
	flow := flowFromFlags(flags)
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
