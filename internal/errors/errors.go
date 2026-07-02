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
)

// Context carries the query context used to build hints.
type Context struct {
	Database   string
	Collection string
	TimeoutMS  int
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

func isSelectionError(err error) bool {
	return strings.Contains(err.Error(), "server selection error")
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
	case isAuthError(err):
		return out.Wrap(err, out.FixableByHuman).WithHint(
			"Authentication failed. Check the connection's credential: agent-mongo credential list")
	case isSelectionError(err):
		return out.Wrap(err, out.FixableByRetry).WithHint(
			"Could not reach the MongoDB server. Check the connection string and network, then retry: agent-mongo connection test")
	default:
		return out.Wrap(err, out.FixableByAgent)
	}
}
