import type { Disposable, DocState, HostAdapter, HostDocSnapshot } from "./types";
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
  };
}
