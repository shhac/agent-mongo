package shared

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// RegisterUsage attaches the conventional per-group `usage` subcommand that
// prints an LLM-optimized reference card.
func RegisterUsage(parent *cobra.Command, verb, text string) {
	parent.AddCommand(&cobra.Command{
		Use:   "usage",
		Short: "Print " + verb + " command documentation (LLM-optimized)",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println(strings.TrimSpace(text))
		},
	})
}
