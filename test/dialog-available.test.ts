import { describe, test, expect } from "bun:test";
import { darwinAvailable } from "../src/lib/dialog/available-darwin.ts";
import { linuxAvailable } from "../src/lib/dialog/available-linux.ts";
import { windowsAvailable } from "../src/lib/dialog/available-windows.ts";
import { DialogError } from "../src/lib/dialog/index.ts";

describe("darwinAvailable", () => {
  test("local terminal session → null (GUI plausibly available)", () => {
    expect(darwinAvailable({ TERM_PROGRAM: "iTerm.app" })).toBeNull();
  });

  test("no env vars at all → null (let dialog itself surface failures)", () => {
    expect(darwinAvailable({})).toBeNull();
  });

  test("SSH session without local terminal → no-gui", () => {
    const err = darwinAvailable({ SSH_CONNECTION: "1.2.3.4 22 5.6.7.8 22" });
    expect(err).toBeInstanceOf(DialogError);
    expect(err?.code).toBe("no-gui");
    expect(err?.message).toContain("SSH");
  });

  test("SSH session WITH local terminal → null (Mac.app over SSH or weird setup)", () => {
    expect(
      darwinAvailable({
        SSH_CONNECTION: "1.2.3.4 22 5.6.7.8 22",
        TERM_PROGRAM: "Apple_Terminal",
      }),
    ).toBeNull();
  });
});

describe("linuxAvailable", () => {
  test("DISPLAY + zenity present → null", () => {
    const env = { DISPLAY: ":0" };
    expect(linuxAvailable(env, (name) => name === "zenity")).toBeNull();
  });

  test("WAYLAND_DISPLAY + kdialog present → null", () => {
    const env = { WAYLAND_DISPLAY: "wayland-0" };
    expect(linuxAvailable(env, (name) => name === "kdialog")).toBeNull();
  });

  test("no display server → no-gui", () => {
    const err = linuxAvailable({}, () => true);
    expect(err?.code).toBe("no-gui");
    expect(err?.message).toContain("DISPLAY");
  });

  test("display set, but no zenity or kdialog → no-gui", () => {
    const err = linuxAvailable({ DISPLAY: ":0" }, () => false);
    expect(err?.code).toBe("no-gui");
    expect(err?.message).toContain("zenity");
    expect(err?.message).toContain("kdialog");
  });

  test("only zenity present is enough", () => {
    expect(linuxAvailable({ DISPLAY: ":0" }, (n) => n === "zenity")).toBeNull();
  });

  test("only kdialog present is enough", () => {
    expect(linuxAvailable({ DISPLAY: ":0" }, (n) => n === "kdialog")).toBeNull();
  });
});

describe("windowsAvailable", () => {
  test("Console session → null", () => {
    expect(windowsAvailable({ SESSIONNAME: "Console" })).toBeNull();
  });

  test("RDP session → null", () => {
    expect(windowsAvailable({ SESSIONNAME: "RDP-Tcp#0" })).toBeNull();
  });

  test("SESSIONNAME unset → no-gui (SSH or service context)", () => {
    const err = windowsAvailable({});
    expect(err?.code).toBe("no-gui");
    expect(err?.message).toContain("SESSIONNAME");
  });
});
