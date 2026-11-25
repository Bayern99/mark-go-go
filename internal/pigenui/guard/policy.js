// Guard policy for Mark Go Go's Pi backend. Pi has no sandbox and does not ask
// before a tool call, so the host loads this extension and Pi tells it about
// every call before the call runs. These are the decisions only: keeping them
// free of Pi's API is what lets this file's tests run under plain Node.
import fs from "node:fs";
import path from "node:path";

export const GUARD_VERSION = "1";

// canonical resolves the part of a path that exists and keeps the rest, so a
// symbolic link cannot move a read or a write away from the paths the host
// allowed. A file that is about to be created has no link of its own yet.
export function canonical(target) {
  let resolved = path.resolve(target);
  const rest = [];
  for (;;) {
    try {
      return path.join(fs.realpathSync.native(resolved), ...rest);
    } catch {
      const parent = path.dirname(resolved);
      if (parent === resolved) {
        return path.join(resolved, ...rest);
      }
      rest.unshift(path.basename(resolved));
      resolved = parent;
    }
  }
}

// isInside answers whether target sits in root or deeper, never at a sibling
// that merely shares root's prefix.
export function isInside(root, target) {
  const relative = path.relative(root, target);
  return relative === "" || (!relative.startsWith("..") && !path.isAbsolute(relative));
}

// blockedReason returns why a tool call must not run, or "" when it may. reads
// stay in the folder the turn was given; writes stay on that turn's out.json.
export function blockedReason(toolName, input, allowed) {
  if (!allowed || !allowed.root) {
    return "This run has no allowed folder, so no tool may run.";
  }
  if (toolName !== "read" && toolName !== "write" && toolName !== "edit") {
    return "A reading view may only read its folder and write out.json.";
  }
  const target = input && typeof input.path === "string" ? input.path : "";
  if (target === "") {
    return "This tool call names no file, so it was blocked.";
  }
  const resolved = canonical(target);
  if (toolName === "read") {
    if (isInside(canonical(allowed.root), resolved)) {
      return "";
    }
    return "A reading view may only read files inside its own folder.";
  }
  if (toolName === "write" || toolName === "edit") {
    if (allowed.out && resolved === canonical(allowed.out) && isInside(canonical(allowed.root), resolved)) {
      return "";
    }
    return "A reading view may only write out.json.";
  }
  return "A reading view may only read its folder and write out.json.";
}
