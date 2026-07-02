// Package serialize converts BSON-decoded values into JSON-safe values with
// an LLM-friendly mapping: ObjectId → bare hex, Date → ISO-8601, Binary →
// base64, int64 → number when safe else string, Decimal128/UUID/RegExp →
// string. No EJSON wrappers ({"$oid": ...}) on output.
package serialize

import (
	"encoding/base64"
	"fmt"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const maxSafeInteger = 1<<53 - 1 // JS Number.MAX_SAFE_INTEGER

// IsSafeInt64 reports whether v is exactly representable in a double — the
// boundary between emitting a JSON number and a string.
func IsSafeInt64(v int64) bool { return v >= -maxSafeInteger && v <= maxSafeInteger }

const isoMillis = "2006-01-02T15:04:05.000Z"

func uuidString(data []byte) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", data[0:4], data[4:6], data[6:8], data[8:10], data[10:16])
}

// Value converts a single BSON-decoded value to a JSON-safe value.
func Value(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case bson.ObjectID:
		return v.Hex()
	case bson.DateTime:
		return v.Time().UTC().Format(isoMillis)
	case time.Time:
		return v.UTC().Format(isoMillis)
	case bson.Binary:
		if v.Subtype == bson.TypeBinaryUUID && len(v.Data) == 16 {
			return uuidString(v.Data)
		}
		return base64.StdEncoding.EncodeToString(v.Data)
	case int64:
		// Numbers only when exactly representable in a double (what JSON
		// consumers can trust), strings otherwise.
		if IsSafeInt64(v) {
			return v
		}
		return fmt.Sprintf("%d", v)
	case bson.Decimal128:
		return v.String()
	case bson.Regex:
		return "/" + v.Pattern + "/" + v.Options
	case bson.Timestamp:
		return map[string]any{"t": v.T, "i": v.I}
	case bson.MinKey:
		return "MinKey"
	case bson.MaxKey:
		return "MaxKey"
	case bson.Null, bson.Undefined:
		return nil
	case bson.D:
		out := make(map[string]any, len(v))
		for _, elem := range v {
			out[elem.Key] = Value(elem.Value)
		}
		return out
	case bson.M:
		out := make(map[string]any, len(v))
		for key, val := range v {
			out[key] = Value(val)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, val := range v {
			out[key] = Value(val)
		}
		return out
	case bson.A:
		return sliceValue(v)
	case []any:
		return sliceValue(v)
	default:
		return v
	}
}

func sliceValue(items []any) []any {
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = Value(item)
	}
	return out
}

// Document converts a BSON document to a JSON-safe map.
func Document(doc bson.D) map[string]any {
	converted, _ := Value(doc).(map[string]any)
	return converted
}

// Documents converts a slice of BSON documents to JSON-safe maps.
func Documents(docs []bson.D) []map[string]any {
	out := make([]map[string]any, len(docs))
	for i, doc := range docs {
		out[i] = Document(doc)
	}
	return out
}

// IsSafeInteger reports whether a float64 holds an integral value (used by
// schema type naming to mirror JS Number.isInteger).
func IsSafeInteger(f float64) bool {
	return f == math.Trunc(f) && !math.IsInf(f, 0) && !math.IsNaN(f)
}
