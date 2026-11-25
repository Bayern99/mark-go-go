# Mark Go Go

[English](README.en.md) · [致谢与许可](ACKNOWLEDGMENTS.md)

Mark Go Go 是一款本地 Markdown 编辑器。你的文档就是磁盘上的普通 `.md` 文件，应用直接打开、编辑和保存它们，不需要账户，也没有云端存储。界面由 Go 和 MyGo Native UI 原生绘制，不使用 Electron、WebView 或 Node.js。目前只发布 macOS（Apple 芯片）安装包。

编辑器之外有三项可选功能：把文档整理成结构化阅读视图的 **GenUI**、文献研究助手 **Feynman**，以及每篇文档独立的 **Git 检查点**。另外，你在终端里运行的 AI Agent 可以通过 `markgogo` 命令读取工作区，并向编辑器提交修改提案。涉及 AI 的部分都调用你本机已安装的 Agent CLI，应用本身不内置模型，也不保存第三方凭证。

## 它能做什么

- **编辑 Markdown**：在渲染后的 **Preview** 里直接修改，或按 `⌘E` 切到 **Markdown** 源码。两种视图共用一份文本和一条撤销历史。保存时只改写你编辑过的块，其余内容的原始字节保持不变。
- **管理多个文件夹**：一个窗口可以打开多个文件夹工作区，打开的文档在侧栏竖排列出。侧栏可以切换文件树和大纲，`⌘K` 命令栏可以搜索文档、文件、标题和命令。重启后恢复打开的文档和滚动位置。
- **生成阅读视图（GenUI，默认开启）**：让本机 AI Agent 把一篇文档整理成由卡片、表格、图表组成的只读视图。视图中每一项都标明依据的原文章节，原文变动后会提示哪些部分已过时，也可以只更新这些部分。
- **研究（Feynman，默认关闭）**：在文档旁的面板中与本机安装的 [Feynman](https://feynman.is) 对话，用于检索文献、审阅、写作和核对引用。Feynman 提出的修改先进入审阅，接受后才写入原文。
- **版本快照（Git 检查点，默认关闭）**：为单篇文档保存本地快照，与当前内容比较，或恢复到旧版本。
- **接入外部 Agent（默认关闭）**：打开本机通道后，Codex、Pi 等在终端里运行的 Agent 可以用 `markgogo` 命令读取当前工作区、提交修改提案、新建文档或写入阅读视图。

## 它不做什么

- 不提供模型、AI 账户或订阅。AI 功能依赖你自己安装和登录的 CLI，用量与数据处理遵循各提供商的条款。
- 没有云同步、多人协作或账户系统。
- 不会自行把文档发给任何服务。只有在你启用 GenUI、Feynman 或外部通道并主动使用时，文档内容才会交给你配置的本机 Agent。Markdown 中引用的远程图片会通过 HTTPS 下载显示。
- 不提供 Linux 和 Windows 安装包。CI 在 Linux 上运行全部自动化测试，Windows 只做交叉编译，两个平台都没有在真机上验证过。

## 安装

从 [Releases](https://github.com/Bayern99/mark-go-go/releases) 下载 `mark-go-go-<版本>-arm64.dmg`，把应用拖进"应用程序"文件夹。安装包是临时签名（ad hoc），没有经过 Apple 公证。首次打开时 macOS 会阻止启动，需要到"系统设置 → 隐私与安全性"中点"仍要打开"。

从源码运行需要 [go.mod](go.mod) 指定的 Go 版本，不需要 Node.js：

```sh
go tool mygo dev      # 本地运行
go tool mygo build    # 构建当前平台的安装包
scripts/verify.sh     # gofmt、mygo vet、全部包的 -race 测试与 Windows amd64 交叉编译
```

## 功能说明

### 编辑

- Preview 支持标题章节折叠、数学公式、Mermaid 图表、脚注、可拖动调宽并自动换行的表格，以及在特殊块内直接编辑该块的 Markdown。
- 批注以 `<!-- note: … -->` 注释保存在原文中，因此在任何 Markdown 工具里都还能看到。
- 文件菜单提供另存为、复制、重命名、移动和还原。`.md` 与 `.markdown` 文件可以从 Finder 直接打开。

### 工作区与外观

- 侧栏的 **Files** 是文件树，**Outline** 是当前文档的大纲，点标题即跳转。`Ctrl+Tab` 在最近的文档之间往返。
- 每个工作区可以选一个配色 Space（共八个，取自中国传统五行色），外观可设为跟随系统、浅色或深色，用来区分不同的文件夹。
- `⌘,` 打开设置中心，应用和插件的设置集中在这里，支持搜索。

| 快捷键 | 操作 |
|---|---|
| `⌘K` / `⌘T` | 命令栏：文档、文件、标题与命令 |
| `⌘E` | 切换 Preview 与 Markdown 源码 |
| `⌘0` | 收起或显示左侧栏（窄窗时为抽屉） |
| `⌥⌘0` | 收起或显示右侧 Feynman 面板 |
| `⌘,` | 设置中心 |
| `Ctrl+Tab` | 在最近的文档之间往返 |

### 插件

三个插件都在 **Settings / Plugins** 或命令栏里单独开关。GenUI 默认开启，Feynman 和 Git 默认关闭。关闭 GenUI 后，文档回到 Preview，已保存的视图不会删除。

**GenUI 阅读视图。** 在文档中央切换到 **GenUI**，点右上角 **Request** 生成视图。生成过程中卡片会逐张出现。视图保存在文档同目录的 `.<文件名>.genui.json` 中，Markdown 原文始终不变。视图里的来源按钮显示依据的章节：悬停或用键盘聚焦可以预览原文摘录，点击跳回原文。原文改动后，相应卡片会标记为已过时，可以单独更新，也可以退回上一版视图。生成由你在 **Settings / GenUI** 选择的本机 Agent 完成，见下文的[后端说明](#genui-后端)。视图格式见 [docs/genui/view-format.md](docs/genui/view-format.md)。

**Feynman。** [Feynman](https://feynman.is) 是 Companion 开发的开源（MIT）研究代理 CLI，基于 Pi 代理运行时和 alphaXiv 构建。Mark Go Go 通过 `feynman --mode rpc` 连接你已安装的 Feynman（[安装指南](https://feynman.is/docs/getting-started/setup)）。窗口宽度在 1180 点以上时，它显示为文档旁可调宽的第二张纸页，较窄时显示为抽屉。

- **Chat**：用 **Actions** 按钮或输入 `/` 调用工作流与命令，可以选择模型和思考级别、附加论文或图片、查看工具调用与引用。
- **Files**：打开 Feynman 生成的报告和草稿。
- **alphaXiv**：管理论文上下文。
- **Review**：比较并接受或拒绝对 Markdown 的修改提案，接受后可以一步撤销。

同一工作区内切换文档时，对话和未发送的草稿都会保留。

**Git 检查点。** 保存文档后，从纸页顶部的 **Checkpoints** 创建快照、查看差异或恢复。在 Git 仓库中，快照保存为独立的本地 Git 引用，不影响你的分支和暂存区。不在仓库中时，每篇文档最多保留 50 个快照，之后执行 `git init` 仍可读取。文档改名后从新路径重新记录。

## AI 集成参考

AI 相关的设置分在两处，作用不同：

- **Settings / GenUI**：选择哪个 Agent 为当前文档生成阅读视图。Agent 只收到这篇文档，产出的视图经应用校验后才保存。
- **Settings / AI Integration**：让你在终端里运行的 Agent 读写整个工作区。

### GenUI 后端

可选四种后端，没有默认指定的厂商。首次使用时按 `codex-acp`、`pi`、`agy` 的顺序探测已安装的命令。**Test & Refresh** 只读取命令版本和模型列表，不会发送文档内容。模型和思考等级在 **Generation** 中选择，命令和参数在 **Advanced** 中修改。

| 后端 | 安装 | 登录 | 说明 |
|---|---|---|---|
| Codex | `codex-acp`，它会调用底层的 `codex`。可用 `CODEX_PATH` 指定 `codex` 的路径，用 `codex-acp --version` 自检 | `codex login`，或在设置页点 **Sign in in Terminal** | 通过 ACP 连接，模型和思考等级以会话声明的选项为准 |
| Pi | 安装 `pi`，确保终端里能运行它 | 先在设置页选模型，再在终端运行 `pi auth login <provider>` | 直接连接 `pi --mode rpc`，不经过 ACP。每次生成启动一个新进程，并加载应用自带的守卫扩展，把读取限制在本次生成的临时目录、写入限制在该目录的 `out.json`。守卫未确认加载时不会生成 |
| Antigravity | 安装提供 `agy` 命令的 Antigravity CLI | `agy acp --login` | 通过 ACP 连接 |
| Custom ACP | 任何通过标准输入输出使用 ACP 协议的 Agent | 使用该 Agent 自己的登录命令 | 在 **Advanced** 中填写命令和参数 |

每次生成都在独立的临时目录中进行，结果通过格式校验后才会替换现有视图。

### 外部 Agent 与 `markgogo` 命令

**Settings / AI Integration** 把一个窗口的工作区交给你在终端里运行的 Agent。

- **`markgogo` 命令**：安装包在 `Contents/Resources/markgogo` 中带有这个独立的命令行程序。点 **Install** 会在 `~/.local/bin` 创建一个指向它的链接，**Uninstall** 只删除这个链接。整个过程不需要 sudo，不修改 shell 配置，也不动其他程序放在那里的文件。从源码直接运行应用时没有这份程序，页面显示 `Not in this build`。此时可以在仓库目录中运行 `go build -o ~/.local/bin/markgogo ./cmd/markgogo` 手动构建。`go install …@latest` 无法构建，因为 `go.mod` 用 `replace` 指向仓库内置的 MyGo。
- **本机通道**：默认关闭。打开后，应用在用户数据目录的 `ipc.sock` 上收发 JSON-RPC 消息，每条消息占一行。目录权限为 `0700`，socket 为 `0600`，只有你本人的账户能连接。通道不监听网络端口，也不使用 Token。页面显示通道是否在监听，以及最近一次外部调用的来源和时间。关闭后停止监听并删除 socket。
- **连接方式**：页面为 Codex、Pi 和其他 Agent 各提供一份提示词和一份 `SKILL.md`。能运行 shell 命令的 Agent 直接调用 `markgogo`。只支持 MCP 的客户端可以注册 `{"command":"markgogo","args":["mcp"]}`，它把 MCP 的标准输入输出转发到同一个 socket。沙箱型客户端（如 Codex）可能不允许访问 Unix socket，这时应在客户端一侧授予权限。
- **命令**：`status`、`read`（章节、行范围、批注）、`propose`、`new`、`open`、`genui format`、`genui write`、`checkpoint`、`agent-prompt`、`mcp`。其中 `read`、`genui format` 和 `agent-prompt` 不需要应用在运行。
- **何时生效**：只有修改提案（`markgogo propose`，MCP 工具 `propose_edit`）会进入审阅队列，在你接受后才修改原文。其余写操作立即生效：
  - `markgogo new`（`create_document`）新建文档，不会覆盖已有文件；
  - `markgogo genui write`（`write_genui_view`）校验后替换阅读视图；
  - `markgogo checkpoint`（`create_checkpoint`）为已保存的文档创建快照。
- **协议**：通道使用 MCP `2026-07-28`（`server/discover` 与逐请求的 `_meta` 版本），同时兼容旧客户端的 `2025-11-25` `initialize` 握手。

### Feynman 的配置

Feynman 启动前，应用把 `~/.feynman` 中已有的凭据、模型、记忆、MCP 配置、技能、提示、扩展、包和会话链接到应用自己的配置目录，不覆盖其中已有的私有文件。链接后，会话和全局 MCP 的修改与命令行版的 Feynman 共享。项目信任列表由应用单独保存，默认拒绝，只批准应用管理的工作目录。Windows 上这一步使用符号链接，需要开启开发者模式或具备创建符号链接的权限，失败时不会改用复制。

## 验证情况

每次提交都在 CI 上运行格式检查、`mygo vet`、全部包的 `-race` 测试和 Windows 交叉编译。macOS 真机上的验收记录见 [docs/acceptance.md](docs/acceptance.md)，包括四种 GenUI 后端的实际生成、Pi 守卫的越界拦截、两个版本的 MCP 协议，以及通过 Pi 和 Codex 接入外部 Agent。

## 文档索引

- [ARCHITECTURE.md](ARCHITECTURE.md)：代码入口与包边界。
- [DESIGN.md](DESIGN.md)：界面规范（Portrait 2.0）与测试契约，修改布局、配色、控件、动效或无障碍行为前阅读。
- [GLOSSARY.md](GLOSSARY.md)：产品术语。
- [docs/adr/](docs/adr/)：技术决策记录。AI 集成为什么分成三部分，见 [ADR 0016](docs/adr/0016-ai-integration-faces.md)。
- [docs/genui/view-format.md](docs/genui/view-format.md)：`.genui.json` 视图格式。
- [docs/agents/native-automation.md](docs/agents/native-automation.md)：macOS 原生窗口、菜单和会话恢复的验证方法。
- [CHANGELOG.md](CHANGELOG.md) · [LICENSE](LICENSE)
