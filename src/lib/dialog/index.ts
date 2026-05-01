/**
 * Dialog: a small abstraction over native OS dialogs for LLM-safe secret entry.
 *
 * The agent driving the CLI never sees what the user types — input goes
 * directly into the OS dialog, and only the resulting value comes back.
 *
 * # Requirements
 *
 * - **Bun** runtime. The default backend uses `Bun.spawn` / `Bun.spawnSync`.
 * - **Platforms**: macOS (osascript), Linux (zenity or kdialog),
 *   Windows (PowerShell). Anything else throws a `DialogError` with
 *   `code: "unsupported"`.
 * - **No npm dependencies.** Only `node:os` from stdlib.
 *
 * # Drop-in portability
 *
 * This package is self-contained. To copy into another Bun project:
 *
 * 1. Copy the entire `src/lib/dialog/` directory.
 * 2. Adjust the import path used by your consumers.
 * 3. (Optional) replace `spawn-backend.ts` with a different backend that
 *    implements the {@link Prompter} interface — e.g. an Electron IPC
 *    bridge, or a localhost-form fallback for headless environments.
 *
 * # Known limitations (today)
 *
 * - One popup per field (no multi-field forms in a single popup).
 * - No multi-line input, display-only fields, or "remember this" checkboxes.
 *
 * # Error classification
 *
 * `classifyError` maps a {@link DialogError} onto a neutral `Category` +
 * hint string. Consumer modules plug this into their own error envelope
 * without re-deriving the mapping.
 */

export type { InputType, Field, Spec, Result, Prompter } from "./types.ts";
export { type Category, type ErrorCode, DialogError, classifyError } from "./errors.ts";
export { getDefault, setDefault } from "./default.ts";
export { validateSpec } from "./validate.ts";
