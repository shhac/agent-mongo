package config

import (
	"strconv"
	"testing"
)

func TestReadSettingsReturnsEmptyWhenNoneStored(t *testing.T) {
	isolate(t)
	s := ReadSettings()
	if s.Defaults != nil || s.Query != nil || s.Truncation != nil {
		t.Errorf("ReadSettings() = %+v, want zero", s)
	}
}

func TestUpdateSettingPersistsTopLevelSetting(t *testing.T) {
	isolate(t)
	if err := updateSetting("defaults.limit", 50); err != nil {
		t.Fatalf("updateSetting() error: %v", err)
	}
	v, ok := getSetting("defaults.limit")
	if !ok || v != 50 {
		t.Errorf("getSetting(defaults.limit) = %d,%v, want 50,true", v, ok)
	}
}

func TestUpdateSettingPersistsNestedSettings(t *testing.T) {
	isolate(t)
	if err := updateSetting("query.timeout", 5000); err != nil {
		t.Fatalf("updateSetting() error: %v", err)
	}
	v, ok := getSetting("query.timeout")
	if !ok || v != 5000 {
		t.Errorf("getSetting(query.timeout) = %d,%v, want 5000,true", v, ok)
	}
}

func TestUpdateSettingCreatesIntermediateObjects(t *testing.T) {
	isolate(t)
	if err := updateSetting("truncation.maxLength", 300); err != nil {
		t.Fatalf("updateSetting() error: %v", err)
	}
	s := ReadSettings()
	if s.Truncation == nil || s.Truncation.MaxLength != 300 {
		t.Errorf("truncation.maxLength = %+v, want 300", s.Truncation)
	}
}

func TestGetSettingReturnsFalseForNonExistentKey(t *testing.T) {
	isolate(t)
	if v, ok := getSetting("nonexistent.key"); ok {
		t.Errorf("getSetting(nonexistent.key) = %d,true, want 0,false", v)
	}
}

func TestGetSettingTraversesDottedPaths(t *testing.T) {
	isolate(t)
	if err := updateSetting("defaults.limit", 25); err != nil {
		t.Fatalf("updateSetting(limit) error: %v", err)
	}
	if err := updateSetting("defaults.sampleSize", 100); err != nil {
		t.Fatalf("updateSetting(sampleSize) error: %v", err)
	}
	if v, ok := getSetting("defaults.limit"); !ok || v != 25 {
		t.Errorf("defaults.limit = %d,%v, want 25,true", v, ok)
	}
	if v, ok := getSetting("defaults.sampleSize"); !ok || v != 100 {
		t.Errorf("defaults.sampleSize = %d,%v, want 100,true", v, ok)
	}
}

func TestResetSettingsClearsAllSettings(t *testing.T) {
	isolate(t)
	if err := updateSetting("defaults.limit", 50); err != nil {
		t.Fatalf("updateSetting(limit) error: %v", err)
	}
	if err := updateSetting("query.timeout", 5000); err != nil {
		t.Fatalf("updateSetting(timeout) error: %v", err)
	}
	if err := ResetSettings(); err != nil {
		t.Fatalf("ResetSettings() error: %v", err)
	}
	s := ReadSettings()
	if s.Defaults != nil || s.Query != nil || s.Truncation != nil {
		t.Errorf("ReadSettings() = %+v, want zero after reset", s)
	}
}

func TestUpdateSettingDoesNotTouchConnectionData(t *testing.T) {
	isolate(t)
	if err := StoreConnection("test", Connection{ConnectionString: "mongodb://test"}); err != nil {
		t.Fatalf("StoreConnection() error: %v", err)
	}
	if err := updateSetting("defaults.limit", 10); err != nil {
		t.Fatalf("updateSetting() error: %v", err)
	}
	conn, ok := GetConnection("test")
	if !ok || conn.ConnectionString != "mongodb://test" {
		t.Errorf("connection = %+v,%v, want intact", conn, ok)
	}
	if v, ok := getSetting("defaults.limit"); !ok || v != 10 {
		t.Errorf("defaults.limit = %d,%v, want 10,true", v, ok)
	}
}

func TestSettingParseEnforcesItsRange(t *testing.T) {
	for _, def := range SettingDefs() {
		for raw, ok := range map[string]bool{
			strconv.Itoa(def.Min):     true,
			strconv.Itoa(def.Max):     true,
			strconv.Itoa(def.Default): true,
			strconv.Itoa(def.Min - 1): false,
			strconv.Itoa(def.Max + 1): false,
			"ten":                     false,
		} {
			if _, err := def.Parse(raw); (err == nil) != ok {
				t.Errorf("%s: Parse(%q) err=%v, want ok=%v", def.Key, raw, err, ok)
			}
		}
	}
}

func TestSettingValueFallsBackToItsDefault(t *testing.T) {
	isolate(t)
	if got := QueryTimeout.Value(); got != 30000 {
		t.Errorf("unset: %d, want the default", got)
	}
	if err := QueryTimeout.Set(4000); err != nil {
		t.Fatal(err)
	}
	if got := QueryTimeout.Value(); got != 4000 {
		t.Errorf("set: %d, want 4000", got)
	}
	if got := MaxDocuments.Value(); got != 100 {
		t.Errorf("a sibling in the same section: %d, want its default", got)
	}
}
