import { type PromptCtx, runAndDecode } from "./run.ts";

/**
 * macOS backend: drives `osascript`'s `display dialog` builtin. Password
 * fields use `with hidden answer` so input is masked.
 */
export async function promptDarwin(ctx: PromptCtx): Promise<string> {
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

/**
 * AppleScript string literal escaping. Backslash must be escaped first
 * (otherwise it would double-escape the escapes we add for `"`).
 */
export function escapeAppleScript(s: string): string {
  return s.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
}
