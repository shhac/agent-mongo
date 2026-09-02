package credential

import (
	"path/filepath"
	"sort"

	"github.com/shhac/agent-mongo/internal/config"
)

// flowValidators registers the flows this build can drive. It is the flow-level
// twin of the kinds table, and exists for the same reason: SupportedFlowTypes
// is derived from its keys, so the list an error tells the reader to choose
// from cannot name a flow nothing implements.
var flowValidators = map[config.FlowType]func(alias string, flow *config.Flow) error{
	config.FlowEnvironment: validateEnvironmentFlow,
	config.FlowFile:        validateFileFlow,
}

// SupportedFlowTypes lists the flows this build implements.
func SupportedFlowTypes() []string {
	names := make([]string, 0, len(flowValidators))
	for flowType := range flowValidators {
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
	validate, ok := flowValidators[flow.Type]
	if !ok {
		return UnsupportedFlowError(alias, flow.Type)
	}
	return validate(alias, flow)
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
