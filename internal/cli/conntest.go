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

// newConnectionTestCommand builds `connection test`. It lives here for the
// reason given in this package's doc comment: it needs the driver.
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
				if credAlias, expiry, ok := sessionExpiry(ctx.Session.Alias); ok {
					receipt["credential"] = credAlias
					receipt["sessionExpiresAt"] = shared.FormatExpiry(expiry)
				}
				return output.PrintRaw(receipt)
			})
		},
	}
}

// sessionExpiry reports when a connection's credential session runs out, when
// it has one. Pure lookup, so what it decides is testable without a deployment
// to ping.
func sessionExpiry(connAlias string) (credAlias string, expiry time.Time, ok bool) {
	conn, found := config.GetConnection(connAlias)
	if !found || conn.Credential == "" {
		return "", time.Time{}, false
	}
	entry, found := config.Read().Credentials[conn.Credential]
	if !found {
		return "", time.Time{}, false
	}
	info := credential.DescribeSession(conn.Credential, entry)
	if !info.LoggedIn || info.ExpiresAt.IsZero() {
		return "", time.Time{}, false
	}
	return conn.Credential, info.ExpiresAt, true
}
