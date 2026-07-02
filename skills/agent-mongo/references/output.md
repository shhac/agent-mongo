# Output format (reference)

## General

Default output is **NDJSON** (`-f jsonl`) — one JSON record per line on stdout.

- **List commands** emit one record per item, then trailing `@`-prefixed metadata lines:
  - `{"@meta": {...}}` — command context (database, collection, sampleSize, totals — varies by command).
  - `{"@pagination": {...}}` — `has_more`, `total_items`, and `next_cursor` when more results exist.
- **Single results** (stats, `query get`, `count`, `distinct`, receipts) print as one JSON line.

Empty/null fields are pruned automatically — a missing key means no value, not `null`.

Errors print one JSON line to **stderr** with exit code 1:

```json
{ "error": "Connection \"x\" not found. Available: local, staging. ...", "fixable_by": "agent" }
```

`fixable_by` tells the caller who resolves it: `agent` (fix input and retry), `human` (needs the user — auth, GUI dialog), `retry` (transient). Errors include valid values when input is invalid, so an agent can self-correct. Timeout errors (MongoDB code 50) carry a `hint` to increase `query.timeout` and check indexes; collection-not-found errors suggest `collection list`.

## Format flag

`-f/--format` switches the shape:

- `jsonl` (default) — NDJSON as above.
- `json` — a single pretty envelope. Lists become `{"data": [...], "@meta": {...}, "@pagination": {...}}`; single objects print pretty and bare.
- `yaml` — the same structure rendered as YAML.

**List as `-f json`:**

```json
{
  "@meta": { "totalSize": 348160 },
  "data": [{ "empty": false, "name": "testdb", "sizeOnDisk": 122880 }]
}
```

## Truncation

Any string field exceeding `truncation.maxLength` (default 200) gets truncated with `…` and a companion `{field}Length` key showing the original character count.

**Default (truncated):**

```json
{ "longBio": "xxxxxxxx…", "longBioLength": 500 }
```

**With `--full` or `--expand longBio` (expanded):**

```json
{ "longBio": "xxxxxxxx...(full 500 chars)...", "longBioLength": 500 }
```

Unlike lin (which only truncates preset field names), agent-mongo truncates **any** string over the limit. The `{field}Length` companion key is present whenever the original exceeded `truncation.maxLength`. Global flags: `--expand <field,...>` or `--full`.

## BSON serialization

All BSON types are converted to JSON-safe values:

| BSON type  | JSON output                                          |
| ---------- | ---------------------------------------------------- |
| ObjectId   | 24-character hex string                              |
| Date       | ISO 8601 string                                      |
| Binary     | Base64-encoded string (UUID subtype → `uuid` string) |
| int64      | Number if ≤ 2^53−1, otherwise string                 |
| Decimal128 | String                                               |
| Timestamp  | String representation                                |
| Regex      | `/pattern/flags` string                              |

In `collection schema`, the `types` array reports BSON type names as MongoDB names them: `ObjectId`, `string`, `int`, `long`, `double`, `decimal`, `boolean`, `date`, `binary`, `regex`, `object`, `array`, `null`.

## Database list (`database list`)

```
{"empty":false,"name":"admin","sizeOnDisk":40960}
{"empty":false,"name":"testdb","sizeOnDisk":122880}
{"@meta":{"totalSize":348160}}
```

## Database stats (`database stats`)

```json
{"collections":3,"dataSize":10834,"database":"testdb","documents":154,"indexSize":61440,"indexes":3,"storageSize":61440}
```

## Collection list (`collection list`)

```
{"name":"users","type":"collection"}
{"name":"activeUsers","type":"view"}
{"@meta":{"database":"testdb"}}
```

## Collection schema (`collection schema`)

```
{"path":"_id","presence":1,"types":["ObjectId"]}
{"path":"name","presence":1,"types":["string"]}
{"path":"tags","presence":0.67,"types":["array"]}
{"path":"tags.$","presence":0.33,"types":["string"]}
{"path":"profile.bio","presence":0.33,"types":["string"]}
{"@meta":{"collection":"users","database":"testdb","sampleSize":3,"totalDocuments":3,"totalFields":21}}
```

Array element types appear as `path.$` entries. Nested objects use dot notation. `presence` is 0.0–1.0 (fraction of sampled documents containing the field). Errors if the collection does not exist.

With `--limit`/`--skip` pagination, a `@pagination` line carries `next_cursor` (the next skip value):

```
{"@meta":{"collection":"users","database":"testdb","sampleSize":3,"totalDocuments":3,"totalFields":21}}
{"@pagination":{"has_more":true,"next_cursor":"3","total_items":21}}
```

## Collection indexes (`collection indexes`)

```
{"key":{"_id":1},"name":"_id_"}
{"@meta":{"collection":"users","database":"testdb"}}
```

Indexes also carry `unique`, `sparse`, and other properties when set.

## Collection stats (`collection stats`)

```json
{"avgDocumentSize":329,"capped":false,"collection":"users","dataSize":988,"database":"testdb","documentCount":3,"indexSize":20480,"indexes":1,"storageSize":20480}
```

## Query find (`query find`)

```
{"_id":"6a46...","age":35,"name":"carol","status":"active"}
{"@meta":{"collection":"users","database":"testdb"}}
{"@pagination":{"has_more":true,"total_items":3}}
```

`has_more` indicates more documents match beyond the limit. `total_items` is the full matching count.

## Query get (`query get`)

```json
{"collection":"users","database":"testdb","document":{"_id":"6a46...","age":35,"name":"carol","status":"active"},"fieldCount":4}
```

`fieldCount` shows the number of top-level fields in the document. Use it to decide if `--projection` is needed.

## Query count (`query count`)

```json
{"collection":"users","count":3,"database":"testdb"}
```

## Query sample (`query sample`)

```
{"_id":"6a46...","age":25,"name":"bob"}
{"_id":"665a...","age":30,"name":"alice"}
{"@meta":{"collection":"users","database":"testdb","sampleSize":2}}
```

## Query distinct (`query distinct`)

```json
{"collection":"users","count":1,"database":"testdb","field":"status","values":["active"]}
```

## Query aggregate (`query aggregate`)

```
{"_id":"done","count":100}
{"_id":"pending","count":50}
{"@meta":{"collection":"orders","count":2,"database":"testdb"}}
```

## Connection list (`connection list`)

```
{"alias":"local","connection_string":"mongodb://localhost:27017/testdb","default":false}
{"alias":"staging","connection_string":"mongodb://db.example.com/app","credential":"ldt","default":false}
{"alias":"test","connection_string":"mongodb://localhost:27099/testdb","default":true}
```

`credential` appears only when the connection references a stored credential.

## Connection test (`connection test`)

```json
{"alias":"test","ok":true,"ping":{"ok":1}}
```

## Credential list (`credential list`)

```
{"name":"ldt","password":"***","storage":"config","usedBy":["staging"],"username":"deploy"}
```

Passwords are always redacted (`***`). `storage` is `keychain` or `config`; `usedBy` lists connections referencing the credential.
