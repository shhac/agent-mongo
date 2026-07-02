package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Ref addresses a collection within a database.
type Ref struct {
	DB         string
	Collection string
}

// ValidateCollectionExists errors with a self-correction hint when the
// collection is absent (schema inference relies on this).
func (s *Session) ValidateCollectionExists(ctx context.Context, ref Ref) error {
	names, err := s.Client.Database(ref.DB).ListCollectionNames(ctx, bson.D{{Key: "name", Value: ref.Collection}})
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf(
			"Collection %q not found in database %q. Use 'collection list %s' to see available collections.",
			ref.Collection, ref.DB, ref.DB)
	}
	return nil
}

type CollectionInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (s *Session) ListCollections(ctx context.Context, dbName string) ([]CollectionInfo, error) {
	cursor, err := s.Client.Database(dbName).ListCollections(ctx, bson.D{})
	if err != nil {
		return nil, err
	}
	var specs []struct {
		Name string `bson:"name"`
		Type string `bson:"type"`
	}
	if err := cursor.All(ctx, &specs); err != nil {
		return nil, err
	}
	collections := make([]CollectionInfo, len(specs))
	for i, spec := range specs {
		collType := spec.Type
		if collType == "" {
			collType = "collection"
		}
		collections[i] = CollectionInfo{Name: spec.Name, Type: collType}
	}
	return collections, nil
}

func (s *Session) CollectionStats(ctx context.Context, ref Ref) (map[string]any, error) {
	var result bson.M
	err := s.Client.Database(ref.DB).
		RunCommand(ctx, bson.D{{Key: "collStats", Value: ref.Collection}}).
		Decode(&result)
	if err != nil {
		return nil, err
	}
	capped := result["capped"]
	if capped == nil {
		capped = false
	}
	return map[string]any{
		"database":        ref.DB,
		"collection":      ref.Collection,
		"documentCount":   result["count"],
		"dataSize":        result["size"],
		"avgDocumentSize": result["avgObjSize"],
		"storageSize":     result["storageSize"],
		"indexes":         result["nindexes"],
		"indexSize":       result["totalIndexSize"],
		"capped":          capped,
	}, nil
}
