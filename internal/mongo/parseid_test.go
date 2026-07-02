package mongo

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestParseID(t *testing.T) {
	hex24 := "665a1b2c3d4e5f6a7b8c9d0e"
	oid, _ := bson.ObjectIDFromHex(hex24)

	tests := []struct {
		name    string
		raw     string
		idType  string
		want    any
		wantErr bool
	}{
		{name: "auto-detect 24-hex becomes ObjectID", raw: hex24, want: oid},
		{name: "auto-detect 23 hex chars stays string", raw: hex24[:23], want: hex24[:23]},
		{name: "auto-detect 25 chars stays string", raw: hex24 + "f", want: hex24 + "f"},
		{name: "auto-detect non-hex stays string", raw: "zzza1b2c3d4e5f6a7b8c9d0e", want: "zzza1b2c3d4e5f6a7b8c9d0e"},
		{name: "type string forces 24-hex to stay string", raw: hex24, idType: "string", want: hex24},
		{name: "type objectid parses hex", raw: hex24, idType: "objectid", want: oid},
		{name: "type objectid rejects malformed", raw: "nope", idType: "objectid", wantErr: true},
		{name: "type number parses float", raw: "42.5", idType: "number", want: 42.5},
		{name: "type number parses int", raw: "7", idType: "number", want: 7.0},
		{name: "type number rejects non-numeric", raw: "abc", idType: "number", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseID(tt.raw, tt.idType)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseID: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}
