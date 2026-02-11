import { afterAll, beforeAll, beforeEach, describe, expect, test } from "bun:test";
import { existsSync, mkdirSync, readFileSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";

const TEST_CONFIG_DIR = join(tmpdir(), `agent-mongo-config-test-${Date.now()}`);

// Set XDG before importing config module so it uses our temp dir
process.env.XDG_CONFIG_HOME = TEST_CONFIG_DIR;

const {
  readConfig,
  writeConfig,
  storeConnection,
  getConnection,
  getConnections,
  getDefaultConnectionAlias,
  removeConnection,
  setDefaultConnection,
  getSettings,
  getSetting,
  updateSetting,
  resetSettings,
} = await import("../src/lib/config.ts");

describe("config CRUD", () => {
  beforeAll(() => {
    process.env.XDG_CONFIG_HOME = TEST_CONFIG_DIR;
    mkdirSync(join(TEST_CONFIG_DIR, "agent-mongo"), { recursive: true });
    writeConfig({});
  });

  afterAll(() => {
    rmSync(TEST_CONFIG_DIR, { recursive: true, force: true });
    delete process.env.XDG_CONFIG_HOME;
  });

  test("readConfig returns empty object when no config exists", () => {
    rmSync(join(TEST_CONFIG_DIR, "agent-mongo", "config.json"), { force: true });
    expect(readConfig()).toEqual({});
  });

  test("writeConfig creates config file", () => {
    writeConfig({ default_connection: "test" });
    const configPath = join(TEST_CONFIG_DIR, "agent-mongo", "config.json");
    expect(existsSync(configPath)).toBe(true);
    const raw = JSON.parse(readFileSync(configPath, "utf8"));
    expect(raw.default_connection).toBe("test");
  });

  test("readConfig reads back what was written", () => {
    writeConfig({ default_connection: "prod", connections: { prod: { connection_string: "mongodb://localhost" } } });
    const config = readConfig();
    expect(config.default_connection).toBe("prod");
    expect(config.connections?.prod?.connection_string).toBe("mongodb://localhost");
  });
});

describe("connections", () => {
  beforeEach(() => {
    process.env.XDG_CONFIG_HOME = TEST_CONFIG_DIR;
    mkdirSync(join(TEST_CONFIG_DIR, "agent-mongo"), { recursive: true });
    writeConfig({});
  });

  afterAll(() => {
    rmSync(TEST_CONFIG_DIR, { recursive: true, force: true });
    delete process.env.XDG_CONFIG_HOME;
  });

  test("getConnections returns empty object when none stored", () => {
    expect(getConnections()).toEqual({});
  });

  test("storeConnection stores a connection and sets it as default", () => {
    storeConnection("local", { connection_string: "mongodb://localhost:27017" });
    expect(getConnection("local")).toEqual({ connection_string: "mongodb://localhost:27017" });
    expect(getDefaultConnectionAlias()).toBe("local");
  });

  test("storeConnection does not overwrite existing default", () => {
    storeConnection("local", { connection_string: "mongodb://localhost:27017" });
    storeConnection("prod", { connection_string: "mongodb://prod:27017", name: "Production" });
    expect(getDefaultConnectionAlias()).toBe("local");
    expect(Object.keys(getConnections())).toEqual(["local", "prod"]);
  });

  test("storeConnection with name and database", () => {
    storeConnection("dev", {
      connection_string: "mongodb://dev:27017",
      name: "Development",
      database: "mydb",
    });
    const conn = getConnection("dev");
    expect(conn?.name).toBe("Development");
    expect(conn?.database).toBe("mydb");
  });

  test("getConnection returns undefined for unknown alias", () => {
    expect(getConnection("nonexistent")).toBeUndefined();
  });

  test("setDefaultConnection switches the active connection", () => {
    storeConnection("a", { connection_string: "mongodb://a" });
    storeConnection("b", { connection_string: "mongodb://b" });
    expect(getDefaultConnectionAlias()).toBe("a");
    setDefaultConnection("b");
    expect(getDefaultConnectionAlias()).toBe("b");
  });

  test("setDefaultConnection throws for unknown alias", () => {
    expect(() => setDefaultConnection("nonexistent")).toThrow("Unknown connection");
  });

  test("removeConnection removes and reassigns default", () => {
    storeConnection("first", { connection_string: "mongodb://first" });
    storeConnection("second", { connection_string: "mongodb://second" });
    removeConnection("first");
    expect(getDefaultConnectionAlias()).toBe("second");
    expect(getConnection("first")).toBeUndefined();
  });

  test("removeConnection clears default when last connection removed", () => {
    storeConnection("only", { connection_string: "mongodb://only" });
    removeConnection("only");
    expect(getDefaultConnectionAlias()).toBeUndefined();
    expect(getConnections()).toEqual({});
  });

  test("removeConnection throws for unknown alias", () => {
    expect(() => removeConnection("nonexistent")).toThrow("Unknown connection");
  });
});

describe("settings", () => {
  beforeEach(() => {
    process.env.XDG_CONFIG_HOME = TEST_CONFIG_DIR;
    mkdirSync(join(TEST_CONFIG_DIR, "agent-mongo"), { recursive: true });
    writeConfig({});
  });

  afterAll(() => {
    rmSync(TEST_CONFIG_DIR, { recursive: true, force: true });
    delete process.env.XDG_CONFIG_HOME;
  });

  test("getSettings returns empty object when no settings stored", () => {
    expect(getSettings()).toEqual({});
  });

  test("updateSetting persists a top-level setting", () => {
    updateSetting("defaults.limit", 50);
    expect(getSetting("defaults.limit")).toBe(50);
  });

  test("updateSetting persists nested settings", () => {
    updateSetting("query.timeout", 5000);
    expect(getSetting("query.timeout")).toBe(5000);
  });

  test("updateSetting creates intermediate objects", () => {
    updateSetting("truncation.maxLength", 300);
    const settings = getSettings();
    expect(settings.truncation?.maxLength).toBe(300);
  });

  test("getSetting returns undefined for non-existent key", () => {
    expect(getSetting("nonexistent.key")).toBeUndefined();
  });

  test("getSetting traverses dotted paths", () => {
    updateSetting("defaults.limit", 25);
    updateSetting("defaults.sampleSize", 100);
    expect(getSetting("defaults.limit")).toBe(25);
    expect(getSetting("defaults.sampleSize")).toBe(100);
  });

  test("resetSettings clears all settings", () => {
    updateSetting("defaults.limit", 50);
    updateSetting("query.timeout", 5000);
    resetSettings();
    expect(getSettings()).toEqual({});
  });

  test("settings isolation: updateSetting does not touch connection data", () => {
    storeConnection("test", { connection_string: "mongodb://test" });
    updateSetting("defaults.limit", 10);
    expect(getConnection("test")).toEqual({ connection_string: "mongodb://test" });
    expect(getSetting("defaults.limit")).toBe(10);
  });
});
