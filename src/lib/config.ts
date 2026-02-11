import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";

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

export type Connection = {
  connection_string: string;
  name?: string;
  database?: string;
};

export type DefaultsSettings = {
  limit?: number;
  sampleSize?: number;
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
