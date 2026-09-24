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
// `connection` is exposed but not read-only: a group tool can reach every
// subcommand, and add/update/set-default/remove all write config. The hint is
// what a host may auto-approve on, so it has to describe the worst call.
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
			markDestructive(cmd, "remove")
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

func markDestructive(group *cobra.Command, names ...string) {
	for _, cmd := range group.Commands() {
		if slices.Contains(names, cmd.Name()) {
			agentmcp.Destructive(cmd)
		}
	}
}
