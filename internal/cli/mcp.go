package cli

import (
	agentmcp "github.com/shhac/lib-agent-mcp"
	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/credential"
)

// registerMCP adds `agent-mongo mcp` — an MCP stdio server reflected from the
// cobra tree. Must be called LAST so the generated tool surface sees the
// complete command set. Data-facing groups are exposed as read-only tools;
// credential management stays CLI-only.
func registerMCP(root *cobra.Command) {
	exposed := map[string]bool{"database": true, "collection": true, "query": true, "connection": true}
	skipped := map[string]bool{"credential": true, "config": true}
	for _, cmd := range root.Commands() {
		name := cmd.Name()
		if exposed[name] {
			agentmcp.Expose(cmd)
			agentmcp.ReadOnly(cmd)
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
