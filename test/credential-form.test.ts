import { describe, test, expect } from "bun:test";
import {
  type Prompter,
  type Spec,
  ErrCancelled,
  ErrNoGUI,
  setDefault,
  wrapSentinel,
} from "../src/lib/dialog/index.ts";
import {
  applyResults,
  buildCredentialSpec,
  promptMissingViaDialog,
} from "../src/cli/credential/form.ts";

describe("buildCredentialSpec", () => {
  test("includes both fields when blank", () => {
    const { spec, slots } = buildCredentialSpec({ name: "acme", username: "", password: "" });
    expect(spec.title).toBe("agent-mongo credential: acme");
    expect(spec.items.map((i) => i.id)).toEqual(["username", "password"]);
    expect(spec.items[1]!.inputType).toBe("password");
    expect(slots).toHaveLength(2);
  });

  test("skips username when supplied", () => {
    const { spec, slots } = buildCredentialSpec({
      name: "acme",
      username: "deploy",
      password: "",
    });
    expect(spec.items.map((i) => i.id)).toEqual(["password"]);
    expect(slots).toHaveLength(1);
    expect(slots[0]!.destKey).toBe("password");
  });

  test("empty spec when both supplied", () => {
    const { spec } = buildCredentialSpec({ name: "acme", username: "u", password: "p" });
    expect(spec.items).toHaveLength(0);
  });
});

describe("applyResults", () => {
  test("matches by ID, not order", () => {
    const { slots } = buildCredentialSpec({ name: "acme", username: "", password: "" });
    const out = applyResults({
      username: "",
      password: "",
      slots,
      results: [
        { id: "password", value: "shh" },
        { id: "username", value: "deploy" },
      ],
    });
    expect(out).toEqual({ username: "deploy", password: "shh" });
  });

  test("preserves pre-existing values", () => {
    const { slots } = buildCredentialSpec({ name: "acme", username: "preset", password: "" });
    const out = applyResults({
      username: "preset",
      password: "",
      slots,
      results: [{ id: "password", value: "shh" }],
    });
    expect(out).toEqual({ username: "preset", password: "shh" });
  });
});

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

  test("prompts for missing fields via stub", async () => {
    let received: Spec | null = null;
    const stub: Prompter = {
      available: () => null,
      async prompt(spec) {
        received = spec;
        return spec.items.map((i) => ({
          id: i.id,
          value: i.id === "password" ? "secret-from-dialog" : "deploy",
        }));
      },
    };
    const restore = setDefault(stub);
    try {
      const r = await promptMissingViaDialog({ name: "acme", username: "", password: "" });
      expect(r.username).toBe("deploy");
      expect(r.password).toBe("secret-from-dialog");
      expect(r.error).toBeUndefined();
      expect(received!.title).toBe("agent-mongo credential: acme");
    } finally {
      restore();
    }
  });

  test("classifies cancellation as retry", async () => {
    const stub: Prompter = {
      available: () => null,
      async prompt() {
        throw wrapSentinel(ErrCancelled, "Database password");
      },
    };
    const restore = setDefault(stub);
    try {
      const r = await promptMissingViaDialog({ name: "acme", username: "", password: "" });
      expect(r.error?.fixableBy).toBe("retry");
      expect(r.error?.hint).toContain("Re-run");
    } finally {
      restore();
    }
  });

  test("classifies no-GUI as human with agent-mongo-specific hint", async () => {
    const stub: Prompter = {
      available: () => wrapSentinel(ErrNoGUI, "no $DISPLAY"),
      async prompt() {
        throw new Error("should not be called");
      },
    };
    const restore = setDefault(stub);
    try {
      const r = await promptMissingViaDialog({ name: "acme", username: "", password: "" });
      expect(r.error?.fixableBy).toBe("human");
      expect(r.error?.hint).toContain("agent-mongo credential add --form");
      expect(r.error?.hint).toContain("--username <u> --password <secret>");
    } finally {
      restore();
    }
  });
});
