// Package config owns ~/.config/agent-mongo/config.json: connections,
// credential entries, and persisted settings. Credentials with keychain-backed
// values hold the "__KEYCHAIN__" sentinel; resolution lives in
// internal/credential.
package config

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shhac/lib-agent-cli/creds"
	"github.com/shhac/lib-agent-cli/xdg"
)

// Kind names the MongoDB authentication mechanism a credential drives. An
// empty Kind reads as KindSCRAM, so configs written before kinds existed keep
// working and are never rewritten just to gain the field.
type Kind string

const KindSCRAM Kind = "scram"

// SupportedKinds lists the kinds this build understands, for self-correcting
// errors when config.json names one it does not.
func SupportedKinds() []string { return []string{string(KindSCRAM)} }

type Credential struct {
	Kind     Kind   `json:"kind,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// ResolvedKind is the only reader of Kind: it supplies the SCRAM default so no
// caller has to remember that an absent kind is not an invalid one.
func (c Credential) ResolvedKind() Kind {
	if c.Kind == "" {
		return KindSCRAM
	}
	return c.Kind
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
// or unparseable — a corrupt file behaves like an empty one rather than
// wedging every command.
func Read() Config {
	var cfg Config
	if err := store().Load(&cfg); err != nil {
		return Config{}
	}
	return cfg
}

// Write replaces the whole config document. It does NOT lock, so it is only
// safe for a write that does not depend on what was already there — anything
// that reads, changes and writes back must go through Update.
func Write(cfg Config) error { return store().Save(cfg) }

// Update applies mutate to a freshly loaded config while holding the store's
// exclusive lock across the read, the mutation and the write.
//
// Without the lock, two concurrent invocations (say `connection add` racing
// `credential add`) each build their write from a snapshot taken before the
// other landed, so all but the last are silently erased. That is worse than an
// ordinary lost write here: a credential entry lost this way leaves its secret
// sitting in the OS keychain with nothing referencing it, so the CLI can no
// longer show or remove it.
//
// This is creds.Store.WithLock rather than creds.Store.Update because Read
// deliberately treats an unparseable file as empty, so a corrupt config can be
// written over rather than wedging every command. Update surfaces the decode
// error and refuses to write, which would make a recoverable corrupt file
// permanent.
//
// Returning an error from mutate aborts without writing, leaving the stored
// document untouched.
func Update(mutate func(cfg *Config) error) error {
	return store().WithLock(func() error {
		cfg := Read()
		if err := mutate(&cfg); err != nil {
			return err
		}
		return store().Save(cfg)
	})
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

// JoinOrNone renders a valid-values list for self-correcting error messages.
func JoinOrNone(values []string) string {
	if len(values) == 0 {
		return "(none)"
	}
	return strings.Join(values, ", ")
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

func ReadSettings() Settings {
	s := Read().Settings
	if s == nil {
		return Settings{}
	}
	return *s
}
