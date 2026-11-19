import type { ResearchCommand } from '../../host/generated/mygo'

const workflows: Record<string, string> = {
  audit: 'Audit paper and code', autoresearch: 'Run experiments', compare: 'Compare sources',
  deepresearch: 'Deep research', draft: 'Draft a paper', lit: 'Literature review',
  log: 'Log research progress', recipe: 'Find a training recipe', replicate: 'Plan a replication',
  review: 'Review a draft', summarize: 'Summarize a source',
}

const sessionActions: Record<string, string> = {
  autocompact: 'Automatic compaction', changelog: 'View changelog', compact: 'Compact conversation',
  export: 'Export conversation', 'follow-up': 'Follow-up delivery', name: 'Rename conversation',
  session: 'Conversation statistics', steering: 'Steering message delivery',
}

const extensionActions: Record<string, string> = {
  help: 'Feynman help', init: 'Initialize research project', outputs: 'Browse research outputs',
  tools: 'Browse research tools', 'service-tier': 'Choose service tier', run: 'Run a subagent',
  subagents: 'Manage subagents', websearch: 'Web search', curator: 'Research curator',
}

export function actionGroup({ name, source }: ResearchCommand) {
  if (name.startsWith('skill:')) return 'Skills'
  if (Object.hasOwn(sessionActions, name)) return 'Session'
  if (source === 'extension') return 'Extensions'
  return 'Workflows'
}

export function actionLabel(name: string) {
  if (Object.hasOwn(workflows, name)) return workflows[name]
  if (Object.hasOwn(sessionActions, name)) return sessionActions[name]
  if (Object.hasOwn(extensionActions, name)) return extensionActions[name]
  const words = name.replace(/^skill:/, '').replace(/[-_]+/g, ' ')
  return words.charAt(0).toUpperCase() + words.slice(1)
}

export function activeAction(draft: string, commands: ResearchCommand[]) {
  const match = /^\/(\S+)(?:\s([\s\S]*))?$/.exec(draft)
  if (!match) return
  const command = commands.find(c => c.name === match[1])
  if (command) return { command, argument: match[2] ?? '' }
}
