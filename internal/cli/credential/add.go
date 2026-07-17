package credential

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/shhac/lib-agent-cli/creds"

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

			// Non-interactive secure path: keep the secret off argv by piping it
			// on stdin. Precedence: --password flag > piped stdin > --form.
			var err error
			password, err = creds.ReadSecret(cmd.InOrStdin(), password)
			if err != nil {
				return err
			}

			if username == "" || password == "" {
				return errors.New(
					"Missing username and/or password. Provide --username, and supply the password via --form (OS dialog), by piping it on stdin, or with --password.")
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
