// Package shared holds the cross-group CLI plumbing: the global-flag DTO that
// leaf commands receive via closure (cobra-idiomatic, no parent-chain walking)
// and small helpers used by every command group.
package shared

import (
	"context"
	"time"
)

// DefaultTimeoutMS is the default query timeout (30s).
const DefaultTimeoutMS = 30_000

// GlobalFlags is a snapshot of the root command's persistent flags, resolved
// against persisted settings (timeout fallback applied in the root pre-run).
type GlobalFlags struct {
	Connection string // -c/--connection
	Expand     string // --expand
	Full       bool   // --full
	Format     string // -f/--format
	TimeoutMS  int    // -t/--timeout > settings query.timeout > 30000
	Debug      bool   // -d/--debug
}

// Timeout returns the effective operation timeout.
func (g *GlobalFlags) Timeout() time.Duration {
	ms := g.TimeoutMS
	if ms <= 0 {
		ms = DefaultTimeoutMS
	}
	return time.Duration(ms) * time.Millisecond
}

// MakeContext builds the per-command context. The deadline gets a small grace
// period beyond the server-side maxTimeMS so MongoDB's own timeout error
// (code 50, which carries better hints) fires before the context does.
func (g *GlobalFlags) MakeContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), g.Timeout()+5*time.Second)
}
