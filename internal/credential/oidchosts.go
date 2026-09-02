package credential

import (
	"regexp"
	"strings"

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

// Whether a flow may replace its allowed-hosts list is recorded on the flow
// itself (flowHandler.mayWidenHosts), so registering a flow is one entry.
//
// The environment and file flows may: their token is already reachable by
// anything that can run agent-mongo — a projected Kubernetes token is a file on
// disk, an Azure or GCP identity is one metadata request away, and the file
// flow's token is a path the caller named. The allowlist is defence in depth
// there, and a self-hosted deployment is a real case that needs it. A flow
// holding a keychain-backed session is the opposite: that material is exactly
// what an agent cannot otherwise obtain, so its binding is not overridable.

// allowedHostsFor is the allowlist policy, kept separate from the check that
// applies it so the decision is visible and testable without a config or a URI.
func allowedHostsFor(flow *config.Flow) []string {
	if flow == nil || len(flow.AllowedHosts) == 0 {
		return DefaultAllowedHosts
	}
	if !FlowMayWidenHosts(flow.Type) {
		return DefaultAllowedHosts
	}
	return flow.AllowedHosts
}

// checkOIDCConnection refuses to let a token be sent somewhere it should not
// go: over a plaintext connection, or to a host outside the allowlist.
func checkOIDCConnection(entry config.Credential, uri string) error {
	if !mongouri.IsTLS(uri) {
		return InsecureConnectionError()
	}
	allowed := allowedHostsFor(entry.Flow)
	host := mongouri.ParseHostFromURI(uri)
	if hostAllowed(host, allowed) {
		return nil
	}
	return HostNotAllowedError(host, allowed)
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
// because DNS is — the driver's own patterns are not, but it applies its list
// only to the human flow, and here it is the whole contract.
func compileHostPattern(pattern string) (*regexp.Regexp, error) {
	escaped := strings.ReplaceAll(pattern, ".", "[.]")
	escaped = strings.ReplaceAll(escaped, "*", ".*")
	return regexp.Compile("(?i)^" + escaped + "$")
}
