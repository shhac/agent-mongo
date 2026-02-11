import { describe, test, expect } from "bun:test";
import { validatePipeline } from "../src/mongo/aggregate.ts";

describe("validatePipeline", () => {
  test("rejects $out stage", () => {
    const pipeline = [{ $match: { status: "active" } }, { $out: "outputCollection" }];
    expect(() => validatePipeline(pipeline)).toThrow(
      'Write stage "$out" is not allowed. agent-mongo is read-only.',
    );
  });

  test("rejects $merge stage", () => {
    const pipeline = [
      { $group: { _id: "$type", count: { $sum: 1 } } },
      { $merge: { into: "results" } },
    ];
    expect(() => validatePipeline(pipeline)).toThrow(
      'Write stage "$merge" is not allowed. agent-mongo is read-only.',
    );
  });

  test("rejects $out even as first stage", () => {
    const pipeline = [{ $out: "test" }];
    expect(() => validatePipeline(pipeline)).toThrow('Write stage "$out"');
  });

  test("rejects $merge even as first stage", () => {
    const pipeline = [{ $merge: "test" }];
    expect(() => validatePipeline(pipeline)).toThrow('Write stage "$merge"');
  });

  test("allows valid read-only pipeline", () => {
    const pipeline = [
      { $match: { status: "active" } },
      { $group: { _id: "$category", total: { $sum: "$amount" } } },
      { $sort: { total: -1 } },
      { $limit: 10 },
    ];
    expect(() => validatePipeline(pipeline)).not.toThrow();
  });

  test("allows empty pipeline", () => {
    expect(() => validatePipeline([])).not.toThrow();
  });

  test("allows $project, $unwind, $lookup stages", () => {
    const pipeline = [
      { $project: { name: 1, status: 1 } },
      { $unwind: "$items" },
      { $lookup: { from: "other", localField: "id", foreignField: "ref", as: "joined" } },
    ];
    expect(() => validatePipeline(pipeline)).not.toThrow();
  });

  test("skips non-object stages", () => {
    // validatePipeline continues past non-object entries
    const pipeline: unknown[] = [null, undefined, "invalid", { $match: { x: 1 } }];
    expect(() => validatePipeline(pipeline as Record<string, unknown>[])).not.toThrow();
  });
});
