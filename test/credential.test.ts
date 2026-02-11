import { afterAll, beforeAll, beforeEach, describe, expect, test } from "bun:test";
import { mkdirSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";

const TEST_CONFIG_DIR = join(tmpdir(), `agent-mongo-credential-test-${Date.now()}`);

// Set XDG before importing config module so it uses our temp dir
process.env.XDG_CONFIG_HOME = TEST_CONFIG_DIR;

const {
  writeConfig,
  storeCredential,
  getCredential,
  getCredentials,
  removeCredential,
  storeConnection,
  getConnectionsUsingCredential,
  updateConnection,
} = await import("../src/lib/config.ts");

const { resolveClientOptions } = await import("../src/mongo/client.ts");

describe("credential CRUD", () => {
  beforeEach(() => {
    process.env.XDG_CONFIG_HOME = TEST_CONFIG_DIR;
    mkdirSync(join(TEST_CONFIG_DIR, "agent-mongo"), { recursive: true });
    writeConfig({});
  });

  afterAll(() => {
    rmSync(TEST_CONFIG_DIR, { recursive: true, force: true });
    delete process.env.XDG_CONFIG_HOME;
  });

  test("getCredentials returns empty object when none stored", () => {
    expect(getCredentials()).toEqual({});
  });

  test("storeCredential stores a credential", () => {
    storeCredential("acme", { username: "deploy", password: "secret123" });
    expect(getCredential("acme")).toEqual({ username: "deploy", password: "secret123" });
  });

  test("storeCredential overwrites existing credential (upsert)", () => {
    storeCredential("acme", { username: "deploy", password: "old-pass" });
    storeCredential("acme", { username: "deploy", password: "new-pass" });
    expect(getCredential("acme")).toEqual({ username: "deploy", password: "new-pass" });
  });

  test("multiple credentials stored independently", () => {
    storeCredential("acme", { username: "deploy", password: "acme-pass" });
    storeCredential("globex", { username: "admin", password: "globex-pass" });
    expect(Object.keys(getCredentials())).toEqual(["acme", "globex"]);
    expect(getCredential("acme")?.username).toBe("deploy");
    expect(getCredential("globex")?.username).toBe("admin");
  });

  test("getCredential returns undefined for unknown name", () => {
    expect(getCredential("nonexistent")).toBeUndefined();
  });

  test("removeCredential removes a credential", () => {
    storeCredential("acme", { username: "deploy", password: "secret" });
    removeCredential("acme");
    expect(getCredential("acme")).toBeUndefined();
    expect(getCredentials()).toEqual({});
  });

  test("removeCredential throws for unknown name", () => {
    expect(() => removeCredential("nonexistent")).toThrow("Unknown credential");
  });

  test("removeCredential throws when credential is in use", () => {
    storeCredential("acme", { username: "deploy", password: "secret" });
    storeConnection("prod", {
      connection_string: "mongodb://prod:27017",
      credential: "acme",
    });
    expect(() => removeCredential("acme")).toThrow("used by connections: prod");
  });

  test("removeCredential succeeds after clearing connection references", () => {
    storeCredential("acme", { username: "deploy", password: "secret" });
    storeConnection("prod", {
      connection_string: "mongodb://prod:27017",
      credential: "acme",
    });
    updateConnection("prod", { credential: undefined });
    removeCredential("acme");
    expect(getCredential("acme")).toBeUndefined();
  });

  test("credential isolation: storeCredential does not touch connection data", () => {
    storeConnection("local", { connection_string: "mongodb://localhost" });
    storeCredential("acme", { username: "deploy", password: "secret" });
    expect(getCredential("acme")).toEqual({ username: "deploy", password: "secret" });
  });
});

describe("getConnectionsUsingCredential", () => {
  beforeEach(() => {
    process.env.XDG_CONFIG_HOME = TEST_CONFIG_DIR;
    mkdirSync(join(TEST_CONFIG_DIR, "agent-mongo"), { recursive: true });
    writeConfig({});
  });

  afterAll(() => {
    rmSync(TEST_CONFIG_DIR, { recursive: true, force: true });
    delete process.env.XDG_CONFIG_HOME;
  });

  test("returns empty array when no connections reference the credential", () => {
    storeCredential("acme", { username: "deploy", password: "secret" });
    storeConnection("local", { connection_string: "mongodb://localhost" });
    expect(getConnectionsUsingCredential("acme")).toEqual([]);
  });

  test("returns connections that reference the credential", () => {
    storeCredential("acme", { username: "deploy", password: "secret" });
    storeConnection("staging", {
      connection_string: "mongodb://staging:27017",
      credential: "acme",
    });
    storeConnection("prod", {
      connection_string: "mongodb://prod:27017",
      credential: "acme",
    });
    storeConnection("local", { connection_string: "mongodb://localhost" });
    expect(getConnectionsUsingCredential("acme").sort()).toEqual(["prod", "staging"]);
  });
});

describe("resolveClientOptions", () => {
  beforeAll(() => {
    process.env.XDG_CONFIG_HOME = TEST_CONFIG_DIR;
    mkdirSync(join(TEST_CONFIG_DIR, "agent-mongo"), { recursive: true });
    writeConfig({});
  });

  afterAll(() => {
    rmSync(TEST_CONFIG_DIR, { recursive: true, force: true });
    delete process.env.XDG_CONFIG_HOME;
  });

  test("returns empty options when no credential alias", () => {
    expect(resolveClientOptions()).toEqual({});
    expect(resolveClientOptions(undefined)).toEqual({});
  });

  test("returns auth options for valid credential", () => {
    storeCredential("acme", { username: "deploy", password: "secret123" });
    const opts = resolveClientOptions("acme");
    expect(opts.auth).toEqual({ username: "deploy", password: "secret123" });
  });

  test("throws for unknown credential alias", () => {
    expect(() => resolveClientOptions("nonexistent")).toThrow('Credential "nonexistent" not found');
  });
});
