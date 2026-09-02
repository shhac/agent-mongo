package connection

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	out "github.com/shhac/lib-agent-output"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/mongouri"
	"github.com/shhac/agent-mongo/internal/output"
)

// credentialResolution is the outcome of deciding how a new connection
// authenticates: credentials extracted from the URI, a referenced stored
// credential, or none.
type credentialResolution struct {
	ConnectionString string // URI with any embedded userinfo stripped
	Alias            string // credential alias the connection references ("" = none)
	Created          bool   // a credential was extracted from the URI and stored
	Storage          string // where the extracted credential landed (credential.Storage*)
}

// resolveCredential moves a user:pass embedded in the URI into a stored
// credential named after the connection alias; combining embedded credentials
// with --credential is ambiguous and rejected.
func resolveCredential(alias, connectionString, credentialAlias string) (credentialResolution, error) {
	username, password, stripped, hasEmbedded := mongouri.SplitURICredentials(connectionString)
	switch {
	case hasEmbedded && credentialAlias != "":
		return credentialResolution{}, fmt.Errorf(
			"Connection string embeds a username/password and --credential %q was also given. Drop the credentials from the URI or the --credential flag.",
			credentialAlias)
	case hasEmbedded:
		if err := refuseCredentialOverwrite(alias, username, password, stripped); err != nil {
			return credentialResolution{}, err
		}
		storage, err := credential.Store(alias, config.Credential{
			Username: username,
			Password: password,
		})
		if err != nil {
			return credentialResolution{}, err
		}
		return credentialResolution{
			ConnectionString: stripped,
			Alias:            alias,
			Created:          true,
			Storage:          storage,
		}, nil
	case credentialAlias != "":
		if err := credential.RequireExists(credentialAlias); err != nil {
			return credentialResolution{}, err
		}
	}
	return credentialResolution{ConnectionString: connectionString, Alias: credentialAlias}, nil
}

// refuseCredentialOverwrite blocks extraction from clobbering an existing
// credential that holds different (or unresolvable) values — connections
// referencing it would silently switch auth. Re-adding the same values stays
// idempotent.
func refuseCredentialOverwrite(alias, username, password, stripped string) error {
	if _, exists := credential.All()[alias]; !exists {
		return nil
	}
	// Only a SCRAM credential holding exactly these values is a no-op re-add;
	// anything else (a different password, an unreadable secret, another kind
	// entirely) would silently change how the connections referencing it
	// authenticate.
	existing, err := credential.Resolve(alias)
	if err == nil && existing.Kind == config.KindSCRAM &&
		existing.Credential.Username == username &&
		existing.Credential.Password == password {
		return nil
	}

	usedBy := ""
	if used := credential.ConnectionsUsing(alias); len(used) > 0 {
		usedBy = " (used by connections: " + strings.Join(used, ", ") + ")"
	}
	refusal := fmt.Errorf(
		"Credential %q already exists with different values%s. Refusing to overwrite it: connections referencing it would silently change auth.",
		alias, usedBy)
	return out.Wrap(refusal, out.FixableByAgent).WithHint(overwriteHint(alias, stripped))
}

// overwriteHint tailors the fix to what is actually changing: same host/URI
// means a credential rotation; a different URI means the embedded credentials
// are the mistake.
func overwriteHint(alias, stripped string) string {
	if conn, ok := config.GetConnection(alias); ok && conn.ConnectionString == stripped {
		return fmt.Sprintf(
			"Only the credential is changing. Rotate it explicitly: agent-mongo credential add %s --form (OS dialog keeps the secret out of agent context; or pass --username/--password)",
			alias)
	}
	return fmt.Sprintf(
		"Drop the username/password from the URI and pass --credential %s to keep the stored credential, or remove it first: agent-mongo credential remove %s",
		alias, alias)
}

func registerAdd(parent *cobra.Command) {
	var database, credentialAlias string
	var setDefault bool

	cmd := &cobra.Command{
		Use:   "add <alias> <connection-string>",
		Short: "Add a MongoDB connection",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			alias := args[0]

			resolved, err := resolveCredential(alias, args[1], credentialAlias)
			if err != nil {
				return err
			}

			err = config.StoreConnection(alias, config.Connection{
				ConnectionString: resolved.ConnectionString,
				Name:             alias,
				Database:         database,
				Credential:       resolved.Alias,
			})
			if err != nil {
				return err
			}

			if setDefault {
				if err := config.SetDefaultConnection(alias); err != nil {
					return err
				}
			}

			resolvedDB := database
			if resolvedDB == "" {
				resolvedDB = mongouri.ParseDBFromURI(resolved.ConnectionString)
			}
			result := map[string]any{
				"ok":         true,
				"alias":      alias,
				"database":   resolvedDB,
				"credential": resolved.Alias,
				"isDefault":  setDefault,
				"hint":       "Test with: agent-mongo connection test " + alias,
			}
			if resolved.Created {
				result["credentialCreated"] = true
				result["credentialStorage"] = resolved.Storage
				result["notice"] = fmt.Sprintf(
					"Embedded username/password moved out of the connection string into credential %q", alias)
			}
			return output.PrintRaw(result)
		},
	}

	cmd.Flags().StringVar(&database, "database", "", "Override database name from URI")
	cmd.Flags().StringVar(&credentialAlias, "credential", "", "Credential alias for authentication")
	cmd.Flags().BoolVar(&setDefault, "default", false, "Set as default connection")
	parent.AddCommand(cmd)
}
