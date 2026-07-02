package mongo

import (
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestValidatePipelineRejectsOut(t *testing.T) {
	pipeline := bson.A{
		bson.D{{Key: "$match", Value: bson.D{{Key: "status", Value: "active"}}}},
		bson.D{{Key: "$out", Value: "outputCollection"}},
	}
	err := ValidatePipeline(pipeline)
	if err == nil || err.Error() != `Write stage "$out" is not allowed. agent-mongo is read-only.` {
		t.Errorf("error = %v, want exact $out rejection", err)
	}
}

func TestValidatePipelineRejectsMerge(t *testing.T) {
	pipeline := bson.A{
		bson.D{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$type"}}}},
		bson.D{{Key: "$merge", Value: bson.D{{Key: "into", Value: "results"}}}},
	}
	err := ValidatePipeline(pipeline)
	if err == nil || err.Error() != `Write stage "$merge" is not allowed. agent-mongo is read-only.` {
		t.Errorf("error = %v, want exact $merge rejection", err)
	}
}

func TestValidatePipelineRejectsOutAsFirstStage(t *testing.T) {
	err := ValidatePipeline(bson.A{bson.D{{Key: "$out", Value: "test"}}})
	if err == nil || !strings.Contains(err.Error(), `Write stage "$out"`) {
		t.Errorf("error = %v, want $out rejection", err)
	}
}

func TestValidatePipelineRejectsMergeAsFirstStage(t *testing.T) {
	err := ValidatePipeline(bson.A{bson.D{{Key: "$merge", Value: "test"}}})
	if err == nil || !strings.Contains(err.Error(), `Write stage "$merge"`) {
		t.Errorf("error = %v, want $merge rejection", err)
	}
}

func TestValidatePipelineAllowsReadOnlyPipeline(t *testing.T) {
	pipeline := bson.A{
		bson.D{{Key: "$match", Value: bson.D{{Key: "status", Value: "active"}}}},
		bson.D{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$category"}}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "total", Value: -1}}}},
		bson.D{{Key: "$limit", Value: 10}},
	}
	if err := ValidatePipeline(pipeline); err != nil {
		t.Errorf("ValidatePipeline() = %v, want nil", err)
	}
}

func TestValidatePipelineAllowsEmptyPipeline(t *testing.T) {
	if err := ValidatePipeline(bson.A{}); err != nil {
		t.Errorf("ValidatePipeline([]) = %v, want nil", err)
	}
}

func TestValidatePipelineAllowsProjectUnwindLookup(t *testing.T) {
	pipeline := bson.A{
		bson.D{{Key: "$project", Value: bson.D{{Key: "name", Value: 1}}}},
		bson.D{{Key: "$unwind", Value: "$items"}},
		bson.D{{Key: "$lookup", Value: bson.D{{Key: "from", Value: "other"}}}},
	}
	if err := ValidatePipeline(pipeline); err != nil {
		t.Errorf("ValidatePipeline() = %v, want nil", err)
	}
}

func TestValidatePipelineSkipsNonObjectStages(t *testing.T) {
	pipeline := bson.A{nil, "invalid", bson.D{{Key: "$match", Value: bson.D{{Key: "x", Value: 1}}}}}
	if err := ValidatePipeline(pipeline); err != nil {
		t.Errorf("ValidatePipeline() = %v, want nil", err)
	}
}
