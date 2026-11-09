import type { Disposable } from '../host/types'

export type NativeEditCommand = 'undo' | 'redo'

export interface NativeEditCapabilities {
  canUndo: boolean
  canRedo: boolean
}

export interface NativeEditTarget {
  capabilities(): NativeEditCapabilities
  perform(command: NativeEditCommand): boolean
  /** False when this document's editor is hidden (GenUI / another center view). */
  editorVisible(): boolean
}

const nativeFields = new Set<HTMLTextAreaElement | HTMLInputElement>()
const targetRoots = new Map<string, ParentNode>()

function isNativeField(el: Element | null): el is HTMLTextAreaElement | HTMLInputElement {
  return el instanceof HTMLTextAreaElement || el instanceof HTMLInputElement
}

function nativeFieldAt(node: EventTarget | null): HTMLTextAreaElement | HTMLInputElement | null {
  if (node instanceof HTMLTextAreaElement || node instanceof HTMLInputElement) return node
  if (node instanceof Node) {
    const host = node instanceof Element ? node : node.parentElement
    const field = host?.closest('textarea, input') ?? null
    if (isNativeField(field)) return field
  }
  return null
}

function composerField(
  field: HTMLTextAreaElement | HTMLInputElement | null,
): HTMLTextAreaElement | HTMLInputElement | null {
  if (!field) return null
  if (field.closest('.composer, .mgo-note-composer, [data-testid="focus-composer"]')) return field
  if (nativeFields.has(field)) return field
  return null
}

/** Native text fields keep WebKit / AppKit undo; the page must not steal that. */
export function nativeComposerHasFocus(root?: ParentNode | null): boolean {
  const active = document.activeElement
  if (!isNativeField(active)) return false
  if (root && !root.contains(active)) return false
  return composerField(active) !== null
}

/**
 * Track a native composer field so its own undo/redo stay local even when
 * the native Edit menu is routed at a document.
 */
export function registerNativeField(field: HTMLTextAreaElement | HTMLInputElement): Disposable {
  nativeFields.add(field)
  return {
    dispose() {
      nativeFields.delete(field)
    },
  }
}

export function nativeFieldCapabilities(
  field: HTMLTextAreaElement | HTMLInputElement | null = composerField(
    nativeFieldAt(document.activeElement),
  ),
): NativeEditCapabilities {
  if (!field || field.readOnly || field.disabled) return { canUndo: false, canRedo: false }
  return { canUndo: true, canRedo: true }
}

const targets = new Map<string, NativeEditTarget>()

export function registerNativeEditTarget(
  id: string,
  target: NativeEditTarget,
  root?: ParentNode | null,
): Disposable {
  targets.set(id, target)
  if (root) targetRoots.set(id, root)
  return {
    dispose() {
      if (targets.get(id) === target) targets.delete(id)
      if (root && targetRoots.get(id) === root) targetRoots.delete(id)
    },
  }
}

export function nativeEditTarget(id: string): NativeEditTarget | undefined {
  return targets.get(id)
}

function execNative(command: NativeEditCommand): boolean {
  try {
    return document.execCommand(command)
  } catch {
    return false
  }
}

/**
 * Apply a native Undo/Redo to the captured document. A focused composer in
 * that document uses the field's own history. Read-only GenUI / ComparisonTable
 * focus does not rewrite the document.
 */
export function applyNativeEdit(id: string, command: NativeEditCommand): boolean {
  const root = targetRoots.get(id)
  if (nativeComposerHasFocus(root)) return execNative(command)
  const target = targets.get(id)
  if (!target || !target.editorVisible()) return false
  return target.perform(command)
}

export function nativeEditCapabilities(id: string): NativeEditCapabilities {
  const root = targetRoots.get(id)
  if (nativeComposerHasFocus(root)) return nativeFieldCapabilities()
  const target = targets.get(id)
  if (!target || !target.editorVisible()) return { canUndo: false, canRedo: false }
  return target.capabilities()
}
