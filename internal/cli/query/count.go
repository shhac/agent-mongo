package query

import (
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/v2/bson"

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
				return printCount(count, ref, filterDoc)
			})
		},
	}

	cmd.Flags().StringVar(&filter, "filter", "", "MongoDB query filter (JSON)")
	parent.AddCommand(cmd)
}

func printCount(count int64, ref mongo.Ref, filter bson.D) error {
	var e echo
	e.doc("filter", filter)
	return output.PrintResultWithMeta(map[string]any{
		"database":   ref.DB,
		"collection": ref.Collection,
		"count":      count,
	}, e.meta())
}
