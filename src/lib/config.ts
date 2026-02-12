import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";
import { keychainGet, keychainSet, keychainDelete, KEYCHAIN_SERVICE } from "./keychain.ts";

function getConfigDir(): string {
  const xdg = process.env.XDG_CONFIG_HOME?.trim();
  if (xdg) {
    return join(xdg, "agent-mongo");
  }
  return join(homedir(), ".config", "agent-mongo");
}

function ensureConfigDir(): string {
  const dir = getConfigDir();
  if (!existsSync(dir)) {
    mkdirSync(dir, { recursive: true });
  }
  return dir;
}

export type Credential = {
  username: string;
  password: string;
};

export type Connection = {
  connection_string: string;
  name?: string;
  database?: string;
  credential?: string;
};

export type DefaultsSettings = {
  limit?: number;
  sampleSize?: number;
  schemaSampleSize?: number;
};

export type QuerySettings = {
  timeout?: number;
  maxDocuments?: number;
};

export type TruncationSettings = {
  maxLength?: number;
};

export type Settings = {
  defaults?: DefaultsSettings;
  query?: QuerySettings;
  truncation?: TruncationSettings;
};

export type Config = {
  default_connection?: string;
  connections?: Record<string, Connection>;
  credentials?: Record<string, Credential>;
  settings?: Settings;
};

export function readConfig(): Config {
  const configPath = join(getConfigDir(), "config.json");
  if (!existsSync(configPath)) {
    return {};
  }
  try {
    return JSON.parse(readFileSync(configPath, "utf8")) as Config;
  } catch {
    return {};
  }
}

export function writeConfig(config: Config): void {
  const dir = ensureConfigDir();
  writeFileSync(join(dir, "config.json"), `${JSON.stringify(config, null, 2)}\n`, "utf8");
}

export function getConnection(alias: string): Connection | undefined {
  return readConfig().connections?.[alias];
}

export function getDefaultConnectionAlias(): string | undefined {
  return readConfig().default_connection;
}

export function getConnections(): Record<string, Connection> {
  return readConfig().connections ?? {};
}

export function storeConnection(alias: string, connection: Connection): void {
  const config = readConfig();
  config.connections = config.connections ?? {};
  config.connections[alias] = connection;
  if (!config.default_connection) {
    config.default_connection = alias;
  }
  writeConfig(config);
}

export function removeConnection(alias: string): void {
  const config = readConfig();
  if (!config.connections?.[alias]) {
    throw new Error(
      `Unknown connection: "${alias}". Valid: ${Object.keys(config.connections ?? {}).join(", ") || "(none)"}`,
    );
  }
  delete config.connections[alias];
  if (config.default_connection === alias) {
    const remaining = Object.keys(config.connections);
    config.default_connection = remaining.length > 0 ? remaining[0] : undefined;
  }
  if (Object.keys(config.connections).length === 0) {
    delete config.connections;
  }
  writeConfig(config);
}

export function setDefaultConnection(alias: string): void {
  const config = readConfig();
  if (!config.connections?.[alias]) {
    throw new Error(
      `Unknown connection: "${alias}". Valid: ${Object.keys(config.connections ?? {}).join(", ") || "(none)"}`,
    );
  }
  config.default_connection = alias;
  writeConfig(config);
}

export function getSettings(): Settings {
  return readConfig().settings ?? {};
}

export function getSetting(key: string): unknown {
  const settings = getSettings();
  const parts = key.split(".");
  let current: unknown = settings;
  for (const part of parts) {
    if (current === null || current === undefined || typeof current !== "object") {
      return undefined;
    }
    current = (current as Record<string, unknown>)[part];
  }
  return current;
}

export function updateSetting(key: string, value: unknown): void {
  const config = readConfig();
  config.settings = config.settings ?? {};

  const parts = key.split(".");
  let current: Record<string, unknown> = config.settings as Record<string, unknown>;
  for (let i = 0; i < parts.length - 1; i++) {
    const part = parts[i]!;
    if (current[part] === undefined || typeof current[part] !== "object") {
      current[part] = {};
    }
    current = current[part] as Record<string, unknown>;
  }
  current[parts.at(-1)!] = value;

  writeConfig(config);
}

export function resetSettings(): void {
  const config = readConfig();
  delete config.settings;
  writeConfig(config);
}

export function getCredential(alias: string): Credential | undefined {
  const cred = readConfig().credentials?.[alias];
  if (!cred) {
    return undefined;
  }

  if (cred.username === "__KEYCHAIN__" || cred.password === "__KEYCHAIN__") {
    const username =
      cred.username === "__KEYCHAIN__"
        ? keychainGet(`username:${alias}`, KEYCHAIN_SERVICE)
        : cred.username;
    const password =
      cred.password === "__KEYCHAIN__"
        ? keychainGet(`password:${alias}`, KEYCHAIN_SERVICE)
        : cred.password;
    if (!username || !password) {
      return undefined;
    }
    return { username, password };
  }

  return cred;
}

export function getCredentials(): Record<string, Credential> {
  return readConfig().credentials ?? {};
}

export function storeCredential(
  alias: string,
  credential: Credential,
): { storage: "keychain" | "config" } {
  const config = readConfig();
  config.credentials = config.credentials ?? {};

  const usernameStored = keychainSet({
    account: `username:${alias}`,
    value: credential.username,
    service: KEYCHAIN_SERVICE,
  });
  const passwordStored = keychainSet({
    account: `password:${alias}`,
    value: credential.password,
    service: KEYCHAIN_SERVICE,
  });

  if (usernameStored && passwordStored) {
    config.credentials[alias] = { username: "__KEYCHAIN__", password: "__KEYCHAIN__" };
    writeConfig(config);
    return { storage: "keychain" };
  }

  // Fallback: store plaintext (clean up any partial keychain entries)
  keychainDelete(`username:${alias}`, KEYCHAIN_SERVICE);
  keychainDelete(`password:${alias}`, KEYCHAIN_SERVICE);
  config.credentials[alias] = credential;
  writeConfig(config);
  return { storage: "config" };
}

export function getConnectionsUsingCredential(credentialAlias: string): string[] {
  const connections = getConnections();
  return Object.entries(connections)
    .filter(([, conn]) => conn.credential === credentialAlias)
    .map(([alias]) => alias);
}

export function removeCredential(alias: string): void {
  const config = readConfig();
  if (!config.credentials?.[alias]) {
    throw new Error(
      `Unknown credential: "${alias}". Valid: ${Object.keys(config.credentials ?? {}).join(", ") || "(none)"}`,
    );
  }
  const usedBy = getConnectionsUsingCredential(alias);
  if (usedBy.length > 0) {
    throw new Error(
      `Credential "${alias}" is used by connections: ${usedBy.join(", ")}. Remove or update those connections first.`,
    );
  }
  keychainDelete(`username:${alias}`, KEYCHAIN_SERVICE);
  keychainDelete(`password:${alias}`, KEYCHAIN_SERVICE);
  delete config.credentials[alias];
  if (Object.keys(config.credentials).length === 0) {
    delete config.credentials;
  }
  writeConfig(config);
}

export function getCredentialStorage(alias: string): "keychain" | "config" {
  const cred = readConfig().credentials?.[alias];
  if (!cred) {
    return "config";
  }
  return cred.username === "__KEYCHAIN__" && cred.password === "__KEYCHAIN__"
    ? "keychain"
    : "config";
}

export function updateConnection(
  alias: string,
  updates: Partial<Omit<Connection, "connection_string">>,
): void {
  const config = readConfig();
  if (!config.connections?.[alias]) {
    throw new Error(
      `Unknown connection: "${alias}". Valid: ${Object.keys(config.connections ?? {}).join(", ") || "(none)"}`,
    );
  }
  const conn = config.connections[alias]!;
  if (updates.database !== undefined) {
    conn.database = updates.database || undefined;
  }
  if ("credential" in updates) {
    conn.credential = updates.credential || undefined;
  }
  writeConfig(config);
}
