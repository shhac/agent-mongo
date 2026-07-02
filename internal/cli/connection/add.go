package connection

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/mongo"
	"github.com/shhac/agent-mongo/internal/output"
)

func unknownCredentialError(alias string) error {
	valid := strings.Join(credential.Aliases(), ", ")
	if valid == "" {
		valid = "(none)"
	}
	return fmt.Errorf(
		"Credential %q not found. Available: %s. Run: agent-mongo credential add <alias> --username <user> --password <pass>",
		alias, valid)
}

func registerAdd(parent *cobra.Command) {
	var database, credentialAlias string
	var setDefault bool

	cmd := &cobra.Command{
		Use:   "add <alias> <connection-string>",
		Short: "Add a MongoDB connection",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			alias, connectionString := args[0], args[1]

			if credentialAlias != "" {
				if _, ok := credential.Get(credentialAlias); !ok {
					return unknownCredentialError(credentialAlias)
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
			return output.PrintRaw(map[string]any{
				"ok":         true,
				"alias":      alias,
				"database":   resolvedDB,
				"credential": credentialAlias,
				"isDefault":  setDefault,
				"hint":       "Test with: agent-mongo connection test " + alias,
			})
		},
	}

	cmd.Flags().StringVar(&database, "database", "", "Override database name from URI")
	cmd.Flags().StringVar(&credentialAlias, "credential", "", "Credential alias for authentication")
	cmd.Flags().BoolVar(&setDefault, "default", false, "Set as default connection")
	parent.AddCommand(cmd)
}
