package serialize

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestValueInt64Boundaries(t *testing.T) {
	const maxSafe = int64(1<<53 - 1)

	tests := []struct {
		name string
		in   int64
		want any
	}{
		{"max safe integer stays number", maxSafe, maxSafe},
		{"min safe integer stays number", -maxSafe, -maxSafe},
		{"just above max becomes string", maxSafe + 1, "9007199254740992"},
		{"just below min becomes string", -maxSafe - 1, "-9007199254740992"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Value(tt.in); got != tt.want {
				t.Errorf("Value(%d) = %v (%T), want %v (%T)", tt.in, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestValueUncommonBSONTypes(t *testing.T) {
	t.Run("uuid subtype with wrong length falls back to base64", func(t *testing.T) {
		short := bson.Binary{Subtype: bson.TypeBinaryUUID, Data: []byte("short")}
		if got := Value(short); got != "c2hvcnQ=" {
			t.Errorf("got %v", got)
		}
	})
	t.Run("timestamp", func(t *testing.T) {
		got, ok := Value(bson.Timestamp{T: 7, I: 3}).(map[string]any)
		if !ok || got["t"] != uint32(7) || got["i"] != uint32(3) {
			t.Errorf("got %#v", got)
		}
	})
	t.Run("minkey maxkey undefined", func(t *testing.T) {
		if got := Value(bson.MinKey{}); got != "MinKey" {
			t.Errorf("MinKey: %v", got)
		}
		if got := Value(bson.MaxKey{}); got != "MaxKey" {
			t.Errorf("MaxKey: %v", got)
		}
		if got := Value(bson.Undefined{}); got != nil {
			t.Errorf("Undefined: %v", got)
		}
	})
}
