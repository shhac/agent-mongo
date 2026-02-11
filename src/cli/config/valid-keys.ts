export type KeyDefinition = {
  key: string;
  type: "number";
  defaultValue: number;
  description: string;
  min?: number;
  max?: number;
};

export const KEY_DEFINITIONS: KeyDefinition[] = [
  {
    key: "defaults.limit",
    type: "number",
    defaultValue: 20,
    description: "Default result limit for list/query commands",
    min: 1,
    max: 1000,
  },
  {
    key: "defaults.sampleSize",
    type: "number",
    defaultValue: 5,
    description: "Default sample size for schema inference",
    min: 1,
    max: 100,
  },
  {
    key: "query.timeout",
    type: "number",
    defaultValue: 30000,
    description: "Query timeout in milliseconds",
    min: 1000,
    max: 300000,
  },
  {
    key: "query.maxDocuments",
    type: "number",
    defaultValue: 100,
    description: "Maximum documents returned per query",
    min: 1,
    max: 10000,
  },
  {
    key: "truncation.maxLength",
    type: "number",
    defaultValue: 200,
    description: "Max string length before truncation (any field)",
    min: 50,
    max: 100000,
  },
];

export const VALID_KEYS: Set<string> = new Set(KEY_DEFINITIONS.map((k) => k.key));

export function parseConfigValue(key: string, raw: string): number {
  const def = KEY_DEFINITIONS.find((k) => k.key === key);
  if (!def) {
    throw new Error(`Unknown key: "${key}"`);
  }

  const num = Number(raw);
  if (!Number.isFinite(num) || !Number.isInteger(num)) {
    throw new Error(`"${key}" must be an integer. Got: "${raw}"`);
  }
  if (def.min !== undefined && num < def.min) {
    throw new Error(`"${key}" minimum is ${def.min}. Got: ${num}`);
  }
  if (def.max !== undefined && num > def.max) {
    throw new Error(`"${key}" maximum is ${def.max}. Got: ${num}`);
  }
  return num;
}
