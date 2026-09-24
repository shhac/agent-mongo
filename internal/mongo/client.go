// Package mongo is the driver-facing domain layer: connection resolution,
// database/collection discovery, schema inference, and read-only queries.
package mongo

import (
	"context"
	"time"

	driver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/mongouri"
)

// Session bundles a connected client with its resolved connection metadata.
type Session struct {
	Client *driver.Client
	Alias  string
	// DBName is the connection's configured database ("" when the URI has
	// none); commands take explicit database arguments so this is advisory.
	DBName string

	// comment tags every data command sent (see tag).
	comment string
	// readPref and readConcern are the connection string's, kept for the
	// commands sent through RunCommand, which would otherwise ignore them.
	readPref    *readpref.ReadPref
	readConcern string
}

// closeTimeout bounds Disconnect, which ends the server sessions this process
// opened. That is housekeeping the server does anyway on its own timer, so it
// must not hold a finished command's exit for the full query timeout when the
// network has gone.
const closeTimeout = 2 * time.Second

func (s *Session) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
	defer cancel()
	_ = s.Client.Disconnect(ctx)
}

// AppName is how agent-mongo introduces itself in the connection handshake.
func AppName(version string) string { return "agent-mongo/" + version }

// Ping establishes and authenticates the connection, within timeout.
func (s *Session) Ping(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.Client.Ping(ctx, nil) // nil: the connection string's read preference
}

// ConnectOpts carries the connection-relevant globals.
type ConnectOpts struct {
	// AliasFlag is the -c/--connection value ("" = resolve env/default).
	AliasFlag string
	// Timeout is the client-level operation timeout. Commands run under a
	// context deadline, which takes precedence and drives maxTimeMS; this
	// covers anything issued without one.
	Timeout time.Duration
	// AppName is reported in the connection handshake, unless the connection
	// string names one.
	AppName string
	// Comment is attached to every data command.
	Comment string
}

// baseClientOptions is the pool shape every agent-mongo connection uses: a
// short-lived, single-shot CLI process wants exactly one pooled connection and
// no warm minimum, because anything larger only slows process exit.
//
// Shared with the login path, which needs the same policy and would otherwise
// restate the numbers with nothing keeping them in step.
//
// appName shows in the server log and currentOp against every connection, and
// is kept out of the way of one the connection string already sets.
func baseClientOptions(uri, appName string) *options.ClientOptions {
	opts := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(1).
		SetMinPoolSize(0).
		SetServerSelectionTimeout(10 * time.Second)
	if opts.AppName == nil && appName != "" {
		opts.SetAppName(appName)
	}
	return opts
}

// clientOptions adds what a query needs on top: a short idle lifetime, the
// operation timeout, and the connection's credential.
func clientOptions(
	conn config.Connection, opts ConnectOpts,
) (*options.ClientOptions, error) {
	clientOpts := baseClientOptions(conn.ConnectionString, opts.AppName).
		SetMaxConnIdleTime(5 * time.Second)
	if opts.Timeout > 0 {
		clientOpts = clientOpts.SetTimeout(opts.Timeout).SetConnectTimeout(opts.Timeout)
	}

	if conn.Credential == "" {
		return clientOpts, nil
	}
	res, err := credential.Resolve(conn.Credential)
	if err != nil {
		return nil, err
	}
	// Re-checked here rather than trusted from when the connection was wired
	// up: the connection string can be edited afterwards, and for a kind that
	// authenticates with a bearer token this is a safety boundary. Asked of the
	// resolution rather than the alias, so the check and the auth mapping
	// provably see the same credential.
	if err := res.CheckConnection(conn.ConnectionString); err != nil {
		return nil, err
	}
	// Before connecting, so a credential nobody has logged in with says so
	// rather than surfacing as a connection failure that hides it.
	if err := res.RequireSession(); err != nil {
		return nil, err
	}
	return applyAuth(clientOpts, conn, res)
}

// Connect resolves the alias, builds client options (CLI-friendly pool
// settings, optional named credential), and connects.
func Connect(opts ConnectOpts) (*Session, error) {
	alias, conn, err := config.ResolveConnection(opts.AliasFlag)
	if err != nil {
		return nil, err
	}

	clientOpts, err := clientOptions(conn, opts)
	if err != nil {
		return nil, err
	}

	client, err := driver.Connect(clientOpts)
	if err != nil {
		return nil, err
	}

	dbName := conn.Database
	if dbName == "" {
		dbName = mongouri.ParseDBFromURI(conn.ConnectionString)
	}
	session := &Session{
		Client:   client,
		Alias:    alias,
		DBName:   dbName,
		comment:  opts.Comment,
		readPref: clientOpts.ReadPreference,
	}
	if clientOpts.ReadConcern != nil {
		session.readConcern = clientOpts.ReadConcern.Level
	}
	return session, nil
}
