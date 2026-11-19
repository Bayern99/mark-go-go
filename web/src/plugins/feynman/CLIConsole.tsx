import { useEffect, useRef, useState } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { events, ResearchConsoleHost } from '../../host/generated/mygo'
import type { ResearchSession } from './session'
import { cliArguments } from './cliArguments'
import { ResearchIcon } from './Conversation'
import '@xterm/xterm/css/xterm.css'
import './console.css'

const commands = [
  ['Interactive Feynman', ''], ['Guided setup', 'setup'], ['Diagnostics', 'doctor'], ['Configuration status', 'status'],
  ['Sign in to a model', 'model login'], ['List models', 'model list'], ['Choose default model', 'model set'],
  ['alphaXiv sign-in', 'alpha login'], ['Search alphaXiv', 'alpha search ""'], ['Web search settings', 'search status'],
  ['MCP servers', 'mcp list'], ['Package presets', 'packages list'], ['Resume a conversation', '--resume'],
  ['Preview setup', 'setup preview'], ['CLI reference', 'help'],
]

export function CLIConsole({ session }: { session: ResearchSession }) {
  const [line, setLine] = useState('')
  const [error, setError] = useState('')
  const [transition, setTransition] = useState(false)
  const mount = useRef<HTMLDivElement>(null)
  const field = useRef<HTMLInputElement>(null)
  const term = useRef<Terminal | null>(null)
  const fitSize = useRef<() => void>(() => {})
  const runVersion = useRef(0)
  const active = session.cliRunning

  useEffect(() => {
    const host = mount.current
    if (!host) return
    let disposed = false
    let attaching = true
    let sequence = 0
    let exitCode: number | undefined
    let exitPrinted = false
    const showExit = () => {
      if (exitCode !== undefined && !attaching && !exitPrinted) {
        terminal.writeln('\r\n[Exit ' + exitCode + ']')
        exitPrinted = true
      }
    }
    const pending: {seq: number; text: string}[] = []
    const terminal = new Terminal({
      scrollback: 2000, cursorBlink: true, fontSize: 12,
      fontFamily: '"IBM Plex Mono", Menlo, monospace',
      screenReaderMode: true,
      theme: { background: '#171b1b', foreground: '#e6e9e5', cursor: '#a5d2bc', selectionBackground: '#365749' },
    })
    const fit = new FitAddon()
    terminal.loadAddon(fit)
    terminal.open(host)
    term.current = terminal
    const fitAndResize = () => {
      if (disposed || !host.clientWidth || !host.clientHeight) return
      fit.fit()
      if (session.cliRunning) void ResearchConsoleHost.resize(session.root, Math.max(20,terminal.cols), Math.max(4,terminal.rows)).catch(() => {})
    }
    fitSize.current = fitAndResize
    const observer = new ResizeObserver(fitAndResize)
    observer.observe(host)
    const write = (seq: number, text: string) => {
      if (seq <= sequence) return
      sequence = seq
      terminal.write(text)
    }
    const off = events.researchConsoleUpdate.on(event => {
      if (disposed || event.root !== session.root) return
      if (event.kind === 'reset') {
        runVersion.current++
        sequence = 0
        exitCode = undefined
        exitPrinted = false
        attaching = false
        pending.length = 0
        terminal.reset()
        fitAndResize()
      } else if (event.kind === 'output') {
        if (attaching) pending.push({ seq: event.seq ?? 0, text: event.text ?? '' })
        else write(event.seq ?? 0, event.text ?? '')
      } else if (event.kind === 'exit') {
        session.endCLI()
        exitCode = event.exitCode ?? 0
        showExit()
      } else if (event.kind === 'error') setError(event.text || 'Feynman CLI failed.')
    })
    const version = runVersion.current
    void ResearchConsoleHost.snapshot(session.root).then(previous => {
      if (disposed || version !== runVersion.current) return
      if (previous?.text) terminal.write(previous.text)
      if (previous?.complete) exitCode = previous.exitCode ?? 0
      sequence = previous?.seq ?? 0
    }).catch(() => {}).finally(() => {
      if (disposed || version !== runVersion.current) return
      attaching = false
      pending.forEach(item => write(item.seq, item.text))
      pending.length = 0
      showExit()
      fitAndResize()
    })
    let inputWrites = Promise.resolve()
    const input = terminal.onData(data => {
      if (!session.cliRunning || disposed) return
      inputWrites = inputWrites.then(() => ResearchConsoleHost.input(session.root, data)).catch(reason => {
        if (!disposed && session.cliRunning) setError(String(reason))
      })
    })
    return () => {
      disposed = true
      off(); input.dispose(); observer.disconnect(); terminal.dispose()
      term.current = null
      fitSize.current = () => {}
    }
  }, [session])

  async function run() {
    if (transition || active) return
    setTransition(true)
    setError('')
    try {
      await session.launchCLI(cliArguments(line))
      fitSize.current()
      if (session.cliRunning) term.current?.focus()
    } catch (e) {
      setError(String(e))
    } finally { setTransition(false) }
  }
  async function stop() {
    setTransition(true)
    try { await ResearchConsoleHost.disconnect(session.root); session.endCLI() }
    catch (e) { setError(String(e)) }
    finally { setTransition(false) }
  }

  return <section className="fy-cli" aria-label="Feynman CLI console">
    <div className="fy-cli-controls">
      <div className="fy-cli-heading"><strong>Feynman CLI</strong>
        <select aria-label="CLI quick actions" value={commands.find(([, args]) => args === line)?.[1] ?? '$custom'}
          disabled={active || transition} onChange={e => {
            const args = e.target.value
            setLine(args)
            requestAnimationFrame(() => { field.current?.focus(); if (args.endsWith('""')) field.current?.setSelectionRange(args.length - 1, args.length - 1) })
          }}>
          <option value="$custom" disabled>Custom arguments</option>
          {commands.map(([label,args]) => <option key={label} value={args}>{label}</option>)}
        </select></div>
      <form className="fy-cli-launch" onSubmit={e => { e.preventDefault(); if (!active) void run() }}>
        <span>feynman</span><input ref={field} aria-label="Feynman arguments" value={line} spellCheck={false}
          placeholder="arguments…" disabled={active || transition} onChange={e => setLine(e.target.value)} />
        {active ? <button type="button" key="stop" className="pill" disabled={transition} onClick={() => void stop()}>Stop CLI</button>
          : <button type="submit" key="run" className="pill" disabled={transition || session.status === 'working' || session.status === 'connecting'}>Run</button>}
      </form>
    </div>
    {error && <p role="alert" className="fy-cli-error">{error}</p>}
    <div className="fy-cli-terminal" ref={mount} />
    <div className="fy-cli-foot"><span>{active ? 'Interactive terminal' : 'Terminal output'}</span>
      <button type="button" className="fy-icon" title="Clear terminal view" aria-label="Clear terminal view" onClick={() => term.current?.clear()}><ResearchIcon name="broom" /></button></div>
  </section>
}
