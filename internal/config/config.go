// Package config owns ~/.config/agent-mongo/config.json — the same file, same
// schema, the TypeScript implementation reads and writes. Credentials with
// keychain-backed values keep the "__KEYCHAIN__" sentinel; resolution lives in
// internal/credential.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shhac/lib-agent-cli/creds"
	"github.com/shhac/lib-agent-cli/xdg"
)

type Credential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Connection struct {
	ConnectionString string `json:"connection_string"`
	Name             string `json:"name,omitempty"`
	Database         string `json:"database,omitempty"`
	Credential       string `json:"credential,omitempty"`
}

type DefaultsSettings struct {
	Limit            int `json:"limit,omitempty"`
	SampleSize       int `json:"sampleSize,omitempty"`
	SchemaSampleSize int `json:"schemaSampleSize,omitempty"`
}

type QuerySettings struct {
	Timeout      int `json:"timeout,omitempty"`
	MaxDocuments int `json:"maxDocuments,omitempty"`
}

type TruncationSettings struct {
	MaxLength int `json:"maxLength,omitempty"`
}

type Settings struct {
	Defaults   *DefaultsSettings   `json:"defaults,omitempty"`
	Query      *QuerySettings      `json:"query,omitempty"`
	Truncation *TruncationSettings `json:"truncation,omitempty"`
}

type Config struct {
	DefaultConnection string                `json:"default_connection,omitempty"`
	Connections       map[string]Connection `json:"connections,omitempty"`
	Credentials       map[string]Credential `json:"credentials,omitempty"`
	Settings          *Settings             `json:"settings,omitempty"`
}

func Dir() string { return xdg.ConfigDir("agent-mongo") }

func filePath() string { return filepath.Join(Dir(), "config.json") }

func store() creds.Store { return creds.Store{Path: filePath()} }

// Read returns the parsed config, or a zero Config when the file is missing
// or unparseable (matching the TS behavior of treating both as empty).
func Read() Config {
	data, err := os.ReadFile(filePath())
	if err != nil {
		return Config{}
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}
	}
	return cfg
}

func Write(cfg Config) error { return store().Save(cfg) }

func GetConnection(alias string) (Connection, bool) {
	conn, ok := Read().Connections[alias]
	return conn, ok
}

func Connections() map[string]Connection {
	conns := Read().Connections
	if conns == nil {
		return map[string]Connection{}
	}
	return conns
}

func ConnectionAliases() []string {
	aliases := make([]string, 0, len(Connections()))
	for alias := range Connections() {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	return aliases
}

func DefaultConnectionAlias() string { return Read().DefaultConnection }

func StoreConnection(alias string, conn Connection) error {
	cfg := Read()
	if cfg.Connections == nil {
		cfg.Connections = map[string]Connection{}
	}
	cfg.Connections[alias] = conn
	if cfg.DefaultConnection == "" {
		cfg.DefaultConnection = alias
	}
	return Write(cfg)
}

func unknownConnectionError(alias string, cfg Config) error {
	valid := make([]string, 0, len(cfg.Connections))
	for a := range cfg.Connections {
		valid = append(valid, a)
	}
	sort.Strings(valid)
	list := strings.Join(valid, ", ")
	if list == "" {
		list = "(none)"
	}
	return fmt.Errorf("Unknown connection: %q. Valid: %s", alias, list)
}

func RemoveConnection(alias string) error {
	cfg := Read()
	if _, ok := cfg.Connections[alias]; !ok {
		return unknownConnectionError(alias, cfg)
	}
	delete(cfg.Connections, alias)
	if cfg.DefaultConnection == alias {
		cfg.DefaultConnection = ""
		remaining := make([]string, 0, len(cfg.Connections))
		for a := range cfg.Connections {
			remaining = append(remaining, a)
		}
		sort.Strings(remaining)
		if len(remaining) > 0 {
			cfg.DefaultConnection = remaining[0]
		}
	}
	return Write(cfg)
}

func SetDefaultConnection(alias string) error {
	cfg := Read()
	if _, ok := cfg.Connections[alias]; !ok {
		return unknownConnectionError(alias, cfg)
	}
	cfg.DefaultConnection = alias
	return Write(cfg)
}

// ConnectionUpdates carries optional field updates: nil means leave unchanged,
// a pointer to the empty string clears the field.
type ConnectionUpdates struct {
	Database   *string
	Credential *string
}

func UpdateConnection(alias string, updates ConnectionUpdates) error {
	cfg := Read()
	conn, ok := cfg.Connections[alias]
	if !ok {
		return unknownConnectionError(alias, cfg)
	}
	if updates.Database != nil {
		conn.Database = *updates.Database
	}
	if updates.Credential != nil {
		conn.Credential = *updates.Credential
	}
	cfg.Connections[alias] = conn
	return Write(cfg)
}

func ReadSettings() Settings {
	s := Read().Settings
	if s == nil {
		return Settings{}
	}
	return *s
}
