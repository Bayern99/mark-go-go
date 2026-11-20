# ADR 0009 · 进程内 MCP Bridge、无头 GenUI 校验与 MyGo 模块边界

Status: accepted

Mark Go Go 在桌面端 Go 进程内提供一个仅监听 `127.0.0.1` 随机端口、按会话校验 Bearer Token 的 Streamable HTTP MCP Server，通过 `<work>/.pi/mcp.json` 自动注入 Feynman（未来可零改动复用到外部 Agent）。同时将 A2UI 1.0 + Catalog 校验下沉为纯 Go 无头校验器，并按 MyGo 最佳实践重整 Go 模块边界。

本 ADR 记录原生 UI 迁移前的宿主决定。迁移后的窗口和视图由 [MyGo Native UI](../../ARCHITECTURE.md) 绘制；原先生成 WebView TypeScript 绑定的方式已退役。

**Considered Options**
- 继续使用整篇拷贝 `<work>/current-document.md` + 提示词拼接，并保留 GenUI 校验对活 WebView `EvalContext` 的依赖：Feynman 看不到未保存缓冲与 `<!-- note: ... -->` 批注，无法提交局部变更提案，且 Go 后端无法无头校验 GenUI 产物。
- 保留无点号模块名 `module markgogo` 与根目录单 `package main`：子模块可直接抢锁读写 `documentHost` 内部字段；且 `third_party/mygo/internal/tsgen/source.go` 要求包路径首段包含 `.` 才会为纯 DTO 子包保留 TypeScript JSDoc 与枚举联合类型。

**Consequences**
- MCP Server 同时提供只读 Resources（`workspace`、`document`、`sections`、`notes`）与结构化 Tools（`propose_edit`、`create_document`、`open_document`、`write_genui_view`、`create_checkpoint`），所有对已有原文的修改均进入 Agent 无关的 Hunk 级 `Change proposal` 队列，并在当前视图模式下以单次可撤销事务合入。
- `go.mod` 模块路径迁移为 `github.com/Bayern99/mark-go-go`，Go 后端拆分为职责单一的 `internal/` 领域包（工作区/文档、审阅/检查点、GenUI 无头校验、Feynman/ACP 适配、MCP Server），`package main` 负责依赖装配。原生窗口直接使用 Go 服务；WebView TypeScript 绑定留在固定的 [迁移前提交](https://github.com/Bayern99/mark-go-go/tree/81522cb48ec9718bffbda0255df1039692a2f6cd)。
- 单文档检查点在 Git 仓库内沿用独立本地引用并补充标签/来源元数据，在非 Git 工作区回退为应用内轻量本地快照。
