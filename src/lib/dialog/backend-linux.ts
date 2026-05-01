import { DialogError } from "./errors.ts";
import { hasBinary } from "./has-binary.ts";
import { type PromptCtx, runAndDecode } from "./run.ts";

/**
 * Linux backend: tries `zenity` (GNOME / generic) first, then `kdialog`
 * (KDE). `available()` should have caught the no-backend case at preflight,
 * but if PATH changes mid-process we throw the right sentinel here too.
 */
export async function promptLinux(ctx: PromptCtx): Promise<string> {
  if (await hasBinary("zenity")) {
    return promptZenity(ctx);
  }
  if (await hasBinary("kdialog")) {
    return promptKdialog(ctx);
  }
  throw new DialogError("no-gui", "install zenity or kdialog");
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
