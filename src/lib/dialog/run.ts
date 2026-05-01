import type { Field } from "./types.ts";
import { DialogError } from "./errors.ts";

/**
 * Per-prompt context. Bundles the per-field args so platform handlers stay
 * within the project's 2-parameter convention.
 */
export type PromptCtx = {
  title: string;
  item: Field;
  signal: AbortSignal | undefined;
};

type CommandResult = { stdout: string; stderr: string; exitCode: number };

export type RunRequest = {
  argv: string[];
  ctx: PromptCtx;
  isCancelStderr?: (stderr: string) => boolean;
};

/**
 * Run argv via Bun.spawn, decode stdout on success, classify cancellation on
 * exit code 1 (the convention shared by zenity, kdialog, and our PowerShell
 * scripts) or via an optional stderr matcher (osascript prints
 * "User canceled.").
 */
export async function runAndDecode(req: RunRequest): Promise<string> {
  const { stdout, stderr, exitCode } = await runCommand(req.argv, req.ctx);
  if (exitCode === 0) {
    return stripTrailingNewline(stdout);
  }
  if (exitCode === 1 || req.isCancelStderr?.(stderr)) {
    throw new DialogError("cancelled", req.ctx.item.label);
  }
  throw new Error(`dialog failed (${req.ctx.item.label}): ${stderr || `exit ${exitCode}`}`);
}

async function runCommand(argv: string[], ctx: PromptCtx): Promise<CommandResult> {
  try {
    const proc = Bun.spawn(argv, { stdout: "pipe", stderr: "pipe", signal: ctx.signal });
    const [stdout, stderr] = await Promise.all([
      new Response(proc.stdout).text(),
      new Response(proc.stderr).text(),
    ]);
    const exitCode = await proc.exited;
    return { stdout, stderr, exitCode: exitCode ?? -1 };
  } catch (err) {
    if (err instanceof Error && err.name === "AbortError") {
      throw new DialogError("cancelled", ctx.item.label);
    }
    throw err;
  }
}

export function stripTrailingNewline(s: string): string {
  return s.endsWith("\n") ? s.slice(0, -1) : s;
}
