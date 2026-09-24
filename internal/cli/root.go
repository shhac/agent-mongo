// Package cli assembles the agent-mongo root command on lib-agent-cli's
// shared scaffolding: family persistent flags (--format, --timeout, --debug,
// --color) plus the domain flags -c/--connection, --expand, --full, then each
// command group, usage and the MCP server.
package cli

import (
	libcli "github.com/shhac/lib-agent-cli/cli"
	_ "github.com/shhac/lib-agent-cli/yaml" // registers the --format yaml encoder
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"

	clicollection "github.com/shhac/agent-mongo/internal/cli/collection"
	"github.com/shhac/agent-mongo/internal/cli/configcmd"
	cliconnection "github.com/shhac/agent-mongo/internal/cli/connection"
	clicredential "github.com/shhac/agent-mongo/internal/cli/credential"
	clidatabase "github.com/shhac/agent-mongo/internal/cli/database"
	cliquery "github.com/shhac/agent-mongo/internal/cli/query"
	"github.com/shhac/agent-mongo/internal/cli/shared"
	"github.com/shhac/agent-mongo/internal/config"
	appoutput "github.com/shhac/agent-mongo/internal/output"
	"github.com/shhac/agent-mongo/internal/truncation"
)

type rootFlags struct {
	libcli.Globals
	Connection string
	Expand     string
	Full       bool

	version string
	command string // path of the command being run, recorded in the pre-run
}

// shared snapshots the live flag values for leaf commands.
func (g *rootFlags) shared() *shared.GlobalFlags {
	return &shared.GlobalFlags{
		Connection: g.Connection,
		TimeoutMS:  g.TimeoutMS,
		Command:    g.command,
		Version:    g.version,
	}
}

func newRootCmd(version string) *cobra.Command {
	g := &rootFlags{version: version}
	root := libcli.NewRoot(libcli.Options{
		Use:           "agent-mongo",
		Short:         "Read-only MongoDB CLI for AI agents",
		Version:       version,
		Globals:       &g.Globals,
		DefaultFormat: output.FormatNDJSON,
		UnknownHint:   "run 'agent-mongo usage' to see the available commands",
		ConfigDefaults: func(cmd *cobra.Command) {
			g.command = cmd.CommandPath()
			applyConfigDefaults(g)
		},
	})

	pf := root.PersistentFlags()
	pf.StringVarP(&g.Connection, "connection", "c", "", "Connection alias to use")
	pf.StringVarP(&g.Expand, "expand", "e", "",
		"Expand truncated fields (comma-separated field names)")
	pf.BoolVarP(&g.Full, "full", "F", false, "Show full content for all truncated fields")

	cliconnection.Register(root, g.shared)
	clicredential.Register(root, g.shared)
	configcmd.Register(root)
	clidatabase.Register(root, g.shared)
	clicollection.Register(root, g.shared)
	cliquery.Register(root, g.shared)
	registerUsage(root)
	registerMCP(root)

	return root
}

// applyConfigDefaults resolves persisted settings into the process-wide
// singletons before any command runs.
func applyConfigDefaults(g *rootFlags) {
	truncation.Configure(truncation.Options{
		Expand:    g.Expand,
		Full:      g.Full,
		MaxLength: config.TruncationMaxLength.Value(),
	})

	appoutput.ConfigureFormat(g.Format)
}

// Run executes the CLI; errors render as {error, fixable_by, hint} on stderr
// with exit code 1.
func Run(version string) { libcli.Run(newRootCmd(version)) }
