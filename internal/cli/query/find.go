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
			query, err := findQuery(mongo.Ref{DB: args[0], Collection: args[1]},
				filter, sort, projection, limit, skip)
			if err != nil {
				return err
			}
			return shared.WithSession(globals(), query.Ref, func(ctx shared.SessionCtx) error {
				result, err := ctx.Session.FindDocuments(ctx.Ctx, query)
				if err != nil {
					return err
				}
				return printFind(result, query)
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

// findQuery is what find sends: the parsed flags plus the defaults the CLI
// supplies (newest first, the configured page size). The echo is printed from
// the same value, so it reports the query that ran rather than a copy of it.
func findQuery(ref mongo.Ref, filter, sort, projection string, limit, skip int) (mongo.FindOpts, error) {
	filterDoc, err := parseOptionalDoc(filter, "filter")
	if err != nil {
		return mongo.FindOpts{}, err
	}
	sortDoc, err := parseOptionalDoc(sort, "sort")
	if err != nil {
		return mongo.FindOpts{}, err
	}
	if sortDoc == nil {
		sortDoc = bson.D{{Key: "_id", Value: -1}}
	}
	projectionDoc, err := parseOptionalDoc(projection, "projection")
	if err != nil {
		return mongo.FindOpts{}, err
	}
	return mongo.FindOpts{
		Ref:        ref,
		Filter:     filterDoc,
		Sort:       sortDoc,
		Projection: projectionDoc,
		Limit:      shared.EffectiveLimit(limit),
		Skip:       skip,
	}, nil
}

// printFind assembles the record set, its metadata and the optional query echo.
// Split from the command so it is testable without a live session, following
// collection.printIndexes.
func printFind(result mongo.FindResult, q mongo.FindOpts) error {
	ref := q.Ref
	meta := output.Meta(map[string]any{
		"database":   ref.DB,
		"collection": ref.Collection,
	})
	maps.Copy(meta, output.PaginationMeta(result.HasMore, "", int(result.TotalMatching)))

	var e echo
	e.doc("filter", q.Filter)
	e.doc("sort", q.Sort)
	e.doc("projection", q.Projection)
	e.num("limit", q.Limit)
	e.num("skip", q.Skip)
	maps.Copy(meta, e.meta())

	return output.PrintList(result.Documents, meta)
}
