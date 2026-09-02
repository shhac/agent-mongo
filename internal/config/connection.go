package config

// Connections are the saved MongoDB endpoints a command resolves against; each
// may reference a stored credential by alias.

import (
	"fmt"
	"sort"
)

type Connection struct {
	ConnectionString string `json:"connection_string"`
	Name             string `json:"name,omitempty"`
	Database         string `json:"database,omitempty"`
	Credential       string `json:"credential,omitempty"`
}

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
	return Update(func(cfg *Config) error {
		if cfg.Connections == nil {
			cfg.Connections = map[string]Connection{}
		}
		cfg.Connections[alias] = conn
		if cfg.DefaultConnection == "" {
			cfg.DefaultConnection = alias
		}
		return nil
	})
}

// UnknownConnectionError is the shared self-correcting error for an alias that
// names no stored connection.
func UnknownConnectionError(alias string) error {
	return unknownConnectionError(alias, Read())
}

func unknownConnectionError(alias string, cfg Config) error {
	valid := make([]string, 0, len(cfg.Connections))
	for a := range cfg.Connections {
		valid = append(valid, a)
	}
	sort.Strings(valid)
	return fmt.Errorf("Unknown connection: %q. Valid: %s", alias, JoinOrNone(valid))
}

func RemoveConnection(alias string) error {
	return Update(func(cfg *Config) error {
		if _, ok := cfg.Connections[alias]; !ok {
			return unknownConnectionError(alias, *cfg)
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
		return nil
	})
}

func SetDefaultConnection(alias string) error {
	return Update(func(cfg *Config) error {
		if _, ok := cfg.Connections[alias]; !ok {
			return unknownConnectionError(alias, *cfg)
		}
		cfg.DefaultConnection = alias
		return nil
	})
}

// ConnectionUpdates carries optional field updates: nil means leave unchanged,
// a pointer to the empty string clears the field.
type ConnectionUpdates struct {
	Database   *string
	Credential *string
}

func UpdateConnection(alias string, updates ConnectionUpdates) error {
	return Update(func(cfg *Config) error {
		conn, ok := cfg.Connections[alias]
		if !ok {
			return unknownConnectionError(alias, *cfg)
		}
		if updates.Database != nil {
			conn.Database = *updates.Database
		}
		if updates.Credential != nil {
			conn.Credential = *updates.Credential
		}
		cfg.Connections[alias] = conn
		return nil
	})
}
