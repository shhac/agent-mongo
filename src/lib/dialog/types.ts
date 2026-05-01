/**
 * Public types for the dialog package. No runtime values — pure shape.
 */

import type { DialogError } from "./errors.ts";

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
 * Prompter renders a {@link Spec} to the user and returns their answers.
 *
 * Implementations must:
 * - Return `Result` entries in the same order as `spec.items`.
 * - Throw a `DialogError` with `code: "cancelled"` if the user dismisses any popup.
 * - Throw a `DialogError` with `code: "no-gui"` or `"unsupported"` when no GUI is available.
 */
export type Prompter = {
  prompt(spec: Spec, signal?: AbortSignal): Promise<Result[]>;
  available(): DialogError | null;
};
