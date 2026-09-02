package credential

import (
	out "github.com/shhac/lib-agent-output"

	"github.com/shhac/agent-mongo/internal/config"
	credstore "github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/output"
)

// flowFromFlags builds the recipe the flags describe, and is the single place
// the flow-type decision lives.
//
// The type is chosen by which selector was given rather than by a --flow name,
// because each flow's selector is also the option it cannot work without.
func flowFromFlags(flags addFlags) (*config.Flow, error) {
	flow := &config.Flow{AllowedHosts: flags.allowedHosts}

	switch {
	case flags.tokenFile != "" && flags.environment != "":
		return nil, out.New(
			"--token-file and --environment select different OIDC flows",
			out.FixableByAgent,
		).WithHint("Pass one: --environment for a platform identity, --token-file for a token another tool wrote.")
	case flags.tokenFile != "":
		flow.Type = config.FlowFile
		flow.Path = flags.tokenFile
	case flags.environment != "":
		flow.Type = config.FlowEnvironment
		flow.Environment = flags.environment
		flow.TokenResource = flags.tokenResource
		flow.ClientID = flags.clientID
	default:
		return nil, out.New(
			"--oidc needs a flow: nothing said how this credential obtains a token",
			out.FixableByAgent,
		).WithHint("Add --environment k8s|azure|gcp for a platform identity, or --token-file <path> for a token another tool wrote.")
	}
	return flow, nil
}

// addOIDC stores a flow recipe. Nothing secret is written: the environment
// flows read an identity the platform already gave this process, so there is no
// password to keep out of the agent's context and no --form equivalent.
//
// The recipe is validated by Store before anything is written, so there is no
// pre-check here to keep in step with it.
func addOIDC(name string, flags addFlags) error {
	flow, err := flowFromFlags(flags)
	if err != nil {
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
	if flow.Path != "" {
		result["path"] = flow.Path
	}
	return output.PrintRaw(result)
}
