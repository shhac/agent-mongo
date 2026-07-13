package connection

import (
	"fmt"

	"github.com/spf13/cobra"

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
		if err := credential.Require(credentialAlias); err != nil {
			return credentialResolution{}, err
		}
	}
	return credentialResolution{ConnectionString: connectionString, Alias: credentialAlias}, nil
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
