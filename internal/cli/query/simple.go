package query

import (
	"maps"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/cli/shared"
	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/mongo"
	"github.com/shhac/agent-mongo/internal/output"
)

func registerCount(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var filter string

	cmd := &cobra.Command{
		Use:   "count <database> <collection>",
		Short: "Count documents matching a filter",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			g := globals()
			ref := mongo.Ref{DB: args[0], Collection: args[1]}
			filterDoc, err := parseOptionalDoc(filter, "filter")
			if err != nil {
				return err
			}
			return shared.WithSessionRef(g, ref, func(ctx shared.SessionCtx) error {
				count, err := ctx.Session.CountDocuments(ctx.Ctx, ref, filterDoc)
				if err != nil {
					return err
				}
				var e echo
				e.doc("filter", filterDoc)
				return output.PrintResultWithMeta(map[string]any{
					"database":   ref.DB,
					"collection": ref.Collection,
					"count":      count,
				}, e.meta())
			})
		},
	}

	cmd.Flags().StringVar(&filter, "filter", "", "MongoDB query filter (JSON)")
	parent.AddCommand(cmd)
}

func registerSample(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var size, filter string

	cmd := &cobra.Command{
		Use:   "sample <database> <collection>",
		Short: "Get random sample of documents",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			g := globals()
			ref := mongo.Ref{DB: args[0], Collection: args[1]}

			requestedSize, err := shared.ParsePositiveInt(size, "--size")
			if err != nil {
				return err
			}
			if requestedSize == 0 {
				requestedSize = config.SettingOr("defaults.sampleSize")
			}
			if max := config.SettingOr("query.maxDocuments"); requestedSize > max {
				requestedSize = max
			}

			filterDoc, err := parseOptionalDoc(filter, "filter")
			if err != nil {
				return err
			}

			return shared.WithSessionRef(g, ref, func(ctx shared.SessionCtx) error {
				docs, err := ctx.Session.SampleDocuments(ctx.Ctx, ref, requestedSize, filterDoc)
				if err != nil {
					return err
				}
				meta := output.Meta(map[string]any{
					"database":   ref.DB,
					"collection": ref.Collection,
					"sampleSize": len(docs),
				})
				var e echo
				e.doc("filter", filterDoc)
				e.num("size", requestedSize)
				maps.Copy(meta, e.meta())
				return output.PrintList(docs, meta)
			})
		},
	}

	cmd.Flags().StringVar(&size, "size", "", "Number of random documents")
	cmd.Flags().StringVar(&filter, "filter", "", "MongoDB query filter (JSON)")
	parent.AddCommand(cmd)
}

func registerDistinct(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var filter string

	cmd := &cobra.Command{
		Use:   "distinct <database> <collection> <field>",
		Short: "Get distinct values for a field",
		Args:  cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			g := globals()
			ref := mongo.Ref{DB: args[0], Collection: args[1]}
			field := args[2]

			filterDoc, err := parseOptionalDoc(filter, "filter")
			if err != nil {
				return err
			}

			return shared.WithSessionRef(g, ref, func(ctx shared.SessionCtx) error {
				values, err := ctx.Session.DistinctValues(ctx.Ctx, ref, field, filterDoc)
				if err != nil {
					return err
				}
				var e echo
				e.str("field", field)
				e.doc("filter", filterDoc)
				return output.PrintResultWithMeta(map[string]any{
					"database":   ref.DB,
					"collection": ref.Collection,
					"field":      field,
					"values":     values,
					"count":      len(values),
				}, e.meta())
			})
		},
	}

	cmd.Flags().StringVar(&filter, "filter", "", "MongoDB query filter (JSON)")
	parent.AddCommand(cmd)
}
