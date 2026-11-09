import { extractHeadings } from '../markdown/headings'
import { notesIn } from '../markdown/notes'
import { parseMdast, plainText } from '../markdown/mdast'
import type {
  DocumentInfo,
  HostAdapter,
  IndexStatus,
  NavEntry,
  NavSearchResult,
  NavSnapshot,
  RecentWorkspace,
} from '../host/types'

export interface BrowserNavFile {
  path: string
  name: string
  parent: string
  depth: number
  text: string
  title: string
  excerpt: string
  notes: number
  modifiedAt: number
  headings: { text: string; offset: number; level: number }[]
}

export interface BrowserNavFolder {
  path: string
  name: string
  parent: string
  depth: number
  markdownCount: number
}

export interface BrowserNavSeed {
  files?: BrowserNavFile[]
  folders?: BrowserNavFolder[]
  status?: IndexStatus
  error?: string
  recents?: RecentWorkspace[]
  pickFolder?: () => Promise<string>
}

export function fileMeta(text: string): {
  title: string
  excerpt: string
  notes: number
  headings: { text: string; offset: number; level: number }[]
} {
  const tree = parseMdast(text)
  const headings = extractHeadings(text).map((h) => ({ text: h.text, offset: h.offset, level: h.level }))
  const notes = notesIn(tree).length
  let excerpt = ''
  for (const node of tree.children) {
    if (node.type === 'heading' || node.type === 'yaml' || node.type === 'html' || node.type === 'code') continue
    const plain = plainText(node).trim()
    if (plain) {
      excerpt = clipExcerpt(plain)
      break
    }
  }
  const title = headings[0]?.text ?? excerpt
  return { title, excerpt, notes, headings }
}

function clipExcerpt(s: string): string {
  const compact = s.replace(/\s+/g, ' ').trim()
  return compact.length > 88 ? `${compact.slice(0, 88)}…` : compact
}

export function parentOf(path: string): string {
  const norm = path.replace(/\\/g, '/')
  const i = norm.lastIndexOf('/')
  return i <= 0 ? '' : norm.slice(0, i)
}

export function depthOf(root: string, path: string): number {
  const rel = relativeTo(root, path)
  if (!rel) return 0
  return rel.split('/').length - 1
}

export function relativeTo(root: string, path: string): string {
  const a = root.replace(/\\/g, '/').replace(/\/$/, '')
  const b = path.replace(/\\/g, '/')
  if (!a) return b.replace(/^\//, '')
  if (b === a) return ''
  if (b.startsWith(`${a}/`)) return b.slice(a.length + 1)
  return b
}

export function breadcrumbs(root: string, path: string): { name: string; path: string; isFile: boolean }[] {
  if (!path) return []
  const norm = path.replace(/\\/g, '/')
  const base = (root || parentOf(norm)).replace(/\\/g, '/').replace(/\/$/, '')
  const parts: { name: string; path: string; isFile: boolean }[] = []
  let cur = norm
  while (cur && cur !== '/') {
    const name = cur.split('/').pop() || cur
    parts.unshift({ name, path: cur, isFile: cur === norm })
    if (cur === base) break
    const next = parentOf(cur)
    if (next === cur) break
    cur = next
  }
  return parts
}

function containsFold(hay: string, needle: string): boolean {
  return needle !== '' && hay.toLowerCase().includes(needle)
}

/**
 * Depth-first tree order: every folder is followed by its descendants.
 * Siblings list folders before files, then names case-insensitively.
 */
function navOrder(root: string, a: NavEntry, b: NavEntry): number {
  const as = (relativeTo(root, a.path) || a.name).split('/')
  const bs = (relativeTo(root, b.path) || b.name).split('/')
  const common = Math.min(as.length, bs.length)
  for (let i = 0; i < common; i += 1) {
    if (as[i] === bs[i]) continue
    const aKind = i === as.length - 1 ? a.kind : 'folder'
    const bKind = i === bs.length - 1 ? b.kind : 'folder'
    if (aKind !== bKind) return aKind === 'folder' ? -1 : 1
    const al = as[i].toLowerCase()
    const bl = bs[i].toLowerCase()
    if (al !== bl) return al < bl ? -1 : 1
    return as[i] < bs[i] ? -1 : 1
  }
  return as.length - bs.length
}

export class BrowserNavIndex {
  files: BrowserNavFile[]
  folders: BrowserNavFolder[]
  status: IndexStatus
  error: string
  collapsed = new Set<string>()
  query = ''
  recents: RecentWorkspace[]
  pickFolderImpl?: () => Promise<string>
  private navListeners = new Set<(snap: NavSnapshot) => void>()
  private searchListeners = new Set<(result: NavSearchResult) => void>()

  constructor(
    private root: string,
    seed: BrowserNavSeed = {},
  ) {
    this.files = seed.files ? seed.files.map((f) => ({ ...f, headings: [...f.headings] })) : []
    this.folders = seed.folders ? seed.folders.map((f) => ({ ...f })) : []
    this.status = seed.status ?? (this.files.length ? 'ready' : 'idle')
    this.error = seed.error ?? ''
    this.recents = seed.recents ? [...seed.recents] : []
    this.pickFolderImpl = seed.pickFolder
  }

  snapshot(open: DocumentInfo[], texts: Map<string, string>): NavSnapshot {
    const byPath = new Map(open.filter((d) => d.path).map((d) => [d.path, d]))
    const entries: NavEntry[] = []
    const visible = (parent: string) => {
      let p = relativeTo(this.root, parent)
      while (p) {
        if (this.collapsed.has(p)) return false
        p = parentOf(p)
      }
      return true
    }
    for (const folder of this.folders) {
      if (folder.markdownCount <= 0) continue
      if (!visible(folder.parent)) continue
      entries.push({
        id: `folder:${folder.path}`,
        kind: 'folder',
        name: folder.name,
        path: folder.path,
        parent: folder.parent,
        depth: folder.depth,
        markdownCount: folder.markdownCount,
        folderState: this.collapsed.has(relativeTo(this.root, folder.path)) ? 'collapsed' : 'open',
      })
    }
    const seen = new Set<string>()
    for (const file of this.files) {
      if (!visible(file.parent)) continue
      const live = texts.get(file.path)
      const meta = live !== undefined ? fileMeta(live) : file
      const openDoc = byPath.get(file.path)
      if (openDoc) seen.add(openDoc.id)
      entries.push({
        id: `file:${file.path}`,
        kind: 'file',
        name: file.name,
        path: file.path,
        parent: file.parent,
        depth: file.depth,
        title: meta.title,
        excerpt: meta.excerpt,
        modifiedAt: file.modifiedAt,
        notes: meta.notes,
        edited: openDoc?.edited,
        conflict: openDoc?.conflict,
        deleted: openDoc?.deleted,
        documentId: openDoc?.id,
      })
    }
    for (const doc of open) {
      if (!doc.path || seen.has(doc.id)) continue
      if (this.root && !doc.path.replace(/\\/g, '/').startsWith(this.root.replace(/\\/g, '/'))) continue
      entries.push({
        id: `open:${doc.id}`,
        kind: 'file',
        name: doc.name,
        path: doc.path,
        parent: parentOf(doc.path),
        depth: depthOf(this.root, doc.path),
        edited: doc.edited,
        conflict: doc.conflict,
        deleted: true,
        documentId: doc.id,
      })
    }
    entries.sort((a, b) => navOrder(this.root, a, b))
    let collapsedCount = 0
    for (const rel of this.collapsed) {
      const folder = this.folders.find((f) => relativeTo(this.root, f.path) === rel)
      if (folder) collapsedCount += folder.markdownCount
    }
    return {
      root: this.root,
      status: this.status,
      indexed: this.files.length,
      total: this.files.length,
      error: this.error || undefined,
      entries,
      collapsedCount,
    }
  }

  search(open: DocumentInfo[], texts: Map<string, string>): NavSearchResult {
    const q = this.query.trim()
    const needle = q.toLowerCase()
    const incomplete = this.status === 'scanning' || this.status === 'indexing'
    const result: NavSearchResult = {
      query: q,
      status: this.status,
      matched: 0,
      indexed: this.files.length,
      total: this.files.length,
      incomplete,
      hits: [],
    }
    if (!q) return result
    const byPath = new Map(open.filter((d) => d.path).map((d) => [d.path, d]))
    for (const file of this.files) {
      const live = texts.get(file.path)
      const meta = live !== undefined ? fileMeta(live) : file
      const headings = meta.headings.filter((h) => containsFold(h.text, needle))
      const matched =
        containsFold(file.path, needle) ||
        containsFold(file.name, needle) ||
        containsFold(meta.title, needle) ||
        containsFold(meta.excerpt, needle) ||
        headings.length > 0
      if (!matched) continue
      result.hits.push({
        path: file.path,
        name: file.name,
        title: meta.title,
        excerpt: meta.excerpt,
        documentId: byPath.get(file.path)?.id,
        headings,
      })
    }
    result.matched = result.hits.length
    return result
  }

  subscribeNav(fn: (snap: NavSnapshot) => void) {
    this.navListeners.add(fn)
    return () => this.navListeners.delete(fn)
  }

  subscribeSearch(fn: (result: NavSearchResult) => void) {
    this.searchListeners.add(fn)
    return () => this.searchListeners.delete(fn)
  }

  emit(open: DocumentInfo[], texts: Map<string, string>) {
    const snap = this.snapshot(open, texts)
    for (const fn of this.navListeners) fn(snap)
    const search = this.search(open, texts)
    for (const fn of this.searchListeners) fn(search)
  }

  toggleFolder(rel: string) {
    if (this.collapsed.has(rel)) this.collapsed.delete(rel)
    else this.collapsed.add(rel)
  }

  reveal(path: string) {
    let p = parentOf(relativeTo(this.root, path))
    while (p) {
      this.collapsed.delete(p)
      p = parentOf(p)
    }
  }

  setQuery(query: string) {
    this.query = query.trim()
  }

  remember(root: string) {
    const name = root.replace(/\\/g, '/').split('/').pop() || root
    this.recents = [{ root, name, openedAt: Date.now() }, ...this.recents.filter((item) => item.root !== root)].slice(0, 12)
  }
}

export async function liveTexts(open: { id: string; path: string }[], document: (id: string) => HostAdapter): Promise<Map<string, string>> {
  const texts = new Map<string, string>()
  await Promise.all(
    open.map(async (doc) => {
      if (!doc.path) return
      try {
        const snap = await document(doc.id).load()
        texts.set(doc.path, snap.text)
      } catch {
        /* keep disk index */
      }
    }),
  )
  return texts
}
