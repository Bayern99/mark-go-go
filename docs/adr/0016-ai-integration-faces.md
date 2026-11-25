# ADR 0016 · AI 集成的三个面：GenUI 插件、可换 Runner 与外部 Agent 通道

Status: accepted

## Context

在父票 #191 之前，Mark Go Go 只有一条接入 AI 的路：GenUI 视图强耦合 `agy acp`，命令、参数与模型选择写死在代码里，错误提示带厂商口吻；同时面向 Claude Code、Codex、Pi 这类外部 Harness 的能力完全没有入口。用户看到的"AI"因此混在一起：一个用来生成阅读视图的后端，和一个能读写工作区的终端 Agent，被当成同一件事；而哪个开关属于哪个面，没有界面上说得清的地方。

本 ADR 记录把这些面分开的决定，以及由此产生的界面对外承诺。协议与实现细节分别记在 ADR 0017（外部通道与 CLI）和 ADR 0018（GenUI 多后端与编排层），本文只写"为什么这样分"，不复述它们。

## Decision

- **三个面分开。** GenUI 是一个可开关的插件（`mgo.plugin.genui`，默认开，关闭即取消失控中的生成、不再监听视图文件，已保存的视图留在磁盘上），它只在 Settings / Plugins 里开关。GenUI 的后端与模型只在 `Settings / GenUI` 里编辑。外部 Agent 只在 `Settings / AI Integration` 里配置。外部 Harness 连上 Mark Go Go 与 GenUI 后端登录互不代表：前者经 CLI 或 socket 读写工作区，后者只产出一篇文档的阅读视图。设置中心的注册与深链见 ADR 0015。
- **后端可以换，编排只有一份。** 生成流程拆成协议无关的 Orchestrator（每轮隔离目录、`source.md`、`view.json`、生成提示词、25 ms `out.json` 预览、合并、候选校验、替换视图）与只负责自己协议的 `Runner`（`Probe`、`ListModels`、`ExecuteTurn`、`Close`）。四个后端平等出现在 2×2 选卡里，没有内置默认的厂商：Codex 用无参数的 `codex-acp`；Pi 直连 `pi --mode rpc`，与 Feynman 共享 `internal/pirpc`；Antigravity 的 `agy acp` 降为可选预设；其余 ACP Agent 走 Custom ACP 填命令与参数。具体实现、`settings.json` v2 的迁移、模型发现与会话写入、Pi 的守卫扩展与每轮重启都在 ADR 0018；`pi-acp` 这类双跳方案的用户通过 Custom ACP 接入，不需要应用为它增加一条路径。
- **外部 Agent 走本机通道，而不是端口。** 面向外部 Harness 的主路径是 `markgogo` CLI 与 `<UserData>/ipc.sock` 上的本机 Unix socket（显式开启，目录 `0700`、socket `0600`），`markgogo mcp` 的 stdio 垫片是折叠起来的补充接法，转发到同一个 socket。不对外开放 HTTP 端口与长期明文 Token；工具面收敛为七项。只有 `propose_edit` 进入审阅队列，接受后才修改原文。`create_document` 立即新建文稿并拒绝覆盖，`write_genui_view` 校验后立即替换阅读视图，`create_checkpoint` 立即为已保存的原文创建快照。这些都是 ADR 0017 的决定，本文只承诺界面对外表达的三件事：默认关闭、路径与监听状态可见、最近一次外部调用的来源与时间可见。
- **`Settings / AI Integration` 是外部 Agent 的唯一编辑器。** 页面上是 2×2 的 `External Agent Capabilities` 概览、`Command Line Tool` 一节（`markgogo` 的安装路径、`Install`/`Uninstall`、持久化的 IPC 开关、socket 状态、最近一次外部调用）、`Connect an Agent` 一节（Claude Code、Codex、Pi、Other 各自的提示词、连接方式与静态 `SKILL.md`）以及折叠的 `MCP (stdio)` 配置（`{"command":"markgogo","args":["mcp"]}`，不含端口与 Token）。
- **安装可回退，不借系统目录。** `Install` 只把本应用携带的 `markgogo` 软链到 `~/.local/bin`（目录不存在就创建），`Uninstall` 只删除这个链接；不写 `/usr/local/bin`、不需要 sudo、不改 shell 配置。路径上已有的别的程序的文件一律不动，页面说明原因。构建若没有随应用提供该二进制，页面把状态标成 `Not in this build`，并给出从源码检出构建（`go build ./cmd/markgogo`）的手动方法，而不是假装装好了；`go.mod` 以 `replace` 指向内置 MyGo，所以 `go install …@latest` 不可用。
- **提示词与技能只有一份。** `internal/agentguide` 保存提示词、静态 `SKILL.md`、四个 Harness 的说明与 stdio 配置；`markgogo agent-prompt` 与设置页读同一份文本，避免两处漂移。`SKILL.md` 只写进应用自己的用户数据目录，由用户复制到其 Harness 读取技能的位置。
- **合规口径写进 README。** 中英文 README 用同一节平等介绍 Codex、Pi、Antigravity 与 Custom ACP 的安装与认证，并明确：Mark Go Go 调用用户本机已安装、未做修改的 CLI；不代管第三方凭证；用户遵守各提供商条款。外部通道同样只在本机、只面向用户自己的账户。

## Consequences

- 关闭 GenUI 不再等于"AI 全关"，打开外部通道也不代表选择了 GenUI 后端；两处开关各自解释自己做的那一件事。
- 新增一个 GenUI 后端只需要实现 `Runner`；新增一个外部 Harness 只需要一份提示词与（可选的）连接命令，两者都不改动设置中心的其它页面。
- `markgogo` 的二进制目前不由 `mygo` 打包流程产出，随包安装要等发布流程补上这一步；在那之前 `Install` 保持禁用，页面给出手动安装方法。
- 真实 Harness（Claude Code、Codex、Pi）与 MCP Inspector 的端到端验收需要启动真实应用与外部客户端，仍待维护者执行；仓库内的自动化覆盖到 socket 与 stdio 垫片的握手、错误码与工具结果。

## Alternatives considered

- **把外部 Agent 接法并进 `Settings / GenUI`。** 入口更少，但会把"生成阅读视图的后端"和"读写工作区的 Agent"混成一个概念，用户无法判断登录的是谁、谁在用他的文件。
- **为外部通道开一个 HTTP 端口或长期 Token。** 单机桌面端没有发放与撤销 Token 的路径，端口还把本机能力暴露到网络；ADR 0017 已否决。
- **直接写 `~/.claude`、`~/.codex` 等第三方配置来"一键接入"。** 越界改别人的配置，也无法在各版本间保持一致；因此 `SKILL.md` 只写应用自己的目录，其余由用户复制。
- **把 `markgogo` 装进 `/usr/local/bin`。** 需要 sudo 且难以回退，于是改为用户自己的 `~/.local/bin` 软链。
