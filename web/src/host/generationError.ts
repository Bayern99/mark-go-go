import type { GenerateFailure, GenerateFailureKind } from './types'

/** A failed generation, as `ViewGenerator.generate` rejects with it. */
export class GenerationError extends Error {
  readonly failure: GenerateFailure

  constructor(failure: GenerateFailure) {
    super(failure.message)
    this.name = 'GenerationError'
    this.failure = failure
  }
}

/** The failure of a run the reader cancelled. */
export function cancelled(): GenerationError {
  return new GenerationError({ kind: 'cancelled', message: 'Generation was cancelled.' })
}

/**
 * The failure an error stands for: a `GenerationError`'s own, or any other
 * error's message as an `agent-error`.
 */
export function failureOf(error: unknown): GenerateFailure {
  if (error instanceof GenerationError) return error.failure
  const message = error instanceof Error ? error.message : String(error)
  return {
    kind: 'agent-error',
    message,
    hint: 'Retry; if it keeps failing, run the agent command in Terminal to see what it says.',
  }
}

const KINDS: Record<GenerateFailureKind, true> = {
  unsaved: true,
  busy: true,
  'agent-not-found': true,
  'auth-required': true,
  'model-invalid': true,
  'agent-message': true,
  'invalid-output': true,
  'agent-error': true,
  timeout: true,
  cancelled: true,
}

/** The host's failure kinds are plain strings on the wire. */
export function isFailureKind(kind: string): kind is GenerateFailureKind {
  return Object.hasOwn(KINDS, kind)
}
