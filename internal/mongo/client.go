// Package mongo is the driver-facing domain layer: connection resolution,
// database/collection discovery, schema inference, and read-only queries.
package mongo

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	driver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
)

// Session bundles a connected client with its resolved connection metadata.
type Session struct {
	Client *driver.Client
	Alias  string
	// DBName is the connection's configured database ("" when the URI has
	// none); commands take explicit database arguments so this is advisory.
	DBName string
}

func (s *Session) Close(ctx context.Context) {
	_ = s.Client.Disconnect(ctx)
}

// ConnectOpts carries the connection-relevant globals.
type ConnectOpts struct {
	// AliasFlag is the -c/--connection value ("" = resolve env/default).
	AliasFlag string
	// Timeout is the effective operation timeout (CSOT, drives maxTimeMS).
	Timeout time.Duration
}

func availableConnections() string {
	list := strings.Join(config.ConnectionAliases(), ", ")
	if list == "" {
		list = "(none)"
	}
	return list
}

// ResolveAlias applies the TS resolution order:
// -c flag > AGENT_MONGO_CONNECTION env > config default > error.
func ResolveAlias(flag string) (string, error) {
	if trimmed := strings.TrimSpace(flag); trimmed != "" {
		return trimmed, nil
	}
	if env := strings.TrimSpace(os.Getenv("AGENT_MONGO_CONNECTION")); env != "" {
		return env, nil
	}
	if def := config.DefaultConnectionAlias(); def != "" {
		return def, nil
	}
	return "", fmt.Errorf(
		"No connection specified. Use -c <alias> or set a default. Available: %s. Run: agent-mongo connection add <alias> <connection-string>",
		availableConnections())
}

// Connect resolves the alias, builds client options (CLI-friendly pool
// settings, optional named credential), and connects.
func Connect(opts ConnectOpts) (*Session, error) {
	alias, err := ResolveAlias(opts.AliasFlag)
	if err != nil {
		return nil, err
	}
	conn, ok := config.GetConnection(alias)
	if !ok {
		return nil, fmt.Errorf(
			"Connection %q not found. Available: %s. Run: agent-mongo connection add <alias> <connection-string>",
			alias, availableConnections())
	}

	clientOpts := options.Client().
		ApplyURI(conn.ConnectionString).
		SetMaxPoolSize(1).
		SetMinPoolSize(0).
		SetMaxConnIdleTime(5 * time.Second).
		SetServerSelectionTimeout(10 * time.Second)
	if opts.Timeout > 0 {
		clientOpts = clientOpts.SetTimeout(opts.Timeout).SetConnectTimeout(opts.Timeout)
	}

	if conn.Credential != "" {
		cred, ok := credential.Get(conn.Credential)
		if !ok {
			available := strings.Join(credential.Aliases(), ", ")
			if available == "" {
				available = "(none)"
			}
			return nil, fmt.Errorf(
				"Credential %q not found. Available: %s. Run: agent-mongo credential add <alias> --username <user> --password <pass>",
				conn.Credential, available)
		}
		clientOpts = clientOpts.SetAuth(options.Credential{
			Username: cred.Username,
			Password: cred.Password,
		})
	}

	client, err := driver.Connect(clientOpts)
	if err != nil {
		return nil, err
	}

	dbName := conn.Database
	if dbName == "" {
		dbName = ParseDBFromURI(conn.ConnectionString)
	}
	return &Session{Client: client, Alias: alias, DBName: dbName}, nil
}
