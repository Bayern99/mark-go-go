import { expect, it } from 'vitest'
import { cliArguments } from '../src/plugins/feynman/cliArguments'

it('passes quoted Feynman arguments literally, including empty values and shell punctuation', () => {
  expect(cliArguments('mcp add library --url "https://example.org/a b" --env \'EXPR=$HOME;*\' ""'))
    .toEqual(['mcp','add','library','--url','https://example.org/a b','--env','EXPR=$HOME;*',''])
  expect(cliArguments('--export "a session.jsonl" papers/my\\ report.html')).toEqual(['--export','a session.jsonl','papers/my report.html'])
  expect(() => cliArguments('alpha search "unterminated')).toThrow(/quoted argument/)
})
