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
- 编辑器提供的命令：`editor.reveal(offset: number)`（偏移越界时返回 `false`）。

## Editor Adapter（`web/src/editor/createEditor.ts`）

```ts
interface MarkdownEditor {
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
- `getMarkdown()` 返回当前 compose 文本（未改动块保持原始字节）。
- 旧接口 `revealHeading(text)` 已删除；调用方（GenUI source chips）改为先经共享标题模块解析出偏移再 `editor.reveal`。

## 锁定块（按原文显示与写回，不可视化编辑）

`yaml-frontmatter`、`github-alert`、`footnote-definition`、`link-reference-definition`、`code-fence-meta`、`raw-html`、`math`（`$$` 块）、`unsupported`（单片段解析结果 ≠ 1 个顶层节点或解析失败）。

## 编辑器装配

CrepeBuilder features：`cursor, listItem, placeholder, linkTooltip, blockEdit, table, imageBlock, toolbar, codeMirror`（语言：TS/JS/JSON/Go/Python/Bash/Markdown）。未启用：`topBar, latex, ai`。

已移除的 Milkdown 内置插件（会破坏未改动块的字节一致）：`MILKDOWN_HEADING_ID`、`MILKDOWN_KEEP_LIST_ORDER`、`MILKDOWN_TRAILING`。

## 尚未实现（后续阶段）

源码模式、搜索替换、大纲面板、AI/Agent、GenUI 自动生成、多标签页。
