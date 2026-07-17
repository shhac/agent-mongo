package credential

import (
	"context"

	"github.com/shhac/lib-agent-cli/dialog"
	out "github.com/shhac/lib-agent-output"
)

// promptMissingViaDialog asks the user (via a native OS dialog) for any secret
// fields not supplied by --username / --password. The LLM driving the CLI
// never sees what the user types — input goes directly into the OS dialog.
func promptMissingViaDialog(
	ctx context.Context, name, username, password string,
) (string, string, error) {
	var items []dialog.Field
	if username == "" {
		items = append(items, dialog.Field{ID: "username", Label: "Database username", InputType: dialog.Text})
	}
	if password == "" {
		items = append(items, dialog.Field{ID: "password", Label: "Database password", InputType: dialog.Password})
	}
	if len(items) == 0 {
		return username, password, nil
	}

	if err := dialog.Available(); err != nil {
		return username, password, classifyDialogErr(err, name)
	}

	results, err := dialog.Prompt(ctx, dialog.Spec{
		Title: "agent-mongo credential: " + name,
		Items: items,
	})
	if err != nil {
		return username, password, classifyDialogErr(err, name)
	}

	for _, r := range results {
		switch r.ID {
		case "username":
			username = r.Value
		case "password":
			password = r.Value
		}
	}
	return username, password, nil
}

// classifyDialogErr adapts a dialog error to the output contract.
// dialog.ClassifyError owns the sentinel→category mapping; this only augments
// the hint with agent-mongo-specific guidance.
func classifyDialogErr(err error, name string) error {
	category, baseHint := dialog.ClassifyError(err)
	hint := baseHint
	var fixableBy out.FixableBy
	switch category {
	case dialog.CategoryHuman:
		fixableBy = out.FixableByHuman
		hint = "agent-mongo credential add --form requires a graphical desktop session. " +
			"Ask the user to run on their local machine, or fall back to non-interactive stdin (keeps the password off argv): " +
			"printf '%s' \"$PASSWORD\" | agent-mongo credential add " + name + " --username <u>"
	case dialog.CategoryRetry:
		fixableBy = out.FixableByRetry
		hint = "User cancelled the dialog. Re-run agent-mongo credential add --form to retry."
	default:
		fixableBy = out.FixableByAgent
	}
	return out.Wrap(err, fixableBy).WithHint(hint)
}
