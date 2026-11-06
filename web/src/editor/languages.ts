import { go } from '@codemirror/lang-go'
import { javascript } from '@codemirror/lang-javascript'
import { json } from '@codemirror/lang-json'
import { markdown } from '@codemirror/lang-markdown'
import { python } from '@codemirror/lang-python'
import {
  LanguageDescription,
  LanguageSupport,
  StreamLanguage,
} from '@codemirror/language'
import { shell } from '@codemirror/legacy-modes/mode/shell'

const bash = new LanguageSupport(StreamLanguage.define(shell))

/** Explicit, small code-block language list (no @codemirror/language-data). */
export const languages: LanguageDescription[] = [
  LanguageDescription.of({
    name: 'TypeScript',
    alias: ['ts', 'tsx', 'typescript'],
    extensions: ['ts', 'tsx', 'mts', 'cts'],
    support: javascript({ typescript: true, jsx: true }),
  }),
  LanguageDescription.of({
    name: 'JavaScript',
    alias: ['js', 'jsx', 'javascript'],
    extensions: ['js', 'jsx', 'mjs', 'cjs'],
    support: javascript({ jsx: true }),
  }),
  LanguageDescription.of({
    name: 'JSON',
    alias: ['json', 'jsonc'],
    extensions: ['json', 'jsonc'],
    support: json(),
  }),
  LanguageDescription.of({
    name: 'Go',
    alias: ['go', 'golang'],
    extensions: ['go'],
    support: go(),
  }),
  LanguageDescription.of({
    name: 'Python',
    alias: ['py', 'python'],
    extensions: ['py'],
    support: python(),
  }),
  LanguageDescription.of({
    name: 'Bash',
    alias: ['sh', 'bash', 'shell', 'zsh'],
    extensions: ['sh', 'bash', 'zsh'],
    support: bash,
  }),
  LanguageDescription.of({
    name: 'Markdown',
    alias: ['md', 'markdown'],
    extensions: ['md', 'markdown'],
    support: markdown(),
  }),
]
