package query

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/cli/shared"
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
				return output.PrintResult(map[string]any{
					"database":   ref.DB,
					"collection": ref.Collection,
					"count":      count,
				})
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

			requestedSize := shared.SampleSizeDefault()
			if size != "" {
				n, err := strconv.Atoi(size)
				if err != nil || n < 1 {
					return fmt.Errorf("Invalid --size: %q. Must be a positive integer.", size)
				}
				requestedSize = n
			}
			if max := shared.MaxDocuments(); requestedSize > max {
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
				items := make([]any, len(docs))
				for i, doc := range docs {
					items[i] = doc
				}
				return output.PrintList(items, map[string]any{
					"@meta": map[string]any{
						"database":   ref.DB,
						"collection": ref.Collection,
						"sampleSize": len(docs),
					},
				})
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
				return output.PrintResult(map[string]any{
					"database":   ref.DB,
					"collection": ref.Collection,
					"field":      field,
					"values":     values,
					"count":      len(values),
				})
			})
		},
	}

	cmd.Flags().StringVar(&filter, "filter", "", "MongoDB query filter (JSON)")
	parent.AddCommand(cmd)
}
