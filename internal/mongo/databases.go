package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type DatabaseInfo struct {
	Name       string `json:"name"`
	SizeOnDisk int64  `json:"sizeOnDisk"`
	Empty      bool   `json:"empty"`
}

type DatabaseList struct {
	Databases []DatabaseInfo
	TotalSize int64
}

func (s *Session) ListDatabases(ctx context.Context) (DatabaseList, error) {
	result, err := s.Client.ListDatabases(ctx, bson.D{})
	if err != nil {
		return DatabaseList{}, err
	}
	databases := make([]DatabaseInfo, len(result.Databases))
	for i, db := range result.Databases {
		databases[i] = DatabaseInfo{Name: db.Name, SizeOnDisk: db.SizeOnDisk, Empty: db.Empty}
	}
	return DatabaseList{Databases: databases, TotalSize: result.TotalSize}, nil
}

func (s *Session) DatabaseStats(ctx context.Context, dbName string) (map[string]any, error) {
	var result bson.M
	err := s.Client.Database(dbName).RunCommand(ctx, bson.D{{Key: "dbStats", Value: 1}}).Decode(&result)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"database":    dbName,
		"collections": result["collections"],
		"documents":   result["objects"],
		"dataSize":    result["dataSize"],
		"storageSize": result["storageSize"],
		"indexes":     result["indexes"],
		"indexSize":   result["indexSize"],
	}, nil
}
