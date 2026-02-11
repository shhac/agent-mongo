import type { Command } from "commander";
import { getConnections, getDefaultConnectionAlias } from "../../lib/config.ts";
import { printJsonRaw } from "../../lib/output.ts";

export function registerList(connection: Command): void {
  connection
    .command("list")
    .description("List saved connections")
    .action(() => {
      const connections = getConnections();
      const defaultAlias = getDefaultConnectionAlias();

      const items = Object.entries(connections).map(([alias, conn]) => ({
        alias,
        connection_string: conn.connection_string,
        database: conn.database,
        credential: conn.credential,
        default: alias === defaultAlias,
      }));

      printJsonRaw({ connections: items });
    });
}
