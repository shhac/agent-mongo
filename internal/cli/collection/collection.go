// Package collection implements `agent-mongo collection` — collection
// discovery: list, schema inference, indexes, stats.
package collection

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/cli/shared"
	"github.com/shhac/agent-mongo/internal/mongo"
	"github.com/shhac/agent-mongo/internal/output"
)

func Register(root *cobra.Command, globals func() *shared.GlobalFlags) {
	cmd := &cobra.Command{Use: "collection", Short: "Collection discovery"}

	cmd.AddCommand(&cobra.Command{
		Use:   "list <database>",
		Short: "List collections in a database",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			g := globals()
			return shared.WithSession(g, func(ctx shared.SessionCtx) error {
				collections, err := ctx.Session.ListCollections(ctx.Ctx, args[0])
				if err != nil {
					return err
				}
				items := make([]any, len(collections))
				for i, coll := range collections {
					items[i] = coll
				}
				return output.PrintList(items, map[string]any{
					"@meta": map[string]any{"database": args[0]},
				})
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
				items := make([]any, len(indexes))
				for i, idx := range indexes {
					items[i] = idx
				}
				return output.PrintList(items, map[string]any{
					"@meta": map[string]any{"database": ref.DB, "collection": ref.Collection},
				})
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

func parsePositiveInt(value, name string) (int, error) {
	if value == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("Invalid %s: %q. Must be a positive integer.", name, value)
	}
	return n, nil
}

func parseNonNegativeInt(value, name string) (int, error) {
	if value == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("Invalid %s: %q. Must be a non-negative integer.", name, value)
	}
	return n, nil
}

func registerSchema(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var sampleSize, depth, limit, skip string

	cmd := &cobra.Command{
		Use:   "schema <database> <collection>",
		Short: "Infer collection schema by sampling documents",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			g := globals()
			ref := mongo.Ref{DB: args[0], Collection: args[1]}

			sample, err := parsePositiveInt(sampleSize, "--sample-size")
			if err != nil {
				return err
			}
			if sample == 0 {
				sample = shared.SchemaSampleSizeDefault()
			}
			maxDepth, err := parsePositiveInt(depth, "--depth")
			if err != nil {
				return err
			}
			limitVal, err := parsePositiveInt(limit, "--limit")
			if err != nil {
				return err
			}
			skipVal, err := parseNonNegativeInt(skip, "--skip")
			if err != nil {
				return err
			}

			return shared.WithSessionRef(g, ref, func(ctx shared.SessionCtx) error {
				result, err := ctx.Session.InferSchema(ctx.Ctx, mongo.SchemaOpts{
					Ref:        ref,
					SampleSize: sample,
					MaxDepth:   maxDepth,
				})
				if err != nil {
					return err
				}
				return printSchema(result, limitVal, skipVal)
			})
		},
	}

	cmd.Flags().StringVar(&sampleSize, "sample-size", "", "Number of documents to sample")
	cmd.Flags().StringVar(&depth, "depth", "", "Max nesting depth for fields (1 = top-level only)")
	cmd.Flags().StringVar(&limit, "limit", "", "Max fields to return (for pagination)")
	cmd.Flags().StringVar(&skip, "skip", "", "Number of fields to skip (for pagination)")
	parent.AddCommand(cmd)
}

func printSchema(result mongo.SchemaResult, limit, skip int) error {
	totalFields := len(result.Fields)
	sliced := result.Fields
	if skip > 0 {
		if skip >= totalFields {
			sliced = nil
		} else {
			sliced = sliced[skip:]
		}
	}
	if limit > 0 && len(sliced) > limit {
		sliced = sliced[:limit]
	}
	hasMore := skip+len(sliced) < totalFields

	items := make([]any, len(sliced))
	for i, field := range sliced {
		items[i] = field
	}

	meta := map[string]any{
		"@meta": map[string]any{
			"database":       result.Database,
			"collection":     result.Collection,
			"sampleSize":     result.SampleSize,
			"totalDocuments": result.TotalDocuments,
			"totalFields":    totalFields,
		},
	}
	if hasMore {
		nextSkip := skip + len(sliced)
		meta["@pagination"] = map[string]any{
			"has_more":    true,
			"next_cursor": strconv.Itoa(nextSkip),
			"total_items": totalFields,
		}
	}
	return output.PrintList(items, meta)
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
