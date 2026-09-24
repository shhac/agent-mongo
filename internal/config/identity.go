package config

import (
	"os"
	"strings"

	out "github.com/shhac/lib-agent-output"
)

// IdentityEnv names the variable that, when set, pins a process to the
// connection named by BoundConnectionEnv. The MCP server sets both for every
// call made by a named principal (mcp pair add <name> --bind
// connection=<alias>), so the principal can reach that connection and no other.
//
// The binding travels in the environment rather than as a -c the server
// appends: the caller controls the arguments (a trailing "--" turns an
// appended flag into a positional), and does not control the environment.
const IdentityEnv = "AGENT_MONGO_REQUIRE_IDENTITY"

// BoundConnectionEnv names the connection a pinned process is bound to.
const BoundConnectionEnv = "AGENT_MONGO_BOUND_CONNECTION"

func IdentityRequired() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(IdentityEnv))) {
	case "", "0", "false", "no":
		return false
	}
	return true
}

// PinnedConnection is the connection a pinned process may use. requested is
// whatever the caller asked for (-c, or a positional alias), and must be empty
// or the bound connection itself. ok is false when the process is not pinned.
//
// A pinned process with no binding fails closed: that is a principal paired
// without one, and falling back to the operator's default — or to whatever the
// caller names — would hand it a connection it was never given.
func PinnedConnection(requested string) (alias string, ok bool, err error) {
	if !IdentityRequired() {
		return "", false, nil
	}
	bound := strings.TrimSpace(os.Getenv(BoundConnectionEnv))
	if bound == "" {
		return "", true, out.New("This MCP principal is not bound to a connection.", out.FixableByHuman).
			WithHint("Pair it with one: agent-mongo mcp pair add <name> --bind connection=<alias>")
	}
	if requested = strings.TrimSpace(requested); requested != "" && requested != bound {
		return "", true, out.New(
			"This session is bound to connection \""+bound+"\"; \""+requested+"\" is not available to it.",
			out.FixableByAgent).WithHint("Omit -c, or pass -c " + bound)
	}
	return bound, true, nil
}
