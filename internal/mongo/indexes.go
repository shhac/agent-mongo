package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/shhac/agent-mongo/internal/serialize"
)

func (s *Session) ListIndexes(ctx context.Context, ref Ref) ([]map[string]any, error) {
	cursor, err := s.Client.Database(ref.DB).Collection(ref.Collection).Indexes().List(ctx)
	if err != nil {
		return nil, err
	}
	var raw []bson.D
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, err
	}

	indexes := make([]map[string]any, len(raw))
	for i, idx := range raw {
		doc := serialize.Document(idx)
		info := map[string]any{
			"name": doc["name"],
			"key":  doc["key"],
		}
		if unique, ok := doc["unique"].(bool); ok && unique {
			info["unique"] = true
		}
		if sparse, ok := doc["sparse"].(bool); ok && sparse {
			info["sparse"] = true
		}
		if expire, ok := doc["expireAfterSeconds"]; ok {
			info["expireAfterSeconds"] = expire
		}
		if partial, ok := doc["partialFilterExpression"]; ok && partial != nil {
			info["partialFilterExpression"] = partial
		}
		indexes[i] = info
	}
	return indexes, nil
}
