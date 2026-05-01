import { type Category, type Field, classifyError, getDefault } from "../../lib/dialog/index.ts";

/**
 * FormError is thrown when the dialog flow fails. Carries the agent-mongo
 * envelope (`fixableBy` + `hint`) that add.ts surfaces to the agent.
 */
export class FormError extends Error {
  readonly fixableBy: Category;
  readonly hint: string;
  constructor(input: { message: string; fixableBy: Category; hint: string }) {
    super(input.message);
    this.name = "FormError";
    this.fixableBy = input.fixableBy;
    this.hint = input.hint;
  }
}

/**
 * Asks the user (via a native OS dialog) for any secret fields not supplied
 * by --username / --password. Returns the potentially filled-in values, or
 * throws a {@link FormError} on dialog failure.
 *
 * The LLM driving the CLI never sees what the user types — input goes
 * directly into the OS dialog, and only the call return makes it back into
 * agent-mongo's process memory.
 */
export async function promptMissingViaDialog(input: {
  name: string;
  username: string;
  password: string;
}): Promise<{ username: string; password: string }> {
  const items: Field[] = [];
  if (!input.username) {
    items.push({ id: "username", label: "Database username", inputType: "text" });
  }
  if (!input.password) {
    items.push({ id: "password", label: "Database password", inputType: "password" });
  }
  if (items.length === 0) {
    return { username: input.username, password: input.password };
  }

  const prompter = getDefault();
  const availabilityErr = prompter.available();
  if (availabilityErr) {
    throw toFormError(availabilityErr, input.name);
  }

  try {
    const results = await prompter.prompt({
      title: `agent-mongo credential: ${input.name}`,
      items,
    });
    const out = { username: input.username, password: input.password };
    for (const r of results) {
      if (r.id === "username" || r.id === "password") {
        out[r.id] = r.value;
      }
    }
    return out;
  } catch (err) {
    throw toFormError(err, input.name);
  }
}

/**
 * Adapts a dialog package error to agent-mongo's form-error envelope.
 * dialog.classifyError owns the sentinel→category mapping; this function only
 * augments the hint with agent-mongo-specific guidance.
 */
function toFormError(err: unknown, name: string): FormError {
  const [fixableBy, baseHint] = classifyError(err);
  let hint = baseHint;
  if (fixableBy === "human") {
    hint =
      "agent-mongo credential add --form requires a graphical desktop session. " +
      "Ask the user to run on their local machine, or fall back to non-interactive: " +
      `agent-mongo credential add ${name} --username <u> --password <secret>`;
  } else if (fixableBy === "retry") {
    hint = "User cancelled the dialog. Re-run agent-mongo credential add --form to retry.";
  }
  return new FormError({
    message: err instanceof Error ? err.message : String(err),
    fixableBy,
    hint,
  });
}
