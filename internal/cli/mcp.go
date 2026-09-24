package cli

import (
	"slices"

	agentmcp "github.com/shhac/lib-agent-mcp"
	"github.com/spf13/cobra"

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
	readOnly := map[string]bool{"database": true, "collection": true, "query": true}
	skipped := map[string]bool{"credential": true, "config": true}
	for _, cmd := range root.Commands() {
		name := cmd.Name()
		if readOnly[name] {
			agentmcp.Expose(cmd)
			agentmcp.ReadOnly(cmd)
		}
		if name == "connection" {
			agentmcp.Expose(cmd)
			exposeReadOnlyLeaves(cmd, "list", "test", "usage")
		}
		if skipped[name] {
			agentmcp.Skip(cmd)
		}
	}

	root.AddCommand(agentmcp.Command(root,
		agentmcp.WithHiddenFlags("color", "expand", "full"),
		agentmcp.WithOAuthKeyringService(credential.Service+".mcp"),
	))
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
