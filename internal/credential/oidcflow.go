package credential

import (
	"context"
	"path/filepath"
	"sort"

	"github.com/shhac/agent-mongo/internal/config"
)

// flowHandler is what an OIDC flow supplies. One entry in the flows table is
// the single place a flow is registered — the same reason the kinds table
// exists, applied one level down, after the flow's policy and its validation
// briefly lived in two separate maps.
type flowHandler struct {
	// validate rejects a recipe this build cannot drive.
	validate func(alias string, flow *config.Flow) error
	// mayWidenHosts says whether an operator may replace this flow's
	// allowed-hosts list. See flowsThatMayWidenHosts' reasoning in oidchosts.go.
	mayWidenHosts bool
	// token fetches the access token when agent-mongo is the one holding it.
	// Nil when the driver obtains the token itself, as it does for the
	// platform-identity flows.
	token func(ctx context.Context, alias string, flow config.Flow) (string, error)
}

var flows = map[config.FlowType]flowHandler{
	config.FlowEnvironment: {
		validate:      validateEnvironmentFlow,
		mayWidenHosts: true,
	},
	config.FlowFile: {
		validate:      validateFileFlow,
		mayWidenHosts: true,
		token:         readFileFlowToken,
	},
}

func flowHandlerFor(flowType config.FlowType) (flowHandler, bool) {
	h, ok := flows[flowType]
	return h, ok
}

// SupportedFlowTypes lists the flows this build implements.
func SupportedFlowTypes() []string {
	names := make([]string, 0, len(flows))
	for flowType := range flows {
		names = append(names, string(flowType))
	}
	sort.Strings(names)
	return names
}

// ValidateFlow checks a flow recipe is one this build can actually drive,
// naming the valid values so a caller can correct itself.
func ValidateFlow(alias string, flow *config.Flow) error {
	if flow == nil {
		return MissingFlowError(alias)
	}
	h, ok := flowHandlerFor(flow.Type)
	if !ok {
		return UnsupportedFlowError(alias, flow.Type)
	}
	return h.validate(alias, flow)
}

// FlowMayWidenHosts reports whether an operator may replace this flow's
// allowed-hosts list, so the CLI can refuse the flag rather than storing a
// value the policy will ignore.
func FlowMayWidenHosts(flowType config.FlowType) bool {
	h, ok := flowHandlerFor(flowType)
	return ok && h.mayWidenHosts
}

// oidcEnvironment is one platform identity provider the driver implements.
// Whether it needs an audience lives beside its name so adding one cannot
// silently skip the check.
type oidcEnvironment struct {
	name          string
	needsResource bool
}

var oidcEnvironments = []oidcEnvironment{
	{name: config.EnvironmentK8s},
	{name: config.EnvironmentAzure, needsResource: true},
	{name: config.EnvironmentGCP, needsResource: true},
}

func environmentNames() []string {
	names := make([]string, 0, len(oidcEnvironments))
	for _, env := range oidcEnvironments {
		names = append(names, env.name)
	}
	return names
}

func validateEnvironmentFlow(alias string, flow *config.Flow) error {
	for _, env := range oidcEnvironments {
		if env.name != flow.Environment {
			continue
		}
		if env.needsResource && flow.TokenResource == "" {
			return MissingTokenResourceError(alias, env.name)
		}
		return nil
	}
	return UnknownEnvironmentError(alias, flow.Environment)
}

// validateFileFlow checks the token path is one this process can act on. The
// file itself is not read here: it is read at authentication time so a rotated
// token is picked up, and a missing file then is an auth failure with its own
// error rather than a reason to refuse to store the credential.
func validateFileFlow(alias string, flow *config.Flow) error {
	if flow.Path == "" {
		return MissingTokenPathError(alias)
	}
	if !filepath.IsAbs(flow.Path) {
		return RelativeTokenPathError(alias, flow.Path)
	}
	return nil
}

func readFileFlowToken(_ context.Context, _ string, flow config.Flow) (string, error) {
	return ReadTokenFile(flow.Path)
}
