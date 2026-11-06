# 功能与接口清单

## 模块职责

| 模块 | 位置 | 职责 |
|---|---|---|
| Host（Go） | `main.go`、`documenthost.go`、`viewfile.go` | `mygo.NewDocuments[string]`：文件读写（原始字节）、脏状态、自动保存、恢复副本、外部修改重载/冲突、File/Edit 菜单；跟随文档旁的视图文件 |
| Host Adapter（TS） | `web/src/host/` | `HostAdapter` 契约；`mygoHost.ts`（MyGo IPC）、`browserHost.ts`（纯浏览器内存实现） |
| Editor Adapter | `web/src/editor/` | CrepeBuilder 装配；块级分段、锁定块、`compose`、源码偏移映射（`reveal` / `onSelectionChange`） |
| Application Core | `web/src/core/` | `DocumentService`、`CommandRegistry`、`ViewRegistry`、`PluginRegistry`、`EditorAccess`（插件侧编辑器选择源） |
| Markdown 标题/章节 | `web/src/markdown/headings.ts`、`sections.ts` | 共享标题提取：ATX + setext，排除代码块/HTML/frontmatter，去内联格式，带源偏移；章节范围与 `fnv1a32` 哈希 |
| GenUI 插件 | `web/src/plugins/genui/` | 中央视图模式 `GenUI`：读取 v2 视图文件（格式见 `view-format.md`），经 A2UI 1.0 适配模块（`a2ui.tsx`）用组件目录渲染；按需加载 |
| Outline 插件 | `web/src/plugins/outline/` | 左侧栏文档大纲：共享标题模块 + 光标所在章节高亮 + 点击跳转 |

## Go ↔ 页面 IPC（服务 `DocumentHost`）

| 名称 | 类型 | 说明 |
|---|---|---|
| `Load()` | `→ Snapshot` | 调用窗口对应文档的当前内容（含未保存编辑） |
| `Update(text)` | `→ void` | 页面 compose 出的全文 → `doc.Set`；冲突状态下仍写入，保存时由 MyGo 询问 |
| `docReloaded` | 事件 `Snapshot` | MyGo 因外部修改重读文件；`version` +1 |
| `docState` | 事件 `DocState` | edited / conflict / deleted / readOnly / name / path |
| `WatchView()` | `→ ViewSnapshot` | 开始跟随调用窗口文档的视图文件，返回开始跟随后的首个快照 |
| `UnwatchView()` | `→ void` | 停止跟随 |
| `viewChanged` | 事件 `ViewSnapshot` | 跟随期间视图文件被写入、替换或删除；文档改名/首次保存后改跟随新路径并推送一次 |

`Snapshot = { text, version, state }`；`version` 从 1 开始，每次从磁盘重载 +1。`ViewSnapshot = { exists, text, error }`。生成客户端：`web/src/host/generated/mygo.ts`（`go tool mygo generate`）。

## HostAdapter（`web/src/host/types.ts`）

```ts
interface ViewFileSource {
  // 首个快照在开始跟随后送达一次，之后每次变化送达
  watchView(fn: (snap: ViewSnapshot) => void): Disposable
}
interface HostAdapter extends ViewFileSource {
  load(): Promise<HostDocSnapshot>
  update(text: string): Promise<void>
  onReload(fn: (snap: HostDocSnapshot) => void): Disposable
  onState(fn: (state: DocState) => void): Disposable
}
```

- `mygoHost.watchView` 对多个监听者计数：首个监听者调用 `WatchView()`，最后一个释放时调用 `UnwatchView()`。
- `createBrowserHost(text, name, { view? })` 用内存视图文件；`host.writeView(text | null)` 模拟写入/删除（测试用）。

## 插件契约（`web/src/core/types.ts`）

```ts
interface AppPlugin { id: string; title: string; activate(ctx: PluginContext): void | Promise<void> }
type ViewLocation = 'left' | 'center'
interface ViewRegistration { id: string; title: string; location: ViewLocation; mount(el: HTMLElement): Disposable }
interface PluginContext {
  documents: { snapshot(): { text: string; version: number }; onDidChange(fn): Disposable }
  views:     { register(v: ViewRegistration): Disposable }
  commands:  { register(id, fn): Disposable; execute(id, ...args): Promise<unknown> }
  editor:    { onSelectionChange(fn: (offset: number) => void): Disposable }
  viewFile:  { watch(fn: (snap: ViewSnapshot) => void): Disposable }
  subscriptions: Disposable[]
}
```

- `PluginRegistry.enable(id)` / `disable(id)`：注册表追踪所有经 ctx 产生的注册与订阅，`disable` 统一释放；`activate` 抛错时回滚。`ctx.editor.onSelectionChange` 同样被追踪，禁用插件即移除其选择监听。
- `PluginRegistry(documents, views, commands, editor, viewFile)`：`editor` 是 `EditorSelectionSource`，App 用 `EditorAccess` 实现：注册表在编辑器创建前构造，编辑器就绪后 `attach(editor)`，之后才注册/启用插件；未 attach 时调用抛错（不会静默失效），App 卸载时 `detach()`。`viewFile` 是 host；`ctx.viewFile.watch` 被追踪，禁用插件即停止跟随视图文件。
- `ViewRegistration.location` 必填：`'left'` 进左侧栏（没有左侧视图时整栏不渲染）；`'center'` 成为一个视图模式，与 `Rich text` / `Markdown` 并列出现在头部的三选一切换里，只在被选中时挂载、离开时卸载。没有右侧栏。
- 编辑器提供的命令：`editor.reveal(offset: number)`（偏移越界时返回 `false`；当前是中央视图时先切回 Rich text 再跳转）、`editor.toggleSourceMode()`（在 Rich text 与 Markdown 之间切换，从中央视图出发则切到编辑器的另一模式；返回当前编辑器模式）。插件跳转一律 `ctx.commands.execute('editor.reveal', offset)`，不另设 reveal API。

## 视图模式（#9）

- 头部 `role="radiogroup"`（`View mode`），每个模式一个 `role="radio"` 按钮（测试钩子 `view-mode-richtext` / `view-mode-markdown` / `view-mode-<中央视图 id>`，`aria-checked` 表示当前模式）。`⌘/` / `Ctrl+/` 在 Rich text 与 Markdown 之间切换。
- 进入中央视图时编辑器只设 `hidden`、不销毁：ProseMirror / CodeMirror 状态、光标、撤销历史和未保存的修改保留；回到编辑器时恢复离开时的滚动位置。切换模式本身不调用 `host.update`。
- 当前模式按文档路径记在页面 `localStorage`（`mgo.viewMode:<path>`），窗口重新打开时恢复；未保存的新文档不记。

## 侧边栏与大纲插件（#4）

- `web/src/components/SidePane.tsx`：渲染 `location: 'left'` 的注册（`aside.side-pane`，测试钩子 `left-pane`）；视图挂载/卸载沿用 `mount(el)` / `Disposable` 语义，由 `ViewHost.tsx` 的 `ViewHost` 完成（中央视图同用）。
- Outline 插件（`createOutlinePlugin()`）注册视图 `outline.panel`（`location: 'left'`，标题 `Outline`），App 与 GenUI 一样注册、启用，并提供头部开关 `Outline: On/Off`。
- 条目来自 `extractHeadings(ctx.documents.snapshot().text)`，按 `level` 缩进；文档变更经 `OUTLINE_DEBOUNCE_MS`（300ms，导出自 `OutlinePanel.tsx`）防抖后重新解析。
- 当前章节 = 最后一个 `offset ≤ 光标偏移` 的标题；光标在第一个标题之前时不高亮任何条目；光标偏移来自 `ctx.editor.onSelectionChange`（WYSIWYG 为顶层块起始偏移，源码为精确偏移），高亮条目标 `aria-current="true"`。
- 点击条目执行 `editor.reveal(该条目自身 offset)`，所以同文标题各跳各的位置；两种模式行为一致。
- 无标题时显示 `No headings in this document.`（`data-testid="outline-empty"`）。
- GenUI 不自带标题列表；在 GenUI 模式下点大纲条目会切回 Rich text 并跳转。
- `App({ host, plugins })` 在 GenUI / Outline 之后追加注册插件（自动化测试注入异步激活插件用），注册与启用逐项检查 `disposed`。

## GenUI 视图模式（#9，取代 #6 的右侧栏与 v1 格式）

- 位置：文档 `<dir>/<name>.md` 的视图文件是同目录隐藏文件 `<dir>/.<name>.md.genui.json`；未保存的新文档没有视图文件。页面不直接读文件，只经 `ctx.viewFile.watch`；超过 1 MiB 或读失败作为 `error` 送达。
- 按需：插件注册中央视图 `genui`；`mount` 时才 `import('./GenUIView')`（独立代码块，含 A2UI 与 zod）并开始 `ctx.viewFile.watch`，卸载时停止跟随。未进入 GenUI 时不加载该代码块，也没有 `WatchView` 调用。
- 格式唯一说明：`web/src/plugins/genui/view-format.md`（`version: 2`；`generatedAt?`、`model?`、`request?`、`sources{ id: { heading, occurrence?, hash? } }`、`a2ui[]` A2UI 1.0 消息）。测试直接渲染其中的 JSON 示例，文档与实现不会分叉。
- 外层解析（`viewFile.ts` 的 `parseViewFile`）：非 JSON、非对象、版本不是 2、缺 `a2ui` 数组、`sources` 条目不合法 → 一条 `role="alert"` 消息；`version: 1` → `This view was made by an older version of Mark Go Go. Generate it again.`（不迁移）。
- A2UI 适配（`a2ui.tsx`，唯一 import `@a2ui/*` 的模块，版本精确锁定 0.15.0）：组件目录 `markgogo.dev:genui/v1` = 基础目录的 `Column` / `Row` / `Tabs` / `Card` / `Text` + 我们的 `KeyPoints`。`MessageProcessor` 以严格校验运行（必须有 `root`、引用必须存在、属性按 zod schema 严格校验、只允许 `createSurface` / `updateComponents`、只能一个 surface）；任一失败 → 整个视图一条错误。目录外的组件名在加载前替换为占位组件，只在该处显示 `“<名字>” is not a component this app can show.`，其余照常。
- `Text` 的 Markdown 由 `SafeMarkdown.tsx` 用应用自己的 mdast 解析器渲染为 React 元素：粗体、斜体、删除线、行内代码、代码块、列表、引用；链接只显示文字（`.genui-link`），图片和原始 HTML 不显示。没有 `innerHTML`。（`@a2ui/markdown-it` 0.2.2 的 peer 依赖是 web_core ^0.14，与 0.15 冲突，未采用。）
- 视图里的交互（切换标签页等）只改变显示，不调用 `host.update`。
- 章节（`sections.ts`）与 `fnv1a32` 哈希不变。
- 没有视图文件时显示 `No view yet.` 与禁用的 `Generate`（生成在 #7 接入）；有文件时显示模型与生成时间。

## GenUI 来源状态与“去原文修改”（#10）

- `sources.ts` 的 `SourceStore`：视图文件的来源表对照当前文档。文档变化后防抖 300ms（与大纲相同）重新提取章节；状态 `fresh` / `changed`（哈希不同）/ `missing`（找不到 heading + occurrence，优先于 changed）/ `unknown`（无记录哈希的手写视图，或组件没有来源）。
- `SourceFrame.tsx`：目录内容组件（目前 `KeyPoints`）都画在同一个框里：底部 `From <标题…>`、状态标记 `Source changed` / `Source not found`（只在这两种状态显示）、`Edit in source` 按钮（`missing` 时禁用）。状态写在 `data-source-status`。
- `Edit in source`：取组件第一个仍在文档里的来源，先立即刷新章节（不用防抖中的旧偏移），再执行命令 `editor.select(start, end)`。App 切到 Rich text、整段选中，并在编辑器区显示 `← Back to GenUI`（`data-testid="return-to-view"`）。点它回到 GenUI；在头部选任一模式后这个按钮消失。
- 滚动位置：GenUI 视图的挂载元素 `.center-view` 就是滚动区。位置按视图文件文本记在插件里（`ScrollMemory`），跨挂载保留；同一份视图文件回来时恢复，换了视图文件从顶部开始。A2UI 首次提交后才渲染组件树，所以恢复时跟随内容增高（`ResizeObserver`，最多 1s）。
- 生成器写入 `hash` 尚未实现（#7）；现在只有带 `hash` 的视图文件才会显示状态。

## Editor Adapter（`web/src/editor/createEditor.ts`）

```ts
type EditorMode = 'wysiwyg' | 'source'

interface MarkdownEditor {
  getMode(): EditorMode
  setMode(mode: EditorMode): void
  toggleMode(): void
  onModeChange(fn: (mode: EditorMode) => void): Disposable
  getMarkdown(): string
  setMarkdown(text: string): void
  onChange(fn: (markdown: string) => void): Disposable
  reveal(offset: number): boolean
  select(from: number, to: number): boolean
  onSelectionChange(fn: (offset: number) => void): Disposable
  setReadonly(readonly: boolean): void
  getView(): EditorView
  destroy(): Promise<void>
}
```

- 跨模块位置一律是当前 Markdown 文本的 UTF-16 源偏移；ProseMirror 位置不出适配器。
- WYSIWYG 模式下按顶层块粒度映射：`reveal(offset)` 滚动到并聚焦包含该偏移的顶层块开头（锁定块用节点选择），偏移超出当前 compose 文本时返回 `false`；只要命中块就会报告该块起始偏移（光标原本就在该块也报告）。`select(from, to)` 选中从包含 `from` 的块到 `to` 之前最后一个块的全部块（`to` 为区间终点，不含），滚动并聚焦，报告起始块偏移；区间越界或 `from > to` 返回 `false`。源码模式下 `select` 是精确的 CodeMirror 选区。`onSelectionChange` 在光标换块或编辑使该块偏移变化时报告光标所在顶层块的源起始偏移，并在注册时同步回调一次当前偏移（大纲可立即高亮当前章节）。块范围每次由最新 compose 文本推导，所以块内编辑以及光标之前的增删不会让偏移失效。
- 选择到块的解析只使用 ProseMirror 文档结构（文本选择取 `$from.node(1)`，文档级选择取 `$from.index(0)` 对应的顶层节点），从不把 ProseMirror 位置当作 Markdown 偏移比较。
- `getMarkdown()`：WYSIWYG 模式返回当前 compose 文本（未改动块保持原始字节）；源码模式返回 CodeMirror 文档文本（逐字节原样）。
- 旧接口 `revealHeading(text)` 已删除；调用方先经共享标题模块解析出偏移再 `editor.reveal`。

### 源码模式

- 源码模式是 CodeMirror 6（`markdown()`、行折行、`history()` + `defaultKeymap`/`historyKeymap`），装配在 `web/src/editor/source/sourceEditor.ts`，与 WYSIWYG 挂在同一个编辑器区域内，同一时间只有一个可见。
- 配置 `EditorState.lineSeparator.of('\n')`：只有 LF 分行，CRLF / 混合换行 / 单独 CR / 无末尾换行的文件进出源码模式都不改变任何字节，源码中的无关行编辑也不改变其他字节。
- WYSIWYG → 源码：CodeMirror 文本取当前 compose 文本，历史为空；光标移到旧模式光标所在块的源起始偏移。
- 源码 → WYSIWYG：若源码文本与进入时相同，保留原 ProseMirror 状态与撤销历史；若不同（或期间发生外部 reload），从新文本经 `buildDocument` / `segmentMarkdown` 重建，WYSIWYG 撤销历史重置。切换后在新模式中 reveal 旧模式的光标偏移。
- 源码模式语义：`onChange` 在用户编辑后带 CodeMirror 文本触发；`setMarkdown`（外部 reload）替换文本、重置 CodeMirror 历史与光标并立即报告光标偏移 0，模式不变；`setReadonly` 同时作用于 CodeMirror（`EditorState.readOnly` + `EditorView.editable` Compartment）；`reveal(offset)` 为精确光标偏移 + `scrollIntoView` + focus；`onSelectionChange` 报告精确的 CodeMirror 偏移，监听器跨模式保持，切换时立即报告。源码有改动后切回 WYSIWYG 的重建会抑制加载中间态的偏移报告，监听器只看到最终 reveal 的块偏移。
- 切换模式本身不触发 `onChange`。App 头部的视图模式切换显示当前模式（`Rich text` / `Markdown` / 中央视图，见「视图模式」），`⌘/` 或 `Ctrl+/` 在两种编辑器模式之间切换，命令 `editor.toggleSourceMode` 供插件调用。

## 锁定块（按原文显示与写回，不可视化编辑）

`yaml-frontmatter`、`github-alert`、`footnote-definition`、`link-reference-definition`、`code-fence-meta`、`raw-html`、`math`（`$$` 块）、`unsupported`（单片段解析结果 ≠ 1 个顶层节点或解析失败）。

## 编辑器装配

CrepeBuilder features：`cursor, listItem, placeholder, linkTooltip, blockEdit, table, imageBlock, toolbar, codeMirror`（语言：TS/JS/JSON/Go/Python/Bash/Markdown）。未启用：`topBar, latex, ai`。源码模式用 CodeMirror 6 独立装配，不经过 Crepe。直接依赖锁定的 CodeMirror 包：`@codemirror/lang-markdown 6.5.2`、`@codemirror/language 6.13.1`、`@codemirror/state 6.7.6`、`@codemirror/view 6.43.15`、`@codemirror/commands 6.11.1`。

已移除的 Milkdown 内置插件（会破坏未改动块的字节一致）：`MILKDOWN_HEADING_ID`、`MILKDOWN_KEEP_LIST_ORDER`、`MILKDOWN_TRAILING`。

## 尚未实现（后续阶段）

搜索替换、GenUI 经 ACP 生成（#7、#8）、多标签页。
