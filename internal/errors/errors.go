// Package errors classifies MongoDB failures into the family error contract
// ({error, fixable_by, hint}) and enhances timeout errors with actionable
// hints (raise the timeout, check indexes).
package errors

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"

	out "github.com/shhac/lib-agent-output"
	driver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/topology"
)

// Context carries the query context used to build hints.
type Context struct {
	Database   string
	Collection string
	TimeoutMS  int
	// Connecting marks an error from establishing the connection, before any
	// query ran: running out of time there is not a slow query.
	Connecting bool
}

const maxTimeExpiredCode = 50

func isTimeout(err error) bool {
	if driver.IsTimeout(err) || stderrors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var cmdErr driver.CommandError
	if stderrors.As(err, &cmdErr) && cmdErr.Code == maxTimeExpiredCode {
		return true
	}
	var srvErr driver.ServerError
	return stderrors.As(err, &srvErr) && srvErr.HasErrorCode(maxTimeExpiredCode)
}

func isAuthError(err error) bool {
	var srvErr driver.ServerError
	if stderrors.As(err, &srvErr) && srvErr.HasErrorCode(18) { // AuthenticationFailed
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "authentication failed") || strings.Contains(msg, "auth error")
}

// isSelectionError reports that no server could be reached. Asked before
// isTimeout: a selection failure wraps the context's deadline error, so the
// driver's IsTimeout reports it as a timeout too, and an unreachable host would
// otherwise be sent off to check its indexes.
func isSelectionError(err error) bool {
	var selErr topology.ServerSelectionError
	return stderrors.As(err, &selErr) || strings.Contains(err.Error(), "server selection error")
}

func unreachable(err error) error {
	return out.Wrap(err, out.FixableByRetry).WithHint(
		"Could not reach the MongoDB server. Check the connection string and network, then retry: agent-mongo connection test")
}

// Enhance wraps a mongo operation error with a fixable_by classification and
// hints. Errors already carrying the contract pass through unchanged.
func Enhance(err error, ectx Context) error {
	if err == nil {
		return nil
	}
	var already *out.Error
	if out.As(err, &already) {
		return err
	}

	switch {
	case isAuthError(err):
		return out.Wrap(err, out.FixableByHuman).WithHint(
			"Authentication failed. Check the connection's credential: agent-mongo credential list")
	case isSelectionError(err):
		return unreachable(err)
	case ectx.Connecting && isTimeout(err):
		return out.Wrap(err, out.FixableByRetry).WithHints(
			fmt.Sprintf("Could not connect within %dms", ectx.TimeoutMS),
			"Check the network, or allow longer with --timeout <ms>, then retry: agent-mongo connection test")
	case isTimeout(err):
		hints := []string{
			fmt.Sprintf("Query timed out after %dms", ectx.TimeoutMS),
			"Increase with: --timeout <ms> or agent-mongo config set query.timeout <ms>",
		}
		if ectx.Database != "" && ectx.Collection != "" {
			hints = append(hints, fmt.Sprintf(
				"Check indexes: agent-mongo collection indexes %s %s", ectx.Database, ectx.Collection))
		}
		return out.Wrap(err, out.FixableByAgent).WithHints(hints...)
	default:
		return out.Wrap(err, out.FixableByAgent)
	}
}
