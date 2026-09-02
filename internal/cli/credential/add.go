package credential

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/shhac/lib-agent-cli/creds"
	out "github.com/shhac/lib-agent-output"

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

// scramFlagNames and oidcFlagNames partition the flag surface by kind. The
// kind is derived from which of them were actually given, so a flag belonging
// to the other kind is a named error rather than silently ignored.
var (
	scramFlagNames = []string{"username", "password", "form"}
	oidcFlagNames  = []string{"environment", "token-resource", "client-id", "allowed-hosts"}
)

func registerAdd(parent *cobra.Command) {
	var flags addFlags

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add or update a stored credential",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kind, err := kindFromFlags(cmd, flags.oidc)
			if err != nil {
				return err
			}
			if kind == config.KindOIDC {
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

	parent.AddCommand(cmd)
}

// kindFromFlags decides which kind is being added.
//
// Pairwise exclusion rules were the obvious way to express this and got it
// wrong twice: the cross product missed --password against the OIDC flags, and
// an OIDC flag given without --oidc was accepted and then ignored, so
// `credential add x --environment k8s` complained about a missing password.
// Deriving the kind from the flags actually given makes both a named error and
// keeps a new flow to one entry in a list.
func kindFromFlags(cmd *cobra.Command, oidc bool) (config.Kind, error) {
	scramGiven := changedFlags(cmd, scramFlagNames)
	oidcGiven := changedFlags(cmd, oidcFlagNames)

	switch {
	case oidc && len(scramGiven) > 0:
		return "", out.New(
			fmt.Sprintf("--oidc cannot be combined with %s: an OIDC credential has no username or password",
				strings.Join(scramGiven, ", ")),
			out.FixableByAgent,
		).WithHint("Drop " + strings.Join(scramGiven, ", ") + ", or drop --oidc.")
	case oidc:
		return config.KindOIDC, nil
	case len(oidcGiven) > 0:
		return "", out.New(
			fmt.Sprintf("%s only applies to an OIDC credential", strings.Join(oidcGiven, ", ")),
			out.FixableByAgent,
		).WithHint("Add --oidc, or drop " + strings.Join(oidcGiven, ", ") + ".")
	default:
		return config.KindSCRAM, nil
	}
}

func changedFlags(cmd *cobra.Command, names []string) []string {
	var given []string
	for _, name := range names {
		if cmd.Flags().Changed(name) {
			given = append(given, "--"+name)
		}
	}
	return given
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
