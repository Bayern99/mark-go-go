# Mark Go Go

[English](README.en.md) · [致谢与许可](ACKNOWLEDGMENTS.md)

Mark Go Go 是基于 Go 与 MyGo Native UI 编写的 macOS 本地 Markdown 编辑器，不依赖 Electron、WebView 或 Node.js。

## 核心能力

- **字节保真的原地 Markdown 编辑**：可编辑的 **Preview** 与 **Markdown** 源码共用同一文本缓冲和 Undo/Redo（`⌘E` 切换），保存时只改写编辑过的区块，保留其余原文的原始字节。支持标题章节折叠、数学公式、Mermaid 图表、脚注、就地块编辑，以及可拖拽调宽、按内容换行的表格。
- **GenUI 衍生阅读视图**：将 Markdown 文档生成只读的卡片、表格、图表和交互控件视图，保存在文档同目录的 `.<文件名>.genui.json` 中，并支持点击来源跳回对应原文标题。v3 视图沿用 **A2UI 1.0** 消息信封与章节哈希（保留局部更新，并兼容 v2 视图），`components` 内使用 **OpenUI Lang 1.0** 语句，解析、求值与渲染均在 Go 原生完成。切到中央 **GenUI** 后，通过右上角 **Request** 生成、增量更新或回退视图，通过齿轮按钮进入设置页选择模型。
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
- [CHANGELOG.md](CHANGELOG.md) · [docs/acceptance.md](docs/acceptance.md) · [LICENSE](LICENSE)
