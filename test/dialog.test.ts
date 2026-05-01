import { describe, test, expect } from "bun:test";
import {
  type Prompter,
  type Spec,
  classifyError,
  ErrCancelled,
  ErrNoGUI,
  ErrUnsupported,
  getDefault,
  setDefault,
  validateSpec,
  wrapSentinel,
} from "../src/lib/dialog/index.ts";

describe("classifyError", () => {
  test("null → agent with empty hint", () => {
    expect(classifyError(null)).toEqual(["agent", ""]);
  });

  test("ErrCancelled directly → retry", () => {
    const [cat] = classifyError(ErrCancelled);
    expect(cat).toBe("retry");
  });

  test("wrapped ErrCancelled → retry", () => {
    const wrapped = wrapSentinel(ErrCancelled, "Database password");
    const [cat, hint] = classifyError(wrapped);
    expect(cat).toBe("retry");
    expect(hint).toContain("Re-run");
  });

  test("wrapped ErrNoGUI → human", () => {
    const wrapped = wrapSentinel(ErrNoGUI, "no $DISPLAY set");
    const [cat, hint] = classifyError(wrapped);
    expect(cat).toBe("human");
    expect(hint).toContain("graphical desktop session");
  });

  test("wrapped ErrUnsupported → human", () => {
    const [cat] = classifyError(wrapSentinel(ErrUnsupported, "haiku"));
    expect(cat).toBe("human");
  });

  test("arbitrary Error → agent", () => {
    expect(classifyError(new Error("something else"))).toEqual(["agent", ""]);
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

  test("stub that throws ErrCancelled propagates as retry category", async () => {
    const stub: Prompter = {
      available: () => null,
      async prompt() {
        throw wrapSentinel(ErrCancelled, "Password");
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
