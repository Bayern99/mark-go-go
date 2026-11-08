# Changelog / 更新日志

All notable changes to **Mark Go Go** are documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

本项目所有重要变更均记录在此文件中。格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，并遵守 [语义化版本 2.0.0](https://semver.org/lang/zh-CN/)。

---

## [Unreleased]

Spec #20 (`integration/spec-20-workspace-portrait`, tickets #21–#31) is implemented and consolidated here. Automated full checks pass at `7b90f1ed2cfa501cde43c3b94e96c0b9348e8847`; packaged-app acceptance is still being exercised, so this entry describes implemented behaviour, not a completed release. Known observed native failures on that source — a normal quit zeroing two unedited files, and restored tabs rendering without an active editor until a tab is clicked — are recorded as FAIL in `docs/acceptance.md`. The final native candidate `47e3b4a` (on top of `6e7e395`) fixes both: a flush delivers the text on screen (`editor.getMarkdown()`) or, without a mounted editor, the core document state, never a change-only buffer; and restore maps the stored active tab to the MyGo id the document really has now. Targeted regression went from 2 failing to 11 passing, with `tsc` and the Go restore tests passing. Packaged-app re-verification is still pending, so those historical FAIL rows stay FAIL until that run is observed.

### English

#### 🗂 Workspace Windows, Document Tabs and Navigation (#21, #22, #23, ADR 0005)
- **One folder, one window, several document tabs**: a workspace window hosts every opened document as a tab. `documentId` stays stable across path, tab, window, first save and Save As; `workspaceId`, `root` and `path` are separate identities.
- **One editing session per file**: reopening the same file from Finder, the File menu, the file tree or a heading search focuses the existing tab instead of creating a second unsaved copy. Directory aliases (`/tmp` vs `/private/tmp`) reuse one workspace after symlink-prefix canonicalisation.
- **Routing**: an already open document wins; otherwise the deepest open workspace whose root contains the file; otherwise a new window at the file's parent folder. First save and Save As keep the document identity and move only that tab into the destination workspace; saving onto another open document is refused.
- **Last-tab behaviour**: closing every tab keeps a folder workspace window (navigation stays usable) and closes an empty untitled window. `⌘W` closes the tab, the window close button asks every dirty document.
- **Navigation**: left-pane file tree ordered depth-first (a folder's descendants directly after it), honest scan/index status (`Indexing n of m files`), heading/body search that also sees unsaved text, draggable and keyboard-resizable splitter, breadcrumb path bar, Welcome with Open Folder / New Document / recent workspaces.
- **Session restore**: `session.json` stores workspaces, tab order, active document and each tab's view mode; quit flushes every window's last editor text before the layout is written. Unsaved bytes remain MyGo recovery copies. Missing roots or files restore partially with an explicit error; closed tabs and workspaces do not come back; cancelled Agent jobs are never replayed.

#### 🪟 Per-Document GenUI Jobs and Settings (#24, #27, #29)
- **One generation slot per document**: documents generate concurrently without blocking each other; settings, text, path and request are captured when a run starts, and a finishing run refuses to write if its document closed or its path changed. Switching tabs never cancels a run; closing that document does.
- **Watch ordering**: the first view snapshot is delivered before the directory watch is set up, a catch-up snapshot follows once it is running, and a watch that cannot start still reports its error. Entering GenUI shows a loading state instead of a blank centre while the view file is being read.
- **Reading state**: KeyPoints fold, ComparisonTable sort and horizontal scroll, and Tabs selection live in a per-document store, survive leaving GenUI and replacing one card, persist the saved tab only once after the first render, and never write the document.
- **One process-wide settings store**: `SetSettings` is an atomic intent patch that writes only the fields present and returns the committed value; a successful write broadcasts `settingsChanged` to every window. Idle panels in any document show the current global model without a remount, Model and Locate-agent write only their own field, and they can no longer revert the other setting or the handshake's cached model list. A run keeps the settings it captured.
- **Partial sources (#28)**: a card that lost some sources keeps `Edit in source` and `Note` on its first surviving source; only when every source is gone are they disabled.

#### 🎨 Portrait Visual Language, Special Blocks and Reading Components (#25, #26, #27, #31, ADR 0006)
- **Light-only Portrait with offline fonts**: application status colours stay separate from document content semantics (Warning / Caution carry their own tokens plus text and icon). IBMPlexSans (incl. SC), Barlow Semi Condensed, IBM Plex Mono and the icon font are bundled `.woff2` files loaded without a network request; there is no appearance switch and no `prefers-color-scheme` handling this round.
- **Special blocks render and edit in place**: frontmatter becomes a key/value table, GitHub alerts a typed and icon-marked callout, footnote definitions a numbered entry with a back-reference, meta code fences a titled and highlighted block, math a source block. `Edit` opens the same in-place `.composer` paper sheet with the block's raw Markdown; save is one undoable transaction, cancel writes nothing, only the edited block's bytes change, and raw HTML / unsupported syntax stay visible as `Kept as written` without executing HTML.
- **ComparisonTable is keyboard sortable**: column headers are real buttons with `aria-sort`; sorting cycles ascending → descending → original order and never calls `host.update`.

#### ⌨️ Native Edit Menu (#30)
- Edit > Undo / Redo act on the captured document's real editor history (ProseMirror or CodeMirror) instead of the empty WebKit undo manager, and `SetNativeEditState` reports availability for the focused window's active document only. A focused in-place Composer textarea keeps native field undo.

#### 🧪 Final correction pass
- Browser `setViewMode` no longer echoes document state, so a view-mode write cannot unlock a read-only document through a stale `readOnly`.
- A saved GenUI tab index is restored once after the first render instead of on every mutation.
- Default workspace overrides now stat the directory, so an omitted override cannot hide a missing workspace folder.

#### Supersession notes
- This entry supersedes the `0.1.0` claim of "full automatic Light and Dark mode styling": only GenUI ever had dark tokens, and ADR 0006 settles this round on a light-only interface. The `0.1.0` entry is kept as written.
- The `0.1.0` single-document-per-window description is superseded by ADR 0005 and the workspace window model above.

### 中文

规格 #20（`integration/spec-20-workspace-portrait`，任务 #21–#31）已实现并在此汇总。自动化全量检查在 `7b90f1ed2cfa501cde43c3b94e96c0b9348e8847` 通过；打包应用的验收仍在进行，本条描述已实现的行为，不代表已发布。该源码上已观察到两项原生失败：正常退出把两份未编辑文件写成 0 字节，恢复后标签存在但需点击才出现活动编辑器；它们在 `docs/acceptance.md` 中标为 FAIL。最终原生候选 `47e3b4a`（基于 `6e7e395`）修复了这两项：flush 交付屏幕上的文本（`editor.getMarkdown()`），没有挂载编辑器时回落到核心文档状态，绝不再交付只记录变更的缓冲；恢复把存储的 active tab 映射到文档现在真实持有的 MyGo id。目标回归由 2 项失败转为 11 项通过，`tsc` 与 Go 恢复测试通过。实际新包复验尚未完成，因此历史 FAIL 行保持 FAIL，直到观察到该复验结果。

#### 🗂 工作区窗口、文档标签与导航（#21、#22、#23，ADR 0005）
- **一个文件夹一个窗口、多个文档标签**：工作区窗口以标签承载打开的原文。`documentId` 跨路径、标签、窗口、首次保存与另存为保持稳定；`workspaceId`、`root`、`path` 各自独立。
- **同一文件只有一个编辑会话**：从 Finder、File 菜单、文件树或章节搜索结果重复打开只定位已有标签。目录别名（`/tmp` 与 `/private/tmp`）在规范化符号链接前缀后复用同一个工作区。
- **路由**：已打开的文档优先；否则进入根目录最深且包含该文件的已打开工作区；再否则在文件父目录新建窗口。首次保存与另存为保留文档身份，只把该标签迁入目标工作区；目标已是另一份打开文档时拒绝保存。
- **最后一个标签**：文件夹工作区关掉全部标签仍保留窗口（导航可用），未命名窗口则关闭。`⌘W` 关闭标签，窗口关闭按钮逐个询问脏文档。
- **导航**：左栏文件树按深度优先排列（文件夹的子孙紧跟其后）、如实显示扫描/索引进度（`Indexing n of m files`）、标题与正文搜索并包含未保存文本、可拖拽也可用键盘调整的分栏、面包屑路径栏、含 Open Folder / New Document / 最近工作区的 Welcome。
- **会话恢复**：`session.json` 保存工作区、标签顺序、活动文档与每个标签的视图模式；退出时先送完每个窗口的最后编辑再写布局。未保存字节仍是 MyGo 恢复副本。缺失的根或文件部分恢复并给出明确错误；已关闭的标签与工作区不复活；取消的 Agent 任务不重放。

#### 🪟 每文档 GenUI 任务与设置（#24、#27、#29）
- **每份文档一个生成槽**：不同文档可并发生成、互不阻塞；设置、文本、路径与生成要求在开始时捕获，结束时若文档已关闭或路径已变则拒绝写入。切换标签不取消运行，关闭该文档才取消。
- **监听顺序**：第一份视图快照在目录监听建立前送出，监听建立后再送一份追赶快照，监听无法启动时仍返回错误。进入 GenUI 时若视图文件尚未可读，中央显示加载状态而不是空白。
- **阅读状态**：KeyPoints 折叠、ComparisonTable 排序与横向滚动、Tabs 选中页保存在每文档的存储里，离开 GenUI 或替换其中一张卡片后仍保留；保存的选中页只在首次渲染后恢复一次；从不写文档。
- **进程级唯一设置存储**：`SetSettings` 是原子意图补丁，只写出现的字段并返回提交后的值；成功写入向每个窗口广播 `settingsChanged`。任何文档的空闲面板无需重新挂载就显示当前全局模型；Model 与 Locate agent 只写各自字段，不再回退另一个设置或握手表里的模型列表。运行中的任务保留开始时捕获的设置。
- **部分来源丢失（#28）**：只剩部分来源的卡片仍在第一个仍存在的来源上保留 `Edit in source` 与 `Note`；全部来源失效时才禁用。

#### 🎨 Portrait 视觉语言、特殊块与阅读组件（#25、#26、#27、#31，ADR 0006）
- **仅浅色的 Portrait 与离线字体**：应用状态色与原文内容语义色分开（Warning / Caution 有独立 Token 并配文字与图标）。IBMPlexSans（含 SC）、Barlow Semi Condensed、IBM Plex Mono 与图标字体都是仓库内 `.woff2`，不请求网络；本轮没有外观切换，也不读取 `prefers-color-scheme`。
- **特殊块可视呈现并原地编辑**：Frontmatter 变为键值表，GitHub alert 变为带类型文字与图标的提示块，脚注定义显示编号与返回引用，带 meta 的代码块显示语言与高亮，数学块显示源码。`Edit` 在同一位置展开白纸 `.composer`，内容是该块的原始 Markdown；保存是一次可撤销的普通事务，取消不写入，只有被编辑块的字节改变；原始 HTML 与不支持的语法仍以 `Kept as written` 显示且不执行 HTML。
- **ComparisonTable 可键盘排序**：列头是真正的按钮并带 `aria-sort`；排序按升序 → 降序 → 原顺序循环，从不调用 `host.update`。

#### ⌨️ 原生 Edit 菜单（#30）
- Edit > Undo / Redo 作用于点击时捕获文档的真实编辑器历史（ProseMirror 或 CodeMirror），而不是空的 WebKit undo manager；`SetNativeEditState` 只为聚焦窗口的活动文档报告可用性。聚焦的原地 Composer `textarea` 保留原生字段撤销。

#### 🧪 最终修正
- 浏览器端 `setViewMode` 不再回显文档状态，视图模式写入不能经旧 `readOnly` 解锁只读文档。
- 保存的 GenUI 标签页只在首次渲染后恢复一次，不再在每次变更时覆盖用户点击。
- 默认工作区覆盖会真正 stat 目录，省略覆盖时不能掩盖缺失的工作区文件夹。

#### 取代说明
- 本条目取代 `0.1.0` 中“浅色/深色模式全自动适配”的说法：只有 GenUI 曾有过深色 Token，ADR 0006 把本轮确定为仅浅色。`0.1.0` 条目原文保留。
- `0.1.0` 中“一份原文一个窗口”的描述已被 ADR 0005 与上面的工作区窗口模型取代。

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
