import type { Command } from "commander";
import { getConnections, getDefaultConnectionAlias } from "../../lib/config.ts";
import { redactSecret } from "../../lib/redact.ts";
import { printJson } from "../../lib/output.ts";

export function registerList(connection: Command): void {
  connection
    .command("list")
    .description("List saved connections (connection strings redacted)")
    .action(() => {
      const connections = getConnections();
      const defaultAlias = getDefaultConnectionAlias();

      const items = Object.entries(connections).map(([alias, conn]) => ({
        alias,
        connection_string: redactSecret(conn.connection_string),
        database: conn.database,
        default: alias === defaultAlias,
      }));

      printJson({ connections: items });
    });
}
