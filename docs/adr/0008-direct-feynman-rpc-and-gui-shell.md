# ADR 0008 · Direct Feynman RPC 与全量原生 GUI 壳

Status: accepted (supersedes ADR 0007's pi-acp / Pi RPC relay / sandbox-exec decisions)

Mark Go Go 直接通过 Go 子进程对接 Feynman 官方内置且有上游 CI 保障的 `feynman --mode rpc`，并用非阻塞子进程封装 Feynman 的全部文档化 CLI 管理命令（`status`/`doctor`、`model`、`alpha`、`mcp`、`search`、`packages`、`update`）为侧边栏原生英文 Portrait 界面，不再引入 `pi-acp`、外部 `node`、MITM 中继脚本、侧带 Unix Socket 或内置 `@xterm/xterm` + `creack/pty` 终端页签。

**Considered Options**
- 保留 `pi-acp` + `scripts/feynman-rpc.mjs` + 侧带 `ui.sock`：多一层第三方上游依赖，丢失 `steer`/`follow_up`/`fork`/`get_tree`/`get_session_stats` 与动态 `sourceInfo`，且依赖 `pi-acp` 私有环境变量与 `session-map.json`。
- 保留内置黑底终端页签承载管理命令：破坏 Portrait 视觉统一、引入 329 KB 前端终端包与 PTY 维护成本，且与对话会话互斥。
- 保留 macOS 专用 `/usr/bin/sandbox-exec` 硬编码写白名单：阻断跨平台支持，且 Feynman 上游新增缓存/配置目录时立即触发 `EPERM`。

**Consequences**
- 命令、工作流、Skills 与扩展列表 100% 由 `get_commands` 动态驱动，Feynman 上游发版无痛跟随；配置写操作走 Feynman 自身 CLI 子命令，避免直接耦合内部文件格式。
- 仅 3 个依赖 `@clack/prompts` 交互式 TTY 的登录向导（`feynman model login`、`feynman alpha login`、`feynman setup`）在 Portrait 面板内一键唤起系统外部 Terminal，并在窗口重获焦点时自动刷新状态，保持零 `xterm`/零 `creack/pty`/零 `auth.json` 格式耦合；其余全部命令与 API-Key 表单均在侧边栏原生完成。
- Feynman 进程继续以 Mark Go Go 为工作区准备的专属目录 `<UserData>/research/<hash>/work` 作为 `--cwd`（承载 `.pi/mcp.json` 与 Feynman 脚手架目录），用户原仓库保持零污染；会话 `.jsonl` 存放在 Feynman 默认目录 `~/.feynman/sessions` 并按 `cwd` 过滤，兼容 `session-search` 技能。
- 移除 `sandbox-exec` 后，文档写安全由产品级的 Change proposal 审阅流、单文档检查点与外部修改冲突检测保障，同步解锁跨平台运行。
- 会话历史列表与删除通过读取标准 Pi `.jsonl` 会话文件头实现，恢复/切换/分支使用原生 RPC 命令。
