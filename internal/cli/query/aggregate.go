package query

import (
	"errors"
	"io"
	"maps"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/shhac/agent-mongo/internal/cli/shared"
	"github.com/shhac/agent-mongo/internal/ejson"
	"github.com/shhac/agent-mongo/internal/mongo"
	"github.com/shhac/agent-mongo/internal/output"
	"github.com/shhac/agent-mongo/internal/serialize"
)

func registerAggregate(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var pipelineFlag string
	var limit int
	var stream bool

	cmd := &cobra.Command{
		Use:   "aggregate <database> <collection> [pipeline]",
		Short: "Run a read-only aggregation pipeline",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()
			ref := mongo.Ref{DB: args[0], Collection: args[1]}

			positional := ""
			if len(args) == 3 {
				positional = args[2]
			}
			pipeline, err := resolvePipeline(positional, pipelineFlag, cmd.InOrStdin())
			if err != nil {
				return err
			}

			maxResults := aggregateLimit(pipeline, limit)
			return shared.WithSession(g, ref, func(ctx shared.SessionCtx) error {
				result, err := ctx.Session.Aggregate(ctx.Ctx, mongo.AggregateOpts{
					Ref:      ref,
					Pipeline: pipeline,
					Limit:    maxResults,
				})
				if err != nil {
					return err
				}
				return printAggregate(result, ref, pipeline, maxResults)
			})
		},
	}

	cmd.Flags().StringVar(&pipelineFlag, "pipeline", "",
		"Aggregation pipeline as JSON array (or pipe via stdin)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max results if the pipeline has no $limit stage (a pipeline's own $limit is capped at query.maxDocuments)")
	cmd.Flags().BoolVar(&stream, "stream", false, "Deprecated no-op: NDJSON is the default output")
	_ = cmd.Flags().MarkHidden("stream")
	parent.AddCommand(cmd)
}

// aggregateLimit is the cap an aggregation runs under. A pipeline with a $limit
// of its own decides how many results it wants, up to query.maxDocuments like
// every other query; one without gets --limit or the default page size.
func aggregateLimit(pipeline bson.A, flag int) int {
	if mongo.HasLimitStage(pipeline) {
		return shared.MaxDocuments()
	}
	return shared.EffectiveLimit(flag)
}

// resolvePipeline takes the pipeline from the positional argument, else
// --pipeline, else stdin.
func resolvePipeline(positional, flag string, stdin io.Reader) (bson.A, error) {
	raw := positional
	if raw == "" {
		raw = flag
	}
	if raw == "" {
		piped, err := readPiped(stdin)
		if err != nil {
			return nil, err
		}
		raw = piped
	}
	return ejson.ParseArray(raw, "pipeline")
}

// readPiped reads a pipeline piped in. A terminal is refused rather than read:
// waiting on one would hang an agent that simply forgot the argument.
func readPiped(stdin io.Reader) (string, error) {
	if isTerminal(stdin) {
		return "", errors.New(
			"Provide pipeline as argument, --pipeline <json>, or pipe a JSON array via stdin.")
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return "", err
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return "", errors.New(
			"Empty stdin. Provide pipeline as argument, --pipeline <json>, or pipe a JSON array via stdin.")
	}
	return trimmed, nil
}

func isTerminal(r io.Reader) bool {
	file, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err != nil || info.Mode()&os.ModeCharDevice != 0
}

// printAggregate assembles the aggregation output and its optional echo, split
// from the command so it is testable without a live session.
func printAggregate(result mongo.AggregateResult, ref mongo.Ref, pipeline bson.A, limit int) error {
	meta := output.Meta(map[string]any{
		"database":   ref.DB,
		"collection": ref.Collection,
		"count":      len(result.Documents),
	})
	maps.Copy(meta, output.PaginationMeta(result.HasMore, "", 0))
	var e echo
	// Both orders matter: the sequence of stages, and each stage's fields.
	e.value("pipeline", serialize.OrderedValue(pipeline))
	e.num("limit", limit)
	maps.Copy(meta, e.meta())
	return output.PrintList(result.Documents, meta)
}
