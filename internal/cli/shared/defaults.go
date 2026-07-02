package shared

import "github.com/shhac/agent-mongo/internal/config"

const (
	defaultPageSize         = 20
	defaultSampleSize       = 5
	defaultSchemaSampleSize = 100
	defaultMaxDocuments     = 100
)

// PageSizeDefault returns defaults.limit or 20.
func PageSizeDefault() int {
	if v, ok := config.GetSetting("defaults.limit"); ok {
		return v
	}
	return defaultPageSize
}

// SampleSizeDefault returns defaults.sampleSize or 5.
func SampleSizeDefault() int {
	if v, ok := config.GetSetting("defaults.sampleSize"); ok {
		return v
	}
	return defaultSampleSize
}

// SchemaSampleSizeDefault returns defaults.schemaSampleSize or 100.
func SchemaSampleSizeDefault() int {
	if v, ok := config.GetSetting("defaults.schemaSampleSize"); ok {
		return v
	}
	return defaultSchemaSampleSize
}

// MaxDocuments returns query.maxDocuments or 100.
func MaxDocuments() int {
	if v, ok := config.GetSetting("query.maxDocuments"); ok {
		return v
	}
	return defaultMaxDocuments
}

// EffectiveLimit resolves a --limit flag value against the configured default
// page size, capped at query.maxDocuments.
func EffectiveLimit(flagValue int) int {
	limit := flagValue
	if limit <= 0 {
		limit = PageSizeDefault()
	}
	if max := MaxDocuments(); limit > max {
		return max
	}
	return limit
}
