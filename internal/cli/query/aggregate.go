package query

import (
	"errors"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/shhac/agent-mongo/internal/cli/shared"
	"github.com/shhac/agent-mongo/internal/ejson"
	"github.com/shhac/agent-mongo/internal/mongo"
	"github.com/shhac/agent-mongo/internal/output"
)

func registerAggregate(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var pipelineFlag string
	var limit int
	var stream bool

	cmd := &cobra.Command{
		Use:   "aggregate <database> <collection> [pipeline]",
		Short: "Run a read-only aggregation pipeline",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(_ *cobra.Command, args []string) error {
			g := globals()
			ref := mongo.Ref{DB: args[0], Collection: args[1]}

			positional := ""
			if len(args) == 3 {
				positional = args[2]
			}
			pipeline, err := resolvePipeline(positional, pipelineFlag)
			if err != nil {
				return err
			}

			return shared.WithSessionRef(g, ref, func(ctx shared.SessionCtx) error {
				docs, err := ctx.Session.Aggregate(ctx.Ctx, mongo.AggregateOpts{
					Ref:      ref,
					Pipeline: pipeline,
					Limit:    shared.EffectiveLimit(limit),
				})
				if err != nil {
					return err
				}
				return output.PrintList(docs, output.Meta(map[string]any{
					"database":   ref.DB,
					"collection": ref.Collection,
					"count":      len(docs),
				}))
			})
		},
	}

	cmd.Flags().StringVar(&pipelineFlag, "pipeline", "",
		"Aggregation pipeline as JSON array (or pipe via stdin)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max results if pipeline has no $limit stage")
	cmd.Flags().BoolVar(&stream, "stream", false, "Deprecated no-op: NDJSON is the default output")
	_ = cmd.Flags().MarkHidden("stream")
	parent.AddCommand(cmd)
}

func resolvePipeline(positional, flag string) (bson.A, error) {
	raw := positional
	if raw == "" {
		raw = flag
	}
	if raw == "" {
		stdin, err := readStdin()
		if err != nil {
			return nil, err
		}
		raw = stdin
	}
	return ejson.ParseArray(raw, "pipeline")
}

func readStdin() (string, error) {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice != 0 {
		return "", errors.New(
			"Provide pipeline as argument, --pipeline <json>, or pipe a JSON array via stdin.")
	}
	data, err := io.ReadAll(os.Stdin)
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
