// Package connection implements `agent-mongo connection` — saved connection
// management.
package connection

import (
	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/cli/shared"
	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/mongouri"
	"github.com/shhac/agent-mongo/internal/output"
)

// Register attaches the connection group.
func Register(root *cobra.Command, globals func() *shared.GlobalFlags) {
	cmd := &cobra.Command{Use: "connection", Short: "Manage MongoDB connections"}

	registerAdd(cmd)
	registerRemove(cmd)
	registerUpdate(cmd)
	registerList(cmd, globals)
	registerTest(cmd, globals)
	registerSetDefault(cmd)
	shared.RegisterUsage(cmd, "connection", usageText)

	root.AddCommand(cmd)
}

func registerRemove(parent *cobra.Command) {
	parent.AddCommand(&cobra.Command{
		Use:   "remove <alias>",
		Short: "Remove a saved connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := config.RemoveConnection(args[0]); err != nil {
				return err
			}
			return output.PrintRaw(map[string]any{"ok": true, "removed": args[0]})
		},
	})
}

func registerList(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	parent.AddCommand(&cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List saved connections",
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			connections := config.Connections()
			defaultAlias := config.DefaultConnectionAlias()
			aliases, err := visibleAliases(globals().Connection)
			if err != nil {
				return err
			}

			items := make([]any, 0, len(aliases))
			for _, alias := range aliases {
				conn := connections[alias]
				items = append(items, map[string]any{
					"alias":             alias,
					"connection_string": mongouri.RedactURI(conn.ConnectionString),
					"database":          conn.Database,
					"credential":        conn.Credential,
					"default":           alias == defaultAlias,
				})
			}
			return output.PrintList(items, nil)
		},
	})
}

// visibleAliases is every saved connection, or only the pinned one: a principal
// bound to one connection has no business learning the others' hosts.
func visibleAliases(flag string) ([]string, error) {
	pinned, ok, err := config.PinnedConnection(flag)
	if !ok {
		return config.ConnectionAliases(), nil
	}
	if err != nil {
		return nil, err
	}
	if _, found := config.GetConnection(pinned); !found {
		return nil, nil
	}
	return []string{pinned}, nil
}

func registerSetDefault(parent *cobra.Command) {
	parent.AddCommand(&cobra.Command{
		Use:   "set-default <alias>",
		Short: "Set the default connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := config.SetDefaultConnection(args[0]); err != nil {
				return err
			}
			return output.PrintRaw(map[string]any{"ok": true, "default": args[0]})
		},
	})
}

const usageText = `connection — Manage MongoDB connections

COMMANDS:
  connection add <alias> <uri> [--database <db>] [--credential <name>] [--default]
    Save a MongoDB connection. Alias is a short name (e.g. local, staging, prod).
    URI: mongodb://... or mongodb+srv://...
    A user:pass embedded in the URI is moved into a stored credential named
    after the connection alias (mutually exclusive with --credential).
    Refuses if that credential already exists with different values — the
    error's hint says how to rotate (credential add --form) or reuse it.
    --database overrides the database from the URI.
    --credential references a stored credential for authentication.
    --default sets this connection as the default.

  connection update <alias> [--credential <name>] [--clear-credential] [--database <db>]
    Update a saved connection. Only specified fields are changed.
    --credential sets or changes the credential reference.
    --clear-credential removes the credential from the connection (mutually exclusive with --credential).

  connection remove <alias>
    Remove a saved connection.

  connection list
    List all saved connections with credential names. Passwords in
    connection strings are redacted.

  connection test [alias] [-c <alias>]
    Ping MongoDB to verify connectivity. Alias as argument or -c flag. Uses default if omitted.

  connection set-default <alias>
    Set which connection is used when -c is not specified.

CREDENTIALS: Use "credential add" to store reusable auth. Reference via --credential.
  An oidc credential additionally requires the connection to use TLS and to
  point at one of that credential's allowed hosts; both are checked when the
  connection is wired up and again at connect.

RESOLUTION ORDER: -c flag > AGENT_MONGO_CONNECTION env > config default > error

CONFIG: ~/.config/agent-mongo/config.json (respects XDG_CONFIG_HOME)`
