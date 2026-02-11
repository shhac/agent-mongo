import type { Command } from "commander";

const USAGE_TEXT = `credential — Manage stored credentials for MongoDB authentication

COMMANDS:
  credential add <name> --username <user> --password <pass>
    Store a named credential. Overwrites if name already exists.
    Credentials are referenced by connections via --credential.

  credential remove <name>
    Remove a stored credential. Fails if any connection references it.
    --force removes anyway and clears credential refs from those connections.

  credential list
    List all stored credentials (passwords always redacted).
    Shows which connections reference each credential.

WORKFLOW:
  1. Store credential:   agent-mongo credential add acme --username deploy --password secret
  2. Add connections:    agent-mongo connection add prod <uri> --credential acme
                         agent-mongo connection add staging <uri> --credential acme
  3. Rotate password:    agent-mongo credential add acme --username deploy --password new-secret
     All connections referencing "acme" pick up the new password automatically.

RESOLUTION: When a connection references a credential, auth is passed to the MongoDB
driver. Connections without a credential use the URI as-is (backward compatible).

CONFIG: ~/.config/agent-mongo/config.json (respects XDG_CONFIG_HOME)
`;

export function registerUsage(credential: Command): void {
  credential
    .command("usage")
    .description("Print credential command documentation (LLM-optimized)")
    .action(() => {
      console.log(USAGE_TEXT.trim());
    });
}
