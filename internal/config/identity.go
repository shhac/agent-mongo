package config

import (
	"os"
	"strings"

	out "github.com/shhac/lib-agent-output"
)

// IdentityEnv names the variable that, when set, pins a process to the
// connection named by -c. The MCP server sets it, with -c, for every call made
// by a named principal whose pairing binds a connection (mcp pair add <name>
// --bind connection=<alias>), so a call that arrives without one fails rather
// than falling back to the operator's default — the one that principal was not
// given.
const IdentityEnv = "AGENT_MONGO_REQUIRE_IDENTITY"

func IdentityRequired() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(IdentityEnv))) {
	case "", "0", "false", "no":
		return false
	}
	return true
}

// PinnedConnection is the connection a pinned process may use: the -c value,
// which must be present. ok is false when the process is not pinned.
func PinnedConnection(flag string) (alias string, ok bool, err error) {
	if !IdentityRequired() {
		return "", false, nil
	}
	alias = strings.TrimSpace(flag)
	if alias == "" {
		return "", true, out.New(IdentityEnv+" is set but no connection was given with -c.", out.FixableByHuman).
			WithHint("This MCP principal has no connection binding. Pair it with one: agent-mongo mcp pair add <name> --bind connection=<alias>")
	}
	return alias, true, nil
}

// CheckPinnedTo refuses a connection other than the pinned one.
func CheckPinnedTo(flag, requested string) error {
	pinned, ok, err := PinnedConnection(flag)
	if !ok || err != nil {
		return err
	}
	if requested != "" && requested != pinned {
		return out.New("This session is bound to connection \""+pinned+"\"; \""+requested+"\" is not available to it.",
			out.FixableByAgent)
	}
	return nil
}
