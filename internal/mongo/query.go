package mongo

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

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
	raw, err := s.runCursor(ctx, opts.DB, findCommand(opts))
	if err != nil {
		return FindResult{}, err
	}

	raw, hasMore := keepLimit(raw, opts.Limit)

	totalMatching, err := s.CountDocuments(ctx, opts.Ref, opts.Filter)
	if err != nil {
		return FindResult{}, err
	}

	return FindResult{
		Documents:     serialize.Documents(raw),
		HasMore:       hasMore,
		TotalMatching: totalMatching,
	}, nil
}

// CountDocuments counts what a filter matches; with no filter, the collection's
// metadata count, which is instant where a full count scans.
func (s *Session) CountDocuments(ctx context.Context, ref Ref, filter bson.D) (int64, error) {
	if len(filter) == 0 {
		return s.estimatedCount(ctx, ref)
	}
	return s.collection(ref).CountDocuments(ctx, filter, commented(options.Count(), s.comment))
}

func (s *Session) estimatedCount(ctx context.Context, ref Ref) (int64, error) {
	return s.collection(ref).EstimatedDocumentCount(ctx,
		commented(options.EstimatedDocumentCount(), s.comment))
}

func (s *Session) collection(ref Ref) *driver.Collection {
	return s.Client.Database(ref.DB).Collection(ref.Collection)
}

type FindByIDOpts struct {
	Ref
	ID         any // from ParseID
	Projection bson.D
}

var objectIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{24}$`)

// IDTypes are the --type values ParseID understands; "" auto-detects.
var IDTypes = []string{"objectid", "string", "number"}

// ParseID interprets a raw _id argument, auto-detecting ObjectIds by shape.
// Pure, so a malformed id or --type fails before anything connects.
func ParseID(raw, idType string) (any, error) {
	if idType != "" && !slices.Contains(IDTypes, idType) {
		return nil, fmt.Errorf("Invalid --type: %q. Valid: %s", idType, strings.Join(IDTypes, ", "))
	}
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
	findOpts := commented(options.FindOne(), s.comment)
	if opts.Projection != nil {
		findOpts = findOpts.SetProjection(opts.Projection)
	}
	var doc bson.D
	err := s.collection(opts.Ref).
		FindOne(ctx, bson.D{{Key: "_id", Value: opts.ID}}, findOpts).
		Decode(&doc)
	if err != nil {
		if errors.Is(err, driver.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return serialize.Document(doc), nil
}

func (s *Session) DistinctValues(
	ctx context.Context, ref Ref, field string, filter bson.D,
) ([]any, error) {
	result := s.collection(ref).Distinct(ctx, field, orEmpty(filter),
		commented(options.Distinct(), s.comment))
	// Asked first because Decode ignores it despite documenting otherwise: a
	// failed distinct would surface as "error decoding key arr: unexpected EOF",
	// losing the timeout or auth failure an agent needs to act on.
	if err := result.Err(); err != nil {
		return nil, err
	}
	var values bson.A
	if err := result.Decode(&values); err != nil {
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
	raw, err := s.runCursor(ctx, ref.DB, aggregateCommand(ref.Collection, samplePipeline(filter, size), size))
	if err != nil {
		return nil, err
	}
	return serialize.Documents(raw), nil
}
