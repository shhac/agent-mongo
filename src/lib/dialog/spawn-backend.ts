import { platform } from "node:os";
import {
  type Field,
  type Prompter,
  type Result,
  type Spec,
  ErrCancelled,
  ErrUnsupported,
  validateSpec,
  wrapSentinel,
} from "./index.ts";
import { platformAvailable } from "./available.ts";

/**
 * Default Prompter. Drives native OS dialogs via `Bun.spawn`. This is the
 * only file in the package that knows about platform-specific binaries.
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

/**
 * Per-prompt context. Bundles the per-field args so platform handlers stay
 * within the project's 2-parameter convention.
 */
type PromptCtx = {
  title: string;
  item: Field;
  signal: AbortSignal | undefined;
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
      throw wrapSentinel(ErrUnsupported, platform());
  }
}

/* ───────── macOS (osascript) ───────── */

async function promptDarwin(ctx: PromptCtx): Promise<string> {
  const hidden = ctx.item.inputType === "password" ? " with hidden answer" : "";
  const script =
    `set r to display dialog "${escapeAppleScript(ctx.item.label)}"` +
    ` default answer ""${hidden}` +
    ` with title "${escapeAppleScript(ctx.title)}"` +
    `\ntext returned of r`;
  return runAndDecode({
    argv: ["osascript", "-e", script],
    ctx,
    isCancelStderr: (stderr) => /User canceled/i.test(stderr),
  });
}

function escapeAppleScript(s: string): string {
  return s.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
}

/* ───────── Linux (zenity → kdialog) ───────── */

async function promptLinux(ctx: PromptCtx): Promise<string> {
  if (await hasBinary("zenity")) {
    return promptZenity(ctx);
  }
  if (await hasBinary("kdialog")) {
    return promptKdialog(ctx);
  }
  throw wrapSentinel(ErrCancelled, ctx.item.label);
}

async function promptZenity(ctx: PromptCtx): Promise<string> {
  const args =
    ctx.item.inputType === "password"
      ? ["zenity", "--password", `--title=${ctx.title}`]
      : ["zenity", "--entry", `--title=${ctx.title}`, `--text=${ctx.item.label}`];
  return runAndDecode({ argv: args, ctx });
}

async function promptKdialog(ctx: PromptCtx): Promise<string> {
  const args =
    ctx.item.inputType === "password"
      ? ["kdialog", "--title", ctx.title, "--password", ctx.item.label]
      : ["kdialog", "--title", ctx.title, "--inputbox", ctx.item.label];
  return runAndDecode({ argv: args, ctx });
}

/* ───────── Windows (PowerShell) ───────── */

async function promptWindows(ctx: PromptCtx): Promise<string> {
  const label = escapePowerShell(ctx.item.label);
  const title = escapePowerShell(ctx.title);
  const script =
    ctx.item.inputType === "password"
      ? `$ErrorActionPreference='Stop';$c=Get-Credential -Message '${label}' -UserName 'user';if(-not $c){exit 1};$c.GetNetworkCredential().Password`
      : `$ErrorActionPreference='Stop';Add-Type -AssemblyName Microsoft.VisualBasic;$v=[Microsoft.VisualBasic.Interaction]::InputBox('${label}','${title}','');if($v -eq ''){exit 1};$v`;
  return runAndDecode({ argv: ["powershell", "-NoProfile", "-Command", script], ctx });
}

function escapePowerShell(s: string): string {
  return s.replace(/'/g, "''");
}

/* ───────── helpers ───────── */

type CommandResult = { stdout: string; stderr: string; exitCode: number };

type RunRequest = {
  argv: string[];
  ctx: PromptCtx;
  isCancelStderr?: (stderr: string) => boolean;
};

/**
 * Run argv, decode stdout on success, classify cancellation on exit code 1
 * (the convention shared by zenity, kdialog, and our PowerShell scripts) or
 * via an optional stderr matcher (osascript prints "User canceled.").
 */
async function runAndDecode(req: RunRequest): Promise<string> {
  const { stdout, stderr, exitCode } = await runCommand(req.argv, req.ctx);
  if (exitCode === 0) {
    return stripTrailingNewline(stdout);
  }
  if (exitCode === 1 || req.isCancelStderr?.(stderr)) {
    throw wrapSentinel(ErrCancelled, req.ctx.item.label);
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
      throw wrapSentinel(ErrCancelled, ctx.item.label);
    }
    throw err;
  }
}

async function hasBinary(name: string): Promise<boolean> {
  const proc = Bun.spawn(["sh", "-c", `command -v ${name}`], {
    stdout: "pipe",
    stderr: "pipe",
  });
  const exitCode = await proc.exited;
  return exitCode === 0;
}

function stripTrailingNewline(s: string): string {
  return s.endsWith("\n") ? s.slice(0, -1) : s;
}
