package credential

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/shhac/lib-agent-cli/creds"

	"github.com/shhac/agent-mongo/internal/config"
	credstore "github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/output"
)

func addSCRAM(cmd *cobra.Command, name string, flags addFlags) error {
	username, password := flags.username, flags.password

	if flags.form {
		filledUser, filledPass, err := promptMissingViaDialog(cmd.Context(), name, username, password)
		if err != nil {
			return err
		}
		username, password = filledUser, filledPass
	}

	// Non-interactive secure path: keep the secret off argv by piping it
	// on stdin. Precedence: --password flag > piped stdin > --form.
	password, err := creds.ReadSecret(cmd.InOrStdin(), password)
	if err != nil {
		return err
	}

	if username == "" || password == "" {
		return errors.New(
			"Missing username and/or password. Provide --username, and supply the password via --form (OS dialog), by piping it on stdin, or with --password. For identity-provider auth instead, use --oidc.")
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
		"kind":       string(config.KindSCRAM),
		"username":   username,
		"storage":    storage,
		"hint":       "Use with: agent-mongo connection add <alias> <uri> --credential " + name,
	})
}
