import { Channel, isMyGo } from "mygo-runtime";
import type {
  CloseResult,
  Disposable,
  DocumentInfo,
  GeneratePhase,
  GenerateProgress,
  GenerateRequest,
  GenerationSettings,
  HostAdapter,
  HostDocSnapshot,
  NativeCommand,
  NativeCommandName,
  NavSearchResult,
  NavSnapshot,
  RecentWorkspace,
  SettingsPatch,
  ViewSnapshot,
  WorkspaceAdapter,
  WorkspaceSnapshot,
} from "./types";
import {
  WorkspaceHost,
  events,
  type DocumentInfo as HostDocumentInfo,
  type GenerateProgress as WireProgress,
  type NativeCommand as WireNativeCommand,
  type NavEntry as WireNavEntry,
  type NavSnapshot as WireNavSnapshot,
  type WorkspaceSnapshot as WireWorkspaceSnapshot,
} from "./generated/mygo";
import { cancelled, failureOf, GenerationError, isFailureKind } from "./generationError";
import { applyNativeEdit } from "../editor/nativeEdit";
import { validateGeneratedView, type GeneratedViewVerdict } from "../plugins/genui/a2ui";

function asNativeCommand(cmd: WireNativeCommand): NativeCommand {
  return { id: cmd.id, command: cmd.command as NativeCommandName, attempt: cmd.attempt };
}

const maxQuitFailureLength = 512;

function quitFlushFailure(id: string, error: unknown): string {
  const detail = error instanceof Error ? error.message : typeof error === "string" ? error : "Unknown error";
  const message = `Could not flush document ${id} before quitting: ${detail || "Unknown error"}`;
  return message.length > maxQuitFailureLength
    ? `${message.slice(0, maxQuitFailureLength - 1)}…`
    : message;
}

function generatingOf(value: unknown): boolean {
  if (!value || typeof value !== 'object' || !('generating' in value)) return false
  return value.generating === true
}

const PHASES: Record<GeneratePhase, true> = {
  starting: true,
  thinking: true,
  reading: true,
  writing: true,
  checking: true,
};

function isPhase(phase: string): phase is GeneratePhase {
  return Object.hasOwn(PHASES, phase);
}

function disposable(off: () => void): Disposable {
  let disposed = false;
  return {
    dispose() {
      if (disposed) return;
      disposed = true;
      off();
    },
  };
}

function documentInfo(state: HostDocumentInfo): DocumentInfo {
  return { ...state, generating: generatingOf(state) }
}

function workspaceSnapshot(snap: WireWorkspaceSnapshot): WorkspaceSnapshot {
  return {
    ...snap,
    kind: snap.kind === 'untitled' ? 'untitled' : 'folder',
    workspaces: snap.workspaces.map((workspace) => ({
      ...workspace,
      kind: workspace.kind === 'untitled' ? 'untitled' : 'folder',
    })),
    documents: snap.documents.map(documentInfo),
    restoreError: snap.restoreError,
  }
}

function navTree(snap: WireNavSnapshot): NavSnapshot {
  return {
    ...snap,
    entries: snap.entries.map((entry: WireNavEntry) => ({
      ...entry,
      kind: entry.kind === 'folder' ? 'folder' : 'file',
    })),
  }
}

function bindDocument(id: string): HostAdapter {
  const viewListeners = new Set<{ fn: (snap: ViewSnapshot) => void }>();
  let offViewChanged: (() => void) | null = null;
  let latest: ViewSnapshot | null = null;
  let cycle = 0;
  let calls: Promise<unknown> = Promise.resolve();
  const inTurn = <T>(call: () => Promise<T>): Promise<T> => {
    const result = calls.then(call);
    calls = result.catch(() => undefined);
    return result;
  };

  const deliver = (snap: ViewSnapshot) => {
    latest = snap;
    for (const listener of viewListeners) listener.fn(snap);
  };

  return {
    id,
    load(): Promise<HostDocSnapshot> {
      return WorkspaceHost.load(id).then((snap) => ({
        ...snap,
        state: documentInfo(snap.state),
      }));
    },
    async update(text: string): Promise<void> {
      await WorkspaceHost.update(id, text);
    },
    async flush(text: string): Promise<void> {
      await WorkspaceHost.flush(id, text);
    },
    async save(): Promise<void> {
      await WorkspaceHost.save(id);
    },
    async saveAs(): Promise<void> {
      await WorkspaceHost.saveAs(id);
    },
    async rename(): Promise<void> {
      await WorkspaceHost.rename(id);
    },
    async moveTo(): Promise<void> {
      await WorkspaceHost.moveTo(id);
    },
    async revert(): Promise<void> {
      await WorkspaceHost.revert(id);
    },
    async setViewMode(mode: string): Promise<void> {
      await WorkspaceHost.setViewMode(id, mode);
    },
    bindFlush(getText: () => string): Disposable {
      flushers.set(id, getText);
      return disposable(() => {
        if (flushers.get(id) === getText) flushers.delete(id);
      });
    },
    onReload(fn: (snap: HostDocSnapshot) => void): Disposable {
      return disposable(
        events.docReloaded.on((snap) => {
          if (snap.id === id) fn({ ...snap, state: documentInfo(snap.state) });
        }),
      );
    },
    onState(fn: (state: DocumentInfo) => void): Disposable {
      return disposable(
        events.docState.on((state) => {
          if (state.id === id) fn(documentInfo(state));
        }),
      );
    },
    watchView(fn: (snap: ViewSnapshot) => void): Disposable {
      const listener = { fn };
      viewListeners.add(listener);
      if (offViewChanged) {
        const snap = latest;
        if (snap) queueMicrotask(() => viewListeners.has(listener) && fn(snap));
      } else {
        const current = ++cycle;
        latest = null;
        offViewChanged = events.viewChanged.on((event) => {
          if (event.id !== id) return;
          deliver({
            exists: event.exists,
            text: event.text,
            error: event.error,
            canUndo: event.canUndo,
          });
        });
        inTurn(() => WorkspaceHost.watchView(id)).catch((err: unknown) => {
          if (current === cycle) deliver({ exists: true, text: "", error: String(err), canUndo: false });
        });
      }
      return disposable(() => {
        viewListeners.delete(listener);
        if (viewListeners.size > 0 || !offViewChanged) return;
        offViewChanged();
        offViewChanged = null;
        latest = null;
        cycle++;
        void inTurn(() => WorkspaceHost.unwatchView(id));
      });
    },
    async undoView(): Promise<void> {
      await WorkspaceHost.undoView(id);
    },
    async generate(req: GenerateRequest, onProgress: (progress: GenerateProgress) => void, signal: AbortSignal) {
      let failed: GenerationError | null = null;
      const progress = new Channel<WireProgress>((step) => {
        if (step.phase === "failed" && step.failure) {
          const { kind, ...rest } = step.failure;
          failed = new GenerationError({ ...rest, kind: isFailureKind(kind) ? kind : "agent-error" });
        } else if (isPhase(step.phase)) {
          onProgress({ phase: step.phase, text: step.text });
        }
      });
      const cancel = () => progress.close();
      if (signal.aborted) throw cancelled();
      signal.addEventListener("abort", cancel, { once: true });
      try {
        await WorkspaceHost.generate(id, req, progress);
      } catch (error) {
        if (signal.aborted) throw cancelled();
        throw failed ?? new GenerationError(failureOf(error));
      } finally {
        signal.removeEventListener("abort", cancel);
      }
    },
    getSettings(): Promise<GenerationSettings> {
      return WorkspaceHost.getSettings();
    },
    setSettings(patch: SettingsPatch): Promise<GenerationSettings> {
      return WorkspaceHost.setSettings(patch);
    },
    onSettings(fn: (settings: GenerationSettings) => void): Disposable {
      // One settings file for the whole app: every window's write arrives here.
      return disposable(events.settingsChanged.on((settings) => fn(settings)));
    },
  };
}


/** The mounted pane's current Markdown, by document id. */
const flushers = new Map<string, () => string>();

/** Tell the native Edit menu whether this document currently can undo/redo. */
export function reportNativeEditState(id: string, canUndo: boolean, canRedo: boolean): void {
  if (!isMyGo()) return
  void WorkspaceHost.setNativeEditState(id, canUndo, canRedo);
}

declare global {
  interface Window {
    /** The view validator the Go host calls before it writes a generated view. */
    __mgoValidateGeneratedView?: (raw: string) => Promise<GeneratedViewVerdict>
  }
}

/**
 * Installs the page-side validator the Go host asks before it writes a view
 * file. It is pure: it reads only its argument, uses the app's own A2UI
 * catalog and message processor, and needs no mounted document or component.
 * An absent bridge, a throw or a rejection all fail closed in the host.
 */
export function installGeneratedViewValidator(): void {
  window.__mgoValidateGeneratedView = async (raw: string) => validateGeneratedView(raw)
}

installGeneratedViewValidator()

/**
 * The one write-back of a pane's text: read the Markdown it really holds,
 * then deliver it once. A document with no mounted pane registered no
 * reader and delivers nothing, never an invented empty buffer.
 */
async function flushDocument(id: string): Promise<void> {
  const read = flushers.get(id);
  if (!read) return;
  await WorkspaceHost.flush(id, read());
}
/**
 * Workspace adapter backed by MyGo. document(id) is stable for an open
 * session in this window; a real close or migrate drops the cache.
 */
export function createMyGoHost(): WorkspaceAdapter {
  const adapters = new Map<string, HostAdapter>();
  let latestQuitAttempt = 0;

  const workspace: WorkspaceAdapter = {
    snapshot(): Promise<WorkspaceSnapshot> {
      return WorkspaceHost.snapshot().then(workspaceSnapshot);
    },
    subscribe(fn: (snap: WorkspaceSnapshot) => void): Disposable {
      return disposable(events.workspaceChanged.on((snap) => fn(workspaceSnapshot(snap))));
    },
    open(path: string): Promise<DocumentInfo> {
      return WorkspaceHost.open(path).then(documentInfo);
    },
    async openWorkspace(root: string): Promise<void> {
      await WorkspaceHost.openWorkspace(root);
    },
    async activateWorkspace(id: string): Promise<void> {
      await WorkspaceHost.activateWorkspace(id);
    },
    async closeWorkspaceRoot(id: string): Promise<CloseResult> {
      for (const documentId of [...flushers.keys()]) await flushDocument(documentId);
      return WorkspaceHost.closeWorkspaceRoot(id);
    },
    newDocument(): Promise<DocumentInfo> {
      return WorkspaceHost.new().then(documentInfo);
    },
    duplicate(id: string): Promise<DocumentInfo> {
      return WorkspaceHost.duplicate(id).then(documentInfo);
    },
    async activate(id: string): Promise<void> {
      await WorkspaceHost.activate(id);
    },
    async closeDocument(id: string, text?: string): Promise<CloseResult> {
      if (text !== undefined) await WorkspaceHost.flush(id, text);
      else await flushDocument(id);
      const result = await WorkspaceHost.closeDocument(id);
      if (result.closed) adapters.delete(id);
      return result;
    },
    async closeWorkspace(): Promise<CloseResult> {
      for (const id of [...flushers.keys()]) await flushDocument(id);
      const result = await WorkspaceHost.closeWorkspace();
      if (result.closed) adapters.clear();
      return result;
    },
    document(id: string): HostAdapter {
      const existing = adapters.get(id);
      if (existing) return existing;
      const created = bindDocument(id);
      adapters.set(id, created);
      return created;
    },
    async navSnapshot(): Promise<NavSnapshot> {
      return navTree(await WorkspaceHost.navSnapshot());
    },
    subscribeNav(fn: (snap: NavSnapshot) => void): Disposable {
      return disposable(events.navChanged.on((snap) => fn(navTree(snap))));
    },
    setFilter(query: string): Promise<NavSearchResult> {
      return WorkspaceHost.setFilter(query);
    },
    subscribeSearch(fn: (result: NavSearchResult) => void): Disposable {
      return disposable(events.navSearchReady.on(fn));
    },
    async toggleFolder(rel: string): Promise<void> {
      await WorkspaceHost.toggleFolder(rel);
    },
    async revealPath(path: string): Promise<void> {
      await WorkspaceHost.revealPath(path);
    },
    pickFolder(): Promise<string> {
      return WorkspaceHost.pickFolder();
    },
    recentWorkspaces(): Promise<RecentWorkspace[]> {
      return WorkspaceHost.recentWorkspaces();
    },
  };

  events.nativeCommand.on((cmd: WireNativeCommand) => {
    const native = asNativeCommand(cmd);
    if (native.command === "flushQuit") {
      const attempt = native.attempt;
      if (!Number.isSafeInteger(attempt) || attempt === undefined || attempt <= latestQuitAttempt) return;
      latestQuitAttempt = attempt;
      void handleNative(workspace, adapters, native, () => latestQuitAttempt === attempt).catch(() => undefined);
      return;
    }
    // A failure is recorded on the captured document's state by the host,
    // which its window shows; it must not become an unhandled rejection.
    void handleNative(workspace, adapters, native).catch(() => undefined);
  });
  events.workspaceChanged.on((snap) => {
    const live = new Set(snap.documents.map((d) => d.id));
    for (const id of [...adapters.keys()]) {
      if (!live.has(id)) adapters.delete(id);
    }
  });

  return workspace;
}

async function handleNative(
  workspace: WorkspaceAdapter,
  adapters: Map<string, HostAdapter>,
  cmd: NativeCommand,
  isCurrentQuitAttempt: () => boolean = () => true,
): Promise<void> {
  const command = cmd.command as NativeCommandName;
  if (command === "closeWorkspace") {
    // closeWorkspace delivers every pane's last input itself, once.
    await workspace.closeWorkspace();
    return;
  }
  if (command === "flushQuit") {
    const attempt = cmd.attempt;
    if (attempt === undefined || !Number.isSafeInteger(attempt) || attempt <= 0) return;
    let failure = "";
    for (const [id, read] of [...flushers]) {
      if (!isCurrentQuitAttempt()) return;
      try {
        await WorkspaceHost.flushQuit(id, read(), attempt);
      } catch (error) {
        if (!failure) failure = quitFlushFailure(id, error);
      }
    }
    if (isCurrentQuitAttempt()) await WorkspaceHost.prepareQuit(attempt, failure);
    return;
  }
  if (command === "undo" || command === "redo") {
    applyNativeEdit(cmd.id, command);
    return;
  }
  if (command === "close") {
    // closeDocument delivers that pane's last input itself, once.
    const result = await workspace.closeDocument(cmd.id);
    if (result.closed) adapters.delete(cmd.id);
    return;
  }
  // Save, Save As, Rename, Move To, Revert and Duplicate deliver the
  // captured document's last input first, once, then run on the host.
  await flushDocument(cmd.id);
  switch (command) {
    case "save":
      await WorkspaceHost.save(cmd.id);
      break;
    case "saveAs":
      await WorkspaceHost.saveAs(cmd.id);
      break;
    case "rename":
      await WorkspaceHost.rename(cmd.id);
      break;
    case "moveTo":
      await WorkspaceHost.moveTo(cmd.id);
      break;
    case "revert":
      await WorkspaceHost.revert(cmd.id);
      break;
    case "duplicate":
      await WorkspaceHost.duplicate(cmd.id);
      break;
    default:
      break;
  }
}
