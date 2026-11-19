export interface DocState {
  name: string
  path: string
  edited: boolean
  conflict: boolean
  deleted: boolean
  readOnly: boolean
  generating?: boolean
  actionError?: string
  viewMode?: string
  restoreError?: string
}

export interface DocumentInfo extends DocState {
  workspaceRoot?: string
  id: string
}

export interface HostDocSnapshot {
  id: string
  text: string
  version: number
  state: DocumentInfo
}

export type WorkspaceKind = 'folder' | 'untitled'

export interface WorkspaceSummary {
  id: string
  root: string
  name: string
  kind: WorkspaceKind
}

export interface WorkspaceSnapshot {
  id: string
  root: string
  name: string
  kind: WorkspaceKind
  workspaces: WorkspaceSummary[]
  documents: DocumentInfo[]
  activeId: string
  restoreError?: string
}

export type FolderState = 'open' | 'collapsed' | 'empty'
export type IndexStatus = 'idle' | 'scanning' | 'indexing' | 'ready' | 'error'

export interface NavHeadingHit {
  text: string
  offset: number
  level: number
}

export interface NavEntry {
  id: string
  kind: 'file' | 'folder'
  name: string
  path: string
  parent: string
  depth: number
  markdownCount?: number
  folderState?: FolderState
  title?: string
  excerpt?: string
  modifiedAt?: number
  notes?: number
  edited?: boolean
  conflict?: boolean
  deleted?: boolean
  documentId?: string
}

export interface NavSnapshot {
  root: string
  status: IndexStatus
  indexed: number
  total: number
  error?: string
  entries: NavEntry[]
  collapsedCount: number
}

export interface NavSearchHit {
  path: string
  name: string
  title: string
  excerpt: string
  documentId?: string
  headings: NavHeadingHit[]
}

export interface NavSearchResult {
  query: string
  status: IndexStatus
  matched: number
  indexed: number
  total: number
  incomplete: boolean
  hits: NavSearchHit[]
}

export interface RecentWorkspace {
  root: string
  name: string
  openedAt: number
}

export interface Crumb {
  name: string
  path: string
  isFile: boolean
}

export interface CloseResult {
  closed: boolean
  canceled: boolean
}

export type NativeCommandName =
  | 'save'
  | 'saveAs'
  | 'close'
  | 'rename'
  | 'moveTo'
  | 'revert'
  | 'duplicate'
  | 'closeWorkspace'
  | 'undo'
  | 'redo'
  | 'flushQuit'

export interface NativeCommand {
  id: string
  command: NativeCommandName
  attempt?: number
}

export interface Disposable {
  dispose(): void
}

export interface ViewSnapshot {
  exists: boolean
  text: string
  error: string
  canUndo: boolean
}

export interface ViewFileSource {
  watchView(fn: (snap: ViewSnapshot) => void): Disposable
  undoView(): Promise<void>
}

export interface SectionRef {
  heading: string
  occurrence: number
  start: number
  end: number
  exclude?: Array<{ start: number; end: number }>
}

export interface GenerateRequest {
  text: string
  mode: 'full' | 'update'
  replaceIds?: string[]
  request?: string
  format: string
  sections: SectionRef[]
}

export type GeneratePhase = 'starting' | 'thinking' | 'reading' | 'writing' | 'checking'

export interface GenerateProgress {
  phase: GeneratePhase
  text: string
}

export type GenerateFailureKind =
  | 'unsaved'
  | 'busy'
  | 'agent-not-found'
  | 'auth-required'
  | 'model-invalid'
  | 'agent-message'
  | 'invalid-output'
  | 'agent-error'
  | 'timeout'
  | 'cancelled'

export interface GenerateFailure {
  kind: GenerateFailureKind
  message: string
  hint?: string
  detail?: string
  models?: string[]
}

export interface GenerationSettings {
  model: string
  agentCommand: string
  models?: string[]
}

export interface SettingsPatch {
  model?: string
  agentCommand?: string
}

export interface ViewGenerator {
  generate(
    req: GenerateRequest,
    onProgress: (progress: GenerateProgress) => void,
    signal: AbortSignal,
  ): Promise<void>
  getSettings(): Promise<GenerationSettings>
  setSettings(patch: SettingsPatch): Promise<GenerationSettings>
  onSettings(fn: (settings: GenerationSettings) => void): Disposable
}

export interface HostAdapter extends ViewFileSource, ViewGenerator {
  readonly id: string
  load(): Promise<HostDocSnapshot>
  update(text: string): Promise<void>
  flush(text: string): Promise<void>
  save(): Promise<void>
  saveAs(): Promise<void>
  rename(): Promise<void>
  moveTo(): Promise<void>
  revert(): Promise<void>
  onReload(fn: (snap: HostDocSnapshot) => void): Disposable
  onState(fn: (state: DocumentInfo) => void): Disposable
  bindFlush?(getText: () => string): Disposable
  setViewMode(mode: string): Promise<void>
}

export interface WorkspaceAdapter {
  snapshot(): Promise<WorkspaceSnapshot>
  subscribe(fn: (snap: WorkspaceSnapshot) => void): Disposable
  open(path: string): Promise<DocumentInfo>
  openWorkspace(root: string): Promise<void>
  activateWorkspace(id: string): Promise<void>
  closeWorkspaceRoot(id: string): Promise<CloseResult>
  newDocument(): Promise<DocumentInfo>
  duplicate(id: string): Promise<DocumentInfo>
  activate(id: string): Promise<void>
  closeDocument(id: string, text?: string): Promise<CloseResult>
  closeWorkspace(): Promise<CloseResult>
  document(id: string): HostAdapter
  navSnapshot(): Promise<NavSnapshot>
  subscribeNav(fn: (snap: NavSnapshot) => void): Disposable
  setFilter(query: string): Promise<NavSearchResult>
  subscribeSearch(fn: (result: NavSearchResult) => void): Disposable
  toggleFolder(rel: string): Promise<void>
  revealPath(path: string): Promise<void>
  pickFolder(): Promise<string>
  recentWorkspaces(): Promise<RecentWorkspace[]>
}
