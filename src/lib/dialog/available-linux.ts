import { DialogError } from "./errors.ts";
import { hasBinarySync } from "./has-binary.ts";

/**
 * Linux: requires a display server ($DISPLAY or $WAYLAND_DISPLAY) and either
 * `zenity` (GNOME / generic) or `kdialog` (KDE) on PATH.
 *
 * Accepts an env object and a binary-lookup function for testability;
 * defaults to `process.env` and the real PATH check.
 */
export function linuxAvailable(
  env: NodeJS.ProcessEnv = process.env,
  hasBin: (name: string) => boolean = hasBinarySync,
): DialogError | null {
  if (!env.DISPLAY && !env.WAYLAND_DISPLAY) {
    return new DialogError("no-gui", "no $DISPLAY or $WAYLAND_DISPLAY set");
  }
  if (hasBin("zenity") || hasBin("kdialog")) {
    return null;
  }
  return new DialogError("no-gui", "install `zenity` (GNOME) or `kdialog` (KDE)");
}
