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
  ViewSnapshot,
  WorkspaceAdapter,
  WorkspaceSnapshot,
} from "./types";
import { WorkspaceHost, events, type GenerateProgress as WireProgress } from "./generated/mygo";
import { cancelled, failureOf, GenerationError, isFailureKind } from "./generationError";
import { applyNativeEdit } from "../editor/nativeEdit";

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
      return WorkspaceHost.load(id);
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
    onReload(fn: (snap: HostDocSnapshot) => void): Disposable {
      return disposable(
        events.docReloaded.on((snap) => {
          if (snap.id === id) fn(snap);
        }),
      );
    },
    onState(fn: (state: DocumentInfo) => void): Disposable {
      return disposable(
        events.docState.on((state) => {
          if (state.id === id) fn(state);
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
    async setSettings(settings: GenerationSettings): Promise<void> {
      await WorkspaceHost.setSettings(settings);
    },
  };
}


const flushers = new Map<string, () => Promise<string | undefined>>();

/** DocumentPane registers how to read its current Markdown for native Save/Close. */
export function registerDocumentFlush(id: string, flush: () => Promise<string | undefined>): () => void {
  flushers.set(id, flush);
  return () => {
    if (flushers.get(id) === flush) flushers.delete(id);
  };
}

/** Tell the native Edit menu whether this document currently can undo/redo. */
export function reportNativeEditState(id: string, canUndo: boolean, canRedo: boolean): void {
  if (!isMyGo()) return
  void WorkspaceHost.setNativeEditState(id, canUndo, canRedo);
}

async function flushDocument(id: string): Promise<void> {
  const flush = flushers.get(id);
  const text = flush ? await flush() : undefined;
  if (text !== undefined) await WorkspaceHost.flush(id, text);
}
/**
 * Workspace adapter backed by MyGo. document(id) is stable for an open
 * session in this window; a real close or migrate drops the cache.
 */
export function createMyGoHost(): WorkspaceAdapter {
  const adapters = new Map<string, HostAdapter>();

  const workspace: WorkspaceAdapter = {
    snapshot(): Promise<WorkspaceSnapshot> {
      return WorkspaceHost.snapshot();
    },
    subscribe(fn: (snap: WorkspaceSnapshot) => void): Disposable {
      return disposable(events.workspaceChanged.on(fn));
    },
    open(path: string): Promise<DocumentInfo> {
      return WorkspaceHost.open(path);
    },
    async openWorkspace(root: string): Promise<void> {
      await WorkspaceHost.openWorkspace(root);
    },
    newDocument(): Promise<DocumentInfo> {
      return WorkspaceHost.new();
    },
    duplicate(id: string): Promise<DocumentInfo> {
      return WorkspaceHost.duplicate(id);
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
  };

  events.nativeCommand.on((cmd: NativeCommand) => {
    void handleNative(workspace, adapters, cmd);
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
): Promise<void> {
  const command = cmd.command as NativeCommandName;
  if (command === "closeWorkspace") {
    for (const id of [...flushers.keys()]) await flushDocument(id);
    await workspace.closeWorkspace();
    return;
  }
  if (command === "undo" || command === "redo") {
    applyNativeEdit(cmd.id, command);
    return;
  }
  await flushDocument(cmd.id);
  const host = adapters.get(cmd.id) ?? workspace.document(cmd.id);
  switch (command) {
    case "save":
      await WorkspaceHost.save(cmd.id);
      break;
    case "saveAs":
      await WorkspaceHost.saveAs(cmd.id);
      break;
    case "close": {
      const result = await WorkspaceHost.closeDocument(cmd.id);
      if (result.closed) adapters.delete(cmd.id);
      break;
    }
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
      void host;
  }
}
