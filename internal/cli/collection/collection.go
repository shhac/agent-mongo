// Package collection implements `agent-mongo collection` — collection
// discovery: list, schema inference, indexes, stats.
package collection

import (
	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/cli/shared"
	"github.com/shhac/agent-mongo/internal/mongo"
	"github.com/shhac/agent-mongo/internal/output"
)

func Register(root *cobra.Command, globals func() *shared.GlobalFlags) {
	cmd := &cobra.Command{Use: "collection", Short: "Collection discovery"}

	cmd.AddCommand(&cobra.Command{
		Use:     "list <database>",
		Aliases: []string{"ls"},
		Short:   "List collections in a database",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			g := globals()
			return shared.WithSession(g, func(ctx shared.SessionCtx) error {
				collections, err := ctx.Session.ListCollections(ctx.Ctx, args[0])
				if err != nil {
					return err
				}
				return output.PrintList(collections, output.Meta(map[string]any{
					"database": args[0],
				}))
			})
		},
	})

	registerSchema(cmd, globals)

	cmd.AddCommand(&cobra.Command{
		Use:   "indexes <database> <collection>",
		Short: "List indexes on a collection",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			g := globals()
			ref := mongo.Ref{DB: args[0], Collection: args[1]}
			return shared.WithSessionRef(g, ref, func(ctx shared.SessionCtx) error {
				indexes, err := ctx.Session.ListIndexes(ctx.Ctx, ref)
				if err != nil {
					return err
				}
				return output.PrintList(indexes, output.Meta(map[string]any{
					"database":   ref.DB,
					"collection": ref.Collection,
				}))
			})
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "stats <database> <collection>",
		Short: "Get collection statistics",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			g := globals()
			ref := mongo.Ref{DB: args[0], Collection: args[1]}
			return shared.WithSessionRef(g, ref, func(ctx shared.SessionCtx) error {
				result, err := ctx.Session.CollectionStats(ctx.Ctx, ref)
				if err != nil {
					return err
				}
				return output.PrintResult(result)
			})
		},
	})

	shared.RegisterUsage(cmd, "collection", usageText)
	root.AddCommand(cmd)
}

const usageText = `collection — Collection discovery

COMMANDS:
  collection list <database> [-c <alias>]
    List all collections in a database. One record per collection: name and
    type (collection or view).

  collection schema <database> <collection> [--sample-size <n>] [--depth <n>] [--limit <n>] [--skip <n>] [-c <alias>]
    Infer collection schema by sampling documents. Default sample: 100 (configurable via defaults.schemaSampleSize).
    One record per field: path, types, presence rate (0.0-1.0).
    Array element types shown as "path.$" entries.
    Errors if collection does not exist.
    Sample size, total documents, and total fields ride on the {"@meta": ...} line.
    --depth <n>    Limit nesting depth (1 = top-level only, 2 = one level of nesting, etc.)
    --limit <n>    Max fields to return (paginate large schemas)
    --skip <n>     Number of fields to skip (use with --limit for pagination)
    When paginated, a {"@pagination": ...} line carries next_cursor (the next skip value).

  collection indexes <database> <collection> [-c <alias>]
    List all indexes with key patterns, uniqueness, and other properties.

  collection stats <database> <collection> [-c <alias>]
    Get collection statistics: document count, data/storage/index sizes, capped flag.

EXAMPLES:
  agent-mongo collection list myapp
  agent-mongo collection schema myapp users
  agent-mongo collection schema myapp users --sample-size 500
  agent-mongo collection schema myapp events --depth 2
  agent-mongo collection schema myapp events --limit 50
  agent-mongo collection schema myapp events --limit 50 --skip 50
  agent-mongo collection indexes myapp users
  agent-mongo collection stats myapp orders`
