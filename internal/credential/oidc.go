package credential

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	out "github.com/shhac/lib-agent-output"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/mongouri"
)

// DefaultAllowedHosts is the set of hosts an OIDC credential may send a token
// to when the credential does not name its own.
//
// It is the driver's own list (x/mongo/driver/auth/oidc.go), reused here for a
// reason the driver leaves open: the driver applies that list only to the human
// flow, so a machine flow will hand a platform identity token to whatever host
// the connection string names. agent-mongo lets an agent run `connection add`,
// so without this an injected connection pointing at an attacker-controlled
// listener would be given a live Kubernetes or Azure token.
var DefaultAllowedHosts = []string{
	"*.mongodb.net",
	"*.mongodb-qa.net",
	"*.mongodb-dev.net",
	"*.mongodbgov.net",
	"*.mongo.com",
	"localhost",
	"127.0.0.1",
	"::1",
}

var oidcEnvironments = []string{
	config.EnvironmentK8s,
	config.EnvironmentAzure,
	config.EnvironmentGCP,
}

// environmentsNeedingResource are the providers the driver refuses to run
// without an explicit audience (mongo/options/clientoptions.go).
var environmentsNeedingResource = map[string]bool{
	config.EnvironmentAzure: true,
	config.EnvironmentGCP:   true,
}

// readOIDC resolves an OIDC credential. There is nothing to fetch: the flows
// implemented so far read an identity the platform hands the driver directly,
// so resolution is validation, and it happens on every read so a hand-edited
// config fails at the point of use rather than somewhere inside the driver.
func readOIDC(alias string, entry config.Credential) (config.Credential, error) {
	if err := ValidateFlow(alias, entry.Flow); err != nil {
		return config.Credential{}, err
	}
	return entry, nil
}

// storeOIDC writes the flow recipe. No keychain: the flows implemented so far
// hold no secret of their own.
func storeOIDC(alias string, cred config.Credential) (string, error) {
	if err := ValidateFlow(alias, cred.Flow); err != nil {
		return "", err
	}
	err := config.Update(func(cfg *config.Config) error {
		if cfg.Credentials == nil {
			cfg.Credentials = map[string]config.Credential{}
		}
		cfg.Credentials[alias] = cred
		return nil
	})
	return StorageConfig, err
}

// oidcAccounts names the keychain accounts an OIDC credential owns. None yet:
// the environment flow keeps no session. The device flow adds one here, and
// Remove picks it up without being told.
func oidcAccounts(string) []string { return nil }

func oidcStorageType(config.Credential) string { return StorageConfig }

// ValidateFlow checks a flow recipe is one this build can actually drive,
// naming the valid values so a caller can correct itself.
func ValidateFlow(alias string, flow *config.Flow) error {
	if flow == nil {
		return flowError(alias,
			"has no flow: an oidc credential needs to say how it obtains a session",
			"Re-add it with a flow: agent-mongo credential add "+alias+" --oidc --environment k8s")
	}

	switch flow.Type {
	case config.FlowEnvironment:
		return validateEnvironmentFlow(alias, flow)
	default:
		return flowError(alias,
			fmt.Sprintf("has unsupported flow type %q. Supported: %s",
				flow.Type, config.JoinOrNone(SupportedFlowTypes())),
			"Upgrade agent-mongo to a build that implements this flow, or re-add the credential with a supported one.")
	}
}

func validateEnvironmentFlow(alias string, flow *config.Flow) error {
	if !slices.Contains(oidcEnvironments, flow.Environment) {
		return flowError(alias,
			fmt.Sprintf("names unknown environment %q. Valid: %s",
				flow.Environment, config.JoinOrNone(oidcEnvironments)),
			"Re-add it with a valid environment: agent-mongo credential add "+alias+" --oidc --environment k8s")
	}
	if environmentsNeedingResource[flow.Environment] && flow.TokenResource == "" {
		return flowError(alias,
			fmt.Sprintf("needs a token resource: the %s environment mints tokens for a named audience",
				flow.Environment),
			fmt.Sprintf("Re-add it with the audience: agent-mongo credential add %s --oidc --environment %s --token-resource <audience>",
				alias, flow.Environment))
	}
	return nil
}

// SupportedFlowTypes lists the flows this build implements.
func SupportedFlowTypes() []string {
	return []string{string(config.FlowEnvironment)}
}

func flowError(alias, problem, hint string) error {
	return out.New(
		fmt.Sprintf("Credential %q %s", alias, problem),
		out.FixableByHuman,
	).WithCause(ErrInvalidFlow).WithHint(hint)
}

// checkOIDCConnection refuses to let a token be sent somewhere it should not
// go: over a plaintext connection, or to a host outside the credential's
// allowlist.
func checkOIDCConnection(entry config.Credential, uri string) error {
	if !mongouri.IsTLS(uri) {
		return out.New(
			"OIDC authentication requires TLS: a bearer token on a plaintext connection is a credential on the wire",
			out.FixableByHuman,
		).WithCause(ErrInsecureConnection).WithHint(
			"Use a mongodb+srv:// URI, or add tls=true to the connection string.")
	}

	allowed := DefaultAllowedHosts
	if entry.Flow != nil && len(entry.Flow.AllowedHosts) > 0 {
		allowed = entry.Flow.AllowedHosts
	}
	host := mongouri.ParseHostFromURI(uri)
	if hostAllowed(host, allowed) {
		return nil
	}
	return out.New(
		fmt.Sprintf("Host %q is not in this credential's allowed hosts: %s",
			host, config.JoinOrNone(allowed)),
		out.FixableByHuman,
	).WithCause(ErrHostNotAllowed).WithHint(
		"Point the connection at an allowed host, or widen the credential deliberately: agent-mongo credential add <alias> --oidc --allowed-hosts <pattern>,<pattern>")
}

// hostAllowed matches a host against glob patterns.
//
// An empty host denies: ParseHostFromURI returns "" for anything it cannot read
// as a connection string, and a URI whose host cannot be determined is exactly
// the one a token must not be sent to.
func hostAllowed(host string, patterns []string) bool {
	if host == "" {
		return false
	}
	// A fully-qualified name may carry a trailing dot; it names the same host.
	host = strings.TrimSuffix(host, ".")
	for _, pattern := range patterns {
		re, err := compileHostPattern(pattern)
		if err != nil {
			continue // a pattern that will not compile matches nothing
		}
		if re.MatchString(host) {
			return true
		}
	}
	return false
}

// compileHostPattern turns a glob into an anchored regexp, the same way the
// driver does for ALLOWED_HOSTS (x/mongo/driver/auth/oidc.go): "." is a literal
// dot and "*" matches any run of characters. Matching is case-insensitive
// because DNS is — the driver's own patterns are not, but its list is only ever
// applied to the human flow, and here it is the whole contract.
func compileHostPattern(pattern string) (*regexp.Regexp, error) {
	escaped := strings.ReplaceAll(pattern, ".", "[.]")
	escaped = strings.ReplaceAll(escaped, "*", ".*")
	return regexp.Compile("(?i)^" + escaped + "$")
}
