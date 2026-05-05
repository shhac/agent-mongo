import { Command, MaximumNArgs } from "vipvot";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { printError, printJsonRaw } from "../../lib/output.ts";
import { resolveConnectionAlias } from "../_globals.ts";

export function buildTestCommand(): Command {
  return Command({
    use: "test [alias]",
    short: "Test a MongoDB connection (ping)",
    args: MaximumNArgs(1),
    run: async (_cmd, args) => {
      const [aliasArg] = args;
      try {
        const alias = aliasArg ?? resolveConnectionAlias();
        const { client, alias: resolved } = await getMongoClient(alias);
        try {
          const result = await client.db("admin").command({ ping: 1 });
          printJsonRaw({
            ok: true,
            alias: resolved,
            ping: result,
          });
        } finally {
          await closeAllClients();
        }
      } catch (err) {
        printError(err instanceof Error ? err.message : "Connection test failed");
      }
    },
  });
}
