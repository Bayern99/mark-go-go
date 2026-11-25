// The guard extension Mark Go Go loads into every Pi generation. The host
// passes the turn's allowed paths through the environment, which never changes
// for the life of the process, and this extension reports that it is guarding
// before the host sends a prompt. If it is not guarding, the host must not
// generate anything: Pi has no sandbox of its own.
import { GUARD_VERSION, blockedReason } from "./policy.js";

const STATUS_KEY = "mgo-guard";

export default function markGoGoGuard(pi) {
  const root = process.env.MGO_GUARD_ROOT || "";
  const out = process.env.MGO_GUARD_OUT || "";
  const token = process.env.MGO_GUARD_TOKEN || "";
  const wanted = process.env.MGO_GUARD_VERSION || "";
  const allowed = { root, out };

  const state = () => {
    if (!root || !out || !token) {
      return "unconfigured";
    }
    if (wanted !== GUARD_VERSION) {
      return "mismatch";
    }
    return "ready";
  };

  pi.on("session_start", async (_event, ctx) => {
    if (ctx && ctx.ui && typeof ctx.ui.setStatus === "function") {
      ctx.ui.setStatus(STATUS_KEY, `${state()}:${GUARD_VERSION}:${token}`);
    }
  });

  pi.on("tool_call", async (event) => {
    if (state() !== "ready") {
      return { block: true, reason: "This run has no guarded file access, so no tool may run." };
    }
    const reason = blockedReason(event && event.toolName, event && event.input, allowed);
    return reason === "" ? undefined : { block: true, reason };
  });
}
