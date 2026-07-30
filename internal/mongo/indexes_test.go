package mongo

import (
	"encoding/json"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// The reported failures: a compound key rendered alphabetically reads as drift
// against the index name, and a `deletedAt: null` clause silently dropped from
// a partial filter turns "excludes soft-deleted docs" into "doesn't".
func TestIndexSpecEmitsVerbatimJSON(t *testing.T) {
	tests := []struct {
		name string
		idx  bson.D
		want string
	}{
		{
			name: "compound key keeps declared order, not alphabetical",
			idx: bson.D{
				{Key: "v", Value: int32(2)},
				{Key: "key", Value: bson.D{{Key: "status", Value: int32(1)}, {Key: "expiryDate", Value: int32(1)}}},
				{Key: "name", Value: "status_1_expiryDate_1"},
			},
			want: `{"name":"status_1_expiryDate_1","key":{"status":1,"expiryDate":1}}`,
		},
		{
			name: "partialFilterExpression keeps null clauses and $in order",
			idx: bson.D{
				{Key: "v", Value: int32(2)},
				{Key: "key", Value: bson.D{{Key: "participantIds", Value: int32(1)}}},
				{Key: "name", Value: "participantIds_1"},
				{Key: "partialFilterExpression", Value: bson.D{
					{Key: "participantIds", Value: bson.D{{Key: "$type", Value: "string"}}},
					{Key: "deletedAt", Value: nil},
					{Key: "status", Value: bson.D{{Key: "$in", Value: bson.A{"pending", "confirmed"}}}},
				}},
			},
			want: `{"name":"participantIds_1","key":{"participantIds":1},` +
				`"partialFilterExpression":{"participantIds":{"$type":"string"},"deletedAt":null,` +
				`"status":{"$in":["pending","confirmed"]}}}`,
		},
		{
			name: "options ride along in server order, version markers dropped",
			idx: bson.D{
				{Key: "v", Value: int32(2)},
				{Key: "unique", Value: true},
				{Key: "key", Value: bson.D{{Key: "email", Value: int32(1)}}},
				{Key: "name", Value: "email_1"},
				{Key: "sparse", Value: true},
				{Key: "hidden", Value: true},
				{Key: "expireAfterSeconds", Value: int32(0)},
			},
			want: `{"name":"email_1","key":{"email":1},"unique":true,"sparse":true,` +
				`"hidden":true,"expireAfterSeconds":0}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(indexSpec(tc.idx))
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("indexSpec()\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}
