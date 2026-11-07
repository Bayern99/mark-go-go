# Changelog / 更新日志

All notable changes to **Mark Go Go** are documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

本项目所有重要变更均记录在此文件中。格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，并遵守 [语义化版本 2.0.0](https://semver.org/lang/zh-CN/)。

---

## [0.1.0] - 2026-11-07

### English

#### 🎨 Portrait Three-Column Layout & Design Language
- **Three-Column Separation of Concerns (ADR 0004)**:
  - **Left Column**: Workspace document navigation.
  - **Center Column**: Dedicated solely to clean document content (Rich text, Markdown, or GenUI view). Completely removes cluttered generation forms from the reading surface.
  - **Right Column**: Document-scoped panels housing real-time `Outline` and `GenUI Management Panel` via top segmented switcher.
- **Narrow-Window Responsive Behavior**: Windows `< 1180px` automatically collapse the right column into an overlay floating drawer (`.r-float`), automatically dismissed when clicking back into center content.
- **Portrait Tactile Design Tokens**:
  - Semantic cold-grey chrome (`--chrome`), white paper sheets (`--paper`), code backgrounds (`--code`), and ink contrast tiers (`--ink` to `--ink-4`).
  - Tactile physical shadows: `--raised-shadow`, `--row-shadow`, `--sunken-shadow`, `--sheet-shadow`, `--pop-shadow`.
  - Full automatic Light and Dark mode styling with strict status color semantics (`--warn` exclusively for `Source changed`, `--danger` for errors and conflicts).

#### ✍️ Block-Preserving Markdown Editor
- **Dual-Engine Editing**:
  - **Rich Text (WYSIWYG)**: Modern distraction-free editing powered by Milkdown 7.23 and `CrepeBuilder`.
  - **Source Mode**: Exact plain-text editing powered by CodeMirror 6 with instant `⌘/` toggling.
- **Block-Preserving Composition Engine**:
  - Saves Markdown by recomposing individual ProseMirror block nodes. Unedited blocks retain their exact raw source bytes.
  - Guarantees zero unintentional formatting churn (preserves custom line breaks, list indentation, and trailing spaces).
  - Preserves complex syntax (YAML frontmatter, GitHub alerts, HTML comments, footnotes) verbatim using dedicated locked block wrappers.

#### 📑 Outline Navigation
- **Real-Time Heading Tree**: Extracts ATX and setext headings using a shared AST parser.
- **Bi-Directional Cursor Tracking**: Clicking an outline item navigates the editor and shifts focus; moving the editor cursor dynamically highlights the corresponding outline heading.
- **Cross-Mode Jumping**: Clicking an outline item in GenUI view mode smoothly switches back to Rich text mode and scrolls to the target section.

#### 🤖 Derived GenUI Reading View (A2UI 1.0)
- **Componentized AI Reader (ADR 0002)**: Renders structured, interactive reading views compiled from companion `.<name>.md.genui.json` view files following the A2UI 1.0 standard.
- **Comprehensive Component Catalog**:
  - 5 Layout Primitives: `Column`, `Row`, `Tabs`, `Card`, `Text`.
  - 5 High-Density Presentation Components: `KeyPoints`, `MetricGrid`, `ComparisonTable`, `Timeline`, `Checklist` (read-only).
- **Source Tracking & Navigation (ADR 0002)**:
  - Each component displays a source reference capsule (`From <heading>`).
  - Detects out-of-date sections (`Source changed`) via normalized `fnv1a32` hashes.
  - Clicking `Edit in source` jumps directly to the source text in Rich text mode and preserves scroll position upon returning to GenUI.

#### ⚡ ACP Agent Generation & ModelMenu
- **Native ACP Agent Runner (ADR 0003)**:
  - Communicates directly with external AI agents (e.g. `agy acp`) using the Agent Client Protocol (ACP).
  - Sandboxed inside an isolated temporary directory containing only a read-only copy of the source document (`source.md`) and view file (`view.json`).
  - Enforces a 5-minute timeout and automatic process tree termination upon app quit.
- **Handshake Model Caching & ModelMenu**:
  - Caches available model lists advertised in ACP `session/new` `configOptions`.
  - Upward tactile popover menu (`.sel-btn` + `.mmenu.pop`) with `Agent’s choice` and checkmarks, dismissing on outside clicks or `Escape`.
- **Partial Regeneration & Undo**:
  - Generates only components marked with `Source changed`, preserving untouched layouts and components.
  - Maintains `.<name>.md.genui.prev.json` for one-step instantaneous undo.

#### 📝 In-Place Paper Composer
- **Elimination of Modal Dialogs**:
  - Replaced all centered `<dialog>` popups with in-place paper composers (`.composer`).
  - Auto-growing textareas with `⌘↩` / `Ctrl+↩` to save and `Escape` to cancel.
  - Notes are non-destructively appended to referenced sections as `<!-- note: ... -->` comments, without triggering false-positive `Source changed` marks.

---

### 中文

#### 🎨 Portrait 极简三栏工作区与设计语言
- **三栏职责分离架构（ADR 0004）**：
  - **左栏（工作区）**：仅承载工作区文档列表与筛选导航。
  - **中央栏（内容画布）**：严格专用于呈现文档内容（Rich text、Markdown、GenUI 视图），彻底剔除中央阅读区顶部的生成表单与按钮。
  - **右栏（当前文档）**：承载只属于当前文档的辅助面板，顶部通过凹陷分段条切换实时大纲（`Outline`）与 `GenUI` 管理面板。
- **窄窗自适应响应式机制**：窗口宽度 `< 1180px` 时右栏自动收起，手动触发时以右侧浮层（`.r-float`）形式展开，点击正文自动收回。
- **Portrait 触觉设计语义 Token**：
  - 引入冷灰窗框（`--chrome`）、白纸卡片（`--paper`）、代码底色（`--code`）以及 `--ink` 至 `--ink-4` 层次对比。
  - 5 级微拟物物理触觉阴影：`--raised-shadow`（凸起药丸按钮）、`--row-shadow`、`--sunken-shadow`（凹陷轨道）、`--sheet-shadow`、`--pop-shadow`。
  - 浅色/深色模式全自动适配，严格限定语义色用途（`--warn` 仅用于“原文已改”，`--danger` 仅用于冲突与错误）。

#### ✍️ 逐块保真的 Markdown 双模式编辑器
- **双引擎编辑**：
  - **所见即所得富文本（WYSIWYG）**：基于 Milkdown 7.23 与定制 `CrepeBuilder`，提供现代无干扰写作体验。
  - **源码模式**：基于 CodeMirror 6，支持精确字符偏移与 `⌘/` 瞬间无损切换。
- **逐块保真组装引擎（Block-Preserving Composition）**：
  - 将 Markdown 拆解为 ProseMirror 块节点，保存时仅重新序列化被编辑过的块，未改动的块保留原文件逐字节字符（包括行尾空格、空行与换行风格）。
  - 彻底解决传统 Markdown 编辑器保存时全篇排版格式变异丢失的问题。
  - 通过锁定块（Locked Blocks）机制，保真封包保护 YAML Frontmatter、GitHub Alert 引用、HTML 原始注释与脚注。

#### 📑 大纲导航与双向光标联动
- **实时标题树**：基于共享 AST 标题解析算法准确提取 ATX 与 setext 标题。
- **双向光标同步**：点击大纲项精准移动光标并定位视图，正文移动光标时大纲项实时同步高亮。
- **跨模式一键回切**：在 GenUI 视图中点击大纲项，自动切回富文本模式并定位到对应章节。

#### 🤖 派生 GenUI 阅读视图（A2UI 1.0）
- **组件化 AI 阅读界面（ADR 0002）**：渲染由隐藏伴随文件 `.<name>.md.genui.json` 存储的 A2UI 1.0 协议标准交互视图。
- **全套组件目录支持**：
  - 5 种基础布局组件：`Column`、`Row`、`Tabs`、`Card`、`Text`。
  - 5 种高级卡片组件：`KeyPoints` 要点卡片、`MetricGrid` 指标网格、`ComparisonTable` 对比表格、`Timeline` 时间线、`Checklist` 只读核对清单。
- **来源追踪与跳转定位**：
  - 每个组件底座配备来源胶囊（`From <标题>`），通过 `fnv1a32` 正文哈希实时追踪“原文已改（`Source changed`）”。
  - 点击“去原文修改（`Edit in source`）”一键切到富文本并选中来源段落，改完返回自动恢复原滚动位置。

#### ⚡ 外部 Agent (ACP) 生成与模型菜单
- **原生 ACP 执行器（ADR 0003）**：
  - 遵循 Agent Client Protocol (ACP) 标准调用本机外部 Agent（如 `agy acp`）。
  - 在完全隔离的独立临时目录运行，只提供原文副本与视图文件，防止误伤工作区其他文件。
  - 具备 5 分钟超时保护与应用退出时自动杀死子进程树机制。
- **握手模型缓存与 ModelMenu 向上弹出菜单**：
  - 自动捕获并持久化每次 ACP `session/new` 握手所返回的可用模型列表。
  - 26px 凸起药丸按钮结合向上展开的白纸弹出菜单（`.mmenu.pop`），支持 `Agent’s choice`、勾选高亮、外部点击与 `Escape` 关闭。
- **局部重生成与单步撤回**：
  - 仅重新生成标记为“原文已改”的组件，保持其余结构与阅读记忆不变。
  - 自动生成 `.<name>.md.genui.prev.json` 备份文件，支持随时一键撤回。

#### 📝 原地白纸批注 Composer
- **告别居中弹窗**：
  - 移除所有居中模态 `<dialog>` 弹窗，正文批注与 GenUI 卡片批注均在目标落位处原地展开白纸输入卡片（`.composer`）。
  - 输入框高度自适应，支持 `⌘↩` / `Ctrl+↩` 快速保存与 `Esc` 取消。
  - 批注以 `<!-- note: 文字 -->` 形式无损写入原文段落末尾，且计算章节哈希时自动排除批注，绝不误触“原文已改”状态。

#### 🛡️ 稳定性与全量自动化测试
- **完整的自动化测试矩阵**：
  - 全量 Go 自动化测试与 race 竞争检查通过；
  - 17 个前端测试套件、189 项 Vitest 单元与集成测试全部通过；
  - 涵盖源码往返保真、大纲光标联动、三栏浮层自适应、A2UI 组件目录渲染、ACP 假 Agent 模拟生成与批注无损写入。
