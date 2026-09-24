package config

// Connections are the saved MongoDB endpoints a command resolves against; each
// may reference a stored credential by alias.

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	out "github.com/shhac/lib-agent-output"
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

func ConnectionAliases() []string { return slices.Sorted(maps.Keys(Connections())) }

// ResolveAlias resolves the connection to use:
// -c flag > AGENT_MONGO_CONNECTION env > config default > error. A process
// pinned by IdentityEnv gets its bound connection, whatever it asks for.
func ResolveAlias(flag string) (string, error) {
	if pinned, ok, err := PinnedConnection(flag); ok {
		return pinned, err
	}
	if trimmed := strings.TrimSpace(flag); trimmed != "" {
		return trimmed, nil
	}
	if env := strings.TrimSpace(os.Getenv("AGENT_MONGO_CONNECTION")); env != "" {
		return env, nil
	}
	if def := DefaultConnectionAlias(); def != "" {
		return def, nil
	}
	return "", out.New(
		"No connection specified. Use -c <alias> or set a default. Available: "+JoinOrNone(ConnectionAliases()),
		out.FixableByAgent).WithHint(addConnectionHint)
}

// ResolveConnection is ResolveAlias and the connection it names.
func ResolveConnection(flag string) (string, Connection, error) {
	alias, err := ResolveAlias(flag)
	if err != nil {
		return "", Connection{}, err
	}
	conn, ok := GetConnection(alias)
	if !ok {
		return "", Connection{}, UnknownConnectionError(alias)
	}
	return alias, conn, nil
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

const addConnectionHint = "Add one with: agent-mongo connection add <alias> <connection-string>"

func unknownConnectionError(alias string, cfg Config) error {
	return out.New(
		fmt.Sprintf("Connection %q not found. Available: %s",
			alias, JoinOrNone(slices.Sorted(maps.Keys(cfg.Connections)))),
		out.FixableByAgent).WithHint(addConnectionHint)
}

func RemoveConnection(alias string) error {
	return Update(func(cfg *Config) error {
		if _, ok := cfg.Connections[alias]; !ok {
			return unknownConnectionError(alias, *cfg)
		}
		delete(cfg.Connections, alias)
		if cfg.DefaultConnection == alias {
			cfg.DefaultConnection = ""
			if remaining := slices.Sorted(maps.Keys(cfg.Connections)); len(remaining) > 0 {
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
