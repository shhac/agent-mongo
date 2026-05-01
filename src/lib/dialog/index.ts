/**
 * Dialog: a small abstraction over native OS dialogs for LLM-safe credential entry.
 *
 * The LLM driving the CLI never sees what the user types — input goes directly
 * into the OS, and only a redacted receipt comes back over stdout.
 *
 * # Backend
 *
 * The default backend uses `Bun.spawn` to drive `osascript` (macOS),
 * `zenity`/`kdialog` (Linux), or PowerShell (Windows). It is the only place in
 * this package that knows about platform-specific binaries — sibling modules
 * import only this file. Tests can swap the default via `setDefault`.
 *
 * # Availability contract
 *
 * `Prompter.available()` returns `null` when a GUI dialog can plausibly be
 * shown. Otherwise it returns an `Error` whose `cause` is one of `ErrNoGUI` or
 * `ErrUnsupported`. `available()` is best-effort — when it cannot pre-classify,
 * it returns `null` and lets the dialog itself surface the error from `prompt()`.
 *
 * # Error classification
 *
 * `classifyError` maps Prompter errors onto a neutral `Category` + hint string.
 * Consumer modules plug this into their own error envelope without re-deriving
 * the sentinel→category mapping.
 */

import { spawnPrompter } from "./spawn-backend.ts";

export type InputType = "text" | "password";

export type Field = {
  id: string;
  label: string;
  inputType: InputType;
};

export type Spec = {
  title: string;
  items: Field[];
};

export type Result = {
  id: string;
  value: string;
};

/**
 * Sentinel errors. Consumers compare via `err.cause === ErrCancelled` etc.,
 * or use {@link classifyError}.
 */
export const ErrCancelled = new Error("cancelled by user");
export const ErrNoGUI = new Error("no GUI dialog available");
export const ErrUnsupported = new Error("platform unsupported");

/**
 * Wrap a sentinel with a contextual message. Preserves the sentinel as `cause`
 * so callers can identify which class of failure occurred.
 */
export function wrapSentinel(sentinel: Error, message: string): Error {
  return new Error(`${message}: ${sentinel.message}`, { cause: sentinel });
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
 * Map a Prompter error onto a Category and a hint string suitable for surfacing
 * to an LLM. Returns `["agent", ""]` for null or unrecognised errors so callers
 * can treat the result uniformly.
 */
export function classifyError(err: unknown): [Category, string] {
  if (err == null) {
    return ["agent", ""];
  }
  const cause = err instanceof Error ? err.cause : undefined;
  if (cause === ErrCancelled || err === ErrCancelled) {
    return ["retry", "User cancelled the dialog. Re-run to retry."];
  }
  if (
    cause === ErrNoGUI ||
    cause === ErrUnsupported ||
    err === ErrNoGUI ||
    err === ErrUnsupported
  ) {
    return [
      "human",
      "A graphical desktop session is required. Run on the user's local machine, or fall back to a non-interactive flow.",
    ];
  }
  return ["agent", ""];
}

/**
 * Prompter renders a {@link Spec} to the user and returns their answers.
 *
 * Implementations must:
 * - Return `Result` entries in the same order as `spec.items`.
 * - Throw an error whose `cause` is {@link ErrCancelled} if the user dismisses any popup.
 * - Throw an error whose `cause` is {@link ErrNoGUI} when `available()` would have rejected.
 */
export type Prompter = {
  prompt(spec: Spec, signal?: AbortSignal): Promise<Result[]>;
  available(): Error | null;
};

let defaultPrompter: Prompter = spawnPrompter;

/**
 * The current default Prompter. Consumers should call `getDefault()` rather
 * than capture this once, so test swaps via {@link setDefault} take effect.
 */
export function getDefault(): Prompter {
  return defaultPrompter;
}

/**
 * Replace the default Prompter and return a function that restores the
 * previous value. Intended for tests.
 */
export function setDefault(p: Prompter): () => void {
  const prev = defaultPrompter;
  defaultPrompter = p;
  return () => {
    defaultPrompter = prev;
  };
}

/**
 * Validate Spec invariants that don't depend on a backend. An empty `items`
 * list is allowed — Prompter implementations short-circuit on it.
 */
export function validateSpec(spec: Spec): void {
  for (const item of spec.items) {
    if (item.inputType !== "text" && item.inputType !== "password") {
      throw new Error(`dialog: invalid inputType ${String(item.inputType)} for field "${item.id}"`);
    }
  }
}
