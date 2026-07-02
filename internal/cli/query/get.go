package query

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/cli/shared"
	"github.com/shhac/agent-mongo/internal/mongo"
	"github.com/shhac/agent-mongo/internal/output"
)

func registerGet(parent *cobra.Command, globals func() *shared.GlobalFlags) {
	var idType, projection string

	cmd := &cobra.Command{
		Use:   "get <database> <collection> <id>",
		Short: "Get a single document by _id",
		Args:  cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			g := globals()
			ref := mongo.Ref{DB: args[0], Collection: args[1]}
			id := args[2]

			if idType != "" && idType != "objectid" && idType != "string" && idType != "number" {
				return fmt.Errorf("Invalid --type: %q. Valid: objectid, string, number", idType)
			}
			projectionDoc, err := parseOptionalDoc(projection, "projection")
			if err != nil {
				return err
			}

			return shared.WithSessionRef(g, ref, func(ctx shared.SessionCtx) error {
				doc, err := ctx.Session.FindByID(ctx.Ctx, mongo.FindByIDOpts{
					Ref:        ref,
					RawID:      id,
					IDType:     idType,
					Projection: projectionDoc,
				})
				if err != nil {
					return err
				}
				if doc == nil {
					return fmt.Errorf("Document not found: _id=%s in %s.%s", id, ref.DB, ref.Collection)
				}
				return output.PrintResult(map[string]any{
					"database":   ref.DB,
					"collection": ref.Collection,
					"fieldCount": len(doc),
					"document":   doc,
				})
			})
		},
	}

	cmd.Flags().StringVar(&idType, "type", "",
		"Force ID type: objectid, string, number (auto-detected by default)")
	cmd.Flags().StringVar(&projection, "projection", "", `Field projection (e.g. {"name": 1, "email": 1})`)
	parent.AddCommand(cmd)
}
