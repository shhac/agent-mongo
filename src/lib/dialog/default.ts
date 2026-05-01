import type { Prompter } from "./types.ts";
import type { spawnPrompter as SpawnPrompterType } from "./spawn-backend.ts";

/**
 * Optional override of the default Prompter. `null` means "use the built-in
 * spawn backend"; non-null replaces it (test stubs, alternative backends).
 *
 * Resolved lazily inside {@link getDefault} so this module doesn't have to
 * evaluate `spawn-backend.ts` at import time — that would create a circular
 * dependency, since `spawn-backend.ts` imports types from this package.
 */
let defaultPrompterOverride: Prompter | null = null;

/**
 * The current default Prompter. Consumers should call `getDefault()` rather
 * than capture this once, so test swaps via {@link setDefault} take effect.
 */
export function getDefault(): Prompter {
  if (defaultPrompterOverride !== null) {
    return defaultPrompterOverride;
  }
  // Dynamic require avoids the circular import at module-init time.
  // The type-only import above carries the shape; this require pulls the runtime value.
  const mod = require("./spawn-backend.ts") as { spawnPrompter: typeof SpawnPrompterType };
  return mod.spawnPrompter;
}

/**
 * Replace the default Prompter and return a function that restores the
 * previous value. Intended for tests.
 */
export function setDefault(p: Prompter): () => void {
  const prev = defaultPrompterOverride;
  defaultPrompterOverride = p;
  return () => {
    defaultPrompterOverride = prev;
  };
}
