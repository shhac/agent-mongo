package cli

import (
	"time"

	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/shhac/agent-mongo/internal/cli/shared"
	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/output"
	"github.com/shhac/agent-mongo/internal/serialize"
)

// newConnectionTestCommand builds `connection test` here (rather than in the
// connection package) so that package stays free of driver dependencies.
func newConnectionTestCommand(globals func() *shared.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "test [alias]",
		Short: "Test a MongoDB connection (ping)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			g := globals()
			if len(args) == 1 {
				g.Connection = args[0]
			}
			return shared.WithSession(g, func(ctx shared.SessionCtx) error {
				var result bson.D
				err := ctx.Session.Client.Database("admin").
					RunCommand(ctx.Ctx, bson.D{{Key: "ping", Value: 1}}).
					Decode(&result)
				if err != nil {
					return err
				}
				receipt := map[string]any{
					"ok":    true,
					"alias": ctx.Session.Alias,
					"ping":  serialize.Document(result),
				}
				// A session can be revoked while still unexpired, so the ping
				// is the real check; the expiry is added so a person can see
				// the next login coming rather than being surprised by it.
				addSessionExpiry(receipt, ctx.Session.Alias)
				return output.PrintRaw(receipt)
			})
		},
	}
}

// addSessionExpiry annotates a successful ping with when the credential's
// session runs out, when it has one.
func addSessionExpiry(receipt map[string]any, connAlias string) {
	conn, ok := config.GetConnection(connAlias)
	if !ok || conn.Credential == "" {
		return
	}
	entry, ok := config.Read().Credentials[conn.Credential]
	if !ok {
		return
	}
	info := credential.DescribeSession(conn.Credential, entry)
	if !info.LoggedIn || info.ExpiresAt.IsZero() {
		return
	}
	receipt["credential"] = conn.Credential
	receipt["sessionExpiresAt"] = info.ExpiresAt.UTC().Format(time.RFC3339)
}
