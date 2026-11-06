import type { PluginContext } from '../../core/types'
import { failureOf } from '../../host/generationError'
import type { GenerateFailure, GenerateProgress, GenerationSettings } from '../../host/types'
import { extractSections } from '../../markdown/sections'
import { catalogDefinition } from './a2ui'
import formatDocument from './view-format.md?raw'

/**
 * What the agent is given to know the format: the format document, then the
 * catalog definition with every component's schema and nesting rules.
 */
export function agentFormat(): string {
  return `${formatDocument.trim()}\n\n## Catalog definition\n\n\`\`\`json\n${JSON.stringify(catalogDefinition(), null, 2)}\n\`\`\`\n`
}

/**
 * What a run is asked to do: a whole new view, or only the components in
 * `replaceIds` replaced in the current one. `request` is the reader's.
 */
export type GenerationRun =
  | { mode: 'full'; request: string }
  | { mode: 'update'; request: string; replaceIds: string[] }

export type GenerationState =
  | { kind: 'idle' }
  | { kind: 'running'; run: GenerationRun; progress: GenerateProgress[] }
  /** The last run failed; `run` is what it was asked to do, so it can be retried. */
  | { kind: 'failed'; failure: GenerateFailure; run: GenerationRun }

/**
 * One window's generation: at most one run at a time, its progress, and the
 * error of the last run that failed. It outlives the GenUI view's mounts, so
 * leaving GenUI while a run goes on does not lose it. The view file itself
 * comes through the view watch.
 */
export class GenerationController {
  private state: GenerationState = { kind: 'idle' }
  private abort: AbortController | undefined
  private readonly listeners = new Set<() => void>()

  constructor(private readonly ctx: PluginContext) {}

  readonly subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }

  readonly getSnapshot = (): GenerationState => this.state

  /**
   * Makes a new view of the document as it reads now, or replaces some of
   * the current view's components, with the reader's request. The text is
   * taken at once: edits made while the agent works show as changed
   * sources in the new view.
   */
  async generate(run: GenerationRun): Promise<void> {
    if (this.state.kind === 'running') return
    const { text } = this.ctx.documents.snapshot()
    const sections = extractSections(text).map((section) => ({
      heading: section.heading.text,
      occurrence: section.occurrence,
      start: section.start,
      end: section.end,
    }))
    const abort = new AbortController()
    this.abort = abort
    this.set({ kind: 'running', run, progress: [] })
    try {
      await this.ctx.viewFile.generate(
        {
          text,
          mode: run.mode,
          replaceIds: run.mode === 'update' ? run.replaceIds : undefined,
          request: run.request.trim() || undefined,
          format: agentFormat(),
          sections,
        },
        (step) => {
          if (this.abort !== abort || this.state.kind !== 'running') return
          this.set({ ...this.state, progress: [...this.state.progress, step] })
        },
        abort.signal,
      )
      if (this.abort === abort) this.set({ kind: 'idle' })
    } catch (error) {
      if (this.abort !== abort) return
      // Cancelling is the reader's choice, not a failure to report.
      const failure = failureOf(error)
      this.set(abort.signal.aborted || failure.kind === 'cancelled' ? { kind: 'idle' } : { kind: 'failed', failure, run })
    } finally {
      if (this.abort === abort) this.abort = undefined
    }
  }

  cancel(): void {
    this.abort?.abort()
  }

  /** Runs the failed run again. */
  retry(): Promise<void> {
    return this.state.kind === 'failed' ? this.generate(this.state.run) : Promise.resolve()
  }

  dismissError(): void {
    if (this.state.kind === 'failed') this.set({ kind: 'idle' })
  }

  getSettings(): Promise<GenerationSettings> {
    return this.ctx.viewFile.getSettings()
  }

  setSettings(settings: GenerationSettings): Promise<void> {
    return this.ctx.viewFile.setSettings(settings)
  }

  dispose(): void {
    this.cancel()
    this.listeners.clear()
  }

  private set(state: GenerationState): void {
    this.state = state
    for (const listener of this.listeners) listener()
  }
}
