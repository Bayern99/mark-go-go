# ADR 0018 · GenUI 多后端、编排层与 Runner 接口

Status: accepted

## Context

GenUI 只有一条路：`agy acp`。生成流程又把 ACP 细节（进程缓存、会话、`configOptions`、权限回调）和每个后端都需要的编排（每轮隔离目录、`source.md`/`view.json`、提示词、25 ms 预览、候选校验与提交）写在同一处。要接入 Codex（`codex-acp`，无参数启动），并让用户配置 Custom ACP，需要先把两层分开，再把命令、参数与模型选择变成每个后端自己的配置。实测 `codex-acp 1.13.1` + `codex-cli 0.172.0`：`session/set_config_option(model, <已声明 id>)` 会随之声明 `reasoning_effort`；写成未声明的复合 id 返回 `-32602`；会话初始模型可能不在服务端模型列表里，且没有思考等级。

## Decision

- 生成拆成**协议无关的 Orchestrator** 与 **Runner**。Orchestrator 负责隔离目录、`source.md`、`view.json`、`GenerationPrompt`、每 25 ms 读取 `out.json` 的预览、`Merge`、候选校验、`ReplaceView`；失败、无效结果或取消都保留原视图。Runner 提供 `Probe`、`ListModels`、`ExecuteTurn`、`Close`，只管自己的协议、进程与会话。
- ACP 把**命令与参数分开**：Codex 用无参数的 `codex-acp`，Antigravity 用 `agy acp`，Custom ACP 用用户提供的命令与参数。`--version` 与 `login` 是独立入口，`CODEX_PATH` 指定底层 `codex`。进程缓存只在确认支持并成功 `session/close` 后保留。
- 模型发现与写入**以 `session/new` 的 `configOptions` 为准**，写入用 `session/set_config_option`；只有缺少 `model` 选项时才读 legacy `models.availableModels`，并在后端支持时用 `session/set_model` 兜底。主路径报错不转成 legacy 写入。选项以每次响应与 `config_option_update` 为准；类别与 ID 兼容 `thought_level`、`thoughtLevel`、`reasoning_effort`。复合 `modelId`（`gpt-6-sol[high]`）只在相符的 legacy 兜底里整段使用，标准路径拆成模型与等级两个已声明值。当前模型没有等级选项时隐藏选择，并跳过写入而不是让会话失败。
- `Probe` 独立确认可执行文件、版本与 ACP 握手，区分已登录、需登录与状态未知；`authMethods` 只说明可用认证方式，不算已登录。只有证据表明 CLI/app-server 或能力不兼容时才是 `agent-outdated`；普通 `-32602` 不算。`ListModels` 用短生命周期会话刷新模型，不提交生成提示或文档内容。
- Finder 启动没有 shell 初始化，因此除了 PATH 还查 `~/.local/bin`、fnm 的 `aliases/default/bin`、`~/.nvm/versions/node/*/bin`、`~/.volta/bin`、`~/.bun/bin`、`/opt/homebrew/bin`、`/usr/local/bin`；不使用瞬态的 `fnm_multishells`。
- `settings.json` 升到 `schemaVersion: 2`，`genui.backend` 选当前后端，`genui.backends[id]` 各自保存命令、参数、模型、模型列表与思考等级。v1 的顶层 `model`、`agentCommand`、`models` 迁到 Antigravity（v1 里就是 `agy`）。新用户按 `codex-acp` → `pi` → `agy` 探测首个已安装且已有 Runner 的后端；Pi 的卡片在 #194 之前只占位并标注即将推出。
- `Settings / GenUI` 是唯一的后端编辑器：2×2 卡片、状态点、`Test & Refresh`、`Sign in in Terminal`（通用系统 Terminal，Codex 用 `codex-acp login`，Antigravity 用 `agy acp --login`）、动态 `Model`、按能力出现的 `Reasoning effort`，以及默认只为 Custom ACP 展开的 `Advanced`（命令与参数）。GenUI 面板只保留后端与模型的只读摘要、`Configure…`、生成要求与生成动作。
- 权限回调不变：允许 `think`、`read`、`search` 与仅限 `out.json` 的 `edit`，拒绝 `execute` 和越界写入；自动批准可能不经过宿主回调，因此它不等于 OS 沙箱。

## Consequences

- 旧 `settings.json` 升级后继续按 `agy` 工作；写入只发生在用户保存设置时。
- 持久化的模型若不在后端当前选项里，界面提示并沿用服务端当前值，不写入未声明的值。
- 真实 Codex 与 Antigravity 的模型刷新、登录与流式生成仍需在有这两个 CLI 的机器上实测；本 ADR 的自动化覆盖使用 Fake ACP。
- Pi 后端（#194）只需实现 `Runner` 并翻新卡片状态，编排层不再改动。
