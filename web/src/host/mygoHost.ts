import type { Disposable, DocState, HostAdapter, HostDocSnapshot, ViewSnapshot } from "./types";
import { DocumentHost, events } from "./generated/mygo";

/**
 * Wraps an unsubscribe function in the host adapter's Disposable.
 * Disposing twice is harmless: the listener is only removed once.
 */
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

/**
 * The document host backed by MyGo: the window's document, its edits and
 * its events come from the Go side, which owns the file, autosave,
 * recovery copies and the conflict with changes made by other apps.
 */
export function createMyGoHost(): HostAdapter {
  // The Go side keeps one watch per window: the first follower starts it,
  // the last one to leave stops it. Every snapshot, the first included,
  // comes as viewChanged in the order the host read them; latest is the
  // last one, for followers that join a running watch. cycle tells a
  // late failure of an old watch from the current one.
  const viewListeners = new Set<{ fn: (snap: ViewSnapshot) => void }>();
  let offViewChanged: (() => void) | null = null;
  let latest: ViewSnapshot | null = null;
  let cycle = 0;
  // The host runs each call on a goroutine of its own, so a watch and an
  // unwatch sent together could run in either order: they go one at a
  // time.
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
    load(): Promise<HostDocSnapshot> {
      return DocumentHost.load();
    },

    async update(text: string): Promise<void> {
      await DocumentHost.update(text);
    },

    onReload(fn: (snap: HostDocSnapshot) => void): Disposable {
      return disposable(events.docReloaded.on((snap) => fn(snap)));
    },

    onState(fn: (state: DocState) => void): Disposable {
      return disposable(events.docState.on((state) => fn(state)));
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
        offViewChanged = events.viewChanged.on(deliver);
        inTurn(() => DocumentHost.watchView()).catch((err: unknown) => {
          if (current === cycle) deliver({ exists: true, text: "", error: String(err) });
        });
      }
      return disposable(() => {
        viewListeners.delete(listener);
        if (viewListeners.size > 0 || !offViewChanged) return;
        offViewChanged();
        offViewChanged = null;
        latest = null;
        cycle++;
        void inTurn(() => DocumentHost.unwatchView());
      });
    },
  };
}
