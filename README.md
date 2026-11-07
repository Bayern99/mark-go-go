# Mark Go Go

[English](#english) | [中文](#chinese)

---

<a name="english"></a>
## English

**Mark Go Go** is a local-first native macOS Markdown editor and AI reader prototype. It combines a native Go host (via MyGo) with a dual-engine editor (Milkdown Crepe WYSIWYG & CodeMirror 6 plain text), a block-preserving composition architecture that keeps raw Markdown bytes intact on save, and an agent-driven generative UI reading surface (GenUI) built on A2UI 1.0.

### Key Features

- **Block-Preserving Markdown Composition**: Saves documents by recomposing individual ProseMirror block nodes. Unedited blocks retain their exact source bytes, eliminating format drift and preserving custom line endings (CRLF/LF) and whitespace.
- **Dual Editing Modes**: Seamlessly toggle between distraction-free Rich Text (WYSIWYG) and exact Source Mode (CodeMirror 6) with `⌘/`.
- **Portrait Three-Column Layout**: Left workspace navigation, center content-only canvas, and right-column document panels (`Outline` and `GenUI`). Responsive `< 1180px` auto-collapsing overlay drawer (`.r-float`).
- **GenUI Derived Reader (A2UI 1.0)**: AI-synthesized interactive reading views utilizing 10 native catalog components (`Column`, `Row`, `Tabs`, `Card`, `Text`, `KeyPoints`, `MetricGrid`, `ComparisonTable`, `Timeline`, `Checklist`).
- **Sandboxed ACP Agent Generation**: Dispatches generation runs to external AI agents (e.g., `agy acp`) inside isolated temporary directories over the Agent Client Protocol (ACP), with model caching via an upward `ModelMenu`, partial component regeneration, and one-step undo.
- **In-Place Paper Composer**: Replaces modal popup dialogs with non-destructive in-place paper composers (`.composer`) for both Rich text notes and GenUI card notes (`<!-- note: ... -->`).

### Documentation

- 🏛️ **Architecture**: [`ARCHITECTURE.md`](ARCHITECTURE.md)
- 📜 **Changelog**: [`CHANGELOG.md`](CHANGELOG.md)
- 🔌 **Interfaces & System Contracts**: [`docs/interfaces.md`](docs/interfaces.md)
- 📋 **Acceptance Test Suite**: [`docs/acceptance.md`](docs/acceptance.md)
- 🎨 **Design Artifacts & Prototypes**: [`docs/design/Mark Go Go 界面设计.html`](docs/design/Mark%20Go%20Go%20界面设计.html)
- 📐 **Architecture Decision Records (ADRs)**:
  - [`0001-architecture-baseline.md`](docs/adr/0001-architecture-baseline.md): Foundational architecture decisions
  - [`0002-genui-derived-center-view.md`](docs/adr/0002-genui-derived-center-view.md): GenUI as a derived center view on A2UI 1.0
  - [`0003-acp-agent-generation.md`](docs/adr/0003-acp-agent-generation.md): External Agent invocation over ACP
  - [`0004-three-column-layout-and-genui-panel.md`](docs/adr/0004-three-column-layout-and-genui-panel.md): Three-column layout and right-pane GenUI management

### Development

```sh
# Install dependencies
npm --prefix web install
npx --prefix web playwright install chromium-headless-shell   # For vitest browser tests

# Launch desktop app in development mode (Vite :5173 + native shell)
go tool mygo dev

# Regenerate Go <-> TypeScript RPC bridge code
go tool mygo generate
```

Standalone web preview (in-memory mock host, sample document):
```sh
npm --prefix web run dev
```

### Verification & Packaging

```sh
# Automated quality checks
go vet ./...
go test -race ./...
npm --prefix web run typecheck
npm --prefix web test

# Build macOS app bundle and DMG installer
go tool mygo build
# Output artifacts:
#   build/darwin-arm64/Mark Go Go.app (12.9 MB)
#   build/darwin-arm64/mark-go-go-0.1.0-arm64.dmg (4.2 MB)
```

---

<a name="chinese"></a>
## 中文

**Mark Go Go** 是一款本地优先的 macOS 原生 Markdown 编辑器与 AI 派生阅读器原型。结合 Go 原生宿主（MyGo）与双引擎编辑器（Milkdown Crepe 所见即所得富文本 + CodeMirror 6 逐字节源码模式），保存时采用逐块保真组装算法保留未修改块的原始字符，并提供基于 A2UI 1.0 协议由外部 AI Agent 驱动的组件化阅读视图（GenUI）。

### 核心特性

- **逐块保真组装保存**：将 Markdown 拆解为 ProseMirror 独立块节点，保存时仅重新序列化被修改过的块，未改动的段落逐字节保留原文（含空格、空行与换行风格 CRLF/LF），彻底告别排版格式跑飞。
- **所见即所得与源码双模式**：按 `⌘/` 瞬间在富文本（WYSIWYG）与高精度源码模式（CodeMirror 6）之间无损切换。
- **Portrait 极简三栏工作区**：左栏工作区文件导航，中央纯净内容画布，右栏专属文档面板（实时大纲 `Outline` 与 `GenUI` 管理面板）。支持 `< 1180px` 窄窗自动收起为右侧浮层（`.r-float`）。
- **派生 GenUI 阅读视图（A2UI 1.0）**：提供 5 种基础布局组件与 5 种高级卡片组件（要点卡片、指标网格、对比表格、时间线、只读核对清单），带来源章节追踪（`From`）、“原文已改”高亮检测与一键跳转定位。
- **沙箱化 ACP Agent 执行器**：遵循 Agent Client Protocol (ACP) 标准调用外部 Agent（如 `agy acp`），在临时沙箱目录内安全生成，支持向上弹出式模型菜单（`ModelMenu`）、局部重生成与单步撤回。
- **原地白纸批注 Composer**：全面取消防碍视线的居中模态弹窗，正文批注与卡片批注均在目标位置原地展开自适应白纸卡片（`.composer`），以 `<!-- note: ... -->` 形式无损写回原文。

### 项目文档

- 🏛️ **系统架构**：[`ARCHITECTURE.md`](ARCHITECTURE.md)
- 📜 **更新日志**：[`CHANGELOG.md`](CHANGELOG.md)
- 🔌 **功能与系统接口规范**：[`docs/interfaces.md`](docs/interfaces.md)
- 📋 **自动化与人工验收记录**：[`docs/acceptance.md`](docs/acceptance.md)
- 🎨 **设计画板与原型预览**：[`docs/design/Mark Go Go 界面设计.html`](docs/design/Mark%20Go%20Go%20界面设计.html)
- 📐 **架构决策记录 (ADR)**：
  - [`0001-architecture-baseline.md`](docs/adr/0001-architecture-baseline.md)：初始架构基线与设计原则
  - [`0002-genui-derived-center-view.md`](docs/adr/0002-genui-derived-center-view.md)：GenUI 作为中央派生只读视图与 A2UI 1.0 标准
  - [`0003-acp-agent-generation.md`](docs/adr/0003-acp-agent-generation.md)：经 ACP 协议调用外部 Agent 生成视图
  - [`0004-three-column-layout-and-genui-panel.md`](docs/adr/0004-three-column-layout-and-genui-panel.md)：三栏职责分工与右栏 GenUI 管理面板

### 开发指南

```sh
# 安装依赖
npm --prefix web install
npx --prefix web playwright install chromium-headless-shell   # 首次运行测试需安装

# 启动本地桌面应用开发环境 (Vite :5173 + Go 桌面窗口)
go tool mygo dev

# 重新生成 Go <-> TypeScript IPC 桥接代码
go tool mygo generate
```

纯浏览器端调试（使用内存假宿主与示例文件）：
```sh
npm --prefix web run dev
```

### 质量检查与打包

```sh
# 自动化测试与质量检查
go vet ./...
go test -race ./...
npm --prefix web run typecheck
npm --prefix web test

# 构建 macOS 原生应用与 DMG 安装包
go tool mygo build
# 输出产物：
#   build/darwin-arm64/Mark Go Go.app (12.9 MB)
#   build/darwin-arm64/mark-go-go-0.1.0-arm64.dmg (4.2 MB)
```
