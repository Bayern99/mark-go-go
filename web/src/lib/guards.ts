/**
 * The package's one object guard, for hand-written boundary parsers of
 * untrusted JSON (the app has no schema-validator dependency). It proves only
 * that the value is a plain object; every field must still be checked.
 */
export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
