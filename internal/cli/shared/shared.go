// Package shared holds the cross-group CLI plumbing: the global-flag DTO that
// leaf commands receive via closure (cobra-idiomatic, no parent-chain walking)
// and small helpers used by every command group.
package shared

import (
	"context"
	"time"

	"github.com/shhac/agent-mongo/internal/config"
)

// GlobalFlags is a snapshot of the root command's persistent flags that leaf
// commands read. Output format and truncation are process-wide, configured in
// the root pre-run, and not repeated here.
type GlobalFlags struct {
	Connection string // -c/--connection
	TimeoutMS  int    // -t/--timeout; 0 = unset, see Timeout

	// Command and Version identify this run to the server (comment, appName),
	// so a DBA can tell an agent-mongo query from the application's own.
	Command string
	Version string
}

// Timeout returns the effective operation timeout: -t/--timeout, else
// query.timeout.
func (g *GlobalFlags) Timeout() time.Duration {
	return time.Duration(SettingDefault(g.TimeoutMS, config.QueryTimeout)) * time.Millisecond
}

// SettingDefault is a count flag's value when given, else the setting's.
func SettingDefault(flag int, setting *config.SettingDef) int {
	if flag > 0 {
		return flag
	}
	return setting.Value()
}

// CappedCount is SettingDefault held to query.maxDocuments, for a flag that
// decides how many documents come back.
func CappedCount(flag int, setting *config.SettingDef) int {
	return min(SettingDefault(flag, setting), MaxDocuments())
}

// EffectiveLimit resolves a --limit flag value against the configured default
// page size, capped at query.maxDocuments.
func EffectiveLimit(flag int) int { return CappedCount(flag, config.DefaultLimit) }

// MaxDocuments is the most any one query returns (query.maxDocuments).
func MaxDocuments() int { return config.MaxDocuments.Value() }

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
