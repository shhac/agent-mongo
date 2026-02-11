import { MongoClient, type Db } from "mongodb";
import { getConnection, getDefaultConnectionAlias, getConnections } from "../lib/config.ts";

type MongoContext = {
  client: MongoClient;
  db: Db;
  alias: string;
};

const activeClients: Map<string, MongoClient> = new Map();

/**
 * Resolve connection alias and return a connected MongoClient + Db.
 *
 * Resolution order:
 * 1. Explicit -c flag
 * 2. AGENT_MONGO_CONNECTION env var
 * 3. Config default connection
 * 4. Error listing available connections
 */
export async function getMongoClient(aliasFlag?: string): Promise<MongoContext> {
  const alias = resolveAlias(aliasFlag);
  const conn = getConnection(alias);
  if (!conn) {
    const available = Object.keys(getConnections());
    throw new Error(
      `Connection "${alias}" not found. Available: ${available.join(", ") || "(none)"}. Run: agent-mongo connection add <alias> <connection-string>`,
    );
  }

  const existing = activeClients.get(alias);
  if (existing) {
    const dbName = conn.database ?? parseDbFromUri(conn.connection_string);
    return { client: existing, db: existing.db(dbName), alias };
  }

  const client = new MongoClient(conn.connection_string);
  await client.connect();
  activeClients.set(alias, client);

  const dbName = conn.database ?? parseDbFromUri(conn.connection_string);
  return { client, db: client.db(dbName), alias };
}

function resolveAlias(flag?: string): string {
  if (flag?.trim()) {
    return flag.trim();
  }

  const envAlias = process.env.AGENT_MONGO_CONNECTION?.trim();
  if (envAlias) {
    return envAlias;
  }

  const defaultAlias = getDefaultConnectionAlias();
  if (defaultAlias) {
    return defaultAlias;
  }

  const available = Object.keys(getConnections());
  throw new Error(
    `No connection specified. Available: ${available.join(", ") || "(none)"}. Run: agent-mongo connection add <alias> <connection-string>`,
  );
}

function parseDbFromUri(uri: string): string | undefined {
  try {
    const url = new URL(uri);
    const path = url.pathname.replace(/^\//, "");
    return path || undefined;
  } catch {
    return undefined;
  }
}

export async function closeAllClients(): Promise<void> {
  for (const client of activeClients.values()) {
    try {
      await client.close();
    } catch {
      // ignore close errors
    }
  }
  activeClients.clear();
}
