package connection

import (
	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/output"
)

func registerUpdate(parent *cobra.Command) {
	var credentialAlias, database string
	var clearCredential bool

	cmd := &cobra.Command{
		Use:   "update <alias>",
		Short: "Update a saved connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			alias := args[0]

			if credentialAlias != "" {
				if err := credential.RequireExists(credentialAlias); err != nil {
					return err
				}
			}

			var updates config.ConnectionUpdates
			var updated []string
			if database != "" {
				updates.Database = &database
				updated = append(updated, "database")
			}
			if clearCredential {
				empty := ""
				updates.Credential = &empty
				updated = append(updated, "credential")
			} else if credentialAlias != "" {
				updates.Credential = &credentialAlias
				updated = append(updated, "credential")
			}

			if err := config.UpdateConnection(alias, updates); err != nil {
				return err
			}
			return output.PrintRaw(map[string]any{"ok": true, "alias": alias, "updated": updated})
		},
	}

	cmd.Flags().StringVar(&credentialAlias, "credential", "", "Credential alias for authentication")
	cmd.Flags().BoolVar(&clearCredential, "clear-credential", false, "Remove credential from connection")
	cmd.Flags().StringVar(&database, "database", "", "Override database name")
	cmd.MarkFlagsMutuallyExclusive("credential", "clear-credential")
	parent.AddCommand(cmd)
}
