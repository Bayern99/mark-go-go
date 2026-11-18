import type { GenerateFailure, GenerateFailureKind } from './types'

export class GenerationError extends Error {
  readonly failure: GenerateFailure

  constructor(failure: GenerateFailure) {
    super(failure.message)
    this.name = 'GenerationError'
    this.failure = failure
  }
}

export function cancelled(): GenerationError {
  return new GenerationError({ kind: 'cancelled', message: 'Generation was cancelled.' })
}

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

export function isFailureKind(kind: string): kind is GenerateFailureKind {
  return Object.hasOwn(KINDS, kind)
}
