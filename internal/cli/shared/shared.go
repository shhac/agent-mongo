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

	// Command and Version identify this run to the server (comment, appName),
	// so a DBA can tell an agent-mongo query from the application's own.
	Command string
	Version string
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
	return min(limit, MaxDocuments())
}

// MaxDocuments is the most any one query returns (query.maxDocuments).
func MaxDocuments() int { return config.SettingOr("query.maxDocuments") }

// MakeContext builds the per-command context: one deadline for everything the
// command sends. The driver derives each command's maxTimeMS from what is left
// of it (less the round trip), so the server stops the work at the same moment
// the CLI stops waiting for it.
//
// It has to be the timeout exactly. A deadline takes precedence over the
// client-level timeout, so any grace added here would lengthen both what the
// caller waits and what the server is allowed to run.
func (g *GlobalFlags) MakeContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), g.Timeout())
}

// FormatExpiry renders a session or token expiry for output. One function so
// every command that reports one uses the same format.
func FormatExpiry(at time.Time) string { return at.UTC().Format(time.RFC3339) }
