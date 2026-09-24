package errors

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"
	"testing"

	out "github.com/shhac/lib-agent-output"
	driver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/topology"
)

func TestEnhanceClassifies(t *testing.T) {
	query := Context{Database: "app", Collection: "orders", TimeoutMS: 1000}
	connecting := Context{TimeoutMS: 1000, Connecting: true}
	selection := topology.ServerSelectionError{Wrapped: context.DeadlineExceeded}

	tests := []struct {
		name      string
		err       error
		ctx       Context
		fixableBy out.FixableBy
		hint      string
	}{
		{
			name: "server-side maxTimeMS expiry", ctx: query,
			err:       driver.CommandError{Code: 50, Message: "operation exceeded time limit"},
			fixableBy: out.FixableByAgent, hint: "collection indexes app orders",
		},
		{
			name: "client deadline during a query", ctx: query,
			err:       fmt.Errorf("reading reply: %w", context.DeadlineExceeded),
			fixableBy: out.FixableByAgent, hint: "Query timed out after 1000ms",
		},
		{
			// Wraps DeadlineExceeded, so the driver calls it a timeout; it is
			// an unreachable server, not a slow query.
			name: "server selection timing out", ctx: query,
			err: selection, fixableBy: out.FixableByRetry, hint: "Could not reach",
		},
		{
			name: "server selection while connecting", ctx: connecting,
			err: selection, fixableBy: out.FixableByRetry, hint: "Could not reach",
		},
		{
			name: "deadline while connecting", ctx: connecting,
			err:       fmt.Errorf("handshake: %w", context.DeadlineExceeded),
			fixableBy: out.FixableByRetry, hint: "Could not connect within 1000ms",
		},
		{
			name: "authentication", ctx: connecting,
			err:       driver.CommandError{Code: 18, Message: "Authentication failed."},
			fixableBy: out.FixableByHuman, hint: "credential list",
		},
		{
			name: "anything else", ctx: query,
			err:       stderrors.New("(BadValue) unknown operator: $bogus"),
			fixableBy: out.FixableByAgent,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got *out.Error
			if !out.As(Enhance(tc.err, tc.ctx), &got) {
				t.Fatalf("not a contract error: %v", tc.err)
			}
			if got.FixableBy != tc.fixableBy {
				t.Errorf("fixable_by = %s, want %s", got.FixableBy, tc.fixableBy)
			}
			if !strings.Contains(got.Hint, tc.hint) {
				t.Errorf("hint = %q, want it to mention %q", got.Hint, tc.hint)
			}
		})
	}
}

func TestEnhancePassesContractErrorsThrough(t *testing.T) {
	original := out.New("Connection \"x\" not found", out.FixableByAgent)
	if got := Enhance(original, Context{}); got != original {
		t.Errorf("a classified error was re-wrapped: %v", got)
	}
}
