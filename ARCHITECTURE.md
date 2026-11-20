# 架构与代码入口

桌面端用 Go/MyGo 管理原文 I/O、工作区窗口和原生生命周期，用 WKWebView 承载 React 界面。浏览器预览和测试使用同一界面的内存宿主。

| 任务 | 代码入口 |
|---|---|
| 启动与 Go 绑定 | [main.go](main.go) · [Web 宿主契约](web/src/host/types.ts) · [IPC 适配](web/src/host/mygoHost.ts) |
| 文档标签、导航与恢复 | [workspace.go](internal/workspace/workspace.go) · [documenthost.go](internal/workspace/documenthost.go) · [index.go](internal/workspace/index.go) · [session.go](internal/workspace/session.go) · [restore_host.go](internal/workspace/restore_host.go) |
| 外壳、三栏与命令 | [WorkspaceShell.tsx](web/src/components/WorkspaceShell.tsx) · [DocumentPane.tsx](web/src/components/DocumentPane.tsx) · [DocumentTabs.tsx](web/src/components/DocumentTabs.tsx) · [core](web/src/core/) · [界面设计](DESIGN.md) |
| 原文编辑与逐块保真 | [编辑器](web/src/editor/createEditor.ts) · [块组合](web/src/editor/blocks/compose.ts) · [Markdown 编辑器](web/src/editor/source/sourceEditor.ts) |
| GenUI 视图与格式 | [viewfile.go](internal/genui/viewfile.go) · [组件目录](web/src/plugins/genui/a2ui.tsx) · [格式和示例](web/src/plugins/genui/view-format.md) |
| ACP 生成、局部更新和缓存 | [generate.go](internal/acp/generate.go) · [update.go](internal/genui/update.go) · [process_cache.go](internal/acp/process_cache.go) |
| 可选插件与安全改文 | [插件开关](web/src/plugins/DesktopPlugins.tsx) · [Feynman 右栏](web/src/plugins/feynman/) · [直接 `--mode rpc` 与异步 CLI](internal/agent/feynman/research.go) · [应用内 MCP](internal/mcp/) · [研究产物](internal/review/artifact.go) · [检查点](internal/review/checkpoints.go) · [插件范围决策](docs/adr/0007-research-and-checkpoints.md) |
| MyGo 升级 | [受控补丁](docs/mygo-document-groups.md) · [验收](docs/acceptance.md) |

行为以实现和测试为准。相关 [ADR](docs/adr/) 记录产品与技术取舍，[术语表](GLOSSARY.md) 统一命名。
