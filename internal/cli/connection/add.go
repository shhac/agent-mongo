package connection

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/mongo"
	"github.com/shhac/agent-mongo/internal/output"
)

func registerAdd(parent *cobra.Command) {
	var database, credentialAlias string
	var setDefault bool

	cmd := &cobra.Command{
		Use:   "add <alias> <connection-string>",
		Short: "Add a MongoDB connection",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			alias, connectionString := args[0], args[1]

			username, password, stripped, hasEmbedded := mongo.SplitURICredentials(connectionString)
			credentialCreated := false
			var credentialStorage string
			switch {
			case hasEmbedded && credentialAlias != "":
				return fmt.Errorf(
					"Connection string embeds a username/password and --credential %q was also given. Drop the credentials from the URI or the --credential flag.",
					credentialAlias)
			case hasEmbedded:
				storage, err := credential.Store(alias, config.Credential{
					Username: username,
					Password: password,
				})
				if err != nil {
					return err
				}
				connectionString = stripped
				credentialAlias = alias
				credentialCreated = true
				credentialStorage = storage
			case credentialAlias != "":
				if _, ok := credential.Get(credentialAlias); !ok {
					return credential.NotFoundError(credentialAlias)
				}
			}

			err := config.StoreConnection(alias, config.Connection{
				ConnectionString: connectionString,
				Name:             alias,
				Database:         database,
				Credential:       credentialAlias,
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
				resolvedDB = mongo.ParseDBFromURI(connectionString)
			}
			result := map[string]any{
				"ok":         true,
				"alias":      alias,
				"database":   resolvedDB,
				"credential": credentialAlias,
				"isDefault":  setDefault,
				"hint":       "Test with: agent-mongo connection test " + alias,
			}
			if credentialCreated {
				result["credentialCreated"] = true
				result["credentialStorage"] = credentialStorage
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
