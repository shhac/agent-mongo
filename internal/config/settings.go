package config

import "fmt"

// GetSetting returns the persisted value for a dotted settings key, and
// whether it is set. Keys are validated upstream by the config command.
func GetSetting(key string) (int, bool) {
	s := ReadSettings()
	switch key {
	case "defaults.limit":
		if s.Defaults != nil && s.Defaults.Limit > 0 {
			return s.Defaults.Limit, true
		}
	case "defaults.sampleSize":
		if s.Defaults != nil && s.Defaults.SampleSize > 0 {
			return s.Defaults.SampleSize, true
		}
	case "defaults.schemaSampleSize":
		if s.Defaults != nil && s.Defaults.SchemaSampleSize > 0 {
			return s.Defaults.SchemaSampleSize, true
		}
	case "query.timeout":
		if s.Query != nil && s.Query.Timeout > 0 {
			return s.Query.Timeout, true
		}
	case "query.maxDocuments":
		if s.Query != nil && s.Query.MaxDocuments > 0 {
			return s.Query.MaxDocuments, true
		}
	case "truncation.maxLength":
		if s.Truncation != nil && s.Truncation.MaxLength > 0 {
			return s.Truncation.MaxLength, true
		}
	}
	return 0, false
}

func UpdateSetting(key string, value int) error {
	cfg := Read()
	if cfg.Settings == nil {
		cfg.Settings = &Settings{}
	}
	s := cfg.Settings
	switch key {
	case "defaults.limit", "defaults.sampleSize", "defaults.schemaSampleSize":
		if s.Defaults == nil {
			s.Defaults = &DefaultsSettings{}
		}
		switch key {
		case "defaults.limit":
			s.Defaults.Limit = value
		case "defaults.sampleSize":
			s.Defaults.SampleSize = value
		case "defaults.schemaSampleSize":
			s.Defaults.SchemaSampleSize = value
		}
	case "query.timeout", "query.maxDocuments":
		if s.Query == nil {
			s.Query = &QuerySettings{}
		}
		if key == "query.timeout" {
			s.Query.Timeout = value
		} else {
			s.Query.MaxDocuments = value
		}
	case "truncation.maxLength":
		if s.Truncation == nil {
			s.Truncation = &TruncationSettings{}
		}
		s.Truncation.MaxLength = value
	default:
		return fmt.Errorf("Unknown key: %q", key)
	}
	return Write(cfg)
}

func ResetSettings() error {
	cfg := Read()
	cfg.Settings = nil
	return Write(cfg)
}
