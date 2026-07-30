package query

import (
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/shhac/agent-mongo/internal/serialize"
)

// MetaKeyQuery is the @-line carrying the executed query back to the caller.
const MetaKeyQuery = "@query"

// echoEnabled is the --echo-query flag, registered once on the query group and
// read by every leaf.
var echoEnabled bool

func registerEchoFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolVar(&echoEnabled, "echo-query", false,
		"Echo the executed query back on an @query line (filter, sort, limit, …)")
}

// echo accumulates what a command actually sent to the server. It records
// *effective* values, not just the flags the caller typed: the default sort and
// the resolved limit are part of what ran, and an agent checking its own query
// needs to see them.
//
// Order is preserved throughout. A filter's field order is not cosmetic when
// reasoning about which index a query can use, and a `field: null` clause means
// something distinct from an absent one — in MongoDB {x: null} matches missing
// and null both, while {x: {$exists: false}} matches only missing. Echoing
// either of those wrongly would answer the question the caller asked with a
// query they did not run, which is worse than not answering it.
type echo struct {
	fields serialize.Ordered
}

// doc records an EJSON-derived document, skipping one that was never set.
func (e *echo) doc(key string, value bson.D) {
	if value == nil {
		return
	}
	e.fields = append(e.fields, serialize.Field{Key: key, Value: serialize.OrderedDocument(value)})
}

// num records a numeric option, skipping the zero value (an unset skip or an
// unused limit is noise, not information).
func (e *echo) num(key string, value int) {
	if value == 0 {
		return
	}
	e.fields = append(e.fields, serialize.Field{Key: key, Value: value})
}

func (e *echo) str(key, value string) {
	if value == "" {
		return
	}
	e.fields = append(e.fields, serialize.Field{Key: key, Value: value})
}

func (e *echo) value(key string, value any) {
	e.fields = append(e.fields, serialize.Field{Key: key, Value: value})
}

// meta returns the @query metadata entry, or nil when the flag is off — so a
// caller can hand the result straight to maps.Copy without branching.
func (e *echo) meta() map[string]any {
	if !echoEnabled || len(e.fields) == 0 {
		return nil
	}
	return map[string]any{MetaKeyQuery: e.fields}
}
