package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/shhac/agent-mongo/internal/serialize"
)

var writeStages = map[string]bool{"$out": true, "$merge": true}

// ValidatePipeline rejects write stages — agent-mongo is read-only. Stages
// carrying sub-pipelines ($facet, $lookup, $unionWith) are checked
// recursively so a nested write stage fails here rather than at the server.
func ValidatePipeline(pipeline bson.A) error {
	for _, stage := range pipeline {
		doc, ok := stage.(bson.D)
		if !ok {
			continue
		}
		for _, elem := range doc {
			if writeStages[elem.Key] {
				return fmt.Errorf("Write stage %q is not allowed. agent-mongo is read-only.", elem.Key)
			}
			if err := validateSubPipelines(elem); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateSubPipelines(elem bson.E) error {
	switch elem.Key {
	case "$facet":
		facets, ok := elem.Value.(bson.D)
		if !ok {
			return nil
		}
		for _, facet := range facets {
			if sub, ok := facet.Value.(bson.A); ok {
				if err := ValidatePipeline(sub); err != nil {
					return err
				}
			}
		}
	case "$lookup", "$unionWith":
		spec, ok := elem.Value.(bson.D)
		if !ok {
			return nil
		}
		for _, field := range spec {
			if field.Key != "pipeline" {
				continue
			}
			if sub, ok := field.Value.(bson.A); ok {
				if err := ValidatePipeline(sub); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// HasLimitStage reports whether the pipeline sets its own top-level $limit.
func HasLimitStage(pipeline bson.A) bool {
	for _, stage := range pipeline {
		doc, ok := stage.(bson.D)
		if !ok {
			continue
		}
		for _, elem := range doc {
			if elem.Key == "$limit" {
				return true
			}
		}
	}
	return false
}

type AggregateOpts struct {
	Ref
	Pipeline bson.A
	// Limit caps the results, whatever the pipeline's own $limit says.
	Limit int
}

type AggregateResult struct {
	Documents []map[string]any
	// HasMore reports that the pipeline produced more than Limit results.
	HasMore bool
}

func (s *Session) Aggregate(ctx context.Context, opts AggregateOpts) (AggregateResult, error) {
	if err := ValidatePipeline(opts.Pipeline); err != nil {
		return AggregateResult{}, err
	}

	// One past the cap, so a capped result can say so. Appended even after a
	// $limit of the pipeline's own, which would otherwise decide how much comes
	// back regardless of query.maxDocuments.
	fetch := opts.Limit + 1
	pipeline := append(append(bson.A{}, opts.Pipeline...),
		bson.D{{Key: "$limit", Value: fetch}})

	raw, err := s.runCursor(ctx, opts.DB, aggregateCommand(opts.Collection, pipeline, fetch))
	if err != nil {
		return AggregateResult{}, err
	}
	hasMore := len(raw) > opts.Limit
	if hasMore {
		raw = raw[:opts.Limit]
	}
	return AggregateResult{Documents: serialize.Documents(raw), HasMore: hasMore}, nil
}
