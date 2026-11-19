import type { DocumentSnapshot } from '../../core'
import { events, ResearchHost, type ResearchArtifact, type ResearchUpdate } from '../../host/generated/mygo'

interface Proposal {
  documentID: string
  base: DocumentSnapshot
  text: string
}

const sessions = new Map<string, ResearchSession>()

export class ResearchSession {
  private listeners = new Set<() => void>()
  private revision = 0
  private owners = 0
  private off?: () => void
  private closing?: Promise<void>
  private generation = 0

  status: 'offline' | 'connecting' | 'ready' | 'working' = 'offline'
  messages: {role: 'user' | 'assistant' | 'notice'; text: string}[] = []
  commands: string[] = []
  files: ResearchArtifact[] = []
  permission?: ResearchUpdate
  proposal?: Proposal
  draft = ''
  progress = ''
  error = ''

  private constructor(readonly root: string) {}

  static acquire(root: string) {
    const session = sessions.get(root) ?? new ResearchSession(root)
    sessions.set(root, session)
    session.owners++
    session.off ??= events.researchUpdate.on(event => {
      if (event.root === root && session.owners > 0) session.receive(event)
    })
    return session
  }

  release() {
    if (--this.owners > 0) return
    queueMicrotask(() => {
      if (this.owners > 0) return
      this.generation++
      this.off?.()
      this.off = undefined
      const wasOnline = this.status !== 'offline'
      this.status = 'offline'
      this.closing = (wasOnline ? ResearchHost.disconnect(this.root) : Promise.resolve())
        .catch(error => { this.error = String(error) }).finally(() => {
          if (!this.owners && sessions.get(this.root) === this) sessions.delete(this.root)
        })
    })
  }

  subscribe = (fn: () => void) => {
    this.listeners.add(fn)
    return () => { this.listeners.delete(fn) }
  }
  snapshot = () => this.revision
  changed() {
    this.revision++
    this.listeners.forEach(fn => fn())
  }

  private receive(update: ResearchUpdate) {
    const { kind, text = '' } = update
    if (kind === 'reset') {
      this.messages = []
      this.commands = []
      this.permission = undefined
      this.progress = ''
    } else if (kind === 'assistant' || kind === 'user' || kind === 'notice') {
      const last = this.messages.at(-1)
      if (last?.role === kind && kind === 'assistant') last.text += text
      else this.messages.push({ role: kind, text })
    } else if (kind === 'progress') this.progress = text
    else if (kind === 'commands') this.commands = update.commands ?? []
    else if (kind === 'permission') this.permission = update
    else if (kind === 'permissionDone' && this.permission?.requestId === update.requestId) this.permission = undefined
    else if (kind === 'error') {
      this.status = 'offline'
      this.permission = undefined
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

  async refresh() {
    try {
      this.files = await ResearchHost.artifacts(this.root)
    } catch (error) {
      this.error = String(error)
    }
    this.changed()
  }

  async send(documentID: string, base?: DocumentSnapshot) {
    const prompt = this.draft.trim()
    if (!prompt || this.status !== 'ready') return
    const generation = this.generation
    this.status = 'working'
    this.progress = ''
    this.error = ''
    this.draft = ''
    this.changed()
    try {
      const text = await ResearchHost.prompt(this.root, prompt, base ? documentID : '', base?.text ?? '')
      if (generation !== this.generation) return
      if (base && base.text !== text) this.proposal = { documentID, base, text }
    } catch (error) {
      if (generation !== this.generation) return
      this.error = String(error)
      if (!this.draft) this.draft = prompt
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
