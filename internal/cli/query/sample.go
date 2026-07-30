package query

import (
	"maps"

	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/shhac/agent-mongo/internal/cli/shared"
	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/mongo"
	"github.com/shhac/agent-mongo/internal/output"
)

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
				return printSample(docs, ref, filterDoc, requestedSize)
			})
		},
	}

	cmd.Flags().StringVar(&size, "size", "", "Number of random documents")
	cmd.Flags().StringVar(&filter, "filter", "", "MongoDB query filter (JSON)")
	parent.AddCommand(cmd)
}

func printSample(docs []map[string]any, ref mongo.Ref, filter bson.D, size int) error {
	meta := output.Meta(map[string]any{
		"database":   ref.DB,
		"collection": ref.Collection,
		"sampleSize": len(docs),
	})
	var e echo
	e.doc("filter", filter)
	e.num("size", size)
	maps.Copy(meta, e.meta())
	return output.PrintList(docs, meta)
}
