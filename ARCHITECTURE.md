# 架构与代码入口

正式桌面窗口由 Go 和 MyGo Native UI 绘制。Go 服务管理原文、工作区、会话、插件与文件操作；窗口中的编辑、导航和阅读视图使用同一份原文状态。

| 任务 | 代码入口 |
|---|---|
| 应用启动与原生窗口 | [main.go](main.go) · [窗口适配](internal/nativeui/window.go) |
| 窗口外壳、导航、标签和抽屉 | [Shell](internal/nativeui/shell.go) · [导航](internal/nativeui/navigation.go) · [标签](internal/nativeui/tabs.go) |
| Markdown Preview、Markdown 编辑与字节保真 | [原文缓冲](internal/nativeeditor/source.go) · [Preview 布局](internal/nativeeditor/preview.go) · [特殊块](internal/nativeeditor/special.go) · [表格](internal/nativeeditor/table.go) |
| 工作区、磁盘 I/O、会话恢复与章节位置 | [工作区服务](internal/workspace/) · [章节定位](internal/workspace/sections.go) |
| GenUI 格式校验、生成与原生阅读 | [格式与生成](internal/genui/) · [原生阅读器](internal/nativegenui/) · [格式说明和示例](docs/genui/view-format.md) |
| Feynman、应用内 MCP、提案和检查点 | [Feynman RPC](internal/agent/feynman/) · [MCP](internal/mcp/) · [提案](internal/review/) · [原生审阅面板](internal/nativereview/) |
| Portrait 样式、字体与图标 | [原生主题](internal/nativeui/portrait.go) · [打包字体和许可](internal/nativeui/fonts/) |
| MyGo 版本与项目补丁 | [MyGo 文档组记录](docs/mygo-document-groups.md) · [MyGo 源码](third_party/mygo/) |

当前行为以 Go 实现、测试和配置为准。产品与技术取舍见 [ADR](docs/adr/)，界面说明见 [DESIGN.md](DESIGN.md)，术语见 [GLOSSARY.md](GLOSSARY.md)。迁移前的 WebView 实现可在 [固定提交 81522cb](https://github.com/Bayern99/mark-go-go/tree/81522cb) 查看。
