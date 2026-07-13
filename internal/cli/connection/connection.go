// Package connection implements `agent-mongo connection` — saved connection
// management. The `test` subcommand is registered from the mongo-backed layer.
package connection

import (
	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/cli/shared"
	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/mongo"
	"github.com/shhac/agent-mongo/internal/output"
)

// Register attaches the connection group. testCmd is the mongo-backed
// `connection test` leaf, built by the caller to keep this package free of
// driver dependencies.
func Register(root *cobra.Command, testCmd *cobra.Command) {
	cmd := &cobra.Command{Use: "connection", Short: "Manage MongoDB connections"}

	registerAdd(cmd)
	registerRemove(cmd)
	registerUpdate(cmd)
	registerList(cmd)
	if testCmd != nil {
		cmd.AddCommand(testCmd)
	}
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

func registerList(parent *cobra.Command) {
	parent.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List saved connections",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			connections := config.Connections()
			defaultAlias := config.DefaultConnectionAlias()

			items := make([]any, 0, len(connections))
			for _, alias := range config.ConnectionAliases() {
				conn := connections[alias]
				items = append(items, map[string]any{
					"alias":             alias,
					"connection_string": mongo.RedactURI(conn.ConnectionString),
					"database":          conn.Database,
					"credential":        conn.Credential,
					"default":           alias == defaultAlias,
				})
			}
			return output.PrintList(items, nil)
		},
	})
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

RESOLUTION ORDER: -c flag > AGENT_MONGO_CONNECTION env > config default > error

CONFIG: ~/.config/agent-mongo/config.json (respects XDG_CONFIG_HOME)`
