package credential

import (
	"strings"

	"github.com/spf13/cobra"

	out "github.com/shhac/lib-agent-output"

	"github.com/shhac/agent-mongo/internal/config"
	credstore "github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/output"
)

// flowSelector is one flow's entry point on the command line: the flag that
// chooses it and how to fill in the recipe it implies.
//
// A list rather than a switch with pairwise conflict checks, because that shape
// was already got wrong twice one level up: a third selector turns one conflict
// arm into a cross product, and a flow whose selector carries no value (the
// device flow is a bare boolean) breaks a "which string is non-empty" test.
type flowSelector struct {
	flag     string
	flowType config.FlowType
	build    func(flags addFlags, flow *config.Flow)
}

var flowSelectors = []flowSelector{
	{
		flag:     "environment",
		flowType: config.FlowEnvironment,
		build: func(flags addFlags, flow *config.Flow) {
			flow.Environment = flags.environment
			flow.TokenResource = flags.tokenResource
			flow.ClientID = flags.clientID
		},
	},
	{
		flag:     "token-file",
		flowType: config.FlowFile,
		build:    func(flags addFlags, flow *config.Flow) { flow.Path = flags.tokenFile },
	},
}

func selectorFlagNames() []string {
	names := make([]string, 0, len(flowSelectors))
	for _, selector := range flowSelectors {
		names = append(names, "--"+selector.flag)
	}
	return names
}

// flowFromFlags builds the recipe the flags describe, and is the single place
// the flow-type decision lives.
//
// Selection is by which flag was given rather than by which value is non-empty,
// so `--token-file ""` is an empty path to complain about rather than a missing
// flow.
func flowFromFlags(cmd *cobra.Command, flags addFlags) (*config.Flow, error) {
	var chosen []flowSelector
	for _, selector := range flowSelectors {
		if cmd.Flags().Changed(selector.flag) {
			chosen = append(chosen, selector)
		}
	}

	if len(chosen) == 0 {
		return nil, out.New(
			"--oidc needs a flow: nothing said how this credential obtains a token",
			out.FixableByAgent,
		).WithHint("Pass one of: " + strings.Join(selectorFlagNames(), ", ") + ".")
	}
	if len(chosen) > 1 {
		given := make([]string, 0, len(chosen))
		for _, selector := range chosen {
			given = append(given, "--"+selector.flag)
		}
		return nil, out.New(
			strings.Join(given, " and ")+" select different OIDC flows",
			out.FixableByAgent,
		).WithHint("Pass exactly one of: " + strings.Join(selectorFlagNames(), ", ") + ".")
	}

	selector := chosen[0]
	flow := &config.Flow{Type: selector.flowType}
	selector.build(flags, flow)

	if len(flags.allowedHosts) > 0 {
		// Refused rather than stored and ignored: a flow whose token lives in
		// the keychain binds to the host it was obtained for, and a config
		// value that silently means nothing is worse than a flag that fails.
		if !credstore.FlowMayWidenHosts(flow.Type) {
			return nil, out.New(
				"--allowed-hosts does not apply to the "+string(flow.Type)+" flow: its host binding is not overridable",
				out.FixableByAgent,
			).WithHint("Drop --allowed-hosts.")
		}
		flow.AllowedHosts = flags.allowedHosts
	}
	return flow, nil
}

// addOIDC stores a flow recipe. Nothing secret is written for the flows
// implemented so far: they read an identity the platform or another tool
// already produced, so there is no password to keep out of the agent's context
// and no --form equivalent.
//
// The recipe is validated by Store before anything is written, so there is no
// pre-check here to keep in step with it.
func addOIDC(cmd *cobra.Command, name string, flags addFlags) error {
	flow, err := flowFromFlags(cmd, flags)
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
