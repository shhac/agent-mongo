import type { Spec } from "./types.ts";

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
