package mongo

import (
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/shhac/agent-mongo/internal/ejson"
)

func mustPipeline(t *testing.T, raw string) bson.A {
	t.Helper()
	pipeline, err := ejson.ParseArray(raw, "pipeline")
	if err != nil {
		t.Fatalf("parse pipeline: %v", err)
	}
	return pipeline
}

func TestValidatePipelineNestedWriteStages(t *testing.T) {
	tests := []struct {
		name     string
		pipeline string
		wantErr  string
	}{
		{
			name:     "$merge inside $facet rejected",
			pipeline: `[{"$facet": {"branch": [{"$merge": {"into": "results"}}]}}]`,
			wantErr:  `Write stage "$merge"`,
		},
		{
			name:     "$out inside $lookup pipeline rejected",
			pipeline: `[{"$lookup": {"from": "other", "pipeline": [{"$out": "evil"}], "as": "joined"}}]`,
			wantErr:  `Write stage "$out"`,
		},
		{
			name:     "$merge inside $unionWith pipeline rejected",
			pipeline: `[{"$unionWith": {"coll": "other", "pipeline": [{"$merge": {"into": "x"}}]}}]`,
			wantErr:  `Write stage "$merge"`,
		},
		{
			name:     "doubly nested $out rejected",
			pipeline: `[{"$facet": {"b": [{"$lookup": {"from": "o", "pipeline": [{"$out": "evil"}], "as": "j"}}]}}]`,
			wantErr:  `Write stage "$out"`,
		},
		{
			name:     "benign $facet and $lookup pass",
			pipeline: `[{"$facet": {"counts": [{"$count": "n"}]}}, {"$lookup": {"from": "o", "pipeline": [{"$match": {"x": 1}}], "as": "j"}}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePipeline(mustPipeline(t, tt.pipeline))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("got %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}
