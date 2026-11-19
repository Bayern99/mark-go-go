import type { DocumentSnapshot } from '../../core'
import { events, ResearchHost, type ResearchArtifact, type ResearchCommand, type ResearchConfig, type ResearchPlanEntry, type ResearchTool, type ResearchUpdate } from '../../host/generated/mygo'

export type ConversationItem =
  | { role: 'user' | 'assistant' | 'notice' | 'thought'; text: string; messageId?: string; image?: string }
  | { role: 'tool'; tool: ResearchTool }
  | { role: 'plan'; steps: ResearchPlanEntry[] }

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
  messages: ConversationItem[] = []
  commands: ResearchCommand[] = []
  activity = new Map<string, string>()
  config: ResearchConfig[] = []
  title = ''
  usage?: {used: number; size: number}
  tab: 'chat' | 'files' = 'chat'
  sources: string[] = []
  configuring = false
  files: ResearchArtifact[] = []
  permission?: ResearchUpdate
  suggestedDraft?: string
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

  async selectConversation(id: string) {
    if (this.status !== 'ready' || this.configuring || this.draft.trim() || this.sources.length || this.proposal) {
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
      const wasOnline = this.status !== 'offline'
      this.status = 'offline'
      this.closing = (wasOnline ? ResearchHost.disconnect(this.root) : Promise.resolve())
        .then(() => undefined)
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
      this.activity.clear()
      this.config = []
      this.title = ''
      this.usage = undefined
      this.permission = undefined
      this.suggestedDraft = undefined
      this.progress = ''
    } else if (kind === 'assistant' || kind === 'user' || kind === 'notice' || kind === 'thought') {
      const last = this.messages.at(-1)
      if (last?.role === kind && last.messageId === update.messageId && !last.image && !update.image && (kind === 'assistant' || kind === 'thought')) {
        last.text += text
      } else {
        this.messages.push({ role: kind, text, messageId: update.messageId, image: update.image })
      }
    } else if (kind === 'tool' && update.tool) {
      const patch = update.tool
      const item = this.messages.findLast(entry => entry.role === 'tool' && entry.tool.toolCallId === patch.toolCallId)
      if (item?.role === 'tool') {
        const terminal = ((item.tool.terminalDelta ?? '') + (patch.terminalDelta ?? '')).slice(-65536)
        item.tool = { ...item.tool, ...patch, terminalDelta: terminal }
      } else {
        this.messages.push({ role: 'tool', tool: patch })
      }
    } else if (kind === 'plan') {
      const previous = this.messages.findLast(entry => entry.role === 'plan' || entry.role === 'user')
      if (previous?.role === 'plan') previous.steps = update.plan ?? []
      else this.messages.push({ role: 'plan', steps: update.plan ?? [] })
    } else if (kind === 'session' || kind === 'setTitle') this.title = text
    else if (kind === 'config') this.config = update.config ?? []
    else if (kind === 'usage' && update.size) this.usage = { used: update.used ?? 0, size: update.size }
    else if (kind === 'progress') this.progress = text
    else if (kind === 'commands') this.commands = update.commands ?? []
    else if ((kind === 'activity' || kind === 'setStatus' || kind === 'setWidget') && update.key) {
      if (text) this.activity.set(update.key, text)
      else this.activity.delete(update.key)
    }
    else if (kind === 'permission' || kind === 'select' || kind === 'confirm' || kind === 'input' || kind === 'editor') this.permission = update
    else if (kind === 'editorText' || kind === 'set_editor_text') {
      if (!this.draft.trim()) this.draft = this.commands.some(command => text === '/' + command.name) ? text + ' ' : text
      else if (text && text !== this.draft) this.suggestedDraft = text
    }
    else if (kind === 'notify') {
      if (text) this.messages.push({ role: 'notice', text })
    }
    else if (kind === 'permissionDone' && this.permission?.requestId === update.requestId) this.permission = undefined
    else if (kind === 'error') {
      this.status = 'offline'
      this.activity.clear()
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
    try {
      this.files = await ResearchHost.artifacts(this.root)
    } catch (error) {
      this.error = String(error)
    }
    this.changed()
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
      if (base && base.text !== text) this.proposal = { documentID, base, text }
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
