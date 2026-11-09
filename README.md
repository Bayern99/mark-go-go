# Mark Go Go

[English](#english) · [中文](#chinese) · [Development / 开发](#development)

## English

Mark Go Go is a local Markdown editor for macOS. Open a folder as a workspace, edit Markdown in Rich text or Markdown mode, and use GenUI to read a generated view alongside the original document.

- A workspace window has a file tree and document tabs. Search paths, titles, excerpts and headings, including changes in open documents. The app restores the previous workspace, tabs and view modes when you reopen it.
- Use `⌘/` to switch between Rich text and Markdown. Rich text preserves the bytes of unedited blocks and lets you edit special blocks in place.
- GenUI is a read-only view derived from the original Markdown. Its cards link back to their sources. The right pane holds the Outline and the controls for generating or updating GenUI through an external ACP agent.
- Portrait is the light interface for all three columns. Fonts and icons ship with the app.

<a name="chinese"></a>

## 中文

Mark Go Go 是 macOS 上的本地 Markdown 编辑器。打开文件夹即可建立工作区，在 Rich text 或 Markdown 模式下编辑原文，也可以切换到由原文派生的 GenUI 阅读视图。

- 工作区窗口有文件树和文档标签。可搜索路径、标题、摘要和章节标题，搜索也会反映已打开原文中的改动。重启后恢复上次的工作区、标签和视图模式。
- 按 `⌘/` 切换 Rich text 与 Markdown。Rich text 逐字节保留未编辑的块，并支持原地编辑特殊块。
- GenUI 是从 Markdown 原文派生的只读视图。卡片可跳转到对应的原文位置。右栏放置大纲和 GenUI 生成控件，生成由外部 ACP Agent 执行。
- 三栏统一使用 Portrait 浅色界面，字体与图标随应用提供。

## Development

Use Go 1.27.3 and Node 24. 开发需要 Go 1.27.3 和 Node 24。

```sh
npm --prefix web ci
npx --prefix web playwright install chromium-headless-shell

# Desktop development / 桌面开发
go tool mygo dev

# Browser preview with an in-memory host / 浏览器预览
npm --prefix web run dev

# Regenerate bindings after Go API changes / Go 接口变更后更新绑定
go tool mygo generate
```

## Verification and packaging

```sh
npm --prefix web run typecheck
npm --prefix web test
npm --prefix web run build
go build ./...
go vet ./...
go test -race ./...

# Build the macOS app and DMG / 构建 macOS 应用和 DMG
go tool mygo build
```

The app and DMG are written under `build/`. 应用和 DMG 输出到 `build/`。

## Documentation

- [Changelog / 更新日志](CHANGELOG.md)
- [Acceptance evidence / 验收记录](docs/acceptance.md)
- [Architecture / 架构](ARCHITECTURE.md) and [ADRs / 架构决策](docs/adr/)
- [Interfaces / 接口](docs/interfaces.md) and [Workspace contract / 工作区契约](docs/workspace-contract.md)
- [Vendored MyGo document groups / MyGo 文档组补丁](docs/mygo-document-groups.md)
- [Portrait design boards / Portrait 设计画板](docs/design/Mark%20Go%20Go%20界面设计.html)
