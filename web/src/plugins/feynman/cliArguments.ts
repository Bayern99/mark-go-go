export function cliArguments(line: string): string[] {
  const args: string[] = []
  let token = ''
  let quote = ''
  let started = false
  for (let i = 0; i < line.length; i++) {
    const ch = line[i]
    if (!quote && /\s/.test(ch)) {
      if (started) args.push(token)
      token = ''
      started = false
      continue
    }
    started = true
    if (ch === quote) quote = ''
    else if (!quote && (ch === '"' || ch === "'")) quote = ch
    else if (ch === '\\' && quote !== "'") {
      if (++i === line.length) throw new Error('Finish the escaped argument.')
      token += line[i]
    } else token += ch
  }
  if (quote) throw new Error('Close the quoted argument before running Feynman.')
  if (started) args.push(token)
  return args
}
