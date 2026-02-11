# Output format (reference)

## General

All commands print JSON to stdout. Errors print `{ "error": "..." }` to stderr with non-zero exit.

Empty/null fields are pruned automatically — missing keys mean no value, not `null`.

Error messages include valid values when input is invalid (e.g., `Connection "x" not found. Available: local, staging`).

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

## Database list (`db list`)

```json
{
  "databases": [{ "name": "myapp", "sizeOnDisk": 1048576, "empty": false }],
  "totalSize": 2097152
}
```

## Database stats (`db stats`)

```json
{
  "db": "myapp",
  "collections": 12,
  "documents": 45000,
  "dataSize": 15728640,
  "storageSize": 8388608,
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
  "fields": [
    { "path": "_id", "types": ["ObjectId"], "presence": 1.0 },
    { "path": "name", "types": ["String"], "presence": 1.0 },
    { "path": "tags", "types": ["Array"], "presence": 0.85 },
    { "path": "tags.$", "types": ["String"], "presence": 0.85 },
    { "path": "address.city", "types": ["String"], "presence": 0.72 }
  ],
  "sampleSize": 100
}
```

Array element types appear as `path.$` entries. Nested objects use dot notation. `presence` is 0.0-1.0 (fraction of sampled documents containing the field).

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
  "ns": "myapp.users",
  "count": 15000,
  "size": 5242880,
  "storageSize": 2097152,
  "totalIndexSize": 524288,
  "capped": false
}
```

## Query find (`query find`)

```json
{
  "documents": [{ "_id": "665a...", "name": "Alice", "age": 30 }],
  "count": 1,
  "hasMore": true,
  "totalMatching": 42
}
```

`hasMore` indicates more documents match beyond the limit. `totalMatching` is the full count.

## Query get (`query get`)

Returns the document directly:

```json
{
  "_id": "665a1b2c3d4e5f6a7b8c9d0e",
  "name": "Alice",
  "email": "alice@example.com",
  "createdAt": "2024-06-01T12:00:00.000Z"
}
```

## Query count (`query count`)

```json
{
  "count": 42
}
```

## Query sample (`query sample`)

```json
{
  "database": "myapp",
  "collection": "users",
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
  "values": ["pending", "shipped", "delivered"],
  "count": 3
}
```

## Query aggregate (`query aggregate`)

```json
{
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
