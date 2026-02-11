# agent-mongo

Read-only MongoDB CLI for AI agents. TypeScript + Bun, compiled to standalone binaries.

## Architecture

```
src/
├── index.ts                     # CLI entry — registers all commands via commander
├── cli/
│   ├── connection/              # connection add/remove/update/list/test/set-default
│   ├── credential/              # credential add/remove/list
│   ├── config/                  # config get/set/reset/list-keys
│   ├── db/                      # db list/stats
│   ├── collection/              # collection list/schema/indexes/stats
│   ├── query/                   # query find/get/count/sample/distinct/aggregate
│   └── usage/                   # LLM-optimized top-level usage text
├── lib/
│   ├── config.ts                # ~/.config/agent-mongo/ config + connection + credential storage
│   ├── output.ts                # printJson, printPaginated, printError
│   ├── compact-json.ts          # pruneEmpty() — strips null/empty fields
│   ├── truncation.ts            # Generic string truncation with {field}Length companion
│   ├── redact.ts                # Connection string redaction for display
│   └── version.ts               # Version from build-time define / env / package.json
└── mongo/
    ├── client.ts                # MongoClient factory (alias resolution, connection pool)
    ├── databases.ts             # listDatabases, getDatabaseStats
    ├── collections.ts           # listCollections, getCollectionStats
    ├── schema.ts                # inferSchema — sample-based field/type discovery
    ├── indexes.ts               # listIndexes
    ├── query.ts                 # findDocuments, findById, countDocuments, getDistinctValues
    ├── aggregate.ts             # runAggregate with $out/$merge rejection
    └── serialize.ts             # BSON → JSON-safe conversion (ObjectId, Date, Binary, Long, etc.)
```

## Key patterns

- **Command registration**: Each `cli/*/index.ts` exports `registerXyzCommand({ program })` called from `index.ts`. Subcommands are in separate files within each directory.
- **Output**: All commands use `printJson()` or `printPaginated()` from `lib/output.ts`. Errors use `printError()`. All output is JSON, empty/null fields auto-pruned via `pruneEmpty()`.
- **Truncation**: Any string field exceeding `truncation.maxLength` gets truncated with `…` and a companion `{field}Length` key. Controlled by `--expand` and `--full` global flags. Unlike lin (which only truncates preset field names), this truncates any string over the limit.
- **Connection resolution**: `-c` flag > `AGENT_MONGO_CONNECTION` env > config default > error listing available connections. Connections can reference named credentials for shared auth.
- **Credential separation**: Credentials (username/password) stored separately from connections (host/options). Connections reference credentials by name via `credential` field. Backward compatible — connections without a credential use the URI as-is.
- **Error messages**: Include valid values so LLMs can self-correct (e.g., `Connection "x" not found. Available: local, staging`).
- **BSON serialization**: `mongo/serialize.ts` converts all BSON types to JSON-safe values before output. ObjectId → hex string, Date → ISO string, Binary → base64, Long → number (if safe) or string.
- **Read-only safety**: No write operations exist. `mongo/aggregate.ts` rejects `$out`/`$merge` stages. Results capped at `query.maxDocuments`.
- **Usage subcommands**: Each command group has a `usage` subcommand providing LLM-friendly docs. When modifying a command's behavior, options, or flags, update its usage text too.
- **Config validation**: `cli/config/valid-keys.ts` defines all valid keys with types, defaults, and min/max ranges. Invalid keys or out-of-range values produce errors listing valid options.

## Commands

Run `bun run dev -- usage` for the full command reference. Each command also supports `<command> usage` for detailed per-command docs.

## Development

```bash
bun install
bun run dev -- <command>     # run in dev mode
bun run typecheck            # tsc --noEmit
bun test                     # bun:test
bun run lint                 # oxlint
bun run format               # oxfmt
```

## Release

```bash
bun run release patch        # bumps version, commits, tags, pushes
bun run build:release        # cross-platform binaries in release/
```

Then create GitHub release and update homebrew-tap formula with new sha256s.

## Conventions

- TypeScript strict mode, ES2022 target, Bun bundler resolution
- `type` over `interface` (enforced by oxlint)
- kebab-case filenames (enforced by oxlint)
- Max 350 lines per file, max 2 params per function (oxlint warnings)
- Pre-commit hook: oxlint fix + oxfmt
- Tests: bun:test, no mocking libraries, inline fixtures, pure functions preferred
