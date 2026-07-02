package config

import (
	"fmt"
	"strconv"
)

// SettingDef is the single source of truth for one persisted setting: dotted
// key, default, valid range, description, and accessors over the typed
// Settings structs (which pin the on-disk JSON shape).
type SettingDef struct {
	Key         string
	Default     int
	Min, Max    int
	Description string
	get         func(Settings) int // 0 = unset
	set         func(*Settings, int)
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

var settingDefs = []SettingDef{
	{
		Key: "defaults.limit", Default: 20, Min: 1, Max: 1000,
		Description: "Default result limit for list/query commands",
		get: func(s Settings) int {
			if s.Defaults == nil {
				return 0
			}
			return s.Defaults.Limit
		},
		set: func(s *Settings, v int) { defaults(s).Limit = v },
	},
	{
		Key: "defaults.sampleSize", Default: 5, Min: 1, Max: 100,
		Description: "Default sample size for query sample",
		get: func(s Settings) int {
			if s.Defaults == nil {
				return 0
			}
			return s.Defaults.SampleSize
		},
		set: func(s *Settings, v int) { defaults(s).SampleSize = v },
	},
	{
		Key: "defaults.schemaSampleSize", Default: 100, Min: 1, Max: 1000,
		Description: "Default sample size for schema inference",
		get: func(s Settings) int {
			if s.Defaults == nil {
				return 0
			}
			return s.Defaults.SchemaSampleSize
		},
		set: func(s *Settings, v int) { defaults(s).SchemaSampleSize = v },
	},
	{
		Key: "query.timeout", Default: 30000, Min: 1000, Max: 300000,
		Description: "Query timeout in milliseconds",
		get: func(s Settings) int {
			if s.Query == nil {
				return 0
			}
			return s.Query.Timeout
		},
		set: func(s *Settings, v int) { query(s).Timeout = v },
	},
	{
		Key: "query.maxDocuments", Default: 100, Min: 1, Max: 10000,
		Description: "Maximum documents returned per query",
		get: func(s Settings) int {
			if s.Query == nil {
				return 0
			}
			return s.Query.MaxDocuments
		},
		set: func(s *Settings, v int) { query(s).MaxDocuments = v },
	},
	{
		Key: "truncation.maxLength", Default: 200, Min: 50, Max: 100000,
		Description: "Max string length before truncation (any field)",
		get: func(s Settings) int {
			if s.Truncation == nil {
				return 0
			}
			return s.Truncation.MaxLength
		},
		set: func(s *Settings, v int) { truncation(s).MaxLength = v },
	},
}

// SettingDefs returns the registry in declaration order.
func SettingDefs() []SettingDef { return settingDefs }

// FindSetting looks up a registry entry by dotted key.
func FindSetting(key string) (SettingDef, bool) {
	for _, def := range settingDefs {
		if def.Key == key {
			return def, true
		}
	}
	return SettingDef{}, false
}

// Parse validates a raw string against the setting's type and range.
func (d SettingDef) Parse(raw string) (int, error) {
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

// GetSetting returns the persisted value for a dotted settings key, and
// whether it is set.
func GetSetting(key string) (int, bool) {
	def, ok := FindSetting(key)
	if !ok {
		return 0, false
	}
	value := def.get(ReadSettings())
	return value, value > 0
}

// SettingOr returns the persisted value for a key, falling back to the
// registry default when unset.
func SettingOr(key string) int {
	def, ok := FindSetting(key)
	if !ok {
		return 0
	}
	if value := def.get(ReadSettings()); value > 0 {
		return value
	}
	return def.Default
}

func UpdateSetting(key string, value int) error {
	def, ok := FindSetting(key)
	if !ok {
		return fmt.Errorf("Unknown key: %q", key)
	}
	cfg := Read()
	if cfg.Settings == nil {
		cfg.Settings = &Settings{}
	}
	def.set(cfg.Settings, value)
	return Write(cfg)
}

func ResetSettings() error {
	cfg := Read()
	cfg.Settings = nil
	return Write(cfg)
}
