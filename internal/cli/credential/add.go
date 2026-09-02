package credential

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/shhac/lib-agent-cli/creds"

	"github.com/shhac/agent-mongo/internal/config"
	credstore "github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/output"
)

// addFlags is every flag `credential add` accepts, across all kinds. Which
// subset applies is decided by --oidc; cobra enforces that the two sets are not
// mixed.
type addFlags struct {
	username string
	password string
	form     bool

	oidc          bool
	environment   string
	tokenResource string
	clientID      string
	allowedHosts  []string
}

func registerAdd(parent *cobra.Command) {
	var flags addFlags

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add or update a stored credential",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.oidc {
				return addOIDC(args[0], flags)
			}
			return addSCRAM(cmd, args[0], flags)
		},
	}

	cmd.Flags().StringVar(&flags.username, "username", "", "MongoDB username")
	cmd.Flags().StringVar(&flags.password, "password", "", "MongoDB password")
	cmd.Flags().BoolVar(&flags.form, "form", false,
		"Prompt for missing username/password via a native OS dialog (LLM-safe; the secret is typed directly into the OS, never seen by the agent)")

	cmd.Flags().BoolVar(&flags.oidc, "oidc", false,
		"Store an OIDC credential: the identity provider issues short-lived tokens, so no password is stored")
	cmd.Flags().StringVar(&flags.environment, "environment", "",
		"OIDC platform identity to use: k8s, azure, or gcp")
	cmd.Flags().StringVar(&flags.tokenResource, "token-resource", "",
		"OIDC token audience (required for --environment azure and gcp)")
	cmd.Flags().StringVar(&flags.clientID, "client-id", "",
		"OIDC managed-identity client id (azure only)")
	cmd.Flags().StringSliceVar(&flags.allowedHosts, "allowed-hosts", nil,
		"Hosts this credential may send a token to (default: MongoDB-owned domains and loopback)")

	// The two kinds share no flags, so mixing them is always a mistake rather
	// than a precedence question to resolve at runtime.
	for _, scramFlag := range []string{"username", "password", "form"} {
		cmd.MarkFlagsMutuallyExclusive("oidc", scramFlag)
	}
	for _, oidcFlag := range []string{"environment", "token-resource", "client-id", "allowed-hosts"} {
		cmd.MarkFlagsMutuallyExclusive("username", oidcFlag)
		cmd.MarkFlagsMutuallyExclusive("form", oidcFlag)
	}

	parent.AddCommand(cmd)
}

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
