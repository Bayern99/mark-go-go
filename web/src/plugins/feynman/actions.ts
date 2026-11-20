import type { ResearchCommand } from '../../host/generated/mygo'

export function actionGroup(command: Pick<ResearchCommand, 'name' | 'source' | 'sourceInfo'>): string {
  const raw = (command.source || command.sourceInfo?.source || '').trim().toLowerCase()
  if (raw === 'skill' || (!raw && command.name.startsWith('skill:'))) return 'Skills'
  if (raw === 'extension') return 'Extensions'
  if (raw === 'prompt') return 'Workflows'
  if (raw) return raw.charAt(0).toUpperCase() + raw.slice(1)
  return 'Workflows'
}

export function actionLabel(name: string): string {
  const cleaned = name.replace(/^(?:skill|prompt):/, '').replace(/[-_:]+/g, ' ').trim()
  if (!cleaned) return name
  return cleaned.charAt(0).toUpperCase() + cleaned.slice(1)
}

export function actionMeta(command: Pick<ResearchCommand, 'sourceInfo'>): string {
  const info = command.sourceInfo
  if (!info) return ''
  const parts = [info.scope, info.origin].filter((part): part is string => Boolean(part && part.trim()))
  return [...new Set(parts)].join(' · ')
}

export function activeAction(draft: string, commands: ResearchCommand[]) {
  const match = /^\/(\S+)(?:\s([\s\S]*))?$/.exec(draft)
  if (!match) return
  const command = commands.find((c) => c.name === match[1])
  if (command) return { command, argument: match[2] ?? '' }
}
