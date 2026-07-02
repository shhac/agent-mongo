package config

import "testing"

func TestReadSettingsReturnsEmptyWhenNoneStored(t *testing.T) {
	isolate(t)
	s := ReadSettings()
	if s.Defaults != nil || s.Query != nil || s.Truncation != nil {
		t.Errorf("ReadSettings() = %+v, want zero", s)
	}
}

func TestUpdateSettingPersistsTopLevelSetting(t *testing.T) {
	isolate(t)
	if err := UpdateSetting("defaults.limit", 50); err != nil {
		t.Fatalf("UpdateSetting() error: %v", err)
	}
	v, ok := GetSetting("defaults.limit")
	if !ok || v != 50 {
		t.Errorf("GetSetting(defaults.limit) = %d,%v, want 50,true", v, ok)
	}
}

func TestUpdateSettingPersistsNestedSettings(t *testing.T) {
	isolate(t)
	if err := UpdateSetting("query.timeout", 5000); err != nil {
		t.Fatalf("UpdateSetting() error: %v", err)
	}
	v, ok := GetSetting("query.timeout")
	if !ok || v != 5000 {
		t.Errorf("GetSetting(query.timeout) = %d,%v, want 5000,true", v, ok)
	}
}

func TestUpdateSettingCreatesIntermediateObjects(t *testing.T) {
	isolate(t)
	if err := UpdateSetting("truncation.maxLength", 300); err != nil {
		t.Fatalf("UpdateSetting() error: %v", err)
	}
	s := ReadSettings()
	if s.Truncation == nil || s.Truncation.MaxLength != 300 {
		t.Errorf("truncation.maxLength = %+v, want 300", s.Truncation)
	}
}

func TestGetSettingReturnsFalseForNonExistentKey(t *testing.T) {
	isolate(t)
	if v, ok := GetSetting("nonexistent.key"); ok {
		t.Errorf("GetSetting(nonexistent.key) = %d,true, want 0,false", v)
	}
}

func TestGetSettingTraversesDottedPaths(t *testing.T) {
	isolate(t)
	if err := UpdateSetting("defaults.limit", 25); err != nil {
		t.Fatalf("UpdateSetting(limit) error: %v", err)
	}
	if err := UpdateSetting("defaults.sampleSize", 100); err != nil {
		t.Fatalf("UpdateSetting(sampleSize) error: %v", err)
	}
	if v, ok := GetSetting("defaults.limit"); !ok || v != 25 {
		t.Errorf("defaults.limit = %d,%v, want 25,true", v, ok)
	}
	if v, ok := GetSetting("defaults.sampleSize"); !ok || v != 100 {
		t.Errorf("defaults.sampleSize = %d,%v, want 100,true", v, ok)
	}
}

func TestResetSettingsClearsAllSettings(t *testing.T) {
	isolate(t)
	if err := UpdateSetting("defaults.limit", 50); err != nil {
		t.Fatalf("UpdateSetting(limit) error: %v", err)
	}
	if err := UpdateSetting("query.timeout", 5000); err != nil {
		t.Fatalf("UpdateSetting(timeout) error: %v", err)
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
	if err := UpdateSetting("defaults.limit", 10); err != nil {
		t.Fatalf("UpdateSetting() error: %v", err)
	}
	conn, ok := GetConnection("test")
	if !ok || conn.ConnectionString != "mongodb://test" {
		t.Errorf("connection = %+v,%v, want intact", conn, ok)
	}
	if v, ok := GetSetting("defaults.limit"); !ok || v != 10 {
		t.Errorf("defaults.limit = %d,%v, want 10,true", v, ok)
	}
}
