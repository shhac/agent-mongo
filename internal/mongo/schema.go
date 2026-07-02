package mongo

import (
	"context"
	"math"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type FieldInfo struct {
	Path     string   `json:"path"`
	Types    []string `json:"types"`
	Presence float64  `json:"presence"`
}

type SchemaResult struct {
	Database       string
	Collection     string
	SampleSize     int
	TotalDocuments int64
	Fields         []FieldInfo
}

type SchemaOpts struct {
	Ref
	SampleSize int
	MaxDepth   int // 0 = unlimited
}

type fieldAgg struct {
	types map[string]bool
	count int
}

type walker struct {
	fields   map[string]*fieldAgg
	seen     map[string]bool
	maxDepth int
}

// InferSchema samples documents and aggregates field paths, types, and
// presence rates. Array element types are recorded under "path.$".
func (s *Session) InferSchema(ctx context.Context, opts SchemaOpts) (SchemaResult, error) {
	if err := s.ValidateCollectionExists(ctx, opts.Ref); err != nil {
		return SchemaResult{}, err
	}

	collection := s.Client.Database(opts.DB).Collection(opts.Collection)
	totalDocuments, err := collection.EstimatedDocumentCount(ctx)
	if err != nil {
		return SchemaResult{}, err
	}

	effectiveSize := opts.SampleSize
	if totalDocuments > 0 && int64(effectiveSize) > totalDocuments {
		effectiveSize = int(totalDocuments)
	}

	cursor, err := collection.Aggregate(ctx, bson.A{
		bson.D{{Key: "$sample", Value: bson.D{{Key: "size", Value: effectiveSize}}}},
	})
	if err != nil {
		return SchemaResult{}, err
	}
	var docs []bson.D
	if err := cursor.All(ctx, &docs); err != nil {
		return SchemaResult{}, err
	}

	return SchemaResult{
		Database:       opts.DB,
		Collection:     opts.Collection,
		SampleSize:     len(docs),
		TotalDocuments: totalDocuments,
		Fields:         InferFields(docs, opts.MaxDepth),
	}, nil
}

// InferFields aggregates field paths, types, and presence rates from sampled
// documents — the pure core of schema inference.
func InferFields(docs []bson.D, maxDepth int) []FieldInfo {
	w := &walker{fields: map[string]*fieldAgg{}, maxDepth: maxDepth}
	for _, doc := range docs {
		w.seen = map[string]bool{}
		w.walkDocument(doc, "", 1)
	}

	fields := make([]FieldInfo, 0, len(w.fields))
	for path, agg := range w.fields {
		types := make([]string, 0, len(agg.types))
		for t := range agg.types {
			types = append(types, t)
		}
		sort.Strings(types)
		presence := 0.0
		if len(docs) > 0 {
			presence = math.Round(float64(agg.count)/float64(len(docs))*100) / 100
		}
		fields = append(fields, FieldInfo{Path: path, Types: types, Presence: presence})
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Path < fields[j].Path })
	return fields
}

func (w *walker) walkDocument(doc bson.D, prefix string, depth int) {
	for _, elem := range doc {
		path := elem.Key
		if prefix != "" {
			path = prefix + "." + elem.Key
		}
		w.recordField(path, typeName(elem.Value))

		if w.maxDepth > 0 && depth >= w.maxDepth {
			continue
		}

		switch v := elem.Value.(type) {
		case bson.D:
			w.walkDocument(v, path, depth+1)
		case bson.A:
			w.walkArrayElements(v, path, depth)
		}
	}
}

func (w *walker) walkArrayElements(arr bson.A, parentPath string, depth int) {
	elemPath := parentPath + ".$"
	for _, elem := range arr {
		w.recordFieldType(elemPath, typeName(elem))

		if w.maxDepth == 0 || depth < w.maxDepth {
			if doc, ok := elem.(bson.D); ok {
				w.walkDocument(doc, elemPath, depth+1)
			}
		}
	}

	if len(arr) > 0 && !w.seen[elemPath] {
		w.seen[elemPath] = true
		if field, ok := w.fields[elemPath]; ok {
			field.count++
		}
	}
}

func (w *walker) recordField(path, tn string) {
	w.recordFieldType(path, tn)
	if !w.seen[path] {
		w.seen[path] = true
		if field, ok := w.fields[path]; ok {
			field.count++
		}
	}
}

func (w *walker) recordFieldType(path, tn string) {
	field, ok := w.fields[path]
	if !ok {
		field = &fieldAgg{types: map[string]bool{}}
		w.fields[path] = field
	}
	field.types[tn] = true
}

const maxSafeInteger = 1<<53 - 1

func typeName(value any) string {
	switch v := value.(type) {
	case nil, bson.Null, bson.Undefined:
		return "null"
	case bson.ObjectID:
		return "ObjectId"
	case bson.DateTime, time.Time:
		return "date"
	case bson.Binary:
		if v.Subtype == bson.TypeBinaryUUID {
			return "uuid"
		}
		return "binary"
	case bson.Decimal128:
		return "decimal"
	case bson.Regex:
		return "regex"
	case bson.A, []any:
		return "array"
	case string:
		return "string"
	case int32:
		return "int"
	case int64:
		// The TS driver promotes safe int64s to JS numbers, which the TS
		// schema reports as "int"; only unsafe magnitudes surface as "long".
		if v >= -maxSafeInteger && v <= maxSafeInteger {
			return "int"
		}
		return "long"
	case float64:
		if v == math.Trunc(v) && !math.IsInf(v, 0) && !math.IsNaN(v) {
			return "int"
		}
		return "double"
	case bool:
		return "boolean"
	case bson.D, bson.M, map[string]any:
		return "object"
	default:
		return "object"
	}
}
