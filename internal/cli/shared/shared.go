// Package shared holds the cross-group CLI plumbing: the global-flag DTO that
// leaf commands receive via closure (cobra-idiomatic, no parent-chain walking)
// and small helpers used by every command group.
package shared

import (
	"context"
	"time"

	"github.com/shhac/agent-mongo/internal/config"
)

// GlobalFlags is a snapshot of the root command's persistent flags, resolved
// against persisted settings (timeout fallback applied in the root pre-run).
type GlobalFlags struct {
	Connection string // -c/--connection
	Expand     string // --expand
	Full       bool   // --full
	Format     string // -f/--format
	TimeoutMS  int    // -t/--timeout > settings query.timeout > 30000
}

// Timeout returns the effective operation timeout.
func (g *GlobalFlags) Timeout() time.Duration {
	ms := g.TimeoutMS
	if ms <= 0 {
		ms = config.SettingOr("query.timeout")
	}
	return time.Duration(ms) * time.Millisecond
}

// EffectiveLimit resolves a --limit flag value against the configured default
// page size, capped at query.maxDocuments.
func EffectiveLimit(flagValue int) int {
	limit := flagValue
	if limit <= 0 {
		limit = config.SettingOr("defaults.limit")
	}
	if max := config.SettingOr("query.maxDocuments"); limit > max {
		return max
	}
	return limit
}

// MakeContext builds the per-command context. The deadline gets a small grace
// period beyond the server-side maxTimeMS so MongoDB's own timeout error
// (code 50, which carries better hints) fires before the context does.
func (g *GlobalFlags) MakeContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), g.Timeout()+5*time.Second)
}

// FormatExpiry renders a session or token expiry for output. One function so
// every command that reports one uses the same format.
func FormatExpiry(at time.Time) string { return at.UTC().Format(time.RFC3339) }
