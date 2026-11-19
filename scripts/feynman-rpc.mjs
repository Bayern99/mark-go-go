#!/usr/bin/env node
import { spawn } from 'node:child_process'
import { once } from 'node:events'
import { createConnection } from 'node:net'

const { MGO_FEYNMAN_EXECUTABLE: executable, MGO_FEYNMAN_UI_SOCKET: socketPath, MGO_FEYNMAN_UI_TOKEN: token, ...env } = process.env
const args = process.argv.slice(2)
const rpc = args.includes('--mode') && args[args.indexOf('--mode') + 1] === 'rpc'
env.PI_ACP_PI_COMMAND = executable
const pi = spawn(executable, args, { env, stdio: rpc ? ['pipe', 'pipe', 'inherit'] : 'inherit' })

for (const signal of ['SIGTERM', 'SIGINT', 'SIGHUP']) process.on(signal, () => pi.kill(signal))
pi.once('error', () => { process.exitCode = 1 })

async function* records(input, limit = 16 << 20) {
  let chunks = []
  let size = 0
  for await (const chunk of input) {
    let start = 0
    while (start < chunk.length) {
      const newline = chunk.indexOf(10, start)
      const end = newline === -1 ? chunk.length : newline + 1
      const part = chunk.subarray(start, end)
      size += part.length
      if (size > limit) throw new Error('Feynman RPC record exceeds the size limit')
      chunks.push(part)
      if (newline !== -1) {
        yield chunks.length === 1 ? part : Buffer.concat(chunks, size)
        chunks = []
        size = 0
      }
      start = end
    }
  }
  if (size) yield Buffer.concat(chunks, size)
}

function parse(record) {
  try { return JSON.parse(record.toString('utf8')) } catch { return null }
}

function visibleExtensionText(message) {
  if (message?.role !== 'custom' || message.display !== true) return ''
  if (typeof message.content === 'string') return message.content
  if (!Array.isArray(message.content)) return ''
  return message.content.filter(block => block?.type === 'text' && typeof block.text === 'string').map(block => block.text).join('\n')
}

async function write(stream, data) {
  if (!stream.write(data)) await once(stream, 'drain')
}

const sockets = new Set()
async function host(request, timeout) {
  const socket = createConnection(socketPath)
  sockets.add(socket)
  try {
    if (timeout > 0) socket.setTimeout(timeout, () => socket.destroy())
    socket.write(JSON.stringify({ ...request, token }) + '\n')
    for await (const line of records(socket, 8 << 20)) return parse(line)
  } catch {
    return null
  } finally {
    sockets.delete(socket)
    socket.destroy()
  }
}

async function relay() {
  const prompts = new Map()
  const idleChecks = new Map()
  let activity = 0
  let streaming = false
  const inputs = (async () => {
    for await (const line of records(process.stdin)) {
      const call = parse(line)
      if (call?.type === 'prompt') {
        activity++
        prompts.set(call.id, streaming ? -1 : activity)
      }
      await write(pi.stdin, line)
    }
    pi.stdin.end()
  })()
  const output = (async () => {
    for await (let line of records(pi.stdout)) {
      const event = parse(line)
      if (event?.type === 'agent_start' || event?.type === 'agent_settled') {
        activity++
        streaming = event.type === 'agent_start'
      }
      if (event?.type === 'response' && idleChecks.has(event.id)) {
        const atStart = idleChecks.get(event.id)
        idleChecks.delete(event.id)
        if (event.success && event.data?.isStreaming === false && !event.data.pendingMessageCount && !streaming && atStart === activity) {
          await write(process.stdout, '{"type":"agent_settled","aborted":false}\n')
        }
        continue
      }
      if (socketPath && event?.type === 'response' && event.command === 'get_commands' && event.success && Array.isArray(event.data?.commands)) {
        const commands = event.data.commands.filter(c => c?.source === 'extension' && typeof c.name === 'string').map(({ name, description }) => ({ name, description }))
        await host({ method: 'commands', commands }, 1500)
      }
      if (socketPath && event?.type === 'message_end') {
        const text = visibleExtensionText(event.message)
        if (text) await host({ method: 'message', text }, 1500)
      }
      if (event?.type === 'response' && event.command === 'get_messages' && Array.isArray(event.data?.messages)) {
        let changed = false
        for (const message of event.data.messages) {
          const text = visibleExtensionText(message)
          if (text) {
            message.role = 'assistant'
            message.content = [{ type: 'text', text }]
            changed = true
          }
        }
        if (changed) line = JSON.stringify(event) + '\n'
      }
      if (socketPath && event?.type === 'extension_ui_request') {
        if (event.method === 'input' || event.method === 'editor') {
          const { method, title, placeholder, prefill } = event
          void host({ method, title, placeholder, prefill }, event.timeout).then(result => {
            const answer = typeof result?.value === 'string' ? { value: result.value } : { cancelled: true }
            return write(pi.stdin, JSON.stringify({ type: 'extension_ui_response', id: event.id, ...answer }) + '\n')
          }).catch(() => {})
          continue
        }
        if ((event.method === 'setStatus' && typeof event.statusKey === 'string') ||
            (event.method === 'setWidget' && typeof event.widgetKey === 'string')) {
          const status = event.method === 'setStatus'
          const key = status ? 'status:' + event.statusKey : 'widget:' + event.widgetKey
          const text = status ? event.statusText : event.widgetLines?.join('\n')
          await host({ method: 'activity', key, text: text ?? '' }, 1500)
          continue
        }
        if (event.method === 'set_editor_text') {
          await host({ method: event.method, text: event.text }, 1500)
          continue
        }
      }
      await write(process.stdout, line)
      if (event?.type === 'response' && event.command === 'prompt') {
        const atStart = prompts.get(event.id)
        prompts.delete(event.id)
        if (event.success && event.data?.disposition === 'handled' && !streaming && atStart === activity) {
          const id = 'mgo-idle-' + event.id
          idleChecks.set(id, atStart)
          await write(pi.stdin, JSON.stringify({ type: 'get_state', id }) + '\n')
        }
      }
    }
  })()
  pi.once('close', () => {
    for (const socket of sockets) socket.destroy()
    process.stdin.destroy()
  })
  await Promise.all([inputs, output])
}

if (rpc) {
  const stop = () => { pi.kill(); process.exitCode = 1 }
  pi.stdin.on('error', stop)
  process.stdout.on('error', stop)
  relay().catch(stop)
}
pi.once('close', (code, signal) => { process.exitCode ||= code ?? (signal ? 1 : 0) })
