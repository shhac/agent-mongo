// Package database implements `agent-mongo database` — database discovery.
package database

import (
	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/cli/shared"
	"github.com/shhac/agent-mongo/internal/output"
)

func Register(root *cobra.Command, globals func() *shared.GlobalFlags) {
	cmd := &cobra.Command{Use: "database", Short: "Database discovery"}

	cmd.AddCommand(&cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all databases",
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			g := globals()
			return shared.WithSession(g, func(ctx shared.SessionCtx) error {
				result, err := ctx.Session.ListDatabases(ctx.Ctx)
				if err != nil {
					return err
				}
				return output.PrintList(result.Databases, output.Meta(map[string]any{
					"totalSize": result.TotalSize,
				}))
			})
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "stats <database>",
		Short: "Get database statistics",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			g := globals()
			return shared.WithSession(g, func(ctx shared.SessionCtx) error {
				result, err := ctx.Session.DatabaseStats(ctx.Ctx, args[0])
				if err != nil {
					return err
				}
				return output.PrintResult(result)
			})
		},
	})

	shared.RegisterUsage(cmd, "database", usageText)
	root.AddCommand(cmd)
}

const usageText = `database — Database discovery

COMMANDS:
  database list [-c <alias>]
    List all databases with sizes. One record per database (name, sizeOnDisk,
    empty flag); total size rides on the trailing {"@meta": ...} line.

  database stats <database> [-c <alias>]
    Get database statistics: collection count, document count, data/storage/index sizes.

EXAMPLES:
  agent-mongo database list
  agent-mongo database list -c production
  agent-mongo database stats myapp`
