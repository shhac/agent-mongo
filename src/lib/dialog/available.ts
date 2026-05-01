import { platform } from "node:os";
import { ErrNoGUI, ErrUnsupported, wrapSentinel } from "./index.ts";

/**
 * Best-effort pre-flight: returns `null` if a GUI dialog can plausibly be
 * shown, otherwise an Error whose `cause` is `ErrNoGUI` or `ErrUnsupported`.
 */
export function platformAvailable(): Error | null {
  switch (platform()) {
    case "darwin":
      return darwinAvailable();
    case "linux":
      return linuxAvailable();
    case "win32":
      return windowsAvailable();
    default:
      return wrapSentinel(ErrUnsupported, platform());
  }
}

/**
 * macOS: osascript fails cleanly if no Aqua session is attached, so we let the
 * dialog itself surface most failures. The one case we pre-flight is
 * "obviously SSH'd in": $SSH_CONNECTION is set and no local terminal app has
 * set $TERM_PROGRAM.
 */
function darwinAvailable(): Error | null {
  if (process.env.SSH_CONNECTION && !process.env.TERM_PROGRAM) {
    return wrapSentinel(ErrNoGUI, "appears to be an SSH session with no local terminal");
  }
  return null;
}

/**
 * Linux: requires a display server ($DISPLAY or $WAYLAND_DISPLAY) and either
 * `zenity` (GNOME / generic) or `kdialog` (KDE) on PATH.
 */
function linuxAvailable(): Error | null {
  if (!process.env.DISPLAY && !process.env.WAYLAND_DISPLAY) {
    return wrapSentinel(ErrNoGUI, "no $DISPLAY or $WAYLAND_DISPLAY set");
  }
  if (hasBinary("zenity") || hasBinary("kdialog")) {
    return null;
  }
  return wrapSentinel(ErrNoGUI, "install `zenity` (GNOME) or `kdialog` (KDE)");
}

/**
 * Windows: Win32-OpenSSH leaves $SESSIONNAME unset when the SSH server runs as
 * a service; service contexts also fail. An interactive desktop session has
 * SESSIONNAME as "Console" or "RDP-Tcp#N". We allow anything non-empty.
 */
function windowsAvailable(): Error | null {
  if (!process.env.SESSIONNAME) {
    return wrapSentinel(ErrNoGUI, "$SESSIONNAME unset (likely SSH or service context)");
  }
  return null;
}

function hasBinary(name: string): boolean {
  const proc = Bun.spawnSync(["sh", "-c", `command -v ${name}`], {
    stdout: "pipe",
    stderr: "pipe",
  });
  return proc.exitCode === 0;
}
