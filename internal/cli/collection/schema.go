package collection

import (
	"maps"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/cli/shared"
	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/mongo"
	"github.com/shhac/agent-mongo/internal/output"
)

func registerSchema(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var sampleSize, depth, limit, skip string

	cmd := &cobra.Command{
		Use:   "schema <database> <collection>",
		Short: "Infer collection schema by sampling documents",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			g := globals()
			ref := mongo.Ref{DB: args[0], Collection: args[1]}

			sample, err := shared.ParsePositiveInt(sampleSize, "--sample-size")
			if err != nil {
				return err
			}
			if sample == 0 {
				sample = config.SettingOr("defaults.schemaSampleSize")
			}
			maxDepth, err := shared.ParsePositiveInt(depth, "--depth")
			if err != nil {
				return err
			}
			limitVal, err := shared.ParsePositiveInt(limit, "--limit")
			if err != nil {
				return err
			}
			skipVal, err := shared.ParseNonNegativeInt(skip, "--skip")
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

// paginateFields slices the field list for --limit/--skip pagination.
// limit == 0 means unlimited.
func paginateFields(fields []mongo.FieldInfo, skip, limit int) ([]mongo.FieldInfo, bool) {
	if skip >= len(fields) {
		return nil, false
	}
	sliced := fields[skip:]
	if limit > 0 && len(sliced) > limit {
		sliced = sliced[:limit]
	}
	hasMore := skip+len(sliced) < len(fields)
	return sliced, hasMore
}

func printSchema(result mongo.SchemaResult, limit, skip int) error {
	sliced, hasMore := paginateFields(result.Fields, skip, limit)

	meta := output.Meta(map[string]any{
		"database":       result.Database,
		"collection":     result.Collection,
		"sampleSize":     result.SampleSize,
		"totalDocuments": result.TotalDocuments,
		"totalFields":    len(result.Fields),
	})
	if hasMore {
		nextSkip := strconv.Itoa(skip + len(sliced))
		maps.Copy(meta, output.PaginationMeta(true, nextSkip, len(result.Fields)))
	}
	return output.PrintList(sliced, meta)
}
