package query

import (
	"maps"

	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/shhac/agent-mongo/internal/cli/shared"
	"github.com/shhac/agent-mongo/internal/mongo"
	"github.com/shhac/agent-mongo/internal/output"
)

func registerFind(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var filter, sort, projection string
	var limit, skip int
	var stream bool

	cmd := &cobra.Command{
		Use:   "find <database> <collection>",
		Short: "Find documents matching a filter",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			g := globals()
			ref := mongo.Ref{DB: args[0], Collection: args[1]}

			filterDoc, err := parseOptionalDoc(filter, "filter")
			if err != nil {
				return err
			}
			sortDoc, err := parseOptionalDoc(sort, "sort")
			if err != nil {
				return err
			}
			if sortDoc == nil {
				sortDoc = bson.D{{Key: "_id", Value: -1}}
			}
			projectionDoc, err := parseOptionalDoc(projection, "projection")
			if err != nil {
				return err
			}

			return shared.WithSessionRef(g, ref, func(ctx shared.SessionCtx) error {
				result, err := ctx.Session.FindDocuments(ctx.Ctx, mongo.FindOpts{
					Ref:        ref,
					Filter:     filterDoc,
					Sort:       sortDoc,
					Projection: projectionDoc,
					Limit:      shared.EffectiveLimit(limit),
					Skip:       skip,
				})
				if err != nil {
					return err
				}

				meta := output.Meta(map[string]any{
					"database":   ref.DB,
					"collection": ref.Collection,
				})
				maps.Copy(meta, output.PaginationMeta(result.HasMore, "", int(result.TotalMatching)))
				return output.PrintList(result.Documents, meta)
			})
		},
	}

	cmd.Flags().StringVar(&filter, "filter", "", "MongoDB query filter (JSON)")
	cmd.Flags().StringVar(&sort, "sort", "", `Sort specification (e.g. {"createdAt": -1})`)
	cmd.Flags().StringVar(&projection, "projection", "", `Field projection (e.g. {"name": 1, "email": 1})`)
	cmd.Flags().IntVar(&limit, "limit", 0, "Max documents to return")
	cmd.Flags().IntVar(&skip, "skip", 0, "Number of documents to skip")
	cmd.Flags().BoolVar(&stream, "stream", false, "Deprecated no-op: NDJSON is the default output")
	_ = cmd.Flags().MarkHidden("stream")
	parent.AddCommand(cmd)
}
