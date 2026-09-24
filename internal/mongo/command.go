package mongo

import (
	"context"
	"slices"

	"go.mongodb.org/mongo-driver/v2/bson"
	driver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// runCursor sends a cursor-returning command (find, aggregate) and drains it.
//
// It exists because Collection.Find and Collection.Aggregate never send
// maxTimeMS: the driver omits it from any command returning a cursor the
// caller iterates (DRIVERS-2722), and v2 removed the per-call override. Sent
// without it, a query the CLI has given up on runs on the server to
// completion. RunCommandCursor has no such exemption, so the driver appends a
// maxTimeMS derived from the context deadline, and the server applies it
// across the cursor's getMores too. The command must not carry its own.
//
// RunCommand defaults to the primary and to no read concern, where the
// collection helpers would have used the connection string's, so both are
// forwarded here. What is not replaced is the collection helpers' retry of a
// read interrupted by a failover: such a query fails and is run again by
// whoever asked for it.
func (s *Session) runCursor(ctx context.Context, db string, cmd bson.D) ([]bson.D, error) {
	if s.readConcern != "" {
		cmd = append(slices.Clip(cmd),
			bson.E{Key: "readConcern", Value: bson.D{{Key: "level", Value: s.readConcern}}})
	}
	opts := options.RunCmd()
	if s.readPref != nil {
		opts.SetReadPreference(s.readPref)
	}
	cursor, err := s.Client.Database(db).RunCommandCursor(ctx, s.tag(cmd), opts)
	if err != nil {
		return nil, err
	}
	defer closeCursor(cursor)
	if s.comment != "" {
		cursor.SetComment(s.comment) // the getMores are not tagged otherwise
	}

	var docs []bson.D
	for cursor.Next(ctx) {
		var doc bson.D
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, cursor.Err()
}

// closeCursor kills a cursor left open by a failed getMore, within the same
// bound as closing the session. Cursor.All would close it with no deadline at
// all, holding the process on a server that has stopped answering.
func closeCursor(cursor *driver.Cursor) {
	ctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
	defer cancel()
	_ = cursor.Close(ctx)
}

// tag names the agent-mongo command a server command came from, so it can be
// told apart from the application's own in the server log, the profiler and
// currentOp.
func (s *Session) tag(cmd bson.D) bson.D {
	if s.comment == "" {
		return cmd
	}
	return append(slices.Clip(cmd), bson.E{Key: "comment", Value: s.comment})
}

// commented is tag for the driver's collection helpers, which take the comment
// as an option.
func commented[T interface{ SetComment(any) T }](opts T, comment string) T {
	if comment == "" {
		return opts
	}
	return opts.SetComment(comment)
}

// findCommand is the find FindDocuments sends: one more document than the
// limit, so a full page can tell whether another follows, and all of them in
// the first batch so no getMore is needed.
func findCommand(opts FindOpts) bson.D {
	fetch := int64(opts.Limit + 1)
	cmd := bson.D{
		{Key: "find", Value: opts.Collection},
		{Key: "filter", Value: orEmpty(opts.Filter)},
	}
	if opts.Sort != nil {
		cmd = append(cmd, bson.E{Key: "sort", Value: opts.Sort})
	}
	if opts.Projection != nil {
		cmd = append(cmd, bson.E{Key: "projection", Value: opts.Projection})
	}
	if opts.Skip > 0 {
		cmd = append(cmd, bson.E{Key: "skip", Value: int64(opts.Skip)})
	}
	return append(cmd,
		bson.E{Key: "limit", Value: fetch},
		bson.E{Key: "batchSize", Value: fetch},
	)
}

// aggregateCommand wraps a pipeline whose result size is already bounded,
// asking for the whole result in the first batch. The batch is one larger than
// the most the pipeline can produce: a batch it exactly fills leaves the server
// unsure the cursor is exhausted, costing a getMore to find out.
func aggregateCommand(collection string, pipeline bson.A, expected int) bson.D {
	cursor := bson.D{}
	if expected > 0 {
		cursor = bson.D{{Key: "batchSize", Value: int64(expected + 1)}}
	}
	return bson.D{
		{Key: "aggregate", Value: collection},
		{Key: "pipeline", Value: pipeline},
		{Key: "cursor", Value: cursor},
	}
}
