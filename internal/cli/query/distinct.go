package query

import (
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/shhac/agent-mongo/internal/cli/shared"
	"github.com/shhac/agent-mongo/internal/mongo"
	"github.com/shhac/agent-mongo/internal/output"
)

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
				return printDistinct(values, ref, field, filterDoc)
			})
		},
	}

	cmd.Flags().StringVar(&filter, "filter", "", "MongoDB query filter (JSON)")
	parent.AddCommand(cmd)
}

func printDistinct(values []any, ref mongo.Ref, field string, filter bson.D) error {
	var e echo
	e.str("field", field)
	e.doc("filter", filter)
	return output.PrintResultWithMeta(map[string]any{
		"database":   ref.DB,
		"collection": ref.Collection,
		"field":      field,
		"values":     values,
		"count":      len(values),
	}, e.meta())
}
