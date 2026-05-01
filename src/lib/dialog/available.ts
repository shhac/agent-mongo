import { platform } from "node:os";
import { DialogError } from "./errors.ts";
import { darwinAvailable } from "./available-darwin.ts";
import { linuxAvailable } from "./available-linux.ts";
import { windowsAvailable } from "./available-windows.ts";

/**
 * Best-effort pre-flight: returns `null` if a GUI dialog can plausibly be
 * shown, otherwise a `DialogError` with code `"no-gui"` or `"unsupported"`.
 */
export function platformAvailable(): DialogError | null {
  switch (platform()) {
    case "darwin":
      return darwinAvailable();
    case "linux":
      return linuxAvailable();
    case "win32":
      return windowsAvailable();
    default:
      return new DialogError("unsupported", platform());
  }
}
