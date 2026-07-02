// Package ejson parses MongoDB Extended JSON strings from --filter, --sort,
// --projection, and --pipeline flags into driver-ready BSON values. bson.D is
// used throughout so key order survives (it matters for sort specs).
package ejson

import (
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Parse parses an Extended JSON object (e.g. {"$date": ...}, {"$oid": ...})
// into an order-preserving document. name is the flag name for error messages.
func Parse(value, name string) (bson.D, error) {
	var doc bson.D
	if err := bson.UnmarshalExtJSON([]byte(value), false, &doc); err != nil {
		return nil, fmt.Errorf("Invalid JSON for --%s: %s", name, value)
	}
	return doc, nil
}

// ParseArray parses an Extended JSON array (e.g. an aggregation pipeline).
func ParseArray(value, name string) (bson.A, error) {
	var arr bson.A
	if err := bson.UnmarshalExtJSON([]byte(value), false, &arr); err != nil {
		preview := value
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		// Distinguish "valid JSON but not an array" from unparseable input.
		var doc bson.D
		if docErr := bson.UnmarshalExtJSON([]byte(value), false, &doc); docErr == nil {
			return nil, fmt.Errorf("--%s must be a JSON array", name)
		}
		return nil, fmt.Errorf("Invalid JSON for --%s: %s", name, preview)
	}
	return arr, nil
}
