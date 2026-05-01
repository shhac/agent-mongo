import { describe, test, expect } from "bun:test";
import {
  type Prompter,
  type Spec,
  DialogError,
  classifyError,
  getDefault,
  setDefault,
  validateSpec,
} from "../src/lib/dialog/index.ts";

describe("classifyError", () => {
  test("null → agent with empty hint", () => {
    expect(classifyError(null)).toEqual(["agent", ""]);
  });

  test("DialogError cancelled → retry", () => {
    const [cat, hint] = classifyError(new DialogError("cancelled", "Database password"));
    expect(cat).toBe("retry");
    expect(hint).toContain("Re-run");
  });

  test("DialogError no-gui → human", () => {
    const [cat, hint] = classifyError(new DialogError("no-gui", "no $DISPLAY set"));
    expect(cat).toBe("human");
    expect(hint).toContain("graphical desktop session");
  });

  test("DialogError unsupported → human", () => {
    const [cat] = classifyError(new DialogError("unsupported", "haiku"));
    expect(cat).toBe("human");
  });

  test("plain Error → agent (not a DialogError)", () => {
    expect(classifyError(new Error("something else"))).toEqual(["agent", ""]);
  });

  test("DialogError survives instanceof through cause-chain wrap", () => {
    const inner = new DialogError("cancelled", "x");
    const wrapper = new Error("outer", { cause: inner });
    expect(classifyError(wrapper)).toEqual(["agent", ""]);
    expect(classifyError(inner)).toEqual(["retry", "User cancelled the dialog. Re-run to retry."]);
  });
});

describe("validateSpec", () => {
  test("empty items is OK", () => {
    expect(() => validateSpec({ title: "x", items: [] })).not.toThrow();
  });

  test("rejects unknown inputType", () => {
    const spec = {
      title: "x",
      items: [{ id: "a", label: "A", inputType: "secret" as never }],
    };
    expect(() => validateSpec(spec)).toThrow(/invalid inputType/);
  });

  test("accepts text and password", () => {
    const spec: Spec = {
      title: "x",
      items: [
        { id: "u", label: "U", inputType: "text" },
        { id: "p", label: "P", inputType: "password" },
      ],
    };
    expect(() => validateSpec(spec)).not.toThrow();
  });
});

describe("setDefault / getDefault", () => {
  test("swaps and restores", async () => {
    const stub: Prompter = {
      available: () => null,
      async prompt(spec) {
        return spec.items.map((i) => ({ id: i.id, value: `stub-${i.id}` }));
      },
    };
    const restore = setDefault(stub);
    try {
      const out = await getDefault().prompt({
        title: "test",
        items: [{ id: "x", label: "X", inputType: "text" }],
      });
      expect(out).toEqual([{ id: "x", value: "stub-x" }]);
    } finally {
      restore();
    }
    expect(getDefault()).not.toBe(stub);
  });

  test("stub that throws DialogError propagates as retry category", async () => {
    const stub: Prompter = {
      available: () => null,
      async prompt() {
        throw new DialogError("cancelled", "Password");
      },
    };
    const restore = setDefault(stub);
    try {
      let caught: unknown = null;
      try {
        await getDefault().prompt({
          title: "test",
          items: [{ id: "p", label: "P", inputType: "password" }],
        });
      } catch (err) {
        caught = err;
      }
      const [cat] = classifyError(caught);
      expect(cat).toBe("retry");
    } finally {
      restore();
    }
  });
});

describe("DialogError", () => {
  test("has name and code fields", () => {
    const err = new DialogError("no-gui", "test");
    expect(err.name).toBe("DialogError");
    expect(err.code).toBe("no-gui");
    expect(err.message).toBe("test");
    expect(err instanceof Error).toBe(true);
    expect(err instanceof DialogError).toBe(true);
  });
});
