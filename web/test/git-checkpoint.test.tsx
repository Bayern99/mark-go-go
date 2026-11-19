import { createRoot } from 'react-dom/client'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { page, userEvent } from 'vitest/browser'
import { App } from '../src/App'
import { applyNativeEdit, nativeEditCapabilities } from '../src/editor/nativeEdit'
import { createBrowserHost } from '../src/host/browserHost'
import type { Checkpoint } from '../src/host/generated/mygo'
import { createGitPlugin } from '../src/plugins/git'

const listCheckpoints = vi.hoisted(() => vi.fn<(documentID: string, path: string) => Promise<Checkpoint[]>>())
const createCheckpoint = vi.hoisted(() =>
  vi.fn<(documentID: string, path: string, expectedText: string, label: string, source: string) => Promise<Checkpoint[]>>(),
)
const readCheckpoint = vi.hoisted(() =>
  vi.fn<(documentID: string, path: string, checkpointID: string) => Promise<string>>(),
)

vi.mock('../src/host/generated/mygo', async (importOriginal) => {
  const native = await importOriginal<typeof import('../src/host/generated/mygo')>()
  return {
    ...native,
    CheckpointHost: {
      list: listCheckpoints,
      create: createCheckpoint,
      read: readCheckpoint,
    },
  }
})

describe('Labeled Checkpoints with provenance & non-Git fallback UI', () => {
  beforeEach(() => {
    listCheckpoints.mockReset()
    createCheckpoint.mockReset()
    readCheckpoint.mockReset()
  })

  it('creates labeled checkpoints, displays provenance badges without native select, compares, and restores undoably in a Git workspace', async () => {
    const snapshots = new Map<string, string>([
      ['1700000001000000', '# Draft\n\nOriginal paragraph.\n'],
      ['1700000002000000', '# Draft\n\nFeynman baseline paragraph.\n'],
    ])
    let items: Checkpoint[] = [
      {
        id: '1700000002000000',
        createdAt: 1700000002000,
        label: 'Before Feynman edit',
        source: 'feynman',
        backend: 'git',
      },
      {
        id: '1700000001000000',
        createdAt: 1700000001000,
        label: 'Initial outline',
        source: 'user',
        backend: 'git',
      },
    ]

    listCheckpoints.mockImplementation(async () => items)
    createCheckpoint.mockImplementation(async (_docID, _path, text, label, source) => {
      const id = '1700000003000000'
      snapshots.set(id, text)
      const created: Checkpoint = {
        id,
        createdAt: 1700000003000,
        label: label || undefined,
        source: source || 'user',
        backend: 'git',
      }
      items = [created, ...items]
      return items
    })
    readCheckpoint.mockImplementation(async (_docID, _path, id) => {
      const found = snapshots.get(id)
      if (!found) throw new Error('This checkpoint is no longer available.')
      return found
    })

    const host = createBrowserHost('# Draft\n\nLive edited paragraph.\n', 'draft.md')
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    try {
      root.render(<App host={host} plugins={[createGitPlugin(host.id)]} />)

      const trigger = page.getByRole('button', { name: 'Git checkpoints' })
      await trigger.click()
      const dialog = page.getByRole('dialog', { name: 'Git checkpoints' })

      await expect.element(dialog.getByText('Before Feynman edit')).toBeVisible()
      await expect.element(dialog.getByText('Feynman', { exact: true })).toBeVisible()
      await expect.element(dialog.getByText('Initial outline')).toBeVisible()
      await expect.element(dialog.getByText('User', { exact: true })).toBeVisible()
      expect(dialog.element().querySelector('select')).toBeNull()

      const labelInput = dialog.getByRole('textbox', { name: 'Checkpoint label' })
      await userEvent.fill(labelInput, 'Ready for review')
      await dialog.getByRole('button', { name: 'Create checkpoint' }).click()

      expect(createCheckpoint).toHaveBeenCalledWith(
        host.id,
        '/demo/draft.md',
        '# Draft\n\nLive edited paragraph.\n',
        'Ready for review',
        'user',
      )
      await expect.element(dialog.getByText('Ready for review')).toBeVisible()
      expect((labelInput.element() as HTMLInputElement).value).toBe('')

      await dialog.getByRole('option', { name: /Initial outline/i }).click()
      await dialog.getByRole('button', { name: 'Compare' }).click()

      await expect.element(dialog.getByText('Changes from line 3')).toBeVisible()
      await expect.element(dialog.getByText(/Original paragraph\./)).toBeVisible()

      await dialog.getByRole('button', { name: 'Restore shown version' }).click()
      await vi.waitFor(async () => {
        expect((await host.load()).text).toBe('# Draft\n\nOriginal paragraph.\n')
      })

      expect(nativeEditCapabilities(host.id).canUndo).toBe(true)
      expect(applyNativeEdit(host.id, 'undo')).toBe(true)
      await vi.waitFor(async () => {
        expect((await host.load()).text).toBe('# Draft\n\nLive edited paragraph.\n')
      })
    } finally {
      root.unmount()
      container.remove()
    }
  })

  it('supports app-local snapshot checkpoints in non-Git workspaces without failing', async () => {
    const snapshots = new Map<string, string>([
      ['1700000010000000', '# Scratch\n\nSaved before MCP agent turn.\n'],
    ])
    let items: Checkpoint[] = [
      {
        id: '1700000010000000',
        createdAt: 1700000010000,
        label: 'Before MCP edit',
        source: 'mcp',
        backend: 'local',
      },
    ]

    listCheckpoints.mockImplementation(async () => items)
    createCheckpoint.mockImplementation(async (_docID, _path, text, label, source) => {
      const id = '1700000020000000'
      snapshots.set(id, text)
      items = [
        {
          id,
          createdAt: 1700000020000,
          label: label || undefined,
          source: source || 'user',
          backend: 'local',
        },
        ...items,
      ]
      return items
    })
    readCheckpoint.mockImplementation(async (_docID, _path, id) => snapshots.get(id) ?? '')

    const host = createBrowserHost('# Scratch\n\nCurrent non-Git buffer.\n', '/non-git-folder/scratch.md')
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    try {
      root.render(<App host={host} plugins={[createGitPlugin(host.id)]} />)

      await page.getByRole('button', { name: 'Git checkpoints' }).click()
      const dialog = page.getByRole('dialog', { name: 'Git checkpoints' })

      await expect.element(dialog.getByText('Before MCP edit')).toBeVisible()
      await expect.element(dialog.getByText('MCP', { exact: true })).toBeVisible()
      await expect.element(dialog.getByText('Local snapshot')).toBeVisible()

      await dialog.getByRole('button', { name: 'Compare' }).click()
      await expect.element(dialog.getByText(/Saved before MCP agent turn\./)).toBeVisible()

      await dialog.getByRole('button', { name: 'Restore shown version' }).click()
      await vi.waitFor(async () => {
        expect((await host.load()).text).toBe('# Scratch\n\nSaved before MCP agent turn.\n')
      })

      expect(applyNativeEdit(host.id, 'undo')).toBe(true)
      await vi.waitFor(async () => {
        expect((await host.load()).text).toBe('# Scratch\n\nCurrent non-Git buffer.\n')
      })
    } finally {
      root.unmount()
      container.remove()
    }
  })
})
