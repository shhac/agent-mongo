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
	Globals *GlobalFlags
}

// WithSession connects using the global flags, runs fn, and closes the
// session. Errors are enhanced with fixable_by classification and hints.
func WithSession(g *GlobalFlags, fn func(SessionCtx) error) error {
	return WithSessionRef(g, mongo.Ref{}, fn)
}

// WithSessionRef is WithSession with database/collection context for
// timeout-error hints (index suggestions).
func WithSessionRef(g *GlobalFlags, ref mongo.Ref, fn func(SessionCtx) error) error {
	ctx, cancel := g.MakeContext()
	defer cancel()

	session, err := mongo.Connect(mongo.ConnectOpts{
		AliasFlag: g.Connection,
		Timeout:   g.Timeout(),
	})
	if err != nil {
		return enhance(err, g, ref)
	}
	defer session.Close(context.Background())

	return enhance(fn(SessionCtx{Ctx: ctx, Session: session, Globals: g}), g, ref)
}

func enhance(err error, g *GlobalFlags, ref mongo.Ref) error {
	if err == nil {
		return nil
	}
	return errors.Enhance(err, errors.Context{
		Database:   ref.DB,
		Collection: ref.Collection,
		TimeoutMS:  int(g.Timeout().Milliseconds()),
	})
}
