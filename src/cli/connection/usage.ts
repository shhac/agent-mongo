import { Command, NoArgs } from "vipvot";

const USAGE_TEXT = `connection — Manage MongoDB connections

COMMANDS:
  connection add <alias> <uri> [--database <db>] [--credential <name>] [--default]
    Save a MongoDB connection. Alias is a short name (e.g. local, staging, prod).
    URI: mongodb://... or mongodb+srv://...
    --database overrides the database from the URI.
    --credential references a stored credential for authentication.
    --default sets this connection as the default.

  connection update <alias> [--credential <name>] [--clear-credential] [--database <db>]
    Update a saved connection. Only specified fields are changed.
    --credential sets or changes the credential reference.
    --clear-credential removes the credential from the connection (mutually exclusive with --credential).

  connection remove <alias>
    Remove a saved connection.

  connection list
    List all saved connections with credential names.

  connection test [alias] [-c <alias>]
    Ping MongoDB to verify connectivity. Alias as argument or -c flag. Uses default if omitted.

  connection set-default <alias>
    Set which connection is used when -c is not specified.

CREDENTIALS: Use "credential add" to store reusable auth. Reference via --credential.

RESOLUTION ORDER: -c flag > AGENT_MONGO_CONNECTION env > config default > error

CONFIG: ~/.config/agent-mongo/config.json (respects XDG_CONFIG_HOME)
`;

export function buildUsageCommand(): Command {
  return Command({
    use: "usage",
    short: "Print connection command documentation (LLM-optimized)",
    args: NoArgs,
    run: () => {
      console.log(USAGE_TEXT.trim());
    },
  });
}
