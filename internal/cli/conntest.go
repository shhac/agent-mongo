package cli

import (
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/shhac/agent-mongo/internal/cli/shared"
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
				return output.PrintRaw(map[string]any{
					"ok":    true,
					"alias": ctx.Session.Alias,
					"ping":  serialize.Document(result),
				})
			})
		},
	}
}
