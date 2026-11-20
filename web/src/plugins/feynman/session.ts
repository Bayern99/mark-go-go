import { isMyGo } from 'mygo-runtime'
import type { DocumentSnapshot } from '../../core'
import { compareProposalText } from '../DocumentDiff'
import {
  events,
  ReviewHost,
  ResearchHost,
  type ChangeProposal,
  type HunkDecision,
  type ResearchArtifact,
  type ResearchCommand,
  type ResearchConfig,
  type ResearchModel,
  type ResearchPlanEntry,
  type ResearchQueueState,
  type ResearchSessionState,
  type ResearchSessionStats,
  type ResearchTool,
  type ResearchUpdate,
  type ReviewUpdateEvent,
} from '../../host/generated/mygo'

export type ConversationItem =
  | { role: 'user' | 'assistant' | 'thought'; text: string; messageId?: string; image?: string }
  | { role: 'notice'; text: string; notifyType?: string; messageId?: string }
  | { role: 'tool'; tool: ResearchTool }
  | { role: 'plan'; steps: ResearchPlanEntry[] }

export interface WidgetEntry {
  lines: string[]
  placement?: string
}

export interface Proposal {
  id: string
  documentID: string
  base: DocumentSnapshot
  text: string
  source?: string
  summary?: string
  decisions: Record<string, HunkDecision>
}

const sessions = new Map<string, ResearchSession>()

export class ResearchSession {
  private listeners = new Set<() => void>()
  private revision = 0
  private owners = 0
  private off?: () => void
  private offReview?: () => void
  private offFiles?: () => void
  private filesRequest = 0
  private closing?: Promise<void>
  private generation = 0

  status: 'offline' | 'connecting' | 'ready' | 'working' = 'offline'
  messages: ConversationItem[] = []
  commands: ResearchCommand[] = []
  activity = new Map<string, string>()
  statuses = new Map<string, string>()
  widgets = new Map<string, WidgetEntry>()
  config: ResearchConfig[] = []
  models: ResearchModel[] = []
  thinkingLevels: string[] = []
  thinkingLevel = ''
  sessionState?: ResearchSessionState
  stats?: ResearchSessionStats
  queue: ResearchQueueState = { steering: [], followUp: [] }
  title = ''
  usage?: { used: number; size: number }
  tab: 'chat' | 'review' = 'chat'
  sources: string[] = []
  configuring = false
  files: ResearchArtifact[] = []
  permission?: ResearchUpdate
  suggestedDraft?: string
  proposals: Proposal[] = []
  activeProposalId = ''
  draft = ''
  progress = ''
  error = ''

  private constructor(readonly root: string) {}

  get proposal(): Proposal | undefined {
    return this.proposals.find((p) => p.id === this.activeProposalId) ?? this.proposals[0]
  }

  set proposal(next: Proposal | undefined) {
    if (!next) {
      if (this.activeProposalId) {
        this.proposals = this.proposals.filter((p) => p.id !== this.activeProposalId)
        this.activeProposalId = this.proposals[0]?.id ?? ''
      } else {
        this.proposals = []
      }
      return
    }
    this.upsertProposal(next)
  }

  upsertProposal(
    input: Omit<Proposal, 'id' | 'decisions'> & { id?: string; decisions?: Record<string, HunkDecision> },
  ): Proposal {
    const existing = this.proposals.find(
      (p) =>
        p.base.path && input.base.path
          ? p.base.path === input.base.path
          : p.documentID !== '' && p.documentID === input.documentID,
    )
    const id = existing?.id || (input.id && !this.proposals.some(p => p.id === input.id) ? input.id : '') ||
      `local-${Array.from(crypto.getRandomValues(new Uint32Array(4)), n => n.toString(16).padStart(8, '0')).join('')}`
    const next: Proposal = {
      id,
      documentID: input.documentID,
      base: input.base,
      text: input.text,
      source: input.source,
      summary: input.summary,
      decisions: input.decisions ?? {},
    }
    if (existing) {
      this.proposals = this.proposals.map((p) => (p.id === existing.id ? next : p))
    } else {
      this.proposals = [...this.proposals, next]
    }
    this.activeProposalId = id
    if (isMyGo()) {
      void ReviewHost.submit({
        id,
        workspaceRoot: this.root,
        documentId: next.documentID,
        path: next.base.path,
        source: next.source ?? 'feynman',
        summary: next.summary,
        baseVersion: next.base.version,
        baseText: next.base.text,
        proposedText: next.text,
      }).catch(() => {})
    }
    this.changed()
    return next
  }

  selectProposal(id: string) {
    if (this.proposals.some((p) => p.id === id)) {
      this.activeProposalId = id
      this.changed()
    }
  }

  async updateProposalBase(id: string, documentID: string, base: DocumentSnapshot) {
    const proposal = this.proposals.find(p => p.id === id)
    if (!proposal) return
    const text = compareProposalText(proposal.base.text, proposal.text, base.text)
    if (base.text === text) {
      this.completeProposal(id)
      return
    }
    await this.reviseProposal(proposal, { ...proposal, documentID, base, text, decisions: {} }, base.text, proposal.text)
  }

  async applyProposal(proposal: Proposal, remainingBaseText: string, remainingText: string, liveText: string) {
    if (remainingText === remainingBaseText) {
      this.completeProposal(proposal.id)
      return
    }
    const base = { ...proposal.base, text: remainingBaseText }
    await this.reviseProposal(proposal, { ...proposal, base, text: remainingText, decisions: {} }, liveText)
  }

  private async reviseProposal(previous: Proposal, next: Proposal, liveText: string, proposedText = next.text) {
    if (isMyGo()) {
      await ReviewHost.revise({
        id: next.id,
        workspaceRoot: this.root,
        documentId: next.documentID,
        path: next.base.path,
        source: next.source,
        summary: next.summary,
        baseVersion: next.base.version,
        baseText: next.base.text,
        proposedText,
      }, previous.text, liveText)
    }
    this.proposals = this.proposals.map(p => p.id === previous.id && p.text === previous.text ? next : p)
    this.changed()
  }

  decideHunk(proposalId: string, hunkId: string, decision: HunkDecision, liveText = '') {
    this.proposals = this.proposals.map((p) =>
      p.id === proposalId ? { ...p, decisions: { ...p.decisions, [hunkId]: decision } } : p,
    )
    if (isMyGo()) {
      void ReviewHost.decideHunk(this.root, proposalId, hunkId, decision, liveText).catch(() => {})
    }
    this.changed()
  }

  acceptAllHunks(proposalId: string, hunkIds: string[], liveText = '') {
    this.proposals = this.proposals.map((p) => {
      if (p.id !== proposalId) return p
      const nextDecisions = { ...p.decisions }
      for (const hid of hunkIds) nextDecisions[hid] = 'accepted'
      return { ...p, decisions: nextDecisions }
    })
    if (isMyGo()) {
      void ReviewHost.acceptAll(this.root, proposalId, liveText).catch(() => {})
    }
    this.changed()
  }

  dismissProposal(proposalId: string) {
    this.proposals = this.proposals.filter((p) => p.id !== proposalId)
    if (this.activeProposalId === proposalId) {
      this.activeProposalId = this.proposals[0]?.id ?? ''
    }
    if (isMyGo()) {
      void ReviewHost.dismiss(this.root, proposalId).catch(() => {})
    }
    this.changed()
  }

  private completeProposal(proposalId: string) {
    this.proposals = this.proposals.filter((p) => p.id !== proposalId)
    if (this.activeProposalId === proposalId) {
      this.activeProposalId = this.proposals[0]?.id ?? ''
    }
    if (isMyGo()) {
      void ReviewHost.complete(this.root, proposalId).catch(() => {})
    }
    this.changed()
  }

  private reviewRevision = -1

  private mergeBackendProposals(event: ReviewUpdateEvent) {
    if (event.revision <= this.reviewRevision) return
    this.reviewRevision = event.revision
    const mapped: Proposal[] = event.proposals.map((cp: ChangeProposal) => {
      const decisions: Record<string, HunkDecision> = {}
      for (const h of cp.hunks) decisions[h.id] = h.decision
      const existing = this.proposals.find((p) => p.id === cp.id)
      return {
        id: cp.id,
        documentID: cp.documentId || existing?.documentID || '',
        base: {
          path: cp.path,
          text: cp.baseText,
          version: cp.baseVersion ?? existing?.base.version ?? 1,
        },
        text: cp.proposedText,
        source: cp.source,
        summary: cp.summary,
        decisions,
      }
    })
    this.proposals = mapped
    if (!mapped.some((p) => p.id === this.activeProposalId)) {
      this.activeProposalId = mapped[0]?.id ?? ''
    }
    this.changed()
  }

  static acquire(root: string) {
    const session = sessions.get(root) ?? new ResearchSession(root)
    sessions.set(root, session)
    session.owners++
    session.off ??= events.researchUpdate?.on((event) => {
      if (event.root === root && session.owners > 0) session.receive(event)
    })
    session.offFiles ??= events.researchFilePublished?.on(({ workspaceRoot, file }) => {
      if (workspaceRoot !== root || session.owners <= 0) return
      if (session.status === 'ready') {
        void session.refresh()
      } else if (session.status === 'working') {
        session.filesRequest++
        session.files = [...session.files.filter((existing) => existing.path !== file.path), file]
          .sort((a, b) => a.path.localeCompare(b.path))
        session.changed()
      }
    })
    if (!session.offReview) {
      try {
        session.offReview = events.reviewUpdated?.on((event: ReviewUpdateEvent) => {
          if (event.workspaceRoot === root && session.owners > 0) {
            session.mergeBackendProposals(event)
          }
        })
      } catch {
      }
    }
    if (isMyGo() && session.owners === 1) {
      void ReviewHost.snapshot?.(root)
        .then((snapshot) => {
          if (session.owners > 0) session.mergeBackendProposals(snapshot)
        })
        .catch(() => {})
    }
    return session
  }

  async selectConversation(id: string) {
    if (
      this.status !== 'ready' ||
      this.configuring ||
      this.draft.trim() ||
      this.sources.length ||
      this.proposals.length > 0
    ) {
      throw new Error('Finish the current draft or edit before switching conversations.')
    }
    const generation = ++this.generation
    this.status = 'connecting'
    this.error = ''
    this.changed()
    try {
      await ResearchHost.selectSession(this.root, id)
    } finally {
      if (generation === this.generation && this.status === 'connecting') {
        this.status = 'ready'
        this.changed()
      }
    }
  }

  release() {
    if (--this.owners > 0) return
    queueMicrotask(() => {
      if (this.owners > 0) return
      this.generation++
      this.off?.()
      this.off = undefined
      this.offReview?.()
      this.offReview = undefined
      this.offFiles?.()
      this.offFiles = undefined
      this.filesRequest++
      const wasOnline = this.status !== 'offline'
      this.status = 'offline'
      this.closing = (wasOnline ? ResearchHost.disconnect(this.root) : Promise.resolve())
        .then(() => undefined)
        .catch((error) => {
          this.error = String(error)
        })
        .finally(() => {
          if (!this.owners && sessions.get(this.root) === this) sessions.delete(this.root)
        })
    })
  }

  subscribe = (fn: () => void) => {
    this.listeners.add(fn)
    return () => {
      this.listeners.delete(fn)
    }
  }
  snapshot = () => this.revision
  changed() {
    this.revision++
    this.listeners.forEach((fn) => fn())
  }

  private receive(update: ResearchUpdate) {
    const { kind, text = '' } = update
    if (kind === 'reset') {
      this.messages = []
      this.commands = []
      this.activity.clear()
      this.statuses.clear()
      this.widgets.clear()
      this.config = []
      this.models = []
      this.thinkingLevels = []
      this.thinkingLevel = ''
      this.sessionState = undefined
      this.stats = undefined
      this.queue = { steering: [], followUp: [] }
      this.title = ''
      this.usage = undefined
      this.permission = undefined
      this.suggestedDraft = undefined
      this.progress = ''
    } else if (kind === 'assistant' || kind === 'user' || kind === 'thought') {
      const last = this.messages.at(-1)
      if (
        last?.role === kind &&
        last.messageId === update.messageId &&
        !last.image &&
        !update.image &&
        (kind === 'assistant' || kind === 'thought')
      ) {
        last.text += text
      } else {
        this.messages.push({ role: kind, text, messageId: update.messageId, image: update.image })
      }
    } else if (kind === 'notice') {
      if (text) this.messages.push({ role: 'notice', text, notifyType: update.notifyType })
    } else if (kind === 'notify') {
      const body = update.message || text
      if (body) {
        this.messages.push({
          role: 'notice',
          text: body,
          notifyType: update.notifyType || 'info',
        })
      }
    } else if (kind === 'tool' && update.tool) {
      const patch = update.tool
      const item = this.messages.findLast(
        (entry) => entry.role === 'tool' && entry.tool.toolCallId === patch.toolCallId,
      )
      if (item?.role === 'tool') {
        const terminal = ((item.tool.terminalDelta ?? '') + (patch.terminalDelta ?? '')).slice(-65536)
        item.tool = { ...item.tool, ...patch, terminalDelta: terminal }
      } else {
        this.messages.push({ role: 'tool', tool: patch })
      }
    } else if (kind === 'plan') {
      const previous = this.messages.findLast((entry) => entry.role === 'plan' || entry.role === 'user')
      if (previous?.role === 'plan') previous.steps = update.plan ?? []
      else this.messages.push({ role: 'plan', steps: update.plan ?? [] })
    } else if (kind === 'session' || kind === 'setTitle') {
      this.title = update.title || text
    } else if (kind === 'config') {
      if (update.config) this.config = update.config
      if (update.models) this.models = update.models
      if (update.thinkingLevels) this.thinkingLevels = update.thinkingLevels
      if (update.thinkingLevel) this.thinkingLevel = update.thinkingLevel
      if (update.state) this.sessionState = update.state
    } else if (kind === 'usage') {
      if (update.stats) this.stats = update.stats
      if (update.size) this.usage = { used: update.used ?? 0, size: update.size }
      else if (update.stats?.contextUsage?.contextWindow) {
        this.usage = {
          used: update.stats.contextUsage.tokens,
          size: update.stats.contextUsage.contextWindow,
        }
      }
    } else if (kind === 'queue') {
      this.queue = update.queue ?? { steering: [], followUp: [] }
    } else if (kind === 'progress') {
      this.progress = text
    } else if (kind === 'compactionStart') {
      this.progress = text || 'Compacting conversation context…'
    } else if (kind === 'compactionEnd') {
      this.progress = ''
      if (text) {
        this.messages.push({ role: 'notice', text, notifyType: update.aborted ? 'warning' : 'error' })
      } else if (update.compaction?.summary) {
        this.messages.push({ role: 'notice', text: 'Conversation context compacted.', notifyType: 'info' })
      }
    } else if (kind === 'retryStart') {
      this.progress = update.retryAttempt
        ? `Retrying (${update.retryAttempt}${update.retryMaxAttempts ? `/${update.retryMaxAttempts}` : ''})…`
        : text || 'Retrying…'
    } else if (kind === 'retryEnd') {
      this.progress = ''
      if (!update.retrySuccess && text) {
        this.messages.push({ role: 'notice', text, notifyType: 'error' })
      }
    } else if (kind === 'agentSettled') {
      this.progress = ''
    } else if (kind === 'commands') {
      this.commands = update.commands ?? []
    } else if (kind === 'setStatus') {
      const key = update.statusKey || update.key || ''
      if (key) {
        if (text) this.statuses.set(key, text)
        else this.statuses.delete(key)
      }
    } else if (kind === 'setWidget') {
      const key = update.widgetKey || update.key || ''
      const lines = update.widgetLines?.length ? update.widgetLines : text ? text.split('\n') : []
      if (key) {
        if (lines.length > 0) this.widgets.set(key, { lines, placement: update.widgetPlacement })
        else this.widgets.delete(key)
      }
    } else if (kind === 'activity' && update.key) {
      if (text) this.activity.set(update.key, text)
      else this.activity.delete(update.key)
    } else if (
      kind === 'permission' ||
      kind === 'select' ||
      kind === 'confirm' ||
      kind === 'input' ||
      kind === 'editor'
    ) {
      this.permission = update
    } else if (kind === 'editorText' || kind === 'set_editor_text') {
      if (!this.draft.trim()) {
        this.draft = this.commands.some((command) => text === '/' + command.name) ? text + ' ' : text
      } else if (text && text !== this.draft) {
        this.suggestedDraft = text
      }
    } else if (kind === 'permissionDone' && this.permission?.requestId === update.requestId) {
      this.permission = undefined
    } else if (kind === 'error') {
      this.status = 'offline'
      this.activity.clear()
      this.statuses.clear()
      this.permission = undefined
      this.progress = ''
      this.error = text
    }
    this.changed()
  }

  async connect() {
    if (this.status !== 'offline' || !this.root) return
    const generation = ++this.generation
    this.status = 'connecting'
    this.error = ''
    this.changed()
    try {
      await this.closing
      if (generation !== this.generation) return
      await ResearchHost.connect(this.root)
      if (generation !== this.generation) return
      this.status = 'ready'
      await this.refresh()
    } catch (error) {
      if (generation !== this.generation) return
      this.status = 'offline'
      this.error = String(error)
    }
    this.changed()
  }

  async configure(id: string, value: string) {
    if (this.status !== 'ready' || this.configuring) return
    this.configuring = true
    this.error = ''
    this.changed()
    try {
      await ResearchHost.setConfig(this.root, id, value)
    } catch (error) {
      this.error = String(error)
    } finally {
      this.configuring = false
      this.changed()
    }
  }

  async setModel(provider: string, modelId: string) {
    if (this.status !== 'ready' || this.configuring) return
    this.configuring = true
    this.error = ''
    this.changed()
    try {
      if (typeof ResearchHost.setModel === 'function') {
        const model = await ResearchHost.setModel(this.root, provider, modelId)
        if (this.sessionState) this.sessionState = { ...this.sessionState, model }
      } else {
        await ResearchHost.setConfig(this.root, 'model', provider ? `${provider}/${modelId}` : modelId)
      }
    } catch (error) {
      this.error = String(error)
    } finally {
      this.configuring = false
      this.changed()
    }
  }

  async setThinkingLevel(level: string) {
    if (this.status !== 'ready' || this.configuring) return
    this.configuring = true
    this.error = ''
    this.changed()
    try {
      if (typeof ResearchHost.setThinkingLevel === 'function') {
        await ResearchHost.setThinkingLevel(this.root, level)
        this.thinkingLevel = level
        if (this.sessionState) this.sessionState = { ...this.sessionState, thinkingLevel: level }
      } else {
        await ResearchHost.setConfig(this.root, 'thinking_level', level)
      }
    } catch (error) {
      this.error = String(error)
    } finally {
      this.configuring = false
      this.changed()
    }
  }

  async renameSession(name: string) {
    const trimmed = name.trim()
    if (!trimmed || this.status !== 'ready' || this.configuring) return
    this.configuring = true
    this.error = ''
    this.changed()
    try {
      await ResearchHost.setSessionName(this.root, trimmed)
      this.title = trimmed
      if (this.sessionState) this.sessionState = { ...this.sessionState, sessionName: trimmed }
    } catch (error) {
      this.error = String(error)
    } finally {
      this.configuring = false
      this.changed()
    }
  }

  async forkFrom(entryId: string) {
    if (!entryId || this.status !== 'ready' || this.configuring) return
    this.configuring = true
    this.error = ''
    this.changed()
    try {
      const result = await ResearchHost.fork(this.root, entryId)
      if (!result.cancelled && result.text && !this.draft.trim()) {
        this.draft = result.text
      }
      return result
    } catch (error) {
      this.error = String(error)
      throw error
    } finally {
      this.configuring = false
      this.changed()
    }
  }

  async attach() {
    try {
      const paths = await ResearchHost.pickSources(this.root)
      this.sources = [...new Set([...this.sources, ...paths])]
    } catch (error) {
      this.error = String(error)
    }
    this.changed()
  }

  async refresh() {
    const request = ++this.filesRequest
    try {
      const files = await ResearchHost.artifacts(this.root)
      if (request !== this.filesRequest) return
      this.files = files
      if (!this.stats && typeof ResearchHost.sessionStats === 'function') {
        try {
          const stats = await ResearchHost.sessionStats(this.root)
          if (stats) {
            this.stats = stats
            if (stats.contextUsage?.contextWindow) {
              this.usage = {
                used: stats.contextUsage.tokens,
                size: stats.contextUsage.contextWindow,
              }
            }
          }
        } catch {
        }
      }
    } catch (error) {
      if (request !== this.filesRequest) return
      this.error = String(error)
    }
    this.changed()
  }

  async steer() {
    const prompt = this.draft.trim()
    if (!prompt || this.status !== 'working') return
    const sources = this.sources
    this.sources = []
    this.draft = ''
    this.error = ''
    this.changed()
    try {
      await ResearchHost.steer(this.root, prompt, sources)
    } catch (error) {
      this.error = String(error)
      if (!this.draft) {
        this.draft = prompt
        this.sources = [...new Set([...sources, ...this.sources])]
      }
      this.changed()
    }
  }

  async followUp() {
    const prompt = this.draft.trim()
    if (!prompt || this.status !== 'working') return
    const sources = this.sources
    this.sources = []
    this.draft = ''
    this.error = ''
    this.changed()
    try {
      await ResearchHost.followUp(this.root, prompt, sources)
      this.queue = {
        ...this.queue,
        followUp: [...this.queue.followUp, prompt],
      }
    } catch (error) {
      this.error = String(error)
      if (!this.draft) {
        this.draft = prompt
        this.sources = [...new Set([...sources, ...this.sources])]
      }
    }
    this.changed()
  }

  async clearQueue() {
    this.error = ''
    try {
      const next = await ResearchHost.clearQueue(this.root)
      this.queue = next ?? { steering: [], followUp: [] }
    } catch (error) {
      this.error = String(error)
    }
    this.changed()
  }

  async abort() {
    this.error = ''
    try {
      if (typeof ResearchHost.abort === 'function') await ResearchHost.abort(this.root)
      else await ResearchHost.cancel(this.root)
    } catch (error) {
      this.error = String(error)
      this.changed()
    }
  }

  async send(documentID: string, base?: DocumentSnapshot) {
    const prompt = this.draft.trim()
    if (!prompt || this.status !== 'ready' || this.configuring) return
    const generation = this.generation
    const sources = this.sources
    this.sources = []
    this.status = 'working'
    this.progress = ''
    this.error = ''
    this.draft = ''
    this.changed()
    try {
      const text = sources.length
        ? await ResearchHost.promptWithSources(this.root, prompt, base ? documentID : '', base?.text ?? '', sources)
        : await ResearchHost.prompt(this.root, prompt, base ? documentID : '', base?.text ?? '')
      if (generation !== this.generation) return
      if (base && text && base.text !== text) {
        this.upsertProposal({ documentID, base, text, source: 'feynman' })
        this.tab = 'review'
      }
    } catch (error) {
      if (generation !== this.generation) return
      this.error = String(error)
      if (!this.draft) {
        this.draft = prompt
        this.sources = [...new Set([...sources, ...this.sources])]
      }
    } finally {
      if (generation === this.generation) {
        if (this.status === 'working') this.status = 'ready'
        this.permission = undefined
        this.progress = ''
        await this.refresh()
      }
    }
  }
}
