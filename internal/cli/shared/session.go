package shared

import (
	"context"

	"github.com/shhac/agent-mongo/internal/errors"
	"github.com/shhac/agent-mongo/internal/mongo"
)

// SessionCtx bundles the per-command context and connected session.
type SessionCtx struct {
	Ctx     context.Context
	Session *mongo.Session
}

// WithSession connects using the global flags, runs fn, and closes the
// session. Errors are enhanced with fixable_by classification and hints; ref
// is the database/collection a command targets, which a timeout hint names
// (mongo.Ref{} when there is none).
//
// The connection is established before the command's deadline starts, under a
// budget of its own. Otherwise DNS, TLS and authentication would spend the
// query's time — shortening the maxTimeMS the server is given, failing a short
// --timeout before any query ran, and reporting an unreachable server as a
// query that needs an index.
func WithSession(g *GlobalFlags, ref mongo.Ref, fn func(SessionCtx) error) error {
	session, err := mongo.Connect(mongo.ConnectOpts{
		AliasFlag: g.Connection,
		Timeout:   g.Timeout(),
		AppName:   mongo.AppName(g.Version),
		Comment:   g.Command,
	})
	if err != nil {
		return enhance(err, g, ref, true)
	}
	defer session.Close()

	if err := session.Ping(g.Timeout()); err != nil {
		return enhance(err, g, ref, true)
	}

	ctx, cancel := g.MakeContext()
	defer cancel()
	return enhance(fn(SessionCtx{Ctx: ctx, Session: session}), g, ref, false)
}

func enhance(err error, g *GlobalFlags, ref mongo.Ref, connecting bool) error {
	if err == nil {
		return nil
	}
	return errors.Enhance(err, errors.Context{
		Database:   ref.DB,
		Collection: ref.Collection,
		TimeoutMS:  int(g.Timeout().Milliseconds()),
		Connecting: connecting,
	})
}
