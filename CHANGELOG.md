# Changelog / 更新日志

All notable changes to **Mark Go Go** are documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

本项目所有重要变更均记录在此文件中。格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，并遵守 [语义化版本 2.0.0](https://semver.org/lang/zh-CN/)。

---

## [0.4.0] - 2026-11-17

- 富文本中的普通表格可以拖拽调整列宽。支持撤销和重做，重开文档或应用后恢复已调整的列宽。列宽保存在本地，Markdown 原文保持原样。
- 宽窗口为表格提供更多空间，正文保留 624 px 的最大阅读宽度。表格的横向滚动滑块可视厚度为 4 px。
- 修复表格列边界在 macOS 上误触发单元格选区的问题，并为引用块表格保留原有内容。
- 源码和 Portrait 界面以 MIT 许可证公开。README 和致谢提供独立的中文、英文页面，安装包随附应用与第三方许可声明。

## [0.3.0] - 2026-11-14

Editor typeset and reading-control rounds on the Milkdown Crepe/ProseMirror editor, keeping the Go/WebView architecture, font system and byte-preserving Markdown model unchanged. Audit, measurements and regression evidence live in [docs/editor-typeset/](docs/editor-typeset/).

### English

#### Editing reliability

- Undo and structural node recreation now restore the document's original Markdown bytes instead of reserialized output; paste, table and heading edits keep byte fidelity.
- Table caret, toolbar and table-control behaviors refined; deferred Crepe cell selections can no longer overwrite newer selections.

#### Editorial rhythm

- `text-wrap: pretty` for prose (table cells excluded), `balance` for H1–H3, and H4–H6 size/color differentiation on the heading ladder — verified zero widows on WebKit across English, Chinese, bilingual and long LLM fixtures.

#### Long-document reading control

- Section folding by heading: a restrained chevron plus `Cmd/Ctrl-Alt-←/→`; the folded range ends at the next same-or-higher-level heading; nested folds keep independent state.
- Folding is display-only: Markdown bytes, undo history and the dirty flag are untouched; folding the section containing the caret moves the caret to the heading end, and outline/search navigation expands folds covering the target.
- Whole code-block collapse to a `› code · N lines` stub via a chevron in the block's own tools row — same nodeview, no CodeMirror rebuild, no height jump.
- Cross-cell drag now creates a real `CellSelection` with `.selectedCell` highlights and proper `data-pm-slice` table slices on copy, while single clicks, intra-cell drags and row/column controls are unchanged.
- Fold state persists per document path with content-signature identity: reordered duplicate headings reattach correctly, and replaced, deleted or ambiguous sections expand rather than misfold. Verified across a full app quit and relaunch.
- The selection toolbar's top placement was kept after measuring a side-margin alternative (occlusion 18px vs ~6–10× pointer travel and insufficient real margin).

### 中文

编辑器排版与长文阅读控制三轮精修，基于 Milkdown Crepe/ProseMirror，保持 Go/WebView 架构、字体系统与字节保真的 Markdown 数据模型不变。审计、测量与回归证据见 [docs/editor-typeset/](docs/editor-typeset/)。

#### 编辑可靠性

- 撤销与结构性节点重建现在恢复文档的原始 Markdown 字节，而非重新序列化结果；粘贴、表格与标题编辑保持字节保真。
- 表格光标、工具栏与表格控件细节修正；延迟的单元格选择不再覆盖新选区。

#### 排版节奏

- 正文启用 `text-wrap: pretty`（表格单元格除外），H1–H3 使用 `balance`，H4–H6 按标题阶梯区分字号与颜色——WebKit 下英文、中文、混排与长文夹具实测零孤行。

#### 长文阅读控制

- 按标题折叠章节：标题行尾折叠箭头与 `Cmd/Ctrl-Alt-←/→` 键盘操作；折叠范围截止于下一个同级或更高级标题；嵌套折叠各自独立。
- 折叠只改变显示状态：Markdown 字节、撤销历史与修改标记均不受影响；折叠含光标的章节时光标移至标题末尾；大纲/搜索定位会先展开覆盖目标的折叠。
- 代码块整段收起为 `› code · N lines` 单行占位，通过块内工具行箭头操作——复用同一 nodeview，不重建 CodeMirror、无高度跳动。
- 跨单元格拖选现在产生真实的 `CellSelection` 与 `.selectedCell` 高亮，复制产出 `data-pm-slice` 表格切片；单击、格内拖选与行列控件行为不变。
- 折叠状态按文档路径持久化，以内容签名识别章节：重排的重名标题正确对应，被替换、删除或无法确定的章节展开而非错挂。已通过完全退出再重开的真机验证。
- 选区工具栏保留顶部方案——实测侧边方案虽无遮挡但指针移动距离增加约 6–10 倍，且实际可用边距不足。



Workspace and Portrait changes for #20. See the [acceptance log](docs/acceptance.md) for automated and native results and the open acceptance items.

### English

#### Window size, diagrams and workspace text

- Workspace windows now start at 1280×820 instead of 940×720, above the three-column layout breakpoint, without requesting fullscreen or maximization.
- Rich text renders Mermaid fences locally as byte-preserving special blocks. In-place Markdown editing, cancel, undo and read-only behavior use the existing editor path; invalid diagrams retain visible source and an error.
- Document tabs use bundled file/close icons, tactile active states and an explicit `+N` overflow count. The command-style document menu includes a heading, quiet search, file icons, paths and selection checks. The file tree uses real folder/file icons and parent-folder context; Model menus include model icons and a section heading. Deep rows keep bounded indentation and progressive metadata reduction.
- The sidebar yields on narrow windows, rendering at most `window width − 310px` down to a 140px floor, so the document sheet stays usable at the 480px minimum window; widening the window restores the user's sidebar width. Inside the sheet, container queries drop the docbar's save status and version chip below 380px and the path crumbs below 300px. The sidebar splitter keeps a 14px pointer/touch target inside its 6px visual slot.
- Recorded the bounded [UI component gaps and typography plan](docs/design/ui-component-gaps.md), including the desktop/browser excerpt mismatch and the future GenUI Diagram contract. Added only Radix Tooltip/DropdownMenu primitives and Sonner copy feedback; no full UI suite or Catalog expansion is included.


#### GenUI generation and architecture evidence

- ACP initialization reports the client identity and checks the negotiated protocol. Generation prompts name canonical absolute input/output paths, avoiding AGY's observed interpretation of `./source.md` as `/source.md`.
- **ACP process reuse**: the app keeps at most one idle agent process under an exclusive per-run lease, with a two-minute idle timeout. Busy documents use independent transient processes, and only a process whose fresh session was closed and acknowledged returns to idle. Each run still gets its own session, working directory and source snapshots; executable/environment changes or any uncertain state discard the process without retry. Measured first sessions on three new processes were 3.16–3.51 s versus 0.25–0.70 s for fresh sessions on one warm process, and `session/close` was acknowledged in 0.18–0.20 ms.
- **Cancellation no longer waits for process exit**: an acknowledged `session/cancel` completes the run on the prompt turn's answer, with a one-second bound before a stuck process is killed. Private run directories are removed for every result; the process directory lasts until the cache closes at teardown. A tool-call scope violation still discards the process.
- **Terminology**: the generation content rule keeps the source's technical terms as written even in translation, without a dictionary or post-processing.
- Added opt-in, `readbench`-tagged native tracing of actual agent processes, ACP sessions/configuration, raw artifacts and commit results. The independent ReadBench runner records real WebView observations and source-grounded answer reviews; model benchmarks are explicit runs, separate from fast deterministic checks.
- Documented the actual UAX #31 component-id constraint in the generation format and corrected historical source-id coverage claims. ADR 0002/0003 distinguish the default ACP generator from replaceable evidence selectors and inference formats.
- Added a build-tagged, app-owned controlled GenUI experiment: real Jev component/structure decisions, Gemini content-only generation over the same ACP path, and deterministic A2UI assembly. Twelve native runs and source reviews are recorded in the [decision report](docs/research/readbench/controlled-genui-decision.md); production generation remains unchanged.

#### Document reliability

- Canceling a window close keeps every tab and its recovery copy, including a tab whose Delete or Don't Save button was already clicked.
- Quit waits for each window to flush its editor and for the session to be saved. If either step fails, the app stays open and offers a retry through Quit.
- The editor catches file reloads during startup. Canceling GenUI generation completes even when a descendant of the ACP agent holds its output pipes open.
- Fixed a macOS crash when AppKit validated a menu item while the WebView was focused.

#### Workspace Windows, Document Tabs and Navigation (#21, #22, #23, ADR 0005)
- **One folder, one window, several document tabs**: a workspace window hosts every opened document as a tab. `documentId` stays stable across path, tab, window, first save and Save As; `workspaceId`, `root` and `path` are separate identities.
- **One editing session per file**: reopening the same file from Finder, the File menu, the file tree or a heading search focuses the existing tab instead of creating a second unsaved copy. Directory aliases (`/tmp` vs `/private/tmp`) reuse one workspace after symlink-prefix canonicalisation.
- **Routing**: an already open document wins; otherwise the deepest open workspace whose root contains the file; otherwise a new window at the file's parent folder. First save and Save As keep the document identity and move only that tab into the destination workspace; saving onto another open document is refused.
- **Last-tab behaviour**: closing every tab keeps a folder workspace window (navigation stays usable) and closes an empty untitled window. `⌘W` closes the tab, the window close button asks every dirty document.
- **Navigation**: left-pane file tree ordered depth-first (a folder's descendants directly after it), honest scan/index status (`Indexing n of m files`), search by path, title, excerpt and heading, including unsaved changes, draggable and keyboard-resizable splitter, breadcrumb path bar, Welcome with Open Folder / New Document / recent workspaces.
- **Session restore**: `session.json` stores workspaces, tab order, active document and each tab's view mode; quit flushes every window's last editor text before the layout is written. Unsaved bytes remain MyGo recovery copies. Missing roots or files restore partially with an explicit error; closed tabs and workspaces do not come back; cancelled Agent jobs are never replayed.

#### Per-Document GenUI Jobs and Settings (#24, #27, #29)
- **One generation slot per document**: documents generate concurrently without blocking each other; settings, text, path and request are captured when a run starts, and a finishing run refuses to write if its document closed or its path changed. Switching tabs never cancels a run; closing that document does.
- **Watch ordering**: the first view snapshot is delivered before the directory watch is set up, a catch-up snapshot follows once it is running, and a watch that cannot start still reports its error. Entering GenUI shows a loading state instead of a blank centre while the view file is being read.
- **Reading state**: KeyPoints fold, ComparisonTable sort and horizontal scroll, and Tabs selection live in a per-document store, survive leaving GenUI and replacing one card, persist the saved tab only once after the first render, and never write the document.
- **One process-wide settings store**: `SetSettings` is an atomic intent patch that writes only the fields present and returns the committed value; a successful write broadcasts `settingsChanged` to every window. Idle panels in any document show the current global model without a remount, Model and Locate-agent write only their own field, and they can no longer revert the other setting or the handshake's cached model list. A run keeps the settings it captured.
- **Partial sources (#28)**: a card that lost some sources keeps `Edit in source` and `Note` on its first surviving source; only when every source is gone are they disabled.

#### Portrait Visual Language, Special Blocks and Reading Components (#25, #26, #27, #31, ADR 0006)
- **Light-only Portrait with offline fonts**: application status colours stay separate from document content semantics (Warning / Caution carry their own tokens plus text and icon). IBMPlexSans (incl. SC), Barlow Semi Condensed, IBM Plex Mono and the icon font are bundled `.woff2` files loaded without a network request; there is no appearance switch and no `prefers-color-scheme` handling this round.
- **Special blocks render and edit in place**: frontmatter becomes a key/value table, GitHub alerts a typed and icon-marked callout, footnote definitions a numbered entry with a back-reference, meta code fences a titled and highlighted block, math a source block. `Edit` opens the same in-place `.composer` paper sheet with the block's raw Markdown; save is one undoable transaction, cancel writes nothing, only the edited block's bytes change, and raw HTML / unsupported syntax stay visible as `Kept as written` without executing HTML.
- **ComparisonTable is keyboard sortable**: column headers are real buttons with `aria-sort`; sorting cycles ascending → descending → original order and never calls `host.update`.

#### Overlay Scrollbars
- **Shared native-scroll overlay**: `ScrollArea` uses OverlayScrollbars 2 for the file tree, Rich text, Markdown, central GenUI, Outline, the GenUI management panel and ComparisonTable. Scroll restoration, source navigation, editor selection, table keyboard focus and the pinned GenUI footer keep their existing behavior through the real viewport.
- **Quiet visual and interaction language**: a 4px rounded thumb inside a 10px hit area uses semantic ink tokens, appears on scrolling or pointer movement, supports direct dragging and fades after 600ms without changing content geometry. Reduced Motion follows the existing global override; third-party code and menu scrollers keep native ownership with thin semantic-color scrollbars.
- **Document tabs use an overflow menu**: the tab bar lives inside the workspace main region, keeps only measured tabs inside its own bounds, always retains the active tab, and moves the rest to a searchable menu. Tabs never scroll, use an overlay scrollbar instance, or render outside the tab row.

#### Native Edit Menu (#30)
- Edit > Undo / Redo act on the captured document's real editor history (ProseMirror or CodeMirror) instead of the empty WebKit undo manager, and `SetNativeEditState` reports availability for the focused window's active document only. A focused in-place Composer textarea keeps native field undo.

#### Final correction pass
- Browser `setViewMode` no longer echoes document state, so a view-mode write cannot unlock a read-only document through a stale `readOnly`.
- A saved GenUI tab index is restored once after the first render instead of on every mutation.
- Default workspace overrides now stat the directory, so an omitted override cannot hide a missing workspace folder.
- Native path changes are checked before any write: a generating document cannot Save As, Rename or Move To; `PathChange` refuses the change and the File menu reflects it via a main-thread menu refresh; `ReservePath` claims a canonical target app-wide until the file operation ends, so two windows cannot take one file. A target the app refuses is never written and never becomes a second session; the OS Save panel's own Replace confirmation during Save As / Rename selection may appear before that rejection and is not suppressed. A failed native command is reported as `DocumentInfo.actionError` in its window.
- Last input is written once per native command (`6478ceb`): `bindFlush` is the single raw-text reader, Save-family commands deliver the captured document once, `close` and `closeWorkspace` each own their flush, and an unmounted pane delivers nothing instead of a synthesized empty buffer.

### 中文

#20 的工作区与 Portrait 更新。自动化检查、原生验证和待验收项见[验收记录](docs/acceptance.md)。

#### 窗口尺寸、图表与工作区文字

- 工作区窗口默认尺寸由 940×720 调整为 1280×820，高于三栏布局断点，不请求全屏或最大化。
- Rich text 本地渲染 Mermaid 围栏，沿用保真特殊块、原地 Markdown 编辑、取消、撤销和只读规则；无效图保留可见原文与错误说明。
- 文档标签使用内置文件/关闭图标、拟物活动态和明确的 `+N` 溢出计数；文档命令菜单包含标题、安静搜索、文件图标、路径与选中勾号。文件树使用真实文件夹/文件图标和父目录上下文；Model 菜单包含模型图标与分组标题。深层行保留缩进边界和逐级元信息让位。
- 窄窗口下侧栏主动让位：渲染宽度不超过 `窗口宽 − 310px`（下限 140px），保证最小窗口 480px 时正文仍可用，窗口放宽后恢复用户设定宽度。文档头栏按 sheet 容器查询逐级让位：小于 380px 收起保存状态与版本号，小于 300px 收起路径面包屑。侧栏分隔条在 6px 视觉槽内保留 14px 触控热区。
- 记录有界的 [UI 组件缺口与文字规划](docs/design/ui-component-gaps.md)，包括桌面与浏览器摘要提取差异、未来 GenUI Diagram 的契约。仅引入 Radix Tooltip / DropdownMenu primitives 与 Sonner 复制反馈，不引入完整 UI 套件或扩充 Catalog。


#### GenUI 生成与架构证据

- ACP 初始化报告客户端身份并核对协商的协议版本；生成要求明确使用规范化后的绝对输入/输出路径，避免 AGY 把 `./source.md` 解释成 `/source.md`。
- **ACP 进程复用**：应用保留至多一个空闲 Agent 进程，每次运行独占租约，空闲两分钟过期；忙碌文档各自使用临时独立进程，只有新会话已关闭且关闭被确认的进程才回到空闲。每次运行仍有自己的会话、工作目录和来源快照；可执行文件或环境变化、任何不确定状态都丢弃进程且不重试。实测三个新进程的首次会话为 3.16–3.51 s，同一热进程上的全新会话为 0.25–0.70 s；`session/close` 约 0.18–0.20 ms 应答。
- **取消不再等待进程退出**：已应答的 `session/cancel` 在提示轮应答时结束本次运行，最多等 1 秒，之后有界杀死卡住的进程。私有运行目录无论结果都删除；进程目录保留到退出时关闭缓存。工具调用越界仍丢弃进程。
- **术语**：生成内容规则要求即使翻译也保留原文的技术术语，不引入词典，也不对生成结果做事后替换。
- 新增显式启用的 `readbench` 构建观测，记录真实 Agent 进程、ACP 会话及配置、原始产物和提交结果。独立 ReadBench 保存真实 WebView 行为和逐条原文审阅；完整模型基准与日常快速确定性检查分开运行。
- 新增仅在实验构建启用的应用内受控 GenUI：真实 Jev 决定组件与结构，同一 ACP/Gemini 仅填内容槽，由代码装配 A2UI。[决策报告](docs/research/readbench/controlled-genui-decision.md)记录 12 次原生运行与逐条原文审阅；生产默认生成方式不变。
- 格式说明补上实际 UAX #31 组件 ID 约束，研究入口更正历史来源 ID 覆盖率的解释；ADR 0002/0003 区分默认 ACP 生成执行器、可替换证据选择与推理格式。

#### 文档可靠性

- 取消关窗时保留所有标签及其恢复副本，即使其中一份文档已经选了 Delete 或 Don't Save。
- 退出前等待每个窗口交付最新编辑并写入会话文件。失败时保留窗口，可重新选择退出。
- 编辑器在启动期间也能接收文件重载。ACP Agent 的子进程继续占用输出管道时，宿主仍能完成 GenUI 取消。
- 修复 WebView 获得焦点时，AppKit 校验菜单项导致的 macOS 崩溃。

#### 工作区窗口、文档标签与导航（#21、#22、#23，ADR 0005）
- **一个文件夹一个窗口、多个文档标签**：工作区窗口以标签承载打开的原文。`documentId` 跨路径、标签、窗口、首次保存与另存为保持稳定；`workspaceId`、`root`、`path` 各自独立。
- **同一文件只有一个编辑会话**：从 Finder、File 菜单、文件树或章节搜索结果重复打开只定位已有标签。目录别名（`/tmp` 与 `/private/tmp`）在规范化符号链接前缀后复用同一个工作区。
- **路由**：已打开的文档优先；否则进入根目录最深且包含该文件的已打开工作区；再否则在文件父目录新建窗口。首次保存与另存为保留文档身份，只把该标签迁入目标工作区；目标已是另一份打开文档时拒绝保存。
- **最后一个标签**：文件夹工作区关掉全部标签仍保留窗口（导航可用），未命名窗口则关闭。`⌘W` 关闭标签，窗口关闭按钮逐个询问脏文档。
- **导航**：左栏文件树按深度优先排列（文件夹的子孙紧跟其后）、如实显示扫描/索引进度（`Indexing n of m files`）、搜索路径、标题、摘要和章节标题，包含未保存的改动、可拖拽也可用键盘调整的分栏、面包屑路径栏、含 Open Folder / New Document / 最近工作区的 Welcome。
- **会话恢复**：`session.json` 保存工作区、标签顺序、活动文档与每个标签的视图模式；退出时先送完每个窗口的最后编辑再写布局。未保存字节仍是 MyGo 恢复副本。缺失的根或文件部分恢复并给出明确错误；已关闭的标签与工作区不复活；取消的 Agent 任务不重放。

#### 每文档 GenUI 任务与设置（#24、#27、#29）
- **每份文档一个生成槽**：不同文档可并发生成、互不阻塞；设置、文本、路径与生成要求在开始时捕获，结束时若文档已关闭或路径已变则拒绝写入。切换标签不取消运行，关闭该文档才取消。
- **监听顺序**：第一份视图快照在目录监听建立前送出，监听建立后再送一份追赶快照，监听无法启动时仍返回错误。进入 GenUI 时若视图文件尚未可读，中央显示加载状态而不是空白。
- **阅读状态**：KeyPoints 折叠、ComparisonTable 排序与横向滚动、Tabs 选中页保存在每文档的存储里，离开 GenUI 或替换其中一张卡片后仍保留；保存的选中页只在首次渲染后恢复一次；从不写文档。
- **进程级唯一设置存储**：`SetSettings` 是原子意图补丁，只写出现的字段并返回提交后的值；成功写入向每个窗口广播 `settingsChanged`。任何文档的空闲面板无需重新挂载就显示当前全局模型；Model 与 Locate agent 只写各自字段，不再回退另一个设置或握手表里的模型列表。运行中的任务保留开始时捕获的设置。
- **部分来源丢失（#28）**：只剩部分来源的卡片仍在第一个仍存在的来源上保留 `Edit in source` 与 `Note`；全部来源失效时才禁用。

#### Portrait 视觉语言、特殊块与阅读组件（#25、#26、#27、#31，ADR 0006）
- **仅浅色的 Portrait 与离线字体**：应用状态色与原文内容语义色分开（Warning / Caution 有独立 Token 并配文字与图标）。IBMPlexSans（含 SC）、Barlow Semi Condensed、IBM Plex Mono 与图标字体都是仓库内 `.woff2`，不请求网络；本轮没有外观切换，也不读取 `prefers-color-scheme`。
- **特殊块可视呈现并原地编辑**：Frontmatter 变为键值表，GitHub alert 变为带类型文字与图标的提示块，脚注定义显示编号与返回引用，带 meta 的代码块显示语言与高亮，数学块显示源码。`Edit` 在同一位置展开白纸 `.composer`，内容是该块的原始 Markdown；保存是一次可撤销的普通事务，取消不写入，只有被编辑块的字节改变；原始 HTML 与不支持的语法仍以 `Kept as written` 显示且不执行 HTML。
- **ComparisonTable 可键盘排序**：列头是真正的按钮并带 `aria-sort`；排序按升序 → 降序 → 原顺序循环，从不调用 `host.update`。

#### Overlay Scrollbars 滚动条
- **统一保留原生滚动的浮层组件**：`ScrollArea` 基于 OverlayScrollbars 2，覆盖文件树、Rich text、Markdown、中央 GenUI、Outline、GenUI 管理面板与 ComparisonTable。滚动恢复、来源跳转、编辑器选择、表格键盘焦点和 GenUI 固定 footer 都通过真实 viewport 保持原行为。
- **克制的视觉与交互**：4px 圆角滑块、10px 命中区，颜色派生自语义墨色 Token；滚动或指针移动时显示，支持直接拖动，停止 600ms 后淡出，不改变内容几何尺寸。Reduced Motion 沿用全局覆盖；第三方代码与菜单滚动区保留原生所有权，使用细窄语义色滚动条。
- **文档标签改用溢出菜单**：标签带只位于工作区主区域，保留经测量后能放入自身边界内的标签，始终保留活动标签，其余进入可搜索菜单。标签不再滚动、不接入 Overlay 实例，也不会渲染到标签带之外。

#### 原生 Edit 菜单（#30）
- Edit > Undo / Redo 作用于点击时捕获文档的真实编辑器历史（ProseMirror 或 CodeMirror），而不是空的 WebKit undo manager；`SetNativeEditState` 只为聚焦窗口的活动文档报告可用性。聚焦的原地 Composer `textarea` 保留原生字段撤销。

#### 最终修正
- 浏览器端 `setViewMode` 不再回显文档状态，视图模式写入不能经旧 `readOnly` 解锁只读文档。
- 保存的 GenUI 标签页只在首次渲染后恢复一次，不再在每次变更时覆盖用户点击。
- 默认工作区覆盖会真正 stat 目录，省略覆盖时不能掩盖缺失的工作区文件夹。
- 原生路径变更在写入前检查：生成中的文档不能 Save As / Rename / Move To；`PathChange` 拒绝该变更，File 菜单经主线程菜单刷新反映状态；`ReservePath` 在全应用范围预留规范化目标直到文件操作结束，两个窗口不能取同一文件。应用拒绝的目标绝不写入，也不会产生第二个会话；OS Save 面板在 Save As / Rename 选择阶段自带的内建 Replace 确认可能先出现，且不在屏蔽范围内。失败的原生命令以 `DocumentInfo.actionError` 显示在该文档窗口。
- 每个原生命令只写回一次最后输入（`6478ceb`）：`bindFlush` 是唯一的原始文本读取器，Save 系列先交付一次捕获文档，`close` 与 `closeWorkspace` 各自负责自己的 flush，未挂载面板不交付任何内容，也不会合成空缓冲。

## [0.1.0] - 2026-11-07

### English

#### Portrait Three-Column Layout & Design Language
- **Three-Column Separation of Concerns (ADR 0004)**:
  - **Left Column**: Workspace document navigation.
  - **Center Column**: Dedicated solely to clean document content (Rich text, Markdown, or GenUI view). Completely removes cluttered generation forms from the reading surface.
  - **Right Column**: Document-scoped panels housing real-time `Outline` and `GenUI Management Panel` via top segmented switcher.
- **Narrow-Window Responsive Behavior**: Windows `< 1180px` automatically collapse the right column into an overlay floating drawer (`.r-float`), automatically dismissed when clicking back into center content.
- **Portrait Tactile Design Tokens**:
  - Semantic cold-grey chrome (`--chrome`), white paper sheets (`--paper`), code backgrounds (`--code`), and ink contrast tiers (`--ink` to `--ink-4`).
  - Tactile physical shadows: `--raised-shadow`, `--row-shadow`, `--sunken-shadow`, `--sheet-shadow`, `--pop-shadow`.
  - Light Portrait styling with dedicated status colours (`--warn` exclusively for `Source changed`, `--danger` for errors and conflicts).

#### Block-Preserving Markdown Editor
- **Dual-Engine Editing**:
  - **Rich Text (WYSIWYG)**: Modern distraction-free editing powered by Milkdown 7.23 and `CrepeBuilder`.
  - **Source Mode**: Exact plain-text editing powered by CodeMirror 6 with instant `⌘/` toggling.
- **Block-Preserving Composition Engine**:
  - Saves Markdown by recomposing individual ProseMirror block nodes. Unedited blocks retain their exact raw source bytes.
  - Guarantees zero unintentional formatting churn (preserves custom line breaks, list indentation, and trailing spaces).
  - Preserves complex syntax (YAML frontmatter, GitHub alerts, HTML comments, footnotes) verbatim using dedicated locked block wrappers.

#### Outline Navigation
- **Real-Time Heading Tree**: Extracts ATX and setext headings using a shared AST parser.
- **Bi-Directional Cursor Tracking**: Clicking an outline item navigates the editor and shifts focus; moving the editor cursor dynamically highlights the corresponding outline heading.
- **Cross-Mode Jumping**: Clicking an outline item in GenUI view mode smoothly switches back to Rich text mode and scrolls to the target section.

#### Derived GenUI Reading View (A2UI 1.0)
- **Componentized AI Reader (ADR 0002)**: Renders structured, interactive reading views compiled from companion `.<name>.md.genui.json` view files following the A2UI 1.0 standard.
- **Comprehensive Component Catalog**:
  - 5 Layout Primitives: `Column`, `Row`, `Tabs`, `Card`, `Text`.
  - 5 High-Density Presentation Components: `KeyPoints`, `MetricGrid`, `ComparisonTable`, `Timeline`, `Checklist` (read-only).
- **Source Tracking & Navigation (ADR 0002)**:
  - Each component displays a source reference capsule (`From <heading>`).
  - Detects out-of-date sections (`Source changed`) via normalized `fnv1a32` hashes.
  - Clicking `Edit in source` jumps directly to the source text in Rich text mode and preserves scroll position upon returning to GenUI.

#### ACP Agent Generation & ModelMenu
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

#### In-Place Paper Composer
- **Elimination of Modal Dialogs**:
  - Replaced all centered `<dialog>` popups with in-place paper composers (`.composer`).
  - Auto-growing textareas with `⌘↩` / `Ctrl+↩` to save and `Escape` to cancel.
  - Notes are non-destructively appended to referenced sections as `<!-- note: ... -->` comments, without triggering false-positive `Source changed` marks.

---

### 中文

#### Portrait 极简三栏工作区与设计语言
- **三栏职责分离架构（ADR 0004）**：
  - **左栏（工作区）**：仅承载工作区文档列表与筛选导航。
  - **中央栏（内容画布）**：严格专用于呈现文档内容（Rich text、Markdown、GenUI 视图），彻底剔除中央阅读区顶部的生成表单与按钮。
  - **右栏（当前文档）**：承载只属于当前文档的辅助面板，顶部通过凹陷分段条切换实时大纲（`Outline`）与 `GenUI` 管理面板。
- **窄窗自适应响应式机制**：窗口宽度 `< 1180px` 时右栏自动收起，手动触发时以右侧浮层（`.r-float`）形式展开，点击正文自动收回。
- **Portrait 触觉设计语义 Token**：
  - 引入冷灰窗框（`--chrome`）、白纸卡片（`--paper`）、代码底色（`--code`）以及 `--ink` 至 `--ink-4` 层次对比。
  - 5 级微拟物物理触觉阴影：`--raised-shadow`（凸起药丸按钮）、`--row-shadow`、`--sunken-shadow`（凹陷轨道）、`--sheet-shadow`、`--pop-shadow`。
  - Portrait 浅色样式为应用状态提供独立语义色（`--warn` 仅用于“原文已改”，`--danger` 仅用于冲突与错误）。

#### 逐块保真的 Markdown 双模式编辑器
- **双引擎编辑**：
  - **所见即所得富文本（WYSIWYG）**：基于 Milkdown 7.23 与定制 `CrepeBuilder`，提供现代无干扰写作体验。
  - **源码模式**：基于 CodeMirror 6，支持精确字符偏移与 `⌘/` 瞬间无损切换。
- **逐块保真组装引擎（Block-Preserving Composition）**：
  - 将 Markdown 拆解为 ProseMirror 块节点，保存时仅重新序列化被编辑过的块，未改动的块保留原文件逐字节字符（包括行尾空格、空行与换行风格）。
  - 彻底解决传统 Markdown 编辑器保存时全篇排版格式变异丢失的问题。
  - 通过锁定块（Locked Blocks）机制，保真封包保护 YAML Frontmatter、GitHub Alert 引用、HTML 原始注释与脚注。

#### 大纲导航与双向光标联动
- **实时标题树**：基于共享 AST 标题解析算法准确提取 ATX 与 setext 标题。
- **双向光标同步**：点击大纲项精准移动光标并定位视图，正文移动光标时大纲项实时同步高亮。
- **跨模式一键回切**：在 GenUI 视图中点击大纲项，自动切回富文本模式并定位到对应章节。

#### 派生 GenUI 阅读视图（A2UI 1.0）
- **组件化 AI 阅读界面（ADR 0002）**：渲染由隐藏伴随文件 `.<name>.md.genui.json` 存储的 A2UI 1.0 协议标准交互视图。
- **全套组件目录支持**：
  - 5 种基础布局组件：`Column`、`Row`、`Tabs`、`Card`、`Text`。
  - 5 种高级卡片组件：`KeyPoints` 要点卡片、`MetricGrid` 指标网格、`ComparisonTable` 对比表格、`Timeline` 时间线、`Checklist` 只读核对清单。
- **来源追踪与跳转定位**：
  - 每个组件底座配备来源胶囊（`From <标题>`），通过 `fnv1a32` 正文哈希实时追踪“原文已改（`Source changed`）”。
  - 点击“去原文修改（`Edit in source`）”一键切到富文本并选中来源段落，改完返回自动恢复原滚动位置。

#### 外部 Agent (ACP) 生成与模型菜单
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

#### 原地白纸批注 Composer
- **告别居中弹窗**：
  - 移除所有居中模态 `<dialog>` 弹窗，正文批注与 GenUI 卡片批注均在目标落位处原地展开白纸输入卡片（`.composer`）。
  - 输入框高度自适应，支持 `⌘↩` / `Ctrl+↩` 快速保存与 `Esc` 取消。
  - 批注以 `<!-- note: 文字 -->` 形式无损写入原文段落末尾，且计算章节哈希时自动排除批注，绝不误触“原文已改”状态。

#### 稳定性与全量自动化测试
- **完整的自动化测试矩阵**：
  - 全量 Go 自动化测试与 race 竞争检查通过；
  - 17 个前端测试套件、189 项 Vitest 单元与集成测试全部通过；
  - 涵盖源码往返保真、大纲光标联动、三栏浮层自适应、A2UI 组件目录渲染、ACP 假 Agent 模拟生成与批注无损写入。
