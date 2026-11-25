# Mark Go Go

[English](README.en.md) · [致谢与许可](ACKNOWLEDGMENTS.md)

Mark Go Go 是基于 Go 与 MyGo Native UI 编写的 macOS 本地 Markdown 编辑器，不依赖 Electron、WebView 或 Node.js。

## 核心能力

- **字节保真的原地 Markdown 编辑**：可编辑的 **Preview** 与 **Markdown** 源码共用同一文本缓冲和 Undo/Redo（`⌘E` 切换），保存时只改写编辑过的区块，保留其余原文的原始字节。支持标题章节折叠、数学公式、Mermaid 图表、脚注、就地块编辑，以及可拖拽调宽、按内容换行的表格。
- **GenUI 衍生阅读视图**：将 Markdown 文档生成只读的卡片、表格、图表和交互控件视图，保存在文档同目录的 `.<文件名>.genui.json` 中，并支持点击来源跳回对应原文标题。v3 视图沿用 **A2UI 1.0** 消息信封与章节哈希（保留局部更新，并兼容 v2 视图），`components` 内使用 **OpenUI Lang 1.0** 语句，解析、求值与渲染均在 Go 原生完成。切到中央 **GenUI** 后，通过右上角 **Request** 生成、增量更新或回退视图；后端、模型与思考等级在 `Settings / GenUI`（`⌘,`）里选择，齿轮按钮深链到那里。
- **多工作区与导航**：多个文件夹工作区共享同一窗口和侧栏竖排文档标签（`Ctrl+Tab` 在最近文档间往返）；侧栏提供 **Files / Outline** 切换、零缩进目录钻取与大纲顶部定位跳转；`⌘K` / `⌘T` 命令栏统一检索打开的文档、工作区文件、标题和命令；`⌘,` 打开统一设置中心。每个工作区可独立指定浅色或深色外观及配色主题（取自五行传统色表），用于区分不同目录。

## 快捷键

| 快捷键 | 操作 |
|---|---|
| `⌘K` / `⌘T` | 打开命令栏（文档、文件、标题与命令） |
| `⌘E` | 切换 Preview 与 Markdown 源码 |
| `⌘0` | 收起或显示左侧栏（窄窗切换抽屉） |
| `⌥⌘0` | 收起或显示右侧 Feynman 面板 |
| `⌘,` | 打开设置中心 |
| `Ctrl+Tab` | 在最近文档间往返 |

## 可选插件

在 **Settings / Plugins**（`⌘,`）或命令栏（`⌘K`）中独立启用 **Feynman**、**GenUI** 和 **Git**。Feynman 与 Git 默认关闭；GenUI 默认开启，关闭后文档回到 `Preview`，已保存的视图不会删除。启用中的插件若拥有自己的设置页，Plugins 行提供 `Configure →` 深链。

### Feynman

[Feynman](https://feynman.is) 是 **Companion** 开发的开源（MIT）终端 AI 研究代理 CLI，基于 **Pi** 代理运行时和 **alphaXiv** 构建，提供文献检索、审阅、写作、核验等研究工作流。Mark Go Go 通过本机 `feynman --mode rpc` 直接连接已安装的 Feynman，不引入额外封装层；宽窗（≥ 1180 pt）在右侧以可调宽面板显示，窄窗以抽屉显示，同一工作区切换文档时保留对话与草稿。[Feynman 安装指南](https://feynman.is/docs/getting-started/setup)

- **Chat / Files / alphaXiv**：在 Chat 中用 **Actions** 或 `/` 调用工作流、扩展与会话操作，选择模型与思考级别、附加论文或图片、查看工具调用与引用链接（Enter 发送，Shift+Enter 换行，发送失败保留草稿）；在 Files 中以普通标签打开生成的报告与草稿；通过 **Review** 对比并应用对 Markdown 的修改提案（支持一次撤销）；在 alphaXiv 栏管理论文上下文。齿轮按钮跳转至设置中心的 Models & Providers、Search & Papers、MCP Servers、Packages 与 Diagnostics；交互式登录在系统终端完成。
- **配置链接规则**：启动前将 `~/.feynman` 中已有的凭据、模型、记忆、MCP 配置、技能、提示、扩展、包和会话链接到应用配置，不覆盖已有私有文件；已链接的会话和全局 MCP 修改与普通 CLI 互通。项目信任文件独立且默认拒绝，只批准应用托管的工作目录。Windows 使用符号链接（需开发者模式或符号链接权限），失败时不改用复制；Windows 运行时尚未实测。

### Git 检查点

按仓库内文件路径将快照保存为独立的本地 Git 引用；不在 Git 仓库中时，每篇文档保留最多 50 个本地快照，之后执行 `git init` 仍可读取。保存文档后，从纸页顶行 **Checkpoints** 创建快照、查看差异或恢复旧版。文档改名后从新路径开始记录。

## 自带本机 Agent

Mark Go Go 不内置模型，也不代管第三方凭证：它调用你本机已安装、未做修改的 Agent CLI，用你自己的账户登录，怎么用由各提供商条款约束。这一节有两件事，界面上也分在两处：**GenUI 后端**只生成本篇文档的阅读视图，**外部 Harness** 通过本机通道读写工作区。

### GenUI 后端：Codex、Pi、Antigravity、Custom ACP

`Settings / GenUI` 是选择后端的唯一入口：2×2 卡片带状态点与 `Test & Refresh`（只读命令、版本与模型列表，不发送文档内容），`Model` 与按能力出现的 `Reasoning effort` 在 `Generation`，命令与参数在 `Advanced`（仅 Custom ACP 默认展开）。四个后端一视同仁，没有内置默认的厂商，新装按 `codex-acp` → `pi` → `agy` 探测。

| 后端 | 安装 | 认证 | 说明 |
|---|---|---|---|
| Codex | `codex-acp`（依赖底层 `codex`；用 `CODEX_PATH` 指定它的路径；`codex-acp --version` 自检） | `codex login`，或在 `Settings / GenUI` 里 `Sign in in Terminal` 运行 `codex-acp login` | 以无参数 ACP 适配器启动；模型与思考等级由会话声明的 `configOptions` 决定 |
| Pi | 安装 `pi`，终端里能运行 `pi` | 先选模型让应用知道 `provider`，再在终端运行 `pi auth login <provider>` | 直连 `pi --mode rpc`，不经 ACP：每轮新起进程，并加载随应用分发的守卫扩展，读写只限本轮的隔离目录与 `out.json`；扩展未确认加载时不生成 |
| Antigravity | 安装提供 `agy` 的 Antigravity CLI | `agy acp --login` | 可选预设，不再是硬编码默认 |
| Custom ACP | 任意在标准输入输出上说 ACP 的 Agent（包括 `pi-acp` 这类双跳方案） | 用该 Agent 自己的命令登录 | 在 `Advanced` 填命令与参数 |

Pi 后端随本版本提供；它的自动化覆盖使用 Fake Pi，真实 Pi 账户下的流式生成与越界写入拦截仍待实测。Antigravity 与 Codex 的模型刷新、登录与流式生成同样需要在装有对应 CLI 的机器上验证。

### 外部 Harness：Claude Code、Codex、Pi、其他

`Settings / AI Integration` 把一个窗口的工作区交给你自己在终端里跑的 Agent：

- **`markgogo` 命令**：独立二进制，`Install` 把它链接到 `~/.local/bin`，`Uninstall` 只删除本应用做的那个链接。不写 `/usr/local/bin`、不需要 sudo、不改 shell 配置；路径上别人的文件一律不动。若这个构建没有随应用提供该二进制，页面会写明"未随应用提供"并给出 `go install github.com/Bayern99/mark-go-go/cmd/markgogo@latest` 或从源码构建的方法。
- **本机通道**：默认关闭。打开后应用在用户数据目录的 `ipc.sock` 上按行收发 JSON-RPC（目录 `0700`、socket `0600`），只接受你本机、你自己的用户；页面显示是否正在监听，以及最近一次外部调用的来源和时间。关闭时停止监听并删除 socket。
- **接法**：页面为 Claude Code、Codex、Pi 与其他 Agent 各给一份提示词（`Copy prompt`）与一份静态 `SKILL.md`（写进应用自己的目录，或复制到你的 Harness 读取技能的位置）。shell 型 Harness 直接运行 `markgogo`；只讲 MCP 的客户端可以在折叠的 `MCP (stdio)` 区取用 `{"command":"markgogo","args":["mcp"]}` 注册 stdio 垫片，它转发到同一个 socket，没有端口也没有 Token。沙箱型客户端（如 Codex）可能拒绝访问 Unix socket；这时请给客户端所需的权限，而不是关掉应用里的边界。
- **CLI 命令**：`status`、`read`（章节、行范围、批注）、`propose`、`new`、`open`、`genui format`、`genui write`、`checkpoint`、`agent-prompt`、`mcp`。只有 `read`、`genui format` 与 `agent-prompt` 不依赖运行中的应用；写操作一律进入审阅队列，由你在编辑器里接受或拒绝。
- **协议**：外部通道说 MCP `2026-07-28`（`server/discover` 与逐请求 `_meta` 版本），并对旧客户端回退 `2025-11-25` 的 `initialize` 握手。这两个版本由本仓库的 socket 与 stdio 垫片测试覆盖；具体 Harness 与 MCP Inspector 的端到端记录见发布前的验收。

**这两件事不要混：** 外部 Harness 连接 Mark Go Go，是用它自己的账户在你的终端里读写工作区；GenUI 后端登录，是让某个 Agent 只为本篇文档生成阅读视图。前者在 `Settings / AI Integration`，后者在 `Settings / GenUI`。

## 开发与验证

需要 [go.mod](go.mod) 指定的 Go 版本，无需 Node.js：

```sh
go tool mygo dev      # 本地运行
go tool mygo build    # 构建当前平台安装包
scripts/verify.sh     # gofmt、mygo vet、全包 -race 测试与 Windows amd64 交叉编译
```

## 文档索引

- [ARCHITECTURE.md](ARCHITECTURE.md)：排查代码位置或了解包边界时阅读（窗口外壳、编辑器、GenUI、Feynman、Portrait 入口）。
- [DESIGN.md](DESIGN.md)：修改界面布局、配色、控件、动效或无障碍行为前阅读（Portrait 2.0 规范与测试契约）。
- [docs/genui/view-format.md](docs/genui/view-format.md)：编写、调试或校验 `.genui.json` 视图与 OpenUI Lang v3 语句时阅读。
- [docs/agents/native-automation.md](docs/agents/native-automation.md)：验证 macOS 原生窗口、菜单或会话恢复时使用。
- [docs/adr/0016-ai-integration-faces.md](docs/adr/0016-ai-integration-faces.md)：了解 AI 集成为什么分成 GenUI 插件、可换后端与外部 Agent 通道三个面时阅读。
- [CHANGELOG.md](CHANGELOG.md) · [docs/acceptance.md](docs/acceptance.md) · [LICENSE](LICENSE)
