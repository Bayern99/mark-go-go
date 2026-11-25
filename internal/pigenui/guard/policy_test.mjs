// Behaviour of the guard policy against real files. Run by
// TestGuardPolicyBlocksPathsOutsideTheTurn (Go) when Node is installed:
//   node guard/policy_test.mjs
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { blockedReason, canonical } from "./policy.js";

const base = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), "mgo-guard-test-")));
const root = path.join(base, "turn");
const sibling = path.join(base, "turn-elsewhere");
const outside = path.join(base, "outside");
for (const dir of [root, sibling, outside]) {
  fs.mkdirSync(dir, { recursive: true });
}
fs.mkdirSync(path.join(root, "notes"), { recursive: true });
fs.writeFileSync(path.join(root, "source.md"), "# Source\n");
fs.writeFileSync(path.join(root, "notes", "detail.md"), "detail\n");
fs.writeFileSync(path.join(root, "out.json"), "{}\n");
fs.writeFileSync(path.join(sibling, "secret.md"), "secret\n");
fs.writeFileSync(path.join(outside, "secret.md"), "secret\n");
fs.symlinkSync(path.join(outside, "secret.md"), path.join(root, "escape.md"));
fs.symlinkSync(outside, path.join(root, "escape-dir"));

// Pi runs with the turn's folder as its working directory, which is what makes
// a relative path resolve inside the folder.
process.chdir(root);
// Pi expands ~ to the home folder; point it outside the turn.
process.env.HOME = outside;

const allowed = { root, out: path.join(root, "out.json") };
const resolvedRoot = canonical(root);

const cases = [
  ["a read of the turn's own source is allowed", "read", { path: "source.md" }, ""],
  ["a read below the turn's folder is allowed", "read", { path: path.join(root, "notes", "detail.md") }, ""],
  ["a read that climbs and comes back stays allowed", "read", { path: path.join(root, "notes", "..", "source.md") }, ""],
  ["a read outside the folder is blocked", "read", { path: path.join(outside, "secret.md") }, "read"],
  ["a read that climbs out is blocked", "read", { path: path.join(root, "..", "outside", "secret.md") }, "read"],
  ["a read of a sibling with the same prefix is blocked", "read", { path: path.join(sibling, "secret.md") }, "read"],
  ["a read through a symbolic link is blocked", "read", { path: path.join(root, "escape.md") }, "read"],
  ["a read through a linked folder is blocked", "read", { path: path.join(root, "escape-dir", "secret.md") }, "read"],
  ["a read named by the root itself is allowed", "read", { path: resolvedRoot }, ""],
  ["a write to out.json is allowed", "write", { path: "out.json" }, ""],
  ["a write to out.json by its absolute path is allowed", "write", { path: path.join(root, "out.json") }, ""],
  ["an edit of out.json is allowed", "edit", { path: "out.json", oldText: "{}", newText: "[]" }, ""],
  ["a write to another file in the folder is blocked", "write", { path: path.join(root, "extra.md") }, "write"],
  ["a write to another out.json below the folder is blocked", "write", { path: path.join(root, "notes", "out.json") }, "write"],
  ["a write that climbs out is blocked", "write", { path: path.join(outside, "out.json") }, "write"],
  ["a read of the home folder through ~ is blocked", "read", { path: "~/secret.md" }, "read"],
  ["a read of the home folder itself is blocked", "read", { path: "~" }, "read"],
  ["a read whose @ prefix Pi strips is blocked outside", "read", { path: "@" + path.join(outside, "secret.md") }, "read"],
  ["a read whose @ prefix Pi strips stays allowed inside", "read", { path: "@source.md" }, ""],
  ["a read by file URL outside the folder is blocked", "read", { path: "file://" + path.join(outside, "secret.md") }, "read"],
  ["a write of out.json through Pi's @ prefix is allowed", "write", { path: "@out.json" }, ""],
  ["a write to the home folder through ~ is blocked", "write", { path: "~/out.json" }, "write"],
  ["a call without a path is blocked", "read", {}, "no file"],
  ["a shell call is blocked", "bash", { command: "ls" }, "may only read"],
];

let failed = 0;
for (const [name, tool, input, want] of cases) {
  const reason = blockedReason(tool, input, allowed);
  const blocked = reason !== "";
  const ok = want === "" ? !blocked : blocked && reason.includes(want);
  if (!ok) {
    failed++;
    console.log(`not ok: ${name} (wanted ${want === "" ? "allowed" : "blocked"}, got ${JSON.stringify(reason)})`);
  }
}

const outAsLink = path.join(base, "linked-turn");
fs.mkdirSync(outAsLink, { recursive: true });
fs.symlinkSync(path.join(outside, "secret.md"), path.join(outAsLink, "out.json"));
const linkedReason = blockedReason("write", { path: "out.json" }, { root: outAsLink, out: path.join(outAsLink, "out.json") });
if (!linkedReason.includes("write")) {
  failed++;
  console.log(`not ok: a write through a linked out.json is blocked (got ${JSON.stringify(linkedReason)})`);
}

if (blockedReason("read", { path: "source.md" }, { root: "", out: "" }) === "") {
  failed++;
  console.log("not ok: a run without allowed paths blocks every call");
}

fs.rmSync(base, { recursive: true, force: true });
if (failed > 0) {
  console.log(`${failed} guard case(s) failed`);
  process.exit(1);
}
console.log("guard policy: every case passed");
