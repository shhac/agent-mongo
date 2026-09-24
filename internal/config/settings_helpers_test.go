package config

import "fmt"

// updateSetting and getSetting drive a setting by key, the way the config
// command does.
func updateSetting(key string, value int) error {
	def, ok := FindSetting(key)
	if !ok {
		return fmt.Errorf("unknown key %q", key)
	}
	return def.Set(value)
}

func getSetting(key string) (int, bool) {
	def, ok := FindSetting(key)
	if !ok {
		return 0, false
	}
	return def.Stored()
}
