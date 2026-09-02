package credential

import (
	"github.com/spf13/cobra"

	credstore "github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/output"
)

// registerLogout adds `credential logout`: end the session, keep the
// credential.
//
// Separate from `credential remove` because they answer different questions.
// Removing a credential unpicks configuration that connections still reference;
// ending an access window is a routine thing to want, and before this the only
// way to do it was to delete the credential and put it back afterwards.
func registerLogout(parent *cobra.Command) {
	parent.AddCommand(&cobra.Command{
		Use:   "logout <name>",
		Short: "End an OIDC credential's session without removing it",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]
			if err := credstore.ClearSession(name); err != nil {
				return err
			}
			return output.PrintRaw(map[string]any{
				"ok":         true,
				"credential": name,
				"loggedOut":  true,
				"hint":       "Log in again with: agent-mongo credential login " + name,
			})
		},
	})
}
