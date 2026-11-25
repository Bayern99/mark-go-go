# Mark Go Go

[English](README.en.md) · [致谢与许可](ACKNOWLEDGMENTS.md)

Mark Go Go 是一款面向 macOS 的纯 Go 原生本地 Markdown 编辑器与阅读研究工作台。窗口、编辑器、侧栏、GenUI 阅读视图与研究面板全部由 MyGo Native UI 原生绘制——**零 Electron、零 WebView、零 Node.js 运行时**，兼顾原文件字节保真、低占用高响应与现代空间化交互。

## 核心特点与优势

- **纯 Go 原生性能与零 WebView 架构**：单进程原生渲染，启动快、内存低、无需打包浏览器内核；遵循 macOS 原生标题栏、菜单、输入法（IME）与 VoiceOver / AX 无障碍规范。
- **字节保真的原地 Markdown 编辑**：可编辑的 **Preview** 与 **Markdown** 源码共享同一缓冲与 Undo/Redo（`⌘E` 切换），保存时保留原有 Markdown 字节而不破坏未改动区块；支持标题章节折叠、数学公式、Mermaid 图表、脚注、就地块编辑，以及可拖拽调宽且自适应中文换行的原生表格。
- **Arc 灵感空间架构 × 五行八境（Wu Xing Spaces）**：窗口本身是带色彩的布景（Scenery），正文是浮于其上的不透明纸页（Page）。每个文件夹工作区可独立选择八种五行 Space（Five Elements、Metal–Water、Wood–Fire、Earth、Metal、Wood、Water、Fire），配备完整浅色与深色外观，16 套配色全部通过 WCAG AA 对比度自动化实算。
- **高效侧栏与全局命令栏**：多个文件夹工作区共享同一窗口和侧栏竖排文档标签，`Ctrl+Tab` 在最近文档间往返；侧栏支持 **Files / Outline** 切换、零缩进目录钻取、大纲实时高亮与顶部定位跳转，指针点击平滑收放侧栏、`⌘0` 瞬时切换；`⌘K` 或 `⌘T` 打开命令栏，一站式搜索打开的文档、工作区文件、标题与全部命令。
- **统一设置中心（`⌘,`）**：集中管理 General、Appearance、Editor、Shortcuts、Plugins、Diagnostics 以及插件专属页面，支持 `/` 实时过滤、深链直达与一键重置，界面中不再散落重复设置项。
- **GenUI 结构化阅读视图**：基于 A2UI 与 OpenUI Lang v3，将长篇 Markdown 生成只读交互式卡片、表格与图表阅读视图，支持点击卡片溯源定位回原文；切到中央 **GenUI** 后，通过右上角 **Request** 生成、增量更新或恢复上一版视图，齿轮按钮直达设置中心配置模型。

## 常用快捷键

| 快捷键 | 操作 |
|---|---|
| `⌘K` / `⌘T` | 打开命令栏（搜索文档、文件、标题与命令） |
| `⌘E` | 在可编辑的 Preview 与 Markdown 源码间切换 |
| `⌘0` | 收起或显示左侧栏（窄窗切换导航抽屉） |
| `⌥⌘0` | 收起或显示右侧 Feynman 研究面板 |
| `⌘,` | 打开统一设置中心 |
| `Ctrl+Tab` | 在最近访问的文档标签间往返 |

## 可选插件

在统一设置中心的 **Settings / Plugins**（`⌘,`）或命令栏（`⌘K`）中可分别启用 **Feynman** 和 **Git** 插件。两项初始均关闭。

### Feynman 研究面板

**Feynman** 直接连接本机 Feynman CLI 的 RPC 模式。安装 Feynman 并启用插件后，宽窗（≥ 1180 pt）在右侧呈现为并排的第二张纸页（可拖动分隔线调宽），窄窗为右侧滑出的玻璃抽屉；同一工作区切换文档时对话与输入草稿自动保留。[Feynman 设置指南](https://feynman.is/docs/getting-started/setup)

Mark Go Go 将 `~/.feynman` 中已有的登录凭据、模型、记忆、MCP 配置、技能、提示、扩展、包和会话等文件与目录链接到应用配置。写入已链接的会话或在设置中修改已链接的全局 MCP 配置，普通 CLI 也能看到。每次启动 Feynman 前都会补充缺失的链接，不覆盖应用配置中已有的内容；应用的项目信任文件独立、默认拒绝，只批准应用托管的工作目录，不修改全局信任。Windows 使用符号链接（需开启开发者模式或具备相应权限），链接失败时不会改用复制；Windows 运行时尚未实测。

- **Chat**：点击 **Actions** 或输入 `/` 按名称查找工作流、扩展与会话操作；支持选择模型与思考级别、附上论文或图片、展开工具调用详情、点击引用跳转，以及通过历史菜单新建或恢复本工作区对话。Enter 发送，Shift+Enter 换行；发送失败时草稿与附件自动保留且可无损互换。
- **Files**：集中浏览研究生成的报告、计划与草稿，直接作为普通文档标签打开；审阅修改提案（Review）后可将差异应用到 Markdown，并支持一次撤销。
- **alphaXiv**：在面板内检索与管理论文上下文，账户与搜索配置直链统一设置中心。
- **研究设置**：点击面板齿轮按钮即可深链至设置中心的 **Models & Providers**、**Search & Papers**、**MCP Servers**、**Packages** 与 **Diagnostics**；交互式登录与配置在系统终端中打开。

### Git 本地检查点

**Git** 检查点按仓库内的文件路径保存为独立的本地 Git 引用。不在 Git 仓库中时，应用为每篇文档保留最多 50 个本地快照；之后运行 `git init` 仍可继续访问。保存文档后，从纸页顶行的 **Checkpoints** 创建快照，或逐行对比差异并恢复旧版本。改名后的文档从新路径开始记录。

## 开发

原生应用使用 [go.mod](go.mod) 指定的 Go 版本，无需 Node.js 或前端构建工具链：

```sh
go tool mygo dev
go tool mygo build
```

预览、Markdown、GenUI 和工作区界面全部由 MyGo Native UI 绘制。26 份 Markdown 语料和共享章节向量由 Go 回归测试直接载入；协议与格式示例见 [GenUI 格式](docs/genui/view-format.md)。

## 验证

```sh
scripts/verify.sh
```

脚本运行 Go 格式检查、`go tool mygo vet`、全包竞态测试（`-race`）以及 Windows amd64 交叉编译。macOS 原生窗口、菜单与会话恢复验证见 [原生 Automation 与 macOS 验证流程](docs/agents/native-automation.md)。历史 WebKit 基线及同机对照方法见 [Portrait 性能记录](docs/agents/portrait-performance.md)。

[架构与代码入口](ARCHITECTURE.md) · [界面设计](DESIGN.md) · [更新日志](CHANGELOG.md) · [验收记录](docs/acceptance.md) · [MIT 许可证](LICENSE)
