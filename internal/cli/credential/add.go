package credential

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/config"
	credstore "github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/output"
)

func registerAdd(parent *cobra.Command) {
	var username, password string
	var form bool

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add or update a stored credential",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if form {
				filledUser, filledPass, err := promptMissingViaDialog(cmd.Context(), name, username, password)
				if err != nil {
					return err
				}
				username, password = filledUser, filledPass
			}

			if username == "" || password == "" {
				return errors.New(
					"Missing --username and/or --password. Pass them on the command line, or use --form for an OS dialog.")
			}

			storage, err := credstore.Store(name, config.Credential{
				Username: username,
				Password: password,
			})
			if err != nil {
				return err
			}

			return output.PrintRaw(map[string]any{
				"ok":         true,
				"credential": name,
				"username":   username,
				"storage":    storage,
				"hint":       "Use with: agent-mongo connection add <alias> <uri> --credential " + name,
			})
		},
	}

	cmd.Flags().StringVar(&username, "username", "", "MongoDB username")
	cmd.Flags().StringVar(&password, "password", "", "MongoDB password")
	cmd.Flags().BoolVar(&form, "form", false,
		"Prompt for missing username/password via a native OS dialog (LLM-safe; the secret is typed directly into the OS, never seen by the agent)")
	parent.AddCommand(cmd)
}
