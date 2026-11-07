# 功能与接口清单

## 模块职责

| 模块 | 位置 | 职责 |
|---|---|---|
| Host（Go） | `main.go`、`documenthost.go`、`viewfile.go`、`generate.go` 等 | `mygo.NewDocuments[string]`：文件读写（原始字节）、脏状态、自动保存、恢复副本、外部修改重载/冲突、File/Edit 菜单；跟随文档旁的视图文件；经 ACP 运行 Agent 生成视图文件（ADR 0003） |
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
| `UndoView()` | `→ void` | 把上一次生成前保留的 `.<name>.genui.prev.json` 改名回视图文件（一次，之后不再有备份）；没有备份或正在生成时拒绝；页面经 `viewChanged` 看到 |
| `viewChanged` | 事件 `ViewSnapshot` | 跟随期间视图文件被写入、替换或删除；文档改名/首次保存后改跟随新路径并推送一次 |
| `Generate(req, progress)` | `GenerateRequest`、`Channel<GenerateProgress>` `→ void` | 经 ACP 运行一次生成（见「GenUI 生成」）；成功后视图文件已原子写入，页面经 `viewChanged` 看到。失败时最后一步是 `{phase: "failed", text, failure: GenerateFailure}`，再以 `message`（+ 空格 + `hint`）拒绝。页面关闭通道、关闭窗口、退出 app 都是取消 |
| `GetSettings()` / `SetSettings(s)` | `Settings {model, agentCommand, models?}` | 存在用户数据目录的 `settings.json`；默认 `{gemini-3.8-flash, agy, [gemini-3.8-flash, gemini-4-argon, claude-opus-5-5]}`；每次 ACP `session/new` 握手成功后自动缓存 `configOptions` 中的模型列表 |

`Snapshot = { text, version, state }`；`version` 从 1 开始，每次从磁盘重载 +1。`ViewSnapshot = { exists, text, error, canUndo }`，`canUndo` = 备份存在（监听同时看视图文件与备份）。生成客户端：`web/src/host/generated/mygo.ts`（`go tool mygo generate`）。

## HostAdapter（`web/src/host/types.ts`）

```ts
interface ViewFileSource {
  // 首个快照在开始跟随后送达一次，之后每次变化送达
  watchView(fn: (snap: ViewSnapshot) => void): Disposable
  undoView(): Promise<void> // 回到上一次生成前的视图，一次
}
interface HostAdapter extends ViewFileSource, ViewGenerator {
  load(): Promise<HostDocSnapshot>
  update(text: string): Promise<void>
  onReload(fn: (snap: HostDocSnapshot) => void): Disposable
  onState(fn: (state: DocState) => void): Disposable
}
interface ViewGenerator {
  generate(req: GenerateRequest, onProgress: (p: GenerateProgress) => void, signal: AbortSignal): Promise<void>
  getSettings(): Promise<GenerationSettings>
  setSettings(settings: GenerationSettings): Promise<void>
}
interface GenerationSettings { model: string; agentCommand: string; models?: string[] }
interface GenerateRequest { text: string; mode: 'full' | 'update'; replaceIds?: string[]; request?: string; format: string; sections: SectionRef[] }
interface SectionRef { heading: string; occurrence: number; start: number; end: number } // UTF-16 偏移
interface GenerateProgress { phase: 'starting' | 'thinking' | 'reading' | 'writing' | 'checking'; text: string }
// generate 总是以 GenerationError（web/src/host/generationError.ts，带 failure）拒绝
interface GenerateFailure { kind: GenerateFailureKind; message: string; hint?: string; detail?: string; models?: string[] }
type GenerateFailureKind = 'unsaved' | 'busy' | 'agent-not-found' | 'auth-required' | 'model-invalid'
  | 'agent-message' | 'invalid-output' | 'agent-error' | 'timeout' | 'cancelled'
```

- `mygoHost.watchView` 对多个监听者计数：首个监听者调用 `WatchView()`，最后一个释放时调用 `UnwatchView()`。
- `mygoHost.generate` 用 `mygo-runtime` 的 `Channel` 接进度，`signal` 中止时关闭通道（取消 Go 端的 ctx）。`failed` 步骤里的 `failure` 在调用被拒绝之前到达，变成 `GenerationError`；未知 kind 当作 `agent-error`，没有 failure 的拒绝也是 `agent-error`；中止的是 `cancelled`。
- `createBrowserHost(text, name, { view?, kept?, generator?, settings?, untitled? })` 用内存视图文件；`host.writeView(text | null)` 模拟写入/删除；`generator(req, progress, signal) → Promise<视图文件文本>` 扮演 Agent（`update` 时也返回完整视图文件，合并只在 Go 端），抛 `GenerationError` 即扮演那种失败，其他错误为 `agent-error`；`host.generations` 记录收到的请求，`host.keptView()` 返回备份（测试用）。规则与 Go 端相同：未保存的文档（`unsaved`）、正在生成时（`busy`）都拒绝；只有成功才写视图文件，被替换的视图成为备份。

## 插件契约（`web/src/core/types.ts`）

```ts
interface AppPlugin { id: string; title: string; activate(ctx: PluginContext): void | Promise<void> }
type ViewLocation = 'left' | 'center'
interface ViewRegistration { id: string; title: string; location: ViewLocation; mount(el: HTMLElement): Disposable }
interface PluginContext {
  documents: { snapshot(): { text: string; version: number; path: string }; onDidChange(fn): Disposable } // path '' = 未保存
  views:     { register(v: ViewRegistration): Disposable }
  commands:  { register(id, fn): Disposable; execute(id, ...args): Promise<unknown> }
  editor:    { onSelectionChange(fn: (offset: number) => void): Disposable }
  viewFile:  { watch(fn: (snap: ViewSnapshot) => void): Disposable
              generate(req, onProgress, signal): Promise<void>; getSettings(); setSettings(s) }
  subscriptions: Disposable[]
}
```

- `PluginRegistry.enable(id)` / `disable(id)`：注册表追踪所有经 ctx 产生的注册与订阅，`disable` 统一释放；`activate` 抛错时回滚。`ctx.editor.onSelectionChange` 同样被追踪，禁用插件即移除其选择监听。
- `PluginRegistry(documents, views, commands, editor, viewFile)`：`editor` 是 `EditorSelectionSource`，App 用 `EditorAccess` 实现：注册表在编辑器创建前构造，编辑器就绪后 `attach(editor)`，之后才注册/启用插件；未 attach 时调用抛错（不会静默失效），App 卸载时 `detach()`。`viewFile` 是 host（`ViewFileSource & ViewGenerator`）；`ctx.viewFile.watch` 被追踪，禁用插件即停止跟随视图文件。`documents.path` 由 App 按 host 的 `DocState.path` 更新。
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
- 按需：插件注册两个视图：中央视图 `genui`（`location: 'center'`，纯只读 A2UI 组件卡片与来源引用底座）与右栏管理面板 `genui.panel`（`location: 'right'`，附带 `status: ViewStatusSource` 供右栏 `GenUI` 标签圆点提示）；任一视图首次 `mount` 时才 `import('./GenUIView')`（独立代码块，含 A2UI 与 zod）并开始引用计数的 `ctx.viewFile.watch`，两处视图都卸载时停止跟随文件。未进入 GenUI 时不加载该代码块，也没有 `WatchView` 调用。
- 中央 `genui` 视图仅渲染只读 A2UI 组件表面（每张内容卡片带 `data-component-id` 与 `SourceFrame` 底座：`From`、`Source changed` / `Source not found`、`Edit in source`、`Note`），不包含生成按钮、生成要求输入框、模型选择或错误横幅；无视图或视图文件不可用时显示安静的空状态/说明并提供按钮经 `rightPane.show('genui.panel')` 打开右栏 `GenUI` 面板。
- 右栏 `genui.panel` 管理面板分为滚动区（`.genui-panel-scroll`）与底部固定区（`.genui-panel-footer`）：
  - 顶部状态与唯一主操作：五种状态互斥显示（无视图 `No GenUI view yet` + `Generate`；全部最新 `Up to date` + 次要 `Regenerate all`；存在过期组件 `N components out of date` + `.stale-list` 点击使中央滚动定位并高亮对应 `[data-component-id]` 卡片 + 主按钮 `Regenerate changed (N)` + 次要 `Regenerate all`；生成中进度 + `Cancel`；生成失败错误框 + `Retry` / `Dismiss`）。
  - 生成要求（`Focus`）：有视图时默认只读显示（空时 `No focus set`），点击 `Edit` 原地展开 `.composer` 白纸输入块（`<textarea aria-label="Generation request">`，支持 `⌘↩` / `Save` 保存、`Escape` / `Cancel` 取消）。
  - 底部固定区（钉在滚动容器外）：常驻只读 `Agent: Antigravity (agy)`、`ModelMenu`、生成时间 `Generated <generatedAt>` 以及 `canUndo` 时按需显示的 `Undo` 按钮。
  - 右栏顶部分段标签状态圆点（`.rdot`）：生成失败时 `tone: 'danger'`，存在 `Source changed` / `Source not found` 组件时 `tone: 'warn'`，其余为 `null`。
- 格式唯一说明：`web/src/plugins/genui/view-format.md`（`version: 2`；`generatedAt?`、`model?`、`request?`、`sources{ id: { heading, occurrence?, hash? } }`、`a2ui[]` A2UI 1.0 消息）。测试直接渲染其中的 JSON 示例，文档与实现不会分叉。
- 外层解析（`viewFile.ts` 的 `parseViewFile`）：非 JSON、非对象、版本不是 2、缺 `a2ui` 数组、`sources` 条目不合法 → 一条 `role="alert"` 消息；`version: 1` → `This view was made by an older version of Mark Go Go. Generate it again.`（不迁移）。
- A2UI 适配（`a2ui.tsx`，唯一 import `@a2ui/*` 的模块，版本精确锁定 0.15.0）：组件目录 `markgogo.dev:genui/v1` = 基础目录的 `Column` / `Row` / `Tabs` / `Card` / `Text` + 内容组件（见 #11）。`MessageProcessor` 以严格校验运行（必须有 `root`、引用必须存在、属性按 zod schema 严格校验、嵌套约束、只允许 `createSurface` / `updateComponents`、只能一个 surface）；任一失败 → 整个视图一条错误，写明组件和原因（嵌套违规以 `UNALLOWED_PARENT:` / `UNALLOWED_CHILD:` 开头）。目录外的组件名在加载前替换为占位组件，只在该处显示 `“<名字>” is not a component this app can show.`，其余照常。
- `Text` 的 Markdown 由 `SafeMarkdown.tsx` 用应用自己的 mdast 解析器渲染为 React 元素：粗体、斜体、删除线、行内代码、代码块、列表、引用；链接只显示文字（`.genui-link`），图片和原始 HTML 不显示。没有 `innerHTML`。（`@a2ui/markdown-it` 0.2.2 的 peer 依赖是 web_core ^0.14，与 0.15 冲突，未采用。）
- 视图里的交互（切换标签页等）只改变显示，不调用 `host.update`。
- 章节（`sections.ts`）与 `fnv1a32` 哈希不变。
- 有视图文件时显示模型与生成时间。

## GenUI 来源状态与“去原文修改”（#10）

- `sources.ts` 的 `SourceStore`：视图文件的来源表对照当前文档。文档变化后防抖 300ms（与大纲相同）重新提取章节；状态 `fresh` / `changed`（哈希不同）/ `missing`（找不到 heading + occurrence，优先于 changed）/ `unknown`（无记录哈希的手写视图，或组件没有来源）。
- `SourceFrame.tsx`：所有内容组件都画在同一个框里：底部 `From <标题…>`、状态标记 `Source changed` / `Source not found`（只在这两种状态显示）、`Edit in source` 按钮（`missing` 时禁用）。状态写在 `data-source-status`。组件经 `useComponentSource(sources)` 取状态与 `edit(text?)`。
- `Edit in source`：取组件第一个仍在文档里的来源，先立即刷新章节（不用防抖中的旧偏移），再执行命令 `editor.select(start, end)`。带 `text` 时（Checklist 项）在各来源节里找这段原文，找到就只选它，否则选第一个来源整节。App 切到 Rich text、选中，并在编辑器区显示 `← Back to GenUI`（`data-testid="return-to-view"`）。点它回到 GenUI；在头部选任一模式后这个按钮消失。
- 滚动位置：GenUI 视图的挂载元素 `.center-view` 就是滚动区。位置按视图文件文本记在插件里（`ScrollMemory`），跨挂载保留；同一份视图文件回来时恢复，换了视图文件从顶部开始。A2UI 首次提交后才渲染组件树，所以恢复时跟随内容增高（`ResizeObserver`，最多 1s）。
- 生成器（#7）按页面给的章节范围写入每个来源的 `hash`；手写的视图文件不带 `hash` 时不显示状态。

## GenUI 组件目录（#11）

- 内容组件（`components.tsx`，共用 `common` 属性 `sources?` / `weight?` 与 `title?`，`.strict()`，不含子组件）：`KeyPoints`（`items`）、`MetricGrid`（`metrics[{label, value, unit?, change?, trend?: up|down|flat}]`）、`ComparisonTable`（`columns` ≥ 2、`rows` 每行与列数相同；点列头升序 → 降序 → 原顺序，数字按值比较；只改显示）、`Timeline`（`events[{when, title, detail?}]`）、`Checklist`（`items[{text, done?, quote?}]`；`role="checkbox"` + `aria-readonly`，点一项经 `edit(quote ?? text)` 选中原文，从不改原文；来源缺失时禁用）。
- 嵌套约束（布局组件上声明）：`Row` 只能在顶层、`Column`、`Tabs` 里；`Tabs` 只能在顶层或 `Column` 里；`Card` 只能装 `Column`、`Text` 或内容组件。占位组件 `MgoUnknown` 被所有约束接纳，并从错误信息里去掉。
- `catalogDefinition()`：A2UI 1.0 目录文档（`Catalog.toJson()`），列出 10 个组件的 JSON Schema（含字段说明）与 `allowedParents` / `allowedChildren`，不含占位组件和函数；约 9 KB。#7 把它和格式文档一起交给 Agent。
- 排版：`Row` 在 GenUI 里换行，每项至少 260px；`.genui-block` `min-width: 0` 与 `overflow-wrap: anywhere`；宽表在自己的框里横向滚动。
- 主题：颜色是 `light-dark()` 令牌（`genui.css` 的 `--genui-*`，并映射到 `--a2ui-*`），跟随继承的 `color-scheme`。构建会把 `light-dark()` 降级成只有源码里声明了 `color-scheme` 的规则才设置的变量，所以 `:root` 声明 `color-scheme: light`；以后的主题切换要改这条规则，不能用内联样式。
- 格式文档 `view-format.md`：每个组件一节，示例用到全部 10 个组件（合约测试：示例无错误渲染，且组件集合等于目录）。

## GenUI 生成（#7，ADR 0003）

- 页面（`generation.ts` 的 `GenerationController`，一个窗口一个，跨 GenUI 挂载保留）：点 Generate 时取 `documents.snapshot().text`（含未保存修改）和它的章节（共享章节模块，UTF-16 偏移），`format` = `view-format.md` + 「Catalog definition」JSON（`catalogDefinition()`），`request` = 生成要求（去首尾空白，空则不发）。一次只跑一个；运行中 GenUI 显示进度（`role="status"` 最新一步 + 已完成的步骤列表）和骨架，旧视图暂时隐藏；只剩 Cancel。生成期间可以离开 GenUI 继续编辑。
- 生成要求输入框预填当前视图文件的 `request`；未保存的文档 Generate 禁用并提示 `Save the document first: the view is stored next to it.`。失败时 `role="alert"` 显示 `message`、`hint`（该做什么）、`model-invalid` 的可用模型列表、`detail`（「The agent said:」下原样的 `<pre>`），按钮：`Retry`（用失败那次的请求重跑）、`Open agent settings`（仅 `agent-not-found` / `model-invalid`）、`Dismiss`；旧视图不变。`Cancel` 只在运行中出现；取消不算失败、不提示。
- 失败分类（Go `failure.go`，`GenerateFailure.kind`）：`agent-not-found`（找不到或启动不了命令）、`auth-required`（任一调用的 JSON-RPC -32000 或含 “authentication required”，包括 stderr；提示 ``Run `<命令> acp --login` in Terminal, then retry.``）、`model-invalid`（`set_config_option` 失败且 `session/new` 列出了模型）、`agent-message`（这一轮结束但没有 `out.json` 而有回复，或 `session/prompt` 返回错误；`detail` 为原话，最多 4000 字符）、`invalid-output`（缺失、过大、符号链接、非 JSON、`a2ui` / `sources` 不合格；有回复时附为 `detail`）、`agent-error`（Agent 提前退出附 stderr、其他 RPC 错误、越界工具调用、内部错误）、`timeout`（默认 5 分钟，`generationJob.Timeout`）、`cancelled`、`unsaved`、`busy`。超时与取消同样 `session/cancel` → 2 秒 → 杀进程 → 删临时目录。
- 退出：关窗口取消调用的 ctx；退出 app 时 `main` 在 `App.Run` 返回后 `waitForGenerations(5s)`，等每个运行结束 Agent、删除临时目录再退出进程。
- 模型选择与设置（#17，ADR 0004）：面板底部固定区展示只读 `Agent: Antigravity (agy)` 与向上弹出菜单 `ModelMenu`（`.sel-btn` 26px 凸起触发钮 + `.mmenu.pop` 10px 圆角白纸弹层 + `Agent’s choice` 与缓存的 `models` 列表 + 右侧勾号 `.chk`）；点选模型后立即经 `setSettings` 持久化并关闭弹层，支持点击外部或按 `Escape`（`document` 级监听）关闭、生成中禁用。常规界面不暴露 `Model` 自由文本输入框与 `Agent command` 输入框。
- 宿主（Go）：临时目录里只有 `source.md`；`<agentCommand> acp`，无其他参数；`initialize` → `session/new` → `set_config_option(model)`（模型非空时）→ `session/prompt`。权限与工具调用按同一条路径规则检查（读 `source.md`、写 `out.json`、思考），越界即取消本次运行。`out.json` ≤ 1 MiB、普通文件、JSON 对象、非空 `a2ui`、合法 `sources`；宿主写 `version: 2`、`generatedAt`、`model`、`request`、带 `hash` 的 `sources`、原样的 `a2ui`，丢弃其他字段，原子替换视图文件；临时目录总是删除。进度：`starting`（启动、Agent 就绪）、`thinking`、`reading` / `writing`（工具标题）、`checking`。
- 共享测试向量 `web/test/fixtures/section-vectors.json` 增加了章节的 UTF-16 `start` / `end` 与标题规范化（`normalize`），TS 与 Go 两边都用它验证。

## GenUI 重新生成与撤回（#12）

- 页面：有视图时按钮是 `Regenerate changed (N)` 与 `Regenerate all`；没有视图时是 `Generate`。N = 表面上 `sources` 状态为「原文已改」的组件数（随来源刷新）；N = 0 时 `Regenerate changed` 禁用。`Regenerate changed` 发 `mode: 'update'`、`replaceIds` = 这些组件 id；两种都带生成要求，失败后 `Retry` 按同一种方式重跑。有备份时显示 `Undo last generation`（生成中禁用）。
- 宿主（`update.go`）：局部生成前先读当前视图（版本 2、有 surface、`replaceIds` 非空且都是视图里的组件），否则不启动 Agent，`agent-error`「The current view cannot be updated: …」。临时目录里放 `source.md` 和 `view.json`（当前视图原样）；权限策略允许读 `view.json`，只允许写 `out.json`。提示词列出要替换的 id 和 surface。`out.json` 的 `a2ui` 必须是**恰好一条** `updateComponents`（只有 `version` 与 `updateComponents` 两个键），是视图的 surface，组件 id 只能是要替换的或视图里没有的，不重复；`sources` 只能新增，或与视图里同名的完全一致。任何一条不符合 → `invalid-output`，视图和备份不变。通过后这条消息追加到 `a2ui` 末尾，所有来源按本次文本重算哈希，`generatedAt` / `model` / `request` 取本次。写入前再读一次视图文件，如果 Agent 工作期间被改过，不写入（`agent-error`）。
- 备份：每次成功写入新视图（全部或局部）前，把当前视图原子写到 `.<name>.genui.prev.json`，权限与视图相同；写视图失败时把备份恢复原样。生成失败不动备份。撤回 = `rename(prev, view)`，只有一份。

## 批注（#13）

- 格式：一个单独的顶层 HTML 注释块 `<!-- note: 文字 -->`。`note` 后面可以有属性 `key="value"`，然后是 `:`、文字、`-->`。语法只在 `web/src/markdown/notes.ts` 里。同一行还有别的内容、不符合这个语法的注释、代码块或列表里的注释都不是批注。本阶段只写文字，不写属性。
- 转义：文字里的 `-->` 写成 `--\>`，`--!>` 写成 `--\!>`（浏览器也把它当注释结束）；已经带反斜杠的再加一个，读回时去掉一个，所以任何文字都读回不变。文字可以多行，读写时去掉首尾空白。
- 编辑器：批注是锁定块（`kind: 'note'`），按原字节写回。只有改过的批注才重新序列化，只改文字部分，冒号之前（包括不认识的属性）原样保留，并沿用这一块原来的间隔。
- Rich text：块菜单（`+` 或 `/`）里的 `Add note` 把批注放在打开菜单的那一块后面；批注上有 `Edit`、`Delete`，都是普通事务，可以撤销。只读文档不能操作。Markdown 模式显示原始文本。
- GenUI：组件的 `Note` 按钮通过命令 `editor.addNote(offset, text)` 把批注加到第一个来源那一节的末尾，停留在 GenUI。宿主编辑器处在 Rich text 或 Markdown 都写出相同的字节。组件旁显示它所有来源节里的批注。只读文档会提示无法添加。
- `editor.addNote(offset, text)`：放在 `offset` 之前开始的最后一个顶层块之后，新批注前面是 `\n\n`。
- 字节保留：插入或删除一块后，后面的块保留自己的间隔（间隔不含空行且换了位置时才用 `\n\n`，否则可能和新的前一块合并）。文件末尾的字节在增删批注时不变。
- 哈希：章节哈希去掉每条批注，以及它和前一块之间的空白，所以结果和加批注之前的文本一样。Go 没有 Markdown 解析器，页面在 `GenerateRequest.sections[].exclude`（UTF-16 范围，按顺序）里告诉它哪些范围要去掉；范围越界时这个来源不写哈希。代码块里的 `<!-- note -->` 不会被去掉。共享向量 `section-vectors.json` 的 `notes` 部分在 TS 和 Go 两边验证。
- 批注随 `source.md` 交给 Agent，格式文档说明它们是读者的意见，不是原文内容。

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

`yaml-frontmatter`、`github-alert`、`footnote-definition`、`link-reference-definition`、`code-fence-meta`、`raw-html`、`note`（批注，可以编辑和删除，见「批注」）、`math`（`$$` 块）、`unsupported`（单片段解析结果 ≠ 1 个顶层节点或解析失败）。

## 编辑器装配

CrepeBuilder features：`cursor, listItem, placeholder, linkTooltip, blockEdit, table, imageBlock, toolbar, codeMirror`（语言：TS/JS/JSON/Go/Python/Bash/Markdown）。未启用：`topBar, latex, ai`。源码模式用 CodeMirror 6 独立装配，不经过 Crepe。直接依赖锁定的 CodeMirror 包：`@codemirror/lang-markdown 6.5.2`、`@codemirror/language 6.13.1`、`@codemirror/state 6.7.6`、`@codemirror/view 6.43.15`、`@codemirror/commands 6.11.1`。

已移除的 Milkdown 内置插件（会破坏未改动块的字节一致）：`MILKDOWN_HEADING_ID`、`MILKDOWN_KEEP_LIST_ORDER`、`MILKDOWN_TRAILING`。

## 尚未实现（后续阶段）

搜索替换、多标签页。
