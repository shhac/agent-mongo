---
name: agent-mongo
description: |
  Read-only MongoDB CLI for AI agents. Use when:
  - Exploring MongoDB databases, collections, schemas, or indexes
  - Querying documents (find, get by ID, count, sample, distinct, aggregate)
  - Managing MongoDB connections or credentials
  - Checking database or collection statistics
  Triggers: "mongodb", "mongo query", "mongo find", "mongo schema", "mongo collection", "mongo database", "mongo connection", "mongo aggregate", "query mongodb", "mongo stats"
allowed-tools: Bash(agent-mongo *) Read Grep Glob
---

# MongoDB exploration with `agent-mongo`

`agent-mongo` is a read-only CLI binary on `$PATH`. Default output is **NDJSON** — one JSON record per line on stdout. List commands emit one record per item, then `@`-prefixed metadata lines (`{"@meta": ...}` for context, `{"@pagination": ...}` for paging). Errors go to stderr as one JSON line `{"error": "...", "fixable_by": "agent"|"human"|"retry", "hint": "..."}` with a non-zero exit.

`fixable_by` tells you who resolves the error: `agent` — fix your input and retry; `human` — needs the user (auth, a GUI dialog); `retry` — transient, run it again.

## Quick start (connections)

Set up a connection:

```bash
agent-mongo connection add local "mongodb://localhost:27017/myapp" --default
agent-mongo connection test
```

For authenticated connections, store credentials separately:

```bash
agent-mongo credential add acme --username deploy --password secret
agent-mongo connection add prod "mongodb+srv://cluster.example.net/myapp" --credential acme --default
```

## Exploring a database

```bash
agent-mongo database list                                # all databases with sizes
agent-mongo collection list myapp                        # all collections in myapp
agent-mongo collection schema myapp users                # infer schema from samples
agent-mongo collection schema myapp users --depth 2      # limit nesting depth
agent-mongo collection schema myapp events --limit 50    # paginate large schemas
agent-mongo collection schema myapp events --limit 50 --skip 50  # next page
agent-mongo collection indexes myapp users               # index key patterns
agent-mongo collection stats myapp orders                # document count, sizes
agent-mongo database stats myapp                         # database-level statistics
```

## Querying documents

```bash
agent-mongo query find myapp users --filter '{"age":{"$gte":21}}' --limit 10
agent-mongo query find myapp orders --sort '{"createdAt":-1}' --projection '{"status":1,"total":1}'
agent-mongo query get myapp users 665a1b2c3d4e5f6a7b8c9d0e      # by _id (auto-detects ObjectId)
agent-mongo query get myapp users 665a1b2c3d4e5f6a7b8c9d0e --projection '{"name":1,"email":1}'
agent-mongo query count myapp orders --filter '{"status":"pending"}'
agent-mongo query sample myapp users --size 10                    # random documents
agent-mongo query sample myapp users --size 10 --filter '{"status":"active"}'  # filtered sample
agent-mongo query distinct myapp orders status                    # unique values
```

`query find` emits one record per document, then a `{"@pagination": {"has_more": ..., "total_items": ...}}` line — `has_more` means more documents match beyond the limit, `total_items` is the full matching count.

All JSON arguments (`--filter`, `--sort`, `--projection`, `--pipeline`) accept MongoDB Extended JSON for BSON types:

```bash
agent-mongo query find myapp events --filter '{"createdAt":{"$gt":{"$date":"2026-01-01T00:00:00Z"}}}'
agent-mongo query find myapp users --filter '{"_id":{"$oid":"665a1b2c3d4e5f6a7b8c9d0e"}}'
```

## Aggregation

```bash
agent-mongo query aggregate myapp orders '[{"$group":{"_id":"$status","count":{"$sum":1}}}]'
agent-mongo query aggregate myapp orders --pipeline '[{"$group":{"_id":"$status","count":{"$sum":1}}}]'
agent-mongo query aggregate myapp events '[{"$match":{"type":"purchase"}},{"$group":{"_id":"$userId","total":{"$sum":"$amount"}}}]'
```

Pipeline can be passed as a positional argument, via `--pipeline` flag, or piped via stdin.

Write stages (`$out`, `$merge`) are rejected — the CLI is strictly read-only.

## Output format

Default is NDJSON (`-f jsonl`). Switch with `-f/--format`:

```bash
agent-mongo database list -f json     # pretty {"data": [...], ...meta} envelope for lists
agent-mongo query count myapp users -f yaml
```

`-f json` gives a single pretty envelope (`{"data": [...]}` for lists, a bare pretty object for single results) — easier to eyeball than NDJSON when you're reading output yourself.

## Connection management

```bash
agent-mongo connection list                              # saved connections + defaults
agent-mongo connection add staging "mongodb://..." --credential acme
agent-mongo connection update prod --credential new-cred
agent-mongo connection set-default staging
agent-mongo connection remove old-conn
agent-mongo connection test prod                         # verify connectivity (positional alias)
agent-mongo connection test -c prod                      # also works with -c flag
```

Connection resolution: `-c` flag > `AGENT_MONGO_CONNECTION` env > config default > error listing available connections.

## Credential management

```bash
agent-mongo credential add acme --username deploy --password secret
agent-mongo credential list                              # passwords always redacted
agent-mongo credential remove acme --force               # even if connections reference it
```

Credentials are stored separately from connections, in the OS secret store when available (macOS Keychain, Linux Secret Service, Windows Credential Manager) with plaintext-config fallback. `credential list` shows the `storage` source per credential. Plaintext entries are auto-upgraded to the keychain on first use (reported via a stderr `{"notice": ...}` line — not an error). When you rotate a password, just re-add the credential — all connections referencing it pick up the new auth automatically.

### LLM-safe entry with `--form`

When you (the agent) are driving the CLI on a user's local machine, do not put a real password on the command line — you'll see it in your own argv. Use `--form` to escalate the secret to a native OS dialog instead. The user types into the OS popup; the secret never enters the agent's context.

```bash
agent-mongo credential add acme --form                              # both fields prompted
agent-mongo credential add acme --username deploy --form            # only password prompted
```

Failure modes return a structured error with `fixable_by`:

- `human` — no GUI session available (SSH, headless host). Ask the user to run on their local machine, or fall back to non-interactive `--username <u> --password <secret>`.
- `retry` — user cancelled the dialog. Re-running the same command is the right next step.

## Truncation

Any string field exceeding `truncation.maxLength` (default 200) gets truncated with `…` and a companion `{field}Length` key showing original length.

```bash
agent-mongo --full query find myapp posts                # expand all fields
agent-mongo --expand description query find myapp posts  # expand specific field
```

These are global flags — place them before or after the command.

## Timeout

Default timeout is 30s (configurable via `query.timeout`). Applies to both connection and query phases. Override per-command with `-t/--timeout <ms>`:

```bash
agent-mongo --timeout 60000 query find myapp large_collection --filter '{"status":"active"}'
agent-mongo --timeout 120000 collection schema myapp events
```

On timeout (MongoDB code 50), the error hint suggests increasing the timeout or checking indexes.

## Configuration

```bash
agent-mongo config list-keys                             # all keys with defaults/ranges
agent-mongo config set defaults.limit 50
agent-mongo config get query.timeout
agent-mongo config reset                                 # restore defaults
```

Key settings: `defaults.limit` (20), `defaults.sampleSize` (5), `defaults.schemaSampleSize` (100), `query.timeout` (30000ms), `query.maxDocuments` (100), `truncation.maxLength` (200).

## MCP server

`agent-mongo mcp` runs the read-only data commands (`database`, `collection`, `query`, `connection`) as MCP tools over stdio (or Streamable HTTP with `--http <addr>`). Credential and config commands are not exposed. See `agent-mongo mcp usage` for registration, OAuth, and Tailscale details.

## Safety

- **Read-only**: No write operations exist
- **Aggregation**: `$out` and `$merge` stages rejected
- **Result cap**: `query.maxDocuments` (default 100)
- **Timeout**: applies to both connections and queries (default 30s), override per-command with `-t/--timeout <ms>`

## Per-command usage docs

Every command group has a `usage` subcommand with detailed, LLM-optimized docs:

```bash
agent-mongo usage                  # top-level overview
agent-mongo connection usage       # connection commands
agent-mongo credential usage       # credential management
agent-mongo database usage          # database commands
agent-mongo collection usage       # collection commands
agent-mongo query usage            # all query commands
agent-mongo config usage           # settings keys, defaults, validation
agent-mongo mcp usage              # MCP server transports and registration
```

Use `agent-mongo <command> usage` when you need deep detail on a specific domain before acting.

## References

- [references/commands.md](references/commands.md): full command map + all flags
- [references/output.md](references/output.md): NDJSON output shapes + field details
