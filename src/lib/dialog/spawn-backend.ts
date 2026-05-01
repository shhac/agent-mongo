import { platform } from "node:os";
import { type Prompter, type Result, type Spec } from "./types.ts";
import { DialogError } from "./errors.ts";
import { validateSpec } from "./validate.ts";
import { platformAvailable } from "./available.ts";
import { promptDarwin } from "./backend-darwin.ts";
import { promptLinux } from "./backend-linux.ts";
import { promptWindows } from "./backend-windows.ts";
import { type PromptCtx } from "./run.ts";

/**
 * Default Prompter. Drives native OS dialogs via `Bun.spawn`. This file is a
 * thin dispatcher — the actual platform-specific logic lives in:
 *
 * - `backend-darwin.ts` — osascript
 * - `backend-linux.ts`  — zenity / kdialog
 * - `backend-windows.ts` — PowerShell (Get-Credential, InputBox)
 *
 * Recipient projects copying this package can swap or remove one platform's
 * file without touching the others.
 *
 * Sibling modules import `getDefault()` from `./index.ts` and never reach
 * into this file directly — the import graph is `consumer → index → backend`,
 * never `consumer → backend`.
 */
export const spawnPrompter: Prompter = {
  available: platformAvailable,
  async prompt(spec: Spec, signal?: AbortSignal): Promise<Result[]> {
    validateSpec(spec);
    if (spec.items.length === 0) {
      return [];
    }
    const preflightErr = platformAvailable();
    if (preflightErr) {
      throw preflightErr;
    }
    const total = spec.items.length;
    const results: Result[] = [];
    for (let i = 0; i < total; i++) {
      signal?.throwIfAborted();
      const item = spec.items[i]!;
      const title = total <= 1 ? spec.title : `${spec.title} (step ${i + 1} of ${total})`;
      results.push({ id: item.id, value: await promptOne({ title, item, signal }) });
    }
    return results;
  },
};

async function promptOne(ctx: PromptCtx): Promise<string> {
  switch (platform()) {
    case "darwin":
      return promptDarwin(ctx);
    case "linux":
      return promptLinux(ctx);
    case "win32":
      return promptWindows(ctx);
    default:
      throw new DialogError("unsupported", platform());
  }
}

// Re-exports for tests that drive the escapers / decoder directly.
export { escapeAppleScript } from "./backend-darwin.ts";
export { escapePowerShell } from "./backend-windows.ts";
export { stripTrailingNewline } from "./run.ts";
