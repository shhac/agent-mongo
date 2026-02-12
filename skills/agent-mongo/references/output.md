# Output format (reference)

## General

All commands print JSON to stdout. Errors print `{ "error": "..." }` to stderr with non-zero exit.

Empty/null fields are pruned automatically — missing keys mean no value, not `null`.

Error messages include valid values when input is invalid (e.g., `Connection "x" not found. Available: local, staging`).

Timeout errors include hints to increase `query.timeout` and check collection indexes. Collection-not-found errors suggest using `collection list` to discover available collections.

## Truncation

Any string field exceeding `truncation.maxLength` (default 200) gets truncated with `…` and a companion `{field}Length` key showing the original character count.

**Default (truncated):**

```json
{
  "description": "This is the beginning of a long document that goes on for many paragraphs…",
  "descriptionLength": 1847
}
```

**With `--full` or `--expand description` (expanded):**

```json
{
  "description": "This is the beginning of a long document that goes on for many paragraphs and includes detailed content...",
  "descriptionLength": 1847
}
```

Unlike lin (which only truncates preset field names), agent-mongo truncates **any** string over the limit. The `{field}Length` companion key is present whenever the original exceeded `truncation.maxLength`.

Global flags: `--expand <field,...>` or `--full`.

## BSON serialization

All BSON types are converted to JSON-safe values:

| BSON type  | JSON output                        |
| ---------- | ---------------------------------- |
| ObjectId   | 24-character hex string            |
| Date       | ISO 8601 string                    |
| Binary     | Base64-encoded string              |
| Long       | Number (if safe integer) or string |
| Decimal128 | String                             |
| Timestamp  | String representation              |
| RegExp     | String (e.g. `/pattern/flags`)     |
| UUID       | String representation              |

## Database list (`database list`)

```json
{
  "databases": [{ "name": "myapp", "sizeOnDisk": 1048576, "empty": false }],
  "totalSize": 2097152
}
```

## Database stats (`database stats`)

```json
{
  "database": "myapp",
  "collections": 12,
  "documents": 45000,
  "dataSize": 15728640,
  "storageSize": 8388608,
  "indexes": 24,
  "indexSize": 2097152
}
```

## Collection list (`collection list`)

```json
[
  { "name": "users", "type": "collection" },
  { "name": "user_summary", "type": "view" }
]
```

## Collection schema (`collection schema`)

```json
{
  "database": "myapp",
  "collection": "users",
  "sampleSize": 100,
  "totalDocuments": 15000,
  "totalFields": 12,
  "fields": [
    { "path": "_id", "types": ["ObjectId"], "presence": 1.0 },
    { "path": "name", "types": ["string"], "presence": 1.0 },
    { "path": "tags", "types": ["array"], "presence": 0.85 },
    { "path": "tags.$", "types": ["string"], "presence": 0.85 },
    { "path": "address.city", "types": ["string"], "presence": 0.72 }
  ]
}
```

With `--limit`/`--skip` pagination:

```json
{
  "database": "myapp",
  "collection": "events",
  "sampleSize": 100,
  "totalDocuments": 10000000,
  "totalFields": 150,
  "fields": [ "... first 50 fields ..." ],
  "pagination": { "hasMore": true, "nextSkip": 50 }
}
```

Array element types appear as `path.$` entries. Nested objects use dot notation. `presence` is 0.0-1.0 (fraction of sampled documents containing the field). Errors if collection does not exist.

## Collection indexes (`collection indexes`)

```json
[
  { "name": "_id_", "key": { "_id": 1 }, "unique": true },
  { "name": "email_1", "key": { "email": 1 }, "unique": true, "sparse": true }
]
```

## Collection stats (`collection stats`)

```json
{
  "database": "myapp",
  "collection": "users",
  "documentCount": 15000,
  "dataSize": 5242880,
  "avgDocumentSize": 349,
  "storageSize": 2097152,
  "indexes": 3,
  "indexSize": 524288,
  "capped": false
}
```

## Query find (`query find`)

```json
{
  "database": "myapp",
  "collection": "users",
  "filter": { "age": { "$gte": 21 } },
  "documents": [{ "_id": "665a...", "name": "Alice", "age": 30 }],
  "count": 1,
  "hasMore": true,
  "totalMatching": 42
}
```

`hasMore` indicates more documents match beyond the limit. `totalMatching` is the full count.

## Query get (`query get`)

```json
{
  "database": "myapp",
  "collection": "users",
  "fieldCount": 4,
  "document": {
    "_id": "665a1b2c3d4e5f6a7b8c9d0e",
    "name": "Alice",
    "email": "alice@example.com",
    "createdAt": "2024-06-01T12:00:00.000Z"
  }
}
```

`fieldCount` shows the number of top-level fields in the document. Use it to decide if `--projection` is needed.

## Query count (`query count`)

```json
{
  "database": "myapp",
  "collection": "orders",
  "filter": { "status": "pending" },
  "count": 42
}
```

## Query sample (`query sample`)

```json
{
  "database": "myapp",
  "collection": "users",
  "filter": {},
  "sampleSize": 2,
  "documents": [
    { "_id": "...", "name": "Alice" },
    { "_id": "...", "name": "Bob" }
  ]
}
```

## Query distinct (`query distinct`)

```json
{
  "database": "myapp",
  "collection": "orders",
  "field": "status",
  "values": ["pending", "shipped", "delivered"],
  "count": 3
}
```

## Query aggregate (`query aggregate`)

```json
{
  "database": "myapp",
  "collection": "orders",
  "documents": [
    { "_id": "pending", "count": 15 },
    { "_id": "shipped", "count": 27 }
  ],
  "count": 2
}
```

## Connection list (`connection list`)

```json
{
  "connections": [
    {
      "alias": "local",
      "connection_string": "mongodb://localhost:27017/myapp",
      "database": "myapp",
      "default": true
    },
    {
      "alias": "prod",
      "connection_string": "mongodb+srv://cluster.example.net/myapp",
      "database": "myapp",
      "credential": "acme"
    }
  ]
}
```

## Credential list (`credential list`)

```json
{
  "credentials": [{ "name": "acme", "username": "deploy", "connections": ["prod", "staging"] }]
}
```

Passwords are always redacted.
