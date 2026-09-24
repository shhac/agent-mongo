package cli

import (
	"slices"

	agentmcp "github.com/shhac/lib-agent-mcp"
	"github.com/shhac/lib-agent-mcp/oauth"
	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
)

// registerMCP adds `agent-mongo mcp` — an MCP stdio server reflected from the
// cobra tree. Must be called LAST so the generated tool surface sees the
// complete command set. Data-facing groups are exposed as read-only tools;
// credential management stays CLI-only.
//
// Of `connection`, only the leaves that read are exposed. The ones that write
// config are not merely unannotated: pointing a connection at another host
// while keeping its stored credential (add --credential, update) would send
// that password to wherever the agent chose, in cleartext under PLAIN, so an
// agent must not be able to reach them at all.
func registerMCP(root *cobra.Command) {
	for _, cmd := range root.Commands() {
		switch cmd.Name() {
		case "database", "collection", "query":
			agentmcp.Expose(cmd)
			agentmcp.ReadOnly(cmd)
		case "connection":
			agentmcp.Expose(cmd)
			exposeReadOnlyLeaves(cmd, "list", "test", "usage")
		case "credential", "config":
			agentmcp.Skip(cmd)
		}
	}

	root.AddCommand(agentmcp.Command(root,
		agentmcp.WithHiddenFlags("color", "expand", "full"),
		agentmcp.WithOAuthKeyringService(credential.Service+".mcp"),
		agentmcp.WithIdentityBinding(mcpIdentityBinding),
	))
}

// bindingKeyConnection is the pairing-binding key naming the connection a
// principal acts through: mcp pair add <name> --bind connection=<alias>.
const bindingKeyConnection = "connection"

// mcpIdentityBinding pins a named principal's calls to the connection its
// pairing was bound to. It is carried in the environment, which the caller
// cannot touch, and a principal paired without a binding is pinned to nothing
// and refused. The MCP server only asks this of named principals; stdio and
// the shared pairing code run as the operator.
func mcpIdentityBinding(p oauth.Verified) (argv, env []string) {
	return nil, []string{
		config.IdentityEnv + "=1",
		config.BoundConnectionEnv + "=" + p.Binding[bindingKeyConnection],
	}
}

// exposeReadOnlyLeaves keeps the named subcommands of a group, marked
// read-only per leaf, and hides the rest. Per leaf rather than on the group, so
// a subcommand added later without a decision disqualifies the group's hint
// instead of inheriting it.
func exposeReadOnlyLeaves(group *cobra.Command, names ...string) {
	for _, cmd := range group.Commands() {
		if slices.Contains(names, cmd.Name()) {
			agentmcp.ReadOnly(cmd)
			continue
		}
		agentmcp.Skip(cmd)
	}
}
