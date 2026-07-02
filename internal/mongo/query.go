package mongo

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"go.mongodb.org/mongo-driver/v2/bson"
	driver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/shhac/agent-mongo/internal/serialize"
)

type FindOpts struct {
	Ref
	Filter     bson.D
	Sort       bson.D
	Projection bson.D
	Limit      int
	Skip       int
}

type FindResult struct {
	Documents     []map[string]any
	Count         int
	HasMore       bool
	TotalMatching int64
}

func orEmpty(filter bson.D) bson.D {
	if filter == nil {
		return bson.D{}
	}
	return filter
}

func (s *Session) FindDocuments(ctx context.Context, opts FindOpts) (FindResult, error) {
	collection := s.Client.Database(opts.DB).Collection(opts.Collection)
	filter := orEmpty(opts.Filter)

	findOpts := options.Find().SetSkip(int64(opts.Skip)).SetLimit(int64(opts.Limit + 1))
	if opts.Sort != nil {
		findOpts = findOpts.SetSort(opts.Sort)
	}
	if opts.Projection != nil {
		findOpts = findOpts.SetProjection(opts.Projection)
	}

	cursor, err := collection.Find(ctx, filter, findOpts)
	if err != nil {
		return FindResult{}, err
	}
	var raw []bson.D
	if err := cursor.All(ctx, &raw); err != nil {
		return FindResult{}, err
	}

	hasMore := len(raw) > opts.Limit
	if hasMore {
		raw = raw[:opts.Limit]
	}

	totalMatching, err := s.countWithFilter(ctx, collection, filter)
	if err != nil {
		return FindResult{}, err
	}

	return FindResult{
		Documents:     serialize.Documents(raw),
		Count:         len(raw),
		HasMore:       hasMore,
		TotalMatching: totalMatching,
	}, nil
}

func (s *Session) countWithFilter(
	ctx context.Context, collection *driver.Collection, filter bson.D,
) (int64, error) {
	if len(filter) == 0 {
		return collection.EstimatedDocumentCount(ctx)
	}
	return collection.CountDocuments(ctx, filter)
}

type FindByIDOpts struct {
	Ref
	RawID      string
	IDType     string // "objectid", "string", "number", or "" for auto-detect
	Projection bson.D
}

var objectIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{24}$`)

// ParseID interprets a raw _id argument, auto-detecting ObjectIds by shape.
func ParseID(raw, idType string) (any, error) {
	if idType == "objectid" || (idType == "" && objectIDPattern.MatchString(raw)) {
		return bson.ObjectIDFromHex(raw)
	}
	if idType == "number" {
		n, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("Invalid number ID: %q", raw)
		}
		return n, nil
	}
	return raw, nil
}

// FindByID returns the serialized document, or nil when not found.
func (s *Session) FindByID(ctx context.Context, opts FindByIDOpts) (map[string]any, error) {
	id, err := ParseID(opts.RawID, opts.IDType)
	if err != nil {
		return nil, err
	}
	findOpts := options.FindOne()
	if opts.Projection != nil {
		findOpts = findOpts.SetProjection(opts.Projection)
	}
	var doc bson.D
	err = s.Client.Database(opts.DB).Collection(opts.Collection).
		FindOne(ctx, bson.D{{Key: "_id", Value: id}}, findOpts).
		Decode(&doc)
	if err != nil {
		if errors.Is(err, driver.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return serialize.Document(doc), nil
}

func (s *Session) CountDocuments(ctx context.Context, ref Ref, filter bson.D) (int64, error) {
	collection := s.Client.Database(ref.DB).Collection(ref.Collection)
	return s.countWithFilter(ctx, collection, orEmpty(filter))
}

func (s *Session) DistinctValues(
	ctx context.Context, ref Ref, field string, filter bson.D,
) ([]any, error) {
	collection := s.Client.Database(ref.DB).Collection(ref.Collection)
	var values bson.A
	err := collection.Distinct(ctx, field, orEmpty(filter)).Decode(&values)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = serialize.Value(v)
	}
	return out, nil
}

// SampleDocuments returns random documents, optionally filtered first.
func (s *Session) SampleDocuments(
	ctx context.Context, ref Ref, size int, filter bson.D,
) ([]map[string]any, error) {
	pipeline := bson.A{}
	if len(filter) > 0 {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: filter}})
	}
	pipeline = append(pipeline, bson.D{{Key: "$sample", Value: bson.D{{Key: "size", Value: size}}}})

	cursor, err := s.Client.Database(ref.DB).Collection(ref.Collection).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	var raw []bson.D
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, err
	}
	return serialize.Documents(raw), nil
}
