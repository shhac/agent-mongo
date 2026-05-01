/**
 * Error model for the dialog package. One Error subclass with a `code`
 * field, plus a `Category`-based classifier suitable for surfacing to an LLM.
 */

/**
 * Error code carried by every Prompter failure. `instanceof DialogError`
 * plus a `code` switch is the only check consumers need — no
 * sentinel-identity comparison, no `err.cause` walking.
 */
export type ErrorCode = "cancelled" | "no-gui" | "unsupported";

/**
 * Single error type for all Prompter failures. The `code` field is the
 * stable classification; `message` carries a contextual phrase ("Database
 * password", "no $DISPLAY set", etc.).
 */
export class DialogError extends Error {
  readonly code: ErrorCode;
  constructor(code: ErrorCode, message: string) {
    super(message);
    this.name = "DialogError";
    this.code = code;
  }
}

/**
 * Category groups Prompter errors by who can fix them.
 *
 * - `human`: environment issue (no GUI, missing zenity, headless host). Don't retry.
 * - `retry`: transient — user cancelled the dialog; re-running the same command is the right next step.
 * - `agent`: anything else (bad spec, programmer error). The caller can probably correct it.
 */
export type Category = "human" | "retry" | "agent";

/**
 * Map a Prompter error onto a Category and a hint string suitable for
 * surfacing to an LLM. Returns `["agent", ""]` for null or unrecognised
 * errors so callers can treat the result uniformly.
 */
export function classifyError(err: unknown): [Category, string] {
  if (!(err instanceof DialogError)) {
    return ["agent", ""];
  }
  switch (err.code) {
    case "cancelled":
      return ["retry", "User cancelled the dialog. Re-run to retry."];
    case "no-gui":
    case "unsupported":
      return [
        "human",
        "A graphical desktop session is required. Run on the user's local machine, or fall back to a non-interactive flow.",
      ];
  }
}
