import type { PluginContext } from '../../core/types'
import { failureOf } from '../../host/generationError'
import type {
  Disposable,
  GenerateFailure,
  GenerateProgress,
  GenerationSettings,
  SettingsPatch,
} from '../../host/types'
import { extractSections } from '../../markdown/sections'
import { catalogDefinition } from './a2ui'

/**
 * The model-only format: the message and source contract, then the actual
 * catalog as compact JSON with every component's schema and nesting rules.
 * view-format.md stays the reader-facing description; its example and
 * property prose would only repeat what the schemas already say.
 */
const formatContract = `A2UI 1.0 view contract.

out.json: {"sources":{"<id>":{"heading":"<plain heading>","occurrence":<n>}},"a2ui":[<messages>]}. occurrence counts repeated headings from 1 and may be left out when it is 1. Leave out "hash": the app writes it.

sources: each component names the sections it draws on in "sources", a list of ids. heading is the heading's plain text exactly as it reads, without # marks or inline formatting; matching ignores case and repeated spaces. A section runs from its heading to the next heading of the same or a higher level, so it includes sub-sections. Reader notes (top-level HTML comment blocks) are not document content.

a2ui: one surface, catalogId "markgogo.dev:genui/v1", message "version" "1.0". A full view is one createSurface whose component tree starts at "root"; an update is one updateComponents for the existing surface. Every component is an object with only "id", "component" and that component's properties. A reference must name a component of the view. ids must be valid Unicode UAX #31 identifiers; ASCII letters, digits and underscores are safe, and hyphens are not. Text content is Markdown; content components take plain text and contain no other component.

Full-view message shape: {"version":"1.0","createSurface":{"surfaceId":"view","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"KeyPoints","items":["A fact from the document"],"sources":["s1"]}]}}. Update message shape: {"version":"1.0","updateComponents":{"surfaceId":"view","components":[<replacement components>]}}. Use the actual source id and existing surface id for the run.

Catalog definition (every component's JSON Schema and allowed parents and children):`

export function agentFormat(): string {
  return `${formatContract}\n${JSON.stringify(catalogDefinition())}\n`
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
 * One document's generation: at most one run at a time, its progress, and the
 * error of the last run that failed. Bound to this pane's PluginContext, so
 * it never retargets through the active tab. It outlives the GenUI view's
 * mounts, so leaving GenUI while a run goes on does not lose it. The view
 * file itself comes through the view watch.
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
      exclude:
        section.notes.length > 0
          ? section.notes.map((note) => ({ start: note.cutStart, end: note.end }))
          : undefined,
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

  /** Changes the fields the patch carries, in the one settings store every document shares. */
  setSettings(patch: SettingsPatch): Promise<GenerationSettings> {
    return this.ctx.viewFile.setSettings(patch)
  }

  /** Follows the shared settings store; see `ViewGenerator.onSettings`. */
  onSettings(fn: (settings: GenerationSettings) => void): Disposable {
    return this.ctx.viewFile.onSettings(fn)
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
