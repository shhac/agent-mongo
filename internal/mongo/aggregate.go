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

func hasLimitStage(pipeline bson.A) bool {
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
	Limit    int
}

func (s *Session) Aggregate(ctx context.Context, opts AggregateOpts) ([]map[string]any, error) {
	if err := ValidatePipeline(opts.Pipeline); err != nil {
		return nil, err
	}

	pipeline := opts.Pipeline
	if !hasLimitStage(pipeline) {
		pipeline = append(append(bson.A{}, pipeline...),
			bson.D{{Key: "$limit", Value: opts.Limit}})
	}

	cursor, err := s.Client.Database(opts.DB).Collection(opts.Collection).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	var raw []bson.D
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, err
	}
	return serialize.Documents(raw), nil
}
