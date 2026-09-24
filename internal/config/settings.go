package config

import (
	"fmt"
	"strconv"
)

// SettingDef is the single source of truth for one persisted setting: dotted
// key, default, valid range, description, and where it lives in the typed
// Settings structs (which pin the on-disk JSON shape).
//
// Code reads a setting through its exported handle (QueryTimeout.Value()), not
// by key: a mistyped handle does not compile, where a mistyped key would read
// as 0 — a zero-length query deadline, or no result cap.
type SettingDef struct {
	Key         string
	Default     int
	Min, Max    int
	Description string
	field       func(*Settings) *int // allocates the section it points into
}

// Value is the persisted value, or the default when unset.
func (d *SettingDef) Value() int {
	if value, set := d.stored(); set {
		return value
	}
	return d.Default
}

// stored reads the persisted value; ReadSettings returns a fresh copy, so
// field allocating a missing section cannot leak into anything shared.
func (d *SettingDef) stored() (int, bool) {
	settings := ReadSettings()
	value := *d.field(&settings)
	return value, value > 0
}

func defaults(s *Settings) *DefaultsSettings {
	if s.Defaults == nil {
		s.Defaults = &DefaultsSettings{}
	}
	return s.Defaults
}

func query(s *Settings) *QuerySettings {
	if s.Query == nil {
		s.Query = &QuerySettings{}
	}
	return s.Query
}

func truncation(s *Settings) *TruncationSettings {
	if s.Truncation == nil {
		s.Truncation = &TruncationSettings{}
	}
	return s.Truncation
}

var (
	DefaultLimit = &SettingDef{
		Key: "defaults.limit", Default: 20, Min: 1, Max: 1000,
		Description: "Default result limit for list/query commands",
		field:       func(s *Settings) *int { return &defaults(s).Limit },
	}
	DefaultSampleSize = &SettingDef{
		Key: "defaults.sampleSize", Default: 5, Min: 1, Max: 100,
		Description: "Default sample size for query sample",
		field:       func(s *Settings) *int { return &defaults(s).SampleSize },
	}
	DefaultSchemaSampleSize = &SettingDef{
		Key: "defaults.schemaSampleSize", Default: 100, Min: 1, Max: 1000,
		Description: "Default sample size for schema inference",
		field:       func(s *Settings) *int { return &defaults(s).SchemaSampleSize },
	}
	QueryTimeout = &SettingDef{
		Key: "query.timeout", Default: 30000, Min: 1000, Max: 300000,
		Description: "Query timeout in milliseconds",
		field:       func(s *Settings) *int { return &query(s).Timeout },
	}
	MaxDocuments = &SettingDef{
		Key: "query.maxDocuments", Default: 100, Min: 1, Max: 10000,
		Description: "Maximum documents returned per query",
		field:       func(s *Settings) *int { return &query(s).MaxDocuments },
	}
	TruncationMaxLength = &SettingDef{
		Key: "truncation.maxLength", Default: 200, Min: 50, Max: 100000,
		Description: "Max string length before truncation (any field)",
		field:       func(s *Settings) *int { return &truncation(s).MaxLength },
	}
)

var settingDefs = []*SettingDef{
	DefaultLimit, DefaultSampleSize, DefaultSchemaSampleSize,
	QueryTimeout, MaxDocuments, TruncationMaxLength,
}

// SettingDefs returns the registry in declaration order.
func SettingDefs() []*SettingDef { return settingDefs }

// FindSetting looks up a registry entry by dotted key — for the config
// command, which is handed keys by name.
func FindSetting(key string) (*SettingDef, bool) {
	for _, def := range settingDefs {
		if def.Key == key {
			return def, true
		}
	}
	return nil, false
}

// Parse validates a raw string against the setting's type and range.
func (d *SettingDef) Parse(raw string) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%q must be an integer. Got: %q", d.Key, raw)
	}
	if n < d.Min {
		return 0, fmt.Errorf("%q minimum is %d. Got: %d", d.Key, d.Min, n)
	}
	if n > d.Max {
		return 0, fmt.Errorf("%q maximum is %d. Got: %d", d.Key, d.Max, n)
	}
	return n, nil
}

// Stored is the persisted value and whether one is set, without the default.
func (d *SettingDef) Stored() (int, bool) { return d.stored() }

// Set persists a value; callers validate it with Parse first.
func (d *SettingDef) Set(value int) error {
	return Update(func(cfg *Config) error {
		if cfg.Settings == nil {
			cfg.Settings = &Settings{}
		}
		*d.field(cfg.Settings) = value
		return nil
	})
}

func ResetSettings() error {
	return Update(func(cfg *Config) error {
		cfg.Settings = nil
		return nil
	})
}
