import type { Disposable } from '../host/types'
import type { CommandHandler } from './types'

export class CommandRegistry {
  private readonly commands = new Map<string, CommandHandler>()

  register(id: string, fn: CommandHandler): Disposable {
    if (this.commands.has(id)) throw new Error(`Command already registered: ${id}`)
    this.commands.set(id, fn)
    return {
      dispose: () => {
        if (this.commands.get(id) === fn) this.commands.delete(id)
      },
    }
  }

  has(id: string): boolean {
    return this.commands.has(id)
  }

  async execute(id: string, ...args: unknown[]): Promise<unknown> {
    const fn = this.commands.get(id)
    if (!fn) throw new Error(`Unknown command: ${id}`)
    return await fn(...args)
  }
}
