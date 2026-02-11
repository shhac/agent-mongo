import type { Command } from "commander";

const USAGE_TEXT = `connection — Manage MongoDB connections

COMMANDS:
  connection add <alias> <uri> [--database <db>] [--credential <name>] [--default]
    Save a MongoDB connection. Alias is a short name (e.g. local, staging, prod).
    URI: mongodb://... or mongodb+srv://...
    --database overrides the database from the URI.
    --credential references a stored credential for authentication.

  connection update <alias> [--credential <name>] [--no-credential] [--database <db>]
    Update a saved connection. Only specified fields are changed.
    --credential sets or changes the credential reference.
    --no-credential removes the credential from the connection.

  connection remove <alias>
    Remove a saved connection.

  connection list
    List all saved connections (connection strings redacted, credential names shown).

  connection test [-c <alias>]
    Ping MongoDB to verify connectivity. Uses default connection if -c omitted.

  connection set-default <alias>
    Set which connection is used when -c is not specified.

CREDENTIALS: Use "credential add" to store reusable auth. Reference via --credential.

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
