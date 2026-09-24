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

// A credential error raised inside the driver's OIDC callback ("run credential
// login") reaches here wrapped in the driver's connection error. It must keep
// its own advice, not be relabelled a generic authentication failure by the
// "auth error" text the driver puts around it.
func TestEnhanceKeepsAContractErrorTheDriverWrapped(t *testing.T) {
	inner := out.New("Credential \"corp\" is not logged in", out.FixableByHuman).
		WithHint("agent-mongo credential login corp")
	wrapped := topology.ConnectionError{Wrapped: fmt.Errorf("auth error: %w", inner)}

	var got *out.Error
	if !out.As(Enhance(wrapped, Context{Connecting: true}), &got) {
		t.Fatal("not a contract error")
	}
	if got.Hint != inner.Hint || got.FixableBy != out.FixableByHuman {
		t.Errorf("got %s / %q, want the callback's own advice", got.FixableBy, got.Hint)
	}
}

func TestEnhanceRecognisesADriverAuthFailureByItsText(t *testing.T) {
	err := stderrors.New("auth error: sasl conversation error: unable to authenticate using mechanism \"SCRAM-SHA-256\"")
	var got *out.Error
	if !out.As(Enhance(err, Context{Connecting: true}), &got) || got.FixableBy != out.FixableByHuman ||
		!strings.Contains(got.Hint, "credential list") {
		t.Errorf("got %+v", got)
	}
}
