import type { Disposable } from '../host/types'

/** The editor slice plugins may observe: Markdown source offsets only. */
export interface EditorSelectionSource {
  onSelectionChange(fn: (offset: number) => void): Disposable
}

/**
 * Backs `PluginContext.editor`. The registry (and every plugin) is built
 * before the editor finishes mounting, so App attaches the editor here once
 * `createMarkdownEditor` resolves. Plugins are only activated after that
 * attach, so an unattached call is a programming error, not a silent no-op.
 */
export class EditorAccess implements EditorSelectionSource {
  private source: EditorSelectionSource | null = null

  attach(source: EditorSelectionSource): void {
    this.source = source
  }

  detach(): void {
    this.source = null
  }

  onSelectionChange(fn: (offset: number) => void): Disposable {
    if (!this.source) throw new Error('Editor is not attached yet')
    return this.source.onSelectionChange(fn)
  }
}
