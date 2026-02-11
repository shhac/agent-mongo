import type { Command } from "commander";

const USAGE_TEXT = `connection — Manage MongoDB connections

COMMANDS:
  connection add <alias> <uri> [--database <db>] [--default]
    Save a MongoDB connection. Alias is a short name (e.g. local, staging, prod).
    URI: mongodb://... or mongodb+srv://...
    --database overrides the database from the URI.

  connection remove <alias>
    Remove a saved connection.

  connection list
    List all saved connections (connection strings redacted).

  connection test [-c <alias>]
    Ping MongoDB to verify connectivity. Uses default connection if -c omitted.

  connection set-default <alias>
    Set which connection is used when -c is not specified.

RESOLUTION ORDER: -c flag > AGENT_MONGO_CONNECTION env > config default > error

CONFIG: ~/.config/agent-mongo/config.json (respects XDG_CONFIG_HOME)
`;

export function registerUsage(connection: Command): void {
  connection
    .command("usage")
    .description("Print connection command documentation (LLM-optimized)")
    .action(() => {
      console.log(USAGE_TEXT.trim());
    });
}
