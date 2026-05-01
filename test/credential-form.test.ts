import { describe, test, expect } from "bun:test";
import { type Prompter, type Spec, DialogError, setDefault } from "../src/lib/dialog/index.ts";
import { FormError, promptMissingViaDialog } from "../src/cli/credential/form.ts";

describe("promptMissingViaDialog", () => {
  test("returns supplied values without invoking the prompter", async () => {
    let called = false;
    const stub: Prompter = {
      available: () => null,
      async prompt() {
        called = true;
        return [];
      },
    };
    const restore = setDefault(stub);
    try {
      const r = await promptMissingViaDialog({ name: "acme", username: "u", password: "p" });
      expect(r).toEqual({ username: "u", password: "p" });
      expect(called).toBe(false);
    } finally {
      restore();
    }
  });

  test("prompts only for missing fields", async () => {
    let received: Spec | null = null;
    const stub: Prompter = {
      available: () => null,
      async prompt(spec) {
        received = spec;
        return spec.items.map((i) => ({ id: i.id, value: "from-dialog" }));
      },
    };
    const restore = setDefault(stub);
    try {
      const r = await promptMissingViaDialog({ name: "acme", username: "preset", password: "" });
      expect(r.username).toBe("preset");
      expect(r.password).toBe("from-dialog");
      expect(received!.items.map((i) => i.id)).toEqual(["password"]);
      expect(received!.title).toBe("agent-mongo credential: acme");
    } finally {
      restore();
    }
  });

  test("prompts for both when both blank", async () => {
    const stub: Prompter = {
      available: () => null,
      async prompt(spec) {
        return spec.items.map((i) => ({
          id: i.id,
          value: i.id === "password" ? "secret-from-dialog" : "deploy",
        }));
      },
    };
    const restore = setDefault(stub);
    try {
      const r = await promptMissingViaDialog({ name: "acme", username: "", password: "" });
      expect(r).toEqual({ username: "deploy", password: "secret-from-dialog" });
    } finally {
      restore();
    }
  });

  test("ignores extraneous result IDs from a misbehaving backend", async () => {
    const stub: Prompter = {
      available: () => null,
      async prompt() {
        return [
          { id: "username", value: "u" },
          { id: "password", value: "p" },
          { id: "rogue", value: "should-be-ignored" },
        ];
      },
    };
    const restore = setDefault(stub);
    try {
      const r = await promptMissingViaDialog({ name: "acme", username: "", password: "" });
      expect(r).toEqual({ username: "u", password: "p" });
    } finally {
      restore();
    }
  });

  test("throws FormError on cancellation (fixableBy=retry)", async () => {
    const stub: Prompter = {
      available: () => null,
      async prompt() {
        throw new DialogError("cancelled", "Database password");
      },
    };
    const restore = setDefault(stub);
    try {
      await expect(
        promptMissingViaDialog({ name: "acme", username: "", password: "" }),
      ).rejects.toBeInstanceOf(FormError);

      let caught: FormError | null = null;
      try {
        await promptMissingViaDialog({ name: "acme", username: "", password: "" });
      } catch (err) {
        if (err instanceof FormError) {
          caught = err;
        }
      }
      expect(caught?.fixableBy).toBe("retry");
      expect(caught?.hint).toContain("Re-run");
    } finally {
      restore();
    }
  });

  test("throws FormError on no-GUI with agent-mongo-specific hint", async () => {
    const stub: Prompter = {
      available: () => new DialogError("no-gui", "no $DISPLAY"),
      async prompt() {
        throw new Error("should not be called");
      },
    };
    const restore = setDefault(stub);
    try {
      let caught: FormError | null = null;
      try {
        await promptMissingViaDialog({ name: "acme", username: "", password: "" });
      } catch (err) {
        if (err instanceof FormError) {
          caught = err;
        }
      }
      expect(caught?.fixableBy).toBe("human");
      expect(caught?.hint).toContain("agent-mongo credential add --form");
      expect(caught?.hint).toContain("--username <u> --password <secret>");
    } finally {
      restore();
    }
  });
});
