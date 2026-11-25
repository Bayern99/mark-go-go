// Guard policy for Mark Go Go's Pi backend. Pi has no sandbox and does not ask
// before a tool call, so the host loads this extension and Pi tells it about
// every call before the call runs. These are the decisions only: keeping them
// free of Pi's API is what lets this file's tests run under plain Node.
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

export const GUARD_VERSION = "1";

const UNICODE_SPACES = /[\u00A0\u2000-\u200A\u202F\u205F\u3000]/g;

// asPiReadsIt rewrites a tool's path the way Pi's file tools do before they
// touch the disk (utils/paths.js normalizePath): unicode spaces, a leading @,
// ~ for the home folder and file:// URLs. Judging the raw text instead would
// let "~/x" or "@/x" pass as a file inside the turn's folder.
export function asPiReadsIt(target) {
  let normalized = target.replace(UNICODE_SPACES, " ");
  if (normalized.startsWith("@")) {
    normalized = normalized.slice(1);
  }
  if (normalized === "~") {
    return os.homedir();
  }
  if (normalized.startsWith("~/") || (process.platform === "win32" && normalized.startsWith("~\\"))) {
    return path.join(os.homedir(), normalized.slice(2));
  }
  if (/^file:\/\//.test(normalized)) {
    return fileURLToPath(normalized);
  }
  return normalized;
}

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
  let resolved;
  try {
    resolved = canonical(asPiReadsIt(target));
  } catch {
    return "This tool call names a path the guard cannot read, so it was blocked.";
  }
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
