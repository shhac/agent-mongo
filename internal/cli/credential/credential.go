// Package credential implements `agent-mongo credential` — stored credential
// management with LLM-safe --form secret entry via a native OS dialog.
package credential

import (
	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/cli/shared"
	"github.com/shhac/agent-mongo/internal/config"
	credstore "github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/output"
)

func Register(root *cobra.Command) {
	cmd := &cobra.Command{Use: "credential", Short: "Manage stored credentials"}
	registerAdd(cmd)
	registerRemove(cmd)
	registerList(cmd)
	shared.RegisterUsage(cmd, "credential", usageText)
	root.AddCommand(cmd)
}

func registerRemove(parent *cobra.Command) {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a stored credential",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]
			usedBy := credstore.ConnectionsUsing(name)

			if len(usedBy) > 0 && force {
				empty := ""
				for _, connAlias := range usedBy {
					err := config.UpdateConnection(connAlias, config.ConnectionUpdates{Credential: &empty})
					if err != nil {
						return err
					}
				}
			}

			if err := credstore.Remove(name); err != nil {
				return err
			}

			result := map[string]any{"ok": true, "removed": name}
			if len(usedBy) > 0 && force {
				result["clearedFrom"] = usedBy
			}
			return output.PrintRaw(result)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false,
		"Remove even if referenced by connections (clears their credential refs)")
	parent.AddCommand(cmd)
}

const usageText = `credential — Manage stored credentials for MongoDB authentication

COMMANDS:
  credential add <name> --form   (recommended)
    Store a named credential (overwrites if name already exists), prompting
    for any missing --username / --password via a native OS dialog. The
    agent never sees the secret — input goes directly into the OS dialog.
    Fails with fixable_by=human if no GUI is available (e.g. SSH session);
    fixable_by=retry if the user cancels the dialog.
    Credentials are referenced by connections via --credential.

  credential add <name> --username <user> --password <pass>
    Same, fully non-interactive. Prefer --form: flag values land in shell
    history and agent context.

  credential remove <name>
    Remove a stored credential. Fails if any connection references it.
    --force removes anyway and clears credential refs from those connections.

  credential list
    List all stored credentials (passwords always redacted).
    Shows which connections reference each credential.

WORKFLOW:
  1. Store credential:   agent-mongo credential add acme --form
  2. Add connections:    agent-mongo connection add prod <uri> --credential acme
                         agent-mongo connection add staging <uri> --credential acme
  3. Rotate password:    agent-mongo credential add acme --form
     All connections referencing "acme" pick up the new password automatically.

RESOLUTION: When a connection references a credential, auth is passed to the MongoDB
driver. Connections without a credential use the URI as-is (backward compatible).

KEYCHAIN: Credentials are stored in the OS keychain when available (macOS
  Keychain, Linux Secret Service, Windows Credential Manager); otherwise they
  fall back to plaintext config. ` + "`credential list`" + ` shows the storage
  source ("keychain" or "config") per credential.
  Plaintext credentials (from older versions or keychain-less hosts) are
  upgraded to the keychain automatically the first time they are used on a
  host with a usable keychain (a {"notice": ...} line on stderr reports it).

CONFIG: ~/.config/agent-mongo/config.json (respects XDG_CONFIG_HOME)`
