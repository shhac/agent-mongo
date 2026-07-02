# Go Rewrite Plan

**Status: IN PROGRESS** — work happens on `paul/migrate-to-go`; merging that branch
back to `main` should leave a pure Go CLI project.

## Why

Every sibling tool (`agent-sql`, `agent-slack`, `lin`, `agent-notion`, …) is Go on
`lib-agent-*`. agent-mongo is the TypeScript/Bun outlier: it hand-rolls the XDG
config dir, keychain access, native secret dialog, output helpers, and cobra-style
scaffolding that the family has since consolidated into shared modules. Migrating
gets us:

- **`lib-agent-cli`** — root command, persistent flags, config/credential storage,
  XDG paths, the `--form` secret dialog. Deletes most of `src/lib/`.
- **`lib-agent-output`** — the family NDJSON wire contract (stdout records,
  `@`-metadata trailers, structured stderr errors with `fixable_by`).
- **`lib-agent-keyring`** — cross-platform OS secret store (macOS Keychain today,
  Secret Service / Credential Manager for free).
- **`lib-agent-mcp`** — `agent-mongo mcp` becomes a one-line MCP stdio server.
- Smaller static binaries, no Bun runtime quirks, one toolchain across the family.

Precedent: agent-sql ran this exact play (see `agent-sql/design-docs/go-rewrite.md`,
status COMPLETE) before the shared libs existed. Ours is smaller — one driver
instead of eight — and the libs now exist.

## Principles

1. **Red-green migration** — write Go tests first from the existing TS test
   expectations (`test/*.test.ts`), then implement until green.
2. **Shared config** — Go reads the same `~/.config/agent-mongo/config.json`
   (same schema, same `__KEYCHAIN__` sentinel, same keychain service
   `app.paulie.agent-mongo` with `username:<alias>` / `password:<alias>` accounts).
   Both binaries work during migration.
3. **Family output contract, not byte parity** — the output layer adopts
   `lib-agent-output` (NDJSON + `@pagination` + stderr `{error, fixable_by, hint}`).
   Byte-level parity is required only for _domain_ behavior: schema inference,
   BSON→JSON mapping, truncation semantics, pipeline validation, error hints.
4. **TS stays runnable until parity** — `src/` + `test/` remain in place on the
   branch as the golden reference; deleted in the final phase.
5. **Feature parity, not feature creep** — no new commands until the port is done.
   (Exception: `mcp` and `--format`, which fall out of the libs for free.)

## Scope

| Area                                                                        | TS lines   | Go estimate | Notes                                           |
| --------------------------------------------------------------------------- | ---------- | ----------- | ----------------------------------------------- |
| `src/lib/` (config, output, keychain, dialog, timeout, version)             | ~900       | ~250        | mostly replaced by lib-agent-cli/keyring/output |
| `src/lib/` domain (serialize, truncation, compact-json, parse-json, errors) | ~250       | ~400        | ported; Go is more verbose                      |
| `src/mongo/`                                                                | ~750       | ~900        | mongo-driver v2                                 |
| `src/cli/` (7 groups, ~30 leaves incl. usage texts)                         | ~1,850     | ~1,600      | cobra; usage texts copied verbatim              |
| **Total source**                                                            | **~3,756** | **~3,200**  |                                                 |
| Tests                                                                       | ~1,780     | ~2,200      | table-driven ports + new integration suite      |

## Directory structure

```
agent-mongo/
  cmd/agent-mongo/main.go     # entry point; version via -ldflags
  internal/
    cli/
      root.go                 # lib-agent-cli NewRoot + domain flags -c/--expand/--full
      connection/             # add/remove/update/list/test/set-default/usage
      credential/             # add (--form via lib-agent-cli dialog)/remove/list/usage
      config/                 # get/set/reset/list-keys/usage — keyDef table w/ min-max
      database/               # list/stats/usage
      collection/             # list/schema/indexes/stats/usage
      query/                  # find/get/count/sample/distinct/aggregate/usage
      usage.go                # top-level LLM reference card
    config/                   # config.json I/O (same file as TS), settings keys
    credential/               # __KEYCHAIN__ sentinel resolution via lib-agent-keyring
    mongo/                    # client factory, databases, collections, indexes,
                              # schema inference, query, aggregate ($out/$merge rejection)
    serialize/                # BSON → JSON-safe (ObjectId→hex, Date→ISO, …)
    truncation/               # any-string truncation + {field}Length companion
    compactjson/              # pruneEmpty
    ejson/                    # extended-JSON parsing for --filter/--sort/--projection/--pipeline
    errors/                   # timeout hints, index suggestions
    integration/              # dockerised mongod end-to-end tests
  design-docs/
  skills/agent-mongo/
  src/ + test/                # TS reference — removed in the final phase
```

## Dependencies

| Module                               | Purpose                                          |
| ------------------------------------ | ------------------------------------------------ |
| `github.com/shhac/lib-agent-cli`     | root cmd, config/credential storage, XDG, dialog |
| `github.com/shhac/lib-agent-output`  | NDJSON contract, errors, pagination              |
| `github.com/shhac/lib-agent-keyring` | OS secret store (indirect via lib-agent-cli)     |
| `github.com/shhac/lib-agent-mcp`     | `agent-mongo mcp`                                |
| `github.com/spf13/cobra`             | CLI framework                                    |
| `go.mongodb.org/mongo-driver/v2`     | MongoDB client + bson/EJSON                      |

All pure Go — cross-compilation stays trivial.

## Behavior decisions

- **Global flags.** lib-agent-cli's `NewRoot` supplies `--format/-f` (json | yaml |
  jsonl, default jsonl), `--timeout/-t` (ms — subsumes the TS `--timeout`),
  `--debug/-d`, `--color`. Domain persistent flags added on top: `-c/--connection`,
  `--expand <fields>`, `--full`.
- **Config subcommands keep the TS surface** (`get`/`set`/`reset`/`list-keys`),
  following agent-sql's pattern: a package-level `keyDef` table with
  type/default/min/max ported straight from `valid-keys.ts`, self-registered
  cobra commands rather than `cli.ConfigCommand` (whose `get/set/unset/list`
  shape would break existing callers for no gain).
- **Truncation is an `output.Pruner`.** The any-string + `{field}Length` walk is
  implemented as a custom pruner composed as
  `output.Chain(output.PruneEmpty, truncation.Pruner(...))` — prune first, then
  truncate, matching the TS `applyTruncation(pruneEmpty(x))` order.
- **Output contract changes (breaking).** Lists stream one record per line with an
  `@pagination` trailer; single results are one JSON line; errors go to stderr as
  `{error, fixable_by, hint?}`. The TS `--stream` flag becomes redundant (NDJSON is
  the default), kept as a no-op alias for one release. SKILL.md and README are
  updated in the same phase.
- **Keep the custom BSON serialization.** ObjectId → bare hex, Date → ISO-8601,
  Binary → base64, Long → number if safe else string, Decimal128/UUID/RegExp →
  string. More LLM-friendly than relaxed EJSON's `{"$oid": …}` wrappers. Input
  parsing still accepts Extended JSON (`$oid`, `$date`, `$numberLong`, …).
- **Truncation and pruneEmpty are preserved as-is** — any string over
  `truncation.maxLength` gets `…` + `{field}Length` companion; null/empty/blank
  fields pruned. `--expand <fields>` / `--full` keep working.
- **Connection resolution unchanged** — `-c` flag > `AGENT_MONGO_CONNECTION` >
  config default > error listing available aliases. Error strings keep listing
  valid values so LLMs can self-correct.
- **Read-only invariants unchanged** — no write commands, `$out`/`$merge`
  rejected, results capped at `query.maxDocuments`.
- **Config keys unchanged** — `defaults.limit/sampleSize/schemaSampleSize`,
  `query.timeout/maxDocuments`, `truncation.maxLength`, same defaults and ranges.

## Migration phases

### Phase 0: Prep — DONE

- [x] `design-docs/` tracked (archive stays ignored), stale findings archived
- [x] This document

### Phase 1: Scaffold

- [ ] `go mod init github.com/shhac/agent-mongo`
- [ ] `cmd/agent-mongo/main.go` (`cli.Run`) + `internal/cli/root.go` via `cli.NewRoot`
- [ ] Domain persistent flags: `-c/--connection`, `--expand`, `--full`
- [ ] Makefile: build/test/lint/release targets (crib agent-sql)
- [ ] TS project still builds/tests alongside

### Phase 2: Config + credentials (no mongo yet)

- [ ] `internal/config/` — read/write the existing config.json
- [ ] `internal/credential/` — `__KEYCHAIN__` sentinel via lib-agent-keyring
- [ ] CLI: `config`, `connection` (except `test`), `credential` groups
- [ ] `credential add --form` via lib-agent-cli `dialog` (+ `dialogtest.Recorder` tests)
- [ ] Red-green: port config.test.ts, credential.test.ts, credential-form.test.ts

### Phase 3: Domain libraries

- [ ] serialize, truncation, compactjson, ejson, errors packages
- [ ] Red-green: serialize.test.ts, truncation.test.ts, compact-json.test.ts,
      parse-json.test.ts, output.test.ts, ndjson-stream.test.ts equivalents

### Phase 4: Mongo layer + commands

- [ ] `internal/mongo/` — client factory (pool opts, alias resolution), databases,
      collections (+existence validation), indexes, schema inference, query
      (find/get/count/sample/distinct), aggregate (validatePipeline)
- [ ] CLI: `database`, `collection`, `query` groups + `connection test`
- [ ] Red-green: schema.test.ts, aggregate.test.ts equivalents
- [ ] Timeout plumbing: `-t/--timeout` (lib flag) > `query.timeout` > 30s via maxTimeMS

### Phase 5: Integration parity

- [ ] `internal/integration/` — seeded throwaway mongod (docker `mongo:8`);
      never runs against configured real connections
- [ ] Golden comparison: TS binary vs Go binary on the same seeded data —
      domain fields must match; output framing asserted against the new contract

### Phase 6: MCP + docs

- [ ] `agent-mongo mcp` via `agentmcp.Command(root)`; `ReadOnly` annotations on
      query/collection/database tools, `Skip` on credential/config
- [ ] All `usage` subcommand texts ported (updated for NDJSON framing)
- [ ] SKILL.md + README + CLAUDE.md updated for Go + new output contract

### Phase 7: Cleanup

- [ ] Remove `src/`, `test/`, package.json, bun/oxlint/oxfmt config; hooks → Go equivalents
- [ ] Release: `.goreleaser.yml` + tag-triggered reusable workflow
      (`shhac/homebrew-tap/.github/workflows/go-release.yml`) as in agent-sql;
      releasing becomes `git tag vX.Y.Z && git push origin vX.Y.Z`
- [ ] Merge to main; release as v1.0.0

## Testing

- Unit: `go test ./...`, table-driven ports of the TS suites.
- Integration: build tag or `-run Integration` gate; spins up `mongo:8` in docker,
  seeds fixture documents covering every BSON type serialize handles, runs both
  binaries (while TS still exists) and diffs domain output.
- The configured `staging`/`production` connections in the local config are real
  clusters — integration tests must never dial them.
