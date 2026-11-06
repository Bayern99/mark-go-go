# 功能与接口清单（第一阶段原型）

## 模块职责

| 模块 | 位置 | 职责 |
|---|---|---|
| Host（Go） | `main.go`、`documenthost.go` | `mygo.NewDocuments[string]`：文件读写（原始字节）、脏状态、自动保存、恢复副本、外部修改重载/冲突、File/Edit 菜单 |
| Host Adapter（TS） | `web/src/host/` | `HostAdapter` 契约；`mygoHost.ts`（MyGo IPC）、`browserHost.ts`（纯浏览器内存实现） |
| Editor Adapter | `web/src/editor/` | CrepeBuilder 装配；块级分段、锁定块、`compose`、源码偏移映射（`reveal` / `onSelectionChange`） |
| Application Core | `web/src/core/` | `DocumentService`、`CommandRegistry`、`ViewRegistry`、`PluginRegistry` |
| Markdown 标题 | `web/src/markdown/headings.ts` | 共享标题提取：ATX + setext，排除代码块/HTML/frontmatter，去内联格式，带源偏移 |
| GenUI 插件 | `web/src/plugins/genui/` | 内部 schema + 组件白名单 + 固定 JSON；标题列表与 source chips 使用共享标题模块 |

## Go ↔ 页面 IPC（服务 `DocumentHost`）

| 名称 | 类型 | 说明 |
|---|---|---|
| `Load()` | `→ Snapshot` | 调用窗口对应文档的当前内容（含未保存编辑） |
| `Update(text)` | `→ void` | 页面 compose 出的全文 → `doc.Set`；冲突状态下仍写入，保存时由 MyGo 询问 |
| `docReloaded` | 事件 `Snapshot` | MyGo 因外部修改重读文件；`version` +1 |
| `docState` | 事件 `DocState` | edited / conflict / deleted / readOnly / name / path |

`Snapshot = { text, version, state }`；`version` 从 1 开始，每次从磁盘重载 +1。生成客户端：`web/src/host/generated/mygo.ts`（`go tool mygo generate`）。

## HostAdapter（`web/src/host/types.ts`）

```ts
interface HostAdapter {
  load(): Promise<HostDocSnapshot>
  update(text: string): Promise<void>
  onReload(fn: (snap: HostDocSnapshot) => void): Disposable
  onState(fn: (state: DocState) => void): Disposable
}
```

## 插件契约（`web/src/core/types.ts`）

```ts
interface AppPlugin { id: string; title: string; activate(ctx: PluginContext): void | Promise<void> }
interface PluginContext {
  documents: { snapshot(): { text: string; version: number }; onDidChange(fn): Disposable }
  views:     { register(v: { id; title; mount(el: HTMLElement): Disposable }): Disposable }
  commands:  { register(id, fn): Disposable; execute(id, ...args): Promise<unknown> }
  subscriptions: Disposable[]
}
```

- `PluginRegistry.enable(id)` / `disable(id)`：注册表追踪所有经 ctx 产生的注册与订阅，`disable` 统一释放；`activate` 抛错时回滚。
- 编辑器提供的命令：`editor.reveal(offset: number)`（偏移越界时返回 `false`）、`editor.toggleSourceMode()`（切换后返回当前模式）。

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
  onSelectionChange(fn: (offset: number) => void): Disposable
  setReadonly(readonly: boolean): void
  getView(): EditorView
  destroy(): Promise<void>
}
```

- 跨模块位置一律是当前 Markdown 文本的 UTF-16 源偏移；ProseMirror 位置不出适配器。
- WYSIWYG 模式下按顶层块粒度映射：`reveal(offset)` 滚动到并聚焦包含该偏移的顶层块开头（锁定块用节点选择），偏移超出当前 compose 文本时返回 `false`；只要命中块就会报告该块起始偏移（光标原本就在该块也报告）。`onSelectionChange` 在光标换块或编辑使该块偏移变化时报告光标所在顶层块的源起始偏移，并在注册时同步回调一次当前偏移（大纲可立即高亮当前章节）。块范围每次由最新 compose 文本推导，所以块内编辑以及光标之前的增删不会让偏移失效。
- 选择到块的解析只使用 ProseMirror 文档结构（文本选择取 `$from.node(1)`，文档级选择取 `$from.index(0)` 对应的顶层节点），从不把 ProseMirror 位置当作 Markdown 偏移比较。
- `getMarkdown()`：WYSIWYG 模式返回当前 compose 文本（未改动块保持原始字节）；源码模式返回 CodeMirror 文档文本（逐字节原样）。
- 旧接口 `revealHeading(text)` 已删除；调用方（GenUI source chips）改为先经共享标题模块解析出偏移再 `editor.reveal`。

### 源码模式

- 源码模式是 CodeMirror 6（`markdown()`、行折行、`history()` + `defaultKeymap`/`historyKeymap`），装配在 `web/src/editor/source/sourceEditor.ts`，与 WYSIWYG 挂在同一个编辑器区域内，同一时间只有一个可见。
- 配置 `EditorState.lineSeparator.of('\n')`：只有 LF 分行，CRLF / 混合换行 / 单独 CR / 无末尾换行的文件进出源码模式都不改变任何字节，源码中的无关行编辑也不改变其他字节。
- WYSIWYG → 源码：CodeMirror 文本取当前 compose 文本，历史为空；光标移到旧模式光标所在块的源起始偏移。
- 源码 → WYSIWYG：若源码文本与进入时相同，保留原 ProseMirror 状态与撤销历史；若不同（或期间发生外部 reload），从新文本经 `buildDocument` / `segmentMarkdown` 重建，WYSIWYG 撤销历史重置。切换后在新模式中 reveal 旧模式的光标偏移。
- 源码模式语义：`onChange` 在用户编辑后带 CodeMirror 文本触发；`setMarkdown`（外部 reload）替换文本、重置 CodeMirror 历史与光标并立即报告光标偏移 0，模式不变；`setReadonly` 同时作用于 CodeMirror（`EditorState.readOnly` + `EditorView.editable` Compartment）；`reveal(offset)` 为精确光标偏移 + `scrollIntoView` + focus；`onSelectionChange` 报告精确的 CodeMirror 偏移，监听器跨模式保持，切换时立即报告。源码有改动后切回 WYSIWYG 的重建会抑制加载中间态的偏移报告，监听器只看到最终 reveal 的块偏移。
- 切换模式本身不触发 `onChange`。App 头部按钮显示当前模式（`Rich text` / `Source`，`aria-pressed` 表示是否为源码模式），`⌘/` 或 `Ctrl+/` 切换，命令 `editor.toggleSourceMode` 供插件调用。

## 锁定块（按原文显示与写回，不可视化编辑）

`yaml-frontmatter`、`github-alert`、`footnote-definition`、`link-reference-definition`、`code-fence-meta`、`raw-html`、`math`（`$$` 块）、`unsupported`（单片段解析结果 ≠ 1 个顶层节点或解析失败）。

## 编辑器装配

CrepeBuilder features：`cursor, listItem, placeholder, linkTooltip, blockEdit, table, imageBlock, toolbar, codeMirror`（语言：TS/JS/JSON/Go/Python/Bash/Markdown）。未启用：`topBar, latex, ai`。源码模式用 CodeMirror 6 独立装配，不经过 Crepe。直接依赖锁定的 CodeMirror 包：`@codemirror/lang-markdown 6.5.2`、`@codemirror/language 6.13.1`、`@codemirror/state 6.7.6`、`@codemirror/view 6.43.15`、`@codemirror/commands 6.11.1`。

已移除的 Milkdown 内置插件（会破坏未改动块的字节一致）：`MILKDOWN_HEADING_ID`、`MILKDOWN_KEEP_LIST_ORDER`、`MILKDOWN_TRAILING`。

## 尚未实现（后续阶段）

搜索替换、大纲面板、AI/Agent、GenUI 自动生成、多标签页。
