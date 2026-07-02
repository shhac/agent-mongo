// Package configcmd implements `agent-mongo config` — persisted settings with
// validated keys, defaults, and min/max ranges.
package configcmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/cli/shared"
	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/output"
)

type keyDef struct {
	key          string
	defaultValue int
	description  string
	min, max     int
}

var keyDefs = []keyDef{
	{"defaults.limit", 20, "Default result limit for list/query commands", 1, 1000},
	{"defaults.sampleSize", 5, "Default sample size for query sample", 1, 100},
	{"defaults.schemaSampleSize", 100, "Default sample size for schema inference", 1, 1000},
	{"query.timeout", 30000, "Query timeout in milliseconds", 1000, 300000},
	{"query.maxDocuments", 100, "Maximum documents returned per query", 1, 10000},
	{"truncation.maxLength", 200, "Max string length before truncation (any field)", 50, 100000},
}

func findKey(key string) (keyDef, bool) {
	for _, def := range keyDefs {
		if def.key == key {
			return def, true
		}
	}
	return keyDef{}, false
}

func validKeys() string {
	names := make([]string, len(keyDefs))
	for i, def := range keyDefs {
		names[i] = def.key
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func unknownKeyError(key string) error {
	return fmt.Errorf("Unknown key: %q. Valid keys: %s", key, validKeys())
}

func parseValue(def keyDef, raw string) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%q must be an integer. Got: %q", def.key, raw)
	}
	if n < def.min {
		return 0, fmt.Errorf("%q minimum is %d. Got: %d", def.key, def.min, n)
	}
	if n > def.max {
		return 0, fmt.Errorf("%q maximum is %d. Got: %d", def.key, def.max, n)
	}
	return n, nil
}

func Register(root *cobra.Command) {
	cmd := &cobra.Command{Use: "config", Short: "Manage CLI settings"}

	cmd.AddCommand(&cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			key := args[0]
			if _, ok := findKey(key); !ok {
				return unknownKeyError(key)
			}
			result := map[string]any{"key": key}
			if value, set := config.GetSetting(key); set {
				result["value"] = value
			}
			return output.PrintRaw(result)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			key := args[0]
			def, ok := findKey(key)
			if !ok {
				return unknownKeyError(key)
			}
			value, err := parseValue(def, args[1])
			if err != nil {
				return err
			}
			if err := config.UpdateSetting(key, value); err != nil {
				return err
			}
			return output.PrintRaw(map[string]any{"ok": true, "key": key, "value": value})
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "reset",
		Short: "Reset all settings to defaults",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := config.ResetSettings(); err != nil {
				return err
			}
			return output.PrintRaw(map[string]any{"ok": true, "message": "Settings reset to defaults"})
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "list-keys",
		Short: "List all valid config keys with defaults",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			keys := make([]map[string]any, len(keyDefs))
			for i, def := range keyDefs {
				keys[i] = map[string]any{
					"key":         def.key,
					"type":        "number",
					"default":     def.defaultValue,
					"description": def.description,
				}
			}
			return output.PrintRaw(map[string]any{"keys": keys})
		},
	})

	shared.RegisterUsage(cmd, "config", usageText)
	root.AddCommand(cmd)
}

const usageText = `config — Manage CLI settings

COMMANDS:
  config get <key>              Get a config value
  config set <key> <value>      Set a config value
  config reset                  Reset all settings to defaults
  config list-keys              List all valid keys with defaults and ranges

KEYS:
  defaults.limit        (20)     Default result limit for list/query commands [1-1000]
  defaults.sampleSize       (5)      Default sample size for query sample [1-100]
  defaults.schemaSampleSize (100)    Default sample size for schema inference [1-1000]
  query.timeout         (30000)  Query timeout in ms [1000-300000]
  query.maxDocuments    (100)    Max documents per query [1-10000]
  truncation.maxLength  (200)    Max string length before truncation [50-100000]

EXAMPLES:
  agent-mongo config set defaults.limit 50
  agent-mongo config get query.timeout
  agent-mongo config reset`
