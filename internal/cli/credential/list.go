package credential

import (
	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/config"
	credstore "github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/output"
)

func registerList(parent *cobra.Command) {
	parent.AddCommand(&cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List stored credentials (passwords redacted)",
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			entries := credstore.All()
			items := make([]any, 0, len(entries))
			for _, name := range credstore.Aliases() {
				items = append(items, listItem(name, entries[name]))
			}
			return output.PrintList(items, nil)
		},
	})
}

// listItem renders one `credential list` row. Username and password appear
// only for the kind that has them, so a row never implies a credential holds
// material it does not.
func listItem(name string, entry config.Credential) map[string]any {
	kind := entry.ResolvedKind()
	item := map[string]any{
		"name":    name,
		"kind":    string(kind),
		"storage": credstore.StorageType(entry),
		"usedBy":  credstore.ConnectionsUsing(name),
	}
	switch kind {
	case config.KindSCRAM:
		username := entry.Username
		if username == credstore.Sentinel {
			username = "(keychain)"
		}
		item["username"] = username
		item["password"] = "***"
	case config.KindOIDC:
		if entry.Flow != nil {
			item["flow"] = string(entry.Flow.Type)
			if entry.Flow.Environment != "" {
				item["environment"] = entry.Flow.Environment
			}
			if entry.Flow.Path != "" {
				item["path"] = entry.Flow.Path
			}
			if len(entry.Flow.AllowedHosts) > 0 {
				item["allowedHosts"] = entry.Flow.AllowedHosts
			}
		}
	}
	return item
}
