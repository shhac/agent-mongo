import { Command, NoArgs } from "vipvot";
import { getConnections, getDefaultConnectionAlias } from "../../lib/config.ts";
import { printJsonRaw } from "../../lib/output.ts";

export function buildListCommand(): Command {
  return Command({
    use: "list",
    short: "List saved connections",
    args: NoArgs,
    run: () => {
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
    },
  });
}
