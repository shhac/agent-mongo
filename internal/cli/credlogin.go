package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	out "github.com/shhac/lib-agent-output"

	"github.com/shhac/agent-mongo/internal/cli/shared"
	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/mongo"
	"github.com/shhac/agent-mongo/internal/output"
)

// newCredentialLoginCommand builds `credential login` here rather than in the
// credential package, so that package stays free of driver dependencies — the
// same reason `connection test` lives here.
func newCredentialLoginCommand(globals func() *shared.GlobalFlags) *cobra.Command {
	var connectionAlias string

	cmd := &cobra.Command{
		Use:   "login <credential>",
		Short: "Log in an OIDC credential against its deployment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := args[0]

			conn, connAlias, err := connectionForLogin(alias, connectionAlias)
			if err != nil {
				return err
			}
			// The same endpoint rules a query is held to: a login carries a
			// token back, so it must not happen over plaintext or against a
			// host this credential is not allowed to talk to.
			if err := credential.CheckConnection(alias, conn.ConnectionString); err != nil {
				return err
			}

			// The prompt is a notice on stderr rather than a record on stdout:
			// it is not the command's result, and an agent relaying it to a
			// person needs it before the command finishes.
			session, err := mongo.DeviceLogin(cmd.Context(), conn, func(p credential.DevicePrompt) {
				out.WriteNotice(cmd.ErrOrStderr(), promptText(p), "")
			})
			if err != nil {
				return err
			}
			if err := credential.SaveSession(alias, session); err != nil {
				return err
			}

			result := map[string]any{
				"ok":         true,
				"credential": alias,
				"connection": connAlias,
				"host":       session.Host,
				"issuer":     session.Issuer,
				"storage":    credential.StorageType(credential.All()[alias]),
			}
			if !session.ExpiresAt.IsZero() {
				result["expiresAt"] = session.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z")
			}
			return output.PrintRaw(result)
		},
	}

	cmd.Flags().StringVar(&connectionAlias, "connection", "",
		"Connection to log in against (default: the one connection using this credential)")
	_ = globals
	return cmd
}

func promptText(p credential.DevicePrompt) string {
	if p.VerificationURIComplete != "" {
		return fmt.Sprintf("To finish signing in, open %s and confirm the code %s",
			p.VerificationURIComplete, p.UserCode)
	}
	return fmt.Sprintf("To finish signing in, open %s and enter the code %s",
		p.VerificationURI, p.UserCode)
}

// connectionForLogin decides which deployment to log in against.
//
// A login needs a connection because the server is what names the identity
// provider. When exactly one connection uses the credential the answer is
// obvious and asking would be noise; when several do, guessing would bind the
// session to the wrong host, so the choice is handed back with the list.
func connectionForLogin(credAlias, requested string) (config.Connection, string, error) {
	if requested != "" {
		conn, ok := config.GetConnection(requested)
		if !ok {
			return config.Connection{}, "", config.UnknownConnectionError(requested)
		}
		return conn, requested, nil
	}

	using := credential.ConnectionsUsing(credAlias)
	switch len(using) {
	case 1:
		conn, _ := config.GetConnection(using[0])
		return conn, using[0], nil
	case 0:
		available := config.ConnectionAliases()
		sort.Strings(available)
		return config.Connection{}, "", out.New(
			fmt.Sprintf("No connection uses credential %q, so there is no deployment to log in against", credAlias),
			out.FixableByAgent,
		).WithHint(fmt.Sprintf(
			"Attach it to a connection, or name one: agent-mongo credential login %s --connection <alias>. Available: %s",
			credAlias, config.JoinOrNone(available)))
	default:
		return config.Connection{}, "", out.New(
			fmt.Sprintf("Credential %q is used by several connections, which may authenticate against different deployments", credAlias),
			out.FixableByAgent,
		).WithHint(fmt.Sprintf(
			"Name the one to log in against: agent-mongo credential login %s --connection <alias>. Using it: %s",
			credAlias, config.JoinOrNone(using)))
	}
}
