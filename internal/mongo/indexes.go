package mongo

import (
	"context"
	"slices"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/shhac/agent-mongo/internal/serialize"
)

func (s *Session) ListIndexes(ctx context.Context, ref Ref) ([]serialize.Ordered, error) {
	cursor, err := s.Client.Database(ref.DB).Collection(ref.Collection).Indexes().List(ctx)
	if err != nil {
		return nil, err
	}
	var raw []bson.D
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, err
	}

	indexes := make([]serialize.Ordered, len(raw))
	for i, idx := range raw {
		indexes[i] = indexSpec(idx)
	}
	return indexes, nil
}

// leadingIndexFields are hoisted to the front of a spec, in this order: the
// server reports `v, key, name, …`, but a spec reads best name-first.
var leadingIndexFields = []string{"name", "key"}

// serverOnlyIndexFields are storage-implementation version markers, of no use
// to anyone comparing a live index against a declared one.
var serverOnlyIndexFields = map[string]bool{
	"v": true, "ns": true, "textIndexVersion": true, "2dsphereIndexVersion": true,
}

// indexSpec renders one listIndexes entry with `name` and `key` first and every
// remaining option in server order.
//
// Unlike document output, an index spec is passed through verbatim — a
// blocklist rather than a whitelist, order preserved, nulls kept, nothing
// truncated (see output.PrintListVerbatim). Normalising it would report an
// index that does not exist: {a:1,b:1} and {b:1,a:1} are different indexes
// serving different queries, and a `field: null` clause in a
// partialFilterExpression is the whole difference between an index that
// excludes soft-deleted documents and one that does not.
func indexSpec(idx bson.D) serialize.Ordered {
	doc := serialize.OrderedDocument(idx)
	spec := make(serialize.Ordered, 0, len(doc))
	for _, key := range leadingIndexFields {
		if value, ok := doc.Lookup(key); ok {
			spec = append(spec, serialize.Field{Key: key, Value: value})
		}
	}
	for _, field := range doc {
		if slices.Contains(leadingIndexFields, field.Key) || serverOnlyIndexFields[field.Key] {
			continue
		}
		spec = append(spec, field)
	}
	return spec
}
