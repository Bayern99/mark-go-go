# 功能与接口清单

## 模块职责

| 模块 | 位置 | 职责 |
|---|---|---|
| Host（Go） | `main.go`、`documenthost.go`、`workspace.go`、`viewfile.go`、`generate.go` 等 | `mygo.NewDocuments[string]` + 绑定服务 `WorkspaceHost`：一个工作区（文件夹）一个窗口，同窗多文档标签、全应用同一文件的单一编辑会话、文件读写（原始字节）、脏状态、自动保存、恢复副本、外部修改重载/冲突、File/Edit 菜单归属；跟随文档旁的视图文件；按文档隔离的 ACP 生成（ADR 0003、0005） |
| 导航索引（Go） | `index.go`、`navmeta.go` | 工作区文件树的增量扫描与索引（`NavSnapshot` 的 `status`/`indexed`/`total` 如实报告未完成）、`SetFilter` 搜索（标题 + 预览 + 章节命中）、已打开文档的未保存文本也参与索引 |
| 会话恢复（Go） | `session.go`、`restore_host.go` | `session.json` 布局（工作区、标签顺序、活动文档、视图模式）、退出前逐窗口 flush 确认后的持久化、启动恢复与缺失路径的部分恢复（#23） |
| Host Adapter（TS） | `web/src/host/` | `WorkspaceAdapter` / `HostAdapter` 契约；`mygoHost.ts`（MyGo IPC）、`browserHost.ts`（纯浏览器内存实现，`createBrowserWorkspace` / `createBrowserHost`） |
| Editor Adapter | `web/src/editor/` | CrepeBuilder 装配；块级分段、锁定块与特殊块呈现、原地 Markdown 编辑、`compose`、源码偏移映射（`reveal` / `onSelectionChange`）、原生 Edit 撤销重做目标（`nativeEdit.ts`） |
| Application Core | `web/src/core/` | `DocumentService`、`CommandRegistry`、`ViewRegistry`、`PluginRegistry`、`EditorAccess`（插件侧编辑器选择源） |
| 工作区壳（TS） | `web/src/App.tsx`、`web/src/components/SidePane.tsx`、`web/src/components/RightPane.tsx`、`web/src/workspace/nav.ts` | 文档标签与每文档一个 `DocumentPane`、左栏文件树/筛选/分栏/面包屑、Welcome 与最近工作区、右栏与窄窗浮层 |
| Markdown 标题/章节 | `web/src/markdown/headings.ts`、`sections.ts` | 共享标题提取：ATX + setext，排除代码块/HTML/frontmatter，去内联格式，带源偏移；章节范围与 `fnv1a32` 哈希 |
| GenUI 插件 | `web/src/plugins/genui/` | 中央视图模式 `GenUI`（读取 v2 视图文件，经 A2UI 1.0 适配模块 `a2ui.tsx` 用组件目录渲染只读卡片）与右侧栏管理面板 `genui.panel`（生成触发、进度、过期组件列表跳转、生成要求 `Focus` 与 `ModelMenu`）；按需加载 |
| Outline 插件 | `web/src/plugins/outline/` | 右侧栏文档大纲（`location: 'right'`）：共享标题模块 + 光标所在章节高亮 + 点击跳转 |

## Go ↔ 页面 IPC（服务 `WorkspaceHost`）

`main.go` 以 `mygo.BindAs("WorkspaceHost", host)` 绑定；页面侧是 `web/src/host/generated/mygo.ts` 的 `WorkspaceHost`（`go tool mygo generate` 生成，不手改）。文档级方法一律带 `documentId`，异步调用、事件、视图跟随与生成都绑定该身份，从不经当前活动标签重定向；工作区级方法作用于调用页面所在的窗口。身份、路由与生命周期的完整契约见 `docs/workspace-contract.md`。

### 工作区级

| 名称 | 类型 | 说明 |
|---|---|---|
| `Snapshot()` | `→ WorkspaceSnapshot` | 当前窗口的工作区：`id`、`root`（未命名窗口为空）、`name`、`kind`（`folder` / `untitled`）、按标签顺序的 `documents: DocumentInfo[]`、`activeId`、`restoreError?` |
| `Open(path)` | `→ DocumentInfo` | 全局路由：先定位同一磁盘文件的已打开标签（`path` 相等、规范化路径相等或 `os.SameFile`），否则进入根目录最深的已打开工作区，再否则以文件父目录打开/新建工作区（ADR 0005） |
| `OpenWorkspace(root)` | `→ void` | 已是某工作区根则定位该窗口，否则新建工作区窗口；根与文件在解析符号链接前缀后比较（macOS `/tmp` 与 `/private/tmp` 是同一工作区） |
| `New()` | `→ DocumentInfo` | 本窗口新增未命名原文（`kind: 'untitled'` 的窗口） |
| `Duplicate(documentId)` | `→ DocumentInfo` | 复制为新的未命名原文 |
| `Activate(documentId)` | `→ void` | 激活标签，不改文件 |
| `CloseDocument(documentId)` | `→ CloseResult` | 关闭一个文档标签；脏文档逐字询问。`{closed:false, canceled:true}` 时窗口与其余标签不变 |
| `CloseWorkspace()` | `→ CloseResult` | 窗口关闭按钮 / File > Close Window：按顺序询问每个脏文档；取消保留整个窗口与全部标签 |
| `NavSnapshot()` / `SetFilter(query)` / `ToggleFolder(rel)` / `RevealPath(path)` / `PickFolder()` / `RecentWorkspaces()` | 见「工作区导航」 | 打开工作区不等整目录扫描；`PickFolder` 取消时以 `ErrCanceled` 拒绝 |

### 文档级

| 名称 | 类型 | 说明 |
|---|---|---|
| `Load(documentId)` | `→ Snapshot` | 该文档当前内容（含未保存编辑）；`Snapshot = { id, text, version, state: DocumentInfo }` |
| `Update(documentId, text)` | `→ void` | 页面 compose 出的全文 → 该文档 `doc.Set`；冲突状态下仍写入，保存时由 MyGo 询问 |
| `Flush(documentId, text)` | `→ void` | 保存/关闭/另存为该文档前送达最后一次未去抖的编辑文本；宿主据此使用该文档内容而不是活动标签 |
| `Save` / `SaveAs` / `Rename` / `MoveTo` / `Revert(documentId)` | `→ void` | 作用于该文档；原生对话框以工作区窗口为父；取消返回 `mygo.ErrCanceled`。生成中的文档禁用 Save As / Rename / Move |
| `WatchView` / `UnwatchView` / `UndoView(documentId)` | `→ void` | 见「视图文件跟随」。`UndoView` 把上一次生成前保留的 `.<name>.genui.prev.json` 改名回视图文件（一次，之后不再有备份）；没有备份或正在生成时拒绝；页面经 `viewChanged` 看到 |
| `SetViewMode(documentId, mode)` | `→ void` | 把该文档的视图模式写入会话；与标签绑定，删除标签前不失效 |
| `SetNativeEditState(documentId, canUndo, canRedo)` | `→ void` | 只为聚焦窗口的活动文档报告编辑器撤销/重做可用性，背景面板不能偷菜单状态 |
| `Generate(documentId, req, progress)` | `GenerateRequest`、`Channel<GenerateProgress>` `→ void` | 经 ACP 为该文档运行一次生成（见「GenUI 生成」）；成功后视图文件已原子写入，页面经 `viewChanged` 看到。失败时最后一步是 `{phase: "failed", text, failure: GenerateFailure}`，再以 `message`（+ 空格 + `hint`）拒绝。页面关闭通道、关闭该文档、退出 app 都是取消 |
| `PrepareQuit()` | `→ void` | 每个工作区窗口在 `flushQuit` 后确认；全部确认后写 `session.json` 再继续退出 |
| `GetSettings()` / `SetSettings(patch)` | `Settings` / `SettingsPatch` | 进程级唯一 `settings.json`，见「生成设置」 |

### 事件

| 事件 | 载荷 | 说明 |
|---|---|---|
| `workspaceChanged` | `WorkspaceSnapshot` | 标签顺序、活动标签、根目录与每个标签的徽标的权威来源 |
| `docReloaded` | `Snapshot` | MyGo 因外部修改重读文件；`version` +1；带 `id` |
| `docState` | `DocumentInfo` | 该文档的 edited / conflict / deleted / readOnly / name / path / generating；背景标签只更新对应标签，不激活它 |
| `viewChanged` | `{ id } & ViewSnapshot` | 跟随期间视图文件被写入、替换或删除；文档改名/首次保存/Save As 后改跟随新路径并推送一次 |
| `navChanged` | `NavSnapshot` | 文件树增量快照；`status` 为 `idle`/`scanning`/`indexing`/`ready`/`error` |
| `navSearchReady` | `NavSearchResult` | 过滤搜索结果；索引未完成时 `incomplete: true` |
| `nativeCommand` | `NativeCommand { id, command }` | 原生 File/Edit 菜单与 `Cmd+W`、`flushQuit`，点击时捕获目标 `documentId` |
| `settingsChanged` | `Settings` | 设置写入成功后广播到**每个窗口**，见「生成设置」 |

`version` 从 1 开始，每次从磁盘重载 +1，不是编辑计数。`ViewSnapshot = { exists, text, error, canUndo }`，`canUndo` = 备份存在（监听同时看视图文件与备份）。

## HostAdapter / WorkspaceAdapter（`web/src/host/types.ts`）

```ts
interface ViewFileSource {
  // 首个快照在开始跟随后送达一次，之后每次变化送达；顺序见「视图文件跟随」
  watchView(fn: (snap: ViewSnapshot) => void): Disposable
  undoView(): Promise<void> // 回到上一次生成前的视图，一次
}
interface ViewGenerator {
  generate(req: GenerateRequest, onProgress: (p: GenerateProgress) => void, signal: AbortSignal): Promise<void>
  getSettings(): Promise<GenerationSettings>
  // 原子写入 patch 中出现的字段，返回提交后的设置
  setSettings(patch: SettingsPatch): Promise<GenerationSettings>
  // 跟随进程级设置存储（不是本文档自己的副本）；先订阅再首次读取
  onSettings(fn: (settings: GenerationSettings) => void): Disposable
}
interface HostAdapter extends ViewFileSource, ViewGenerator {
  readonly id: string // 稳定文档身份；生命周期内不变，永不随活动标签改变
  load(): Promise<HostDocSnapshot> // 含 id
  update(text: string): Promise<void>
  flush(text: string): Promise<void> // 保存/关闭/另存为前最后一次编辑器文本
  save(): Promise<void>; saveAs(): Promise<void>; rename(): Promise<void>
  moveTo(): Promise<void>; revert(): Promise<void>
  onReload(fn: (snap: HostDocSnapshot) => void): Disposable
  onState(fn: (state: DocumentInfo) => void): Disposable
  bindFlush?(getText: () => string): Disposable // 原生命令 flush 用
  setViewMode(mode: string): Promise<void> // 绑定 id
}
interface SettingsPatch { model?: string; agentCommand?: string }
interface WorkspaceAdapter {
  snapshot(): Promise<WorkspaceSnapshot>
  subscribe(fn: (snap: WorkspaceSnapshot) => void): Disposable
  open(path: string): Promise<DocumentInfo>
  openWorkspace(root: string): Promise<void>
  newDocument(): Promise<DocumentInfo>
  duplicate(id: string): Promise<DocumentInfo>
  activate(id: string): Promise<void>
  closeDocument(id: string, text?: string): Promise<CloseResult>
  closeWorkspace(): Promise<CloseResult>
  document(id: string): HostAdapter // 对打开的会话终身绑定
  navSnapshot(): Promise<NavSnapshot>
  subscribeNav(fn: (snap: NavSnapshot) => void): Disposable
  setFilter(query: string): Promise<NavSearchResult>
  subscribeSearch(fn: (result: NavSearchResult) => void): Disposable
  toggleFolder(rel: string): Promise<void>
  revealPath(path: string): Promise<void>
  pickFolder(): Promise<string>
  recentWorkspaces(): Promise<RecentWorkspace[]>
}
interface GenerationSettings { model: string; agentCommand: string; models?: string[] }
interface GenerateRequest { text: string; mode: 'full' | 'update'; replaceIds?: string[]; request?: string; format: string; sections: SectionRef[] }
interface SectionRef { heading: string; occurrence: number; start: number; end: number; exclude?: Array<{ start: number; end: number }> } // UTF-16 偏移
interface GenerateProgress { phase: 'starting' | 'thinking' | 'reading' | 'writing' | 'checking'; text: string }
// generate 总是以 GenerationError（web/src/host/generationError.ts，带 failure）拒绝
interface GenerateFailure { kind: GenerateFailureKind; message: string; hint?: string; detail?: string; models?: string[] }
type GenerateFailureKind = 'unsaved' | 'busy' | 'agent-not-found' | 'auth-required' | 'model-invalid'
  | 'agent-message' | 'invalid-output' | 'agent-error' | 'timeout' | 'cancelled'
```

- `mygoHost.watchView` 按 `documentId` 对多个监听者计数：首个监听者调用 `WatchView(id)`，最后一个释放时调用 `UnwatchView(id)`。
- `mygoHost.generate` 用 `mygo-runtime` 的 `Channel` 接进度，`signal` 中止时关闭通道（取消 Go 端的 ctx）。`failed` 步骤里的 `failure` 在调用被拒绝之前到达，变成 `GenerationError`；未知 kind 当作 `agent-error`，没有 failure 的拒绝也是 `agent-error`；中止的是 `cancelled`。
- 原生菜单：`nativeCommand` 事件按点击时捕获的 `id` 分派。Save / Save As / Rename / Move / Revert / Duplicate / Close 先 `flush` 该文档再调用对应 `WorkspaceHost` 方法；`closeWorkspace` 与 `flushQuit` 先 flush 本窗口全部文档（后者再 `PrepareQuit`）；`undo` / `redo` 交给 `nativeEdit.applyNativeEdit(id, …)`，不经过 `UndoView`。
- `createBrowserHost(text, name, { view?, kept?, generator?, settings?, untitled? })` 用内存视图文件；`host.writeView(text | null)` 模拟写入/删除；`generator(req, progress, signal) → Promise<视图文件文本>` 扮演 Agent（`update` 时也返回完整视图文件，合并只在 Go 端），抛 `GenerationError` 即扮演那种失败，其他错误为 `agent-error`；`host.generations` 记录收到的请求，`host.keptView()` 返回备份（测试用）。规则与 Go 端相同：未保存的文档（`unsaved`）、正在生成时（`busy`）都拒绝；只有成功才写视图文件，被替换的视图成为备份。`setViewMode` 只写视图模式、不推送文档状态（与原生只在 `workspaceChanged` 中反映一致），避免用旧的 `readOnly` 回显解锁只读文档。
- `createBrowserWorkspace(docs)` 是多文档浏览器适配器：一个 `BrowserSettings` 存储由它的所有标签共享（`HostAdapter.useSettings`），每个 `BrowserHost` 的生成开始时同步读取一次。`App` 接受 `{ workspace }` 或单个 `host`（内部包成单文档工作区），`DocumentPane` 接受 `{ host }`，是既有编辑器 / GenUI / Outline 测试的接缝。

## 插件契约（`web/src/core/types.ts`）

```ts
interface AppPlugin { id: string; title: string; activate(ctx: PluginContext): void | Promise<void> }
type ViewLocation = 'left' | 'center' | 'right'
type ViewStatusDot = { tone: 'warn' | 'danger'; title?: string } | null
interface ViewStatusSource { getSnapshot(): ViewStatusDot; subscribe(fn: () => void): () => void }
interface ViewRegistration { id: string; title: string; location: ViewLocation; status?: ViewStatusSource; mount(el: HTMLElement): Disposable }
interface PluginContext {
  documents: { snapshot(): { text: string; version: number; path: string }; onDidChange(fn): Disposable } // path '' = 未保存
  views:     { register(v: ViewRegistration): Disposable }
  commands:  { register(id, fn): Disposable; execute(id, ...args): Promise<unknown> }
  editor:    { onSelectionChange(fn: (offset: number) => void): Disposable }
  viewFile:  { watch(fn: (snap: ViewSnapshot) => void): Disposable
              generate(req, onProgress, signal): Promise<void>
              getSettings(); setSettings(patch: SettingsPatch): Promise<GenerationSettings>
              onSettings(fn: (settings: GenerationSettings) => void): Disposable }
  subscriptions: Disposable[]
}
```

- `PluginRegistry.enable(id)` / `disable(id)`：注册表追踪所有经 ctx 产生的注册与订阅，`disable` 统一释放；`activate` 抛错时回滚。`ctx.editor.onSelectionChange` 同样被追踪，禁用插件即移除其选择监听。
- `PluginRegistry(documents, views, commands, editor, viewFile)`：`editor` 是 `EditorSelectionSource`，App 用 `EditorAccess` 实现：注册表在编辑器创建前构造，编辑器就绪后 `attach(editor)`，之后才注册/启用插件；未 attach 时调用抛错（不会静默失效），App 卸载时 `detach()`。`viewFile` 是该面板的 host（`ViewFileSource & ViewGenerator`，`generate` / `watch` / `setSettings` 都绑定该面板的 `id`）；`ctx.viewFile.watch` 与 `ctx.viewFile.onSettings` 被追踪，禁用插件即停止跟随视图文件与设置。`documents.path` 由 App 按该文档的 `DocumentInfo.path` 更新。
- `ViewRegistration.location` 必填：`'left'` 进左侧栏（`SidePane`）；`'center'` 成为一个视图模式，与 `Rich text` / `Markdown` 并列出现在头部的三选一切换里，只在被选中时挂载、离开时卸载；`'right'` 进右侧栏（`RightPane`，顶部 `Outline` / `GenUI` 分段切换，可带 `status` 圆点提示，窗口宽度 `< 1180px` 时收起为浮层）。
- 编辑器提供的命令：`editor.reveal(offset: number)`（偏移越界时返回 `false`；当前是中央视图时先切回 Rich text 再跳转）、`editor.toggleSourceMode()`（在 Rich text 与 Markdown 之间切换，从中央视图出发则切到编辑器的另一模式；返回当前编辑器模式）。插件跳转一律 `ctx.commands.execute('editor.reveal', offset)`，不另设 reveal API。

## 视图模式（#9）

- 头部 `role="radiogroup"`（`View mode`），每个模式一个 `role="radio"` 按钮（测试钩子 `view-mode-richtext` / `view-mode-markdown` / `view-mode-<中央视图 id>`，`aria-checked` 表示当前模式）。`⌘/` / `Ctrl+/` 在 Rich text 与 Markdown 之间切换。
- 进入中央视图时编辑器只设 `hidden`、不销毁：ProseMirror / CodeMirror 状态、光标、撤销历史和未保存的修改保留；回到编辑器时恢复离开时的滚动位置。切换模式本身不调用 `host.update`。
- 当前模式随标签保存在 Go 的会话里（`SetViewMode(documentId, mode)` → `session.json`，由 `DocState.viewMode` 送回页面），页面另以 `localStorage` 的 `mgo.viewMode:<path>` 作为旧版本/重开路径的兼容回退。未保存的新文档不写 `localStorage`；重启恢复按 `documentId` 复原。

## 工作区、文档与标签（#21，ADR 0005）

- 一个文件夹 = 一个工作区 = 一个窗口。窗口顶部的文档标签在同一个窗口里切换原文；左栏始终是该文件夹的文件导航。
- 身份分工：`documentId`（Go `mygo.Document.ID()`，跨路径、标签、窗口、首次保存与 Save As 稳定）、`workspaceId`（窗口内标签集合）、`root`（文件夹路径，未命名窗口为空）、`path`（磁盘文件，未保存时为空）、标签（UI 导航项）。文档适配器在生命周期内绑定一个 `documentId`；异步调用、事件、视图跟随与生成都不经活动标签重定向。
- 同一份磁盘文件全应用只保留一个打开的编辑会话：从 Finder、File > Open、文件树或 `RevealPath` 重复打开只定位已有标签，不产生第二份待保存内容。根本身按解析符号链接后的规范化路径比较，`/tmp` 与 `/private/tmp` 这类别名不会分裂成两个工作区。
- 未命名窗口（`kind: 'untitled'`，`root` 为空）承载未命名原文；关掉最后一个标签即关闭窗口。绑定文件夹的工作区关掉最后一个标签仍保留窗口，中央显示空状态。
- 首次保存 / Save As 后：文档保留 `documentId`、内容、版本、视图跟随与生成槽；标签离开原窗口，按路由加入包含新路径的工作区（取根目录最深的已打开工作区，否则在父目录新开窗口）；同一未命名窗口里的其他未命名原文留在原处；目标路径已是另一份打开文档时拒绝保存，不静默覆盖。
- 标签栏：`role="tablist"`，每个标签 `role="tab"` 与 `aria-selected`，显示文件名（同名标签追加父目录名，如 `notes.md — alpha`）、未保存圆点、生成中、冲突、已删除、只读徽标与关闭按钮。方向键移动，`Delete` / `⌘W` 关闭当前标签。
- 生成中则禁用该文档的 Save As / Rename / Move（提示先取消或等待），普通保存、继续编辑与标签切换仍可用，其他文档不受影响。

## 工作区导航（#22）

- 左栏 `SidePane` 是工作区导航：`WorkspaceAdapter.navSnapshot()` 取当前窗口的 `NavSnapshot`（`root`、`status`、`indexed`、`total`、`error?`、按树顺序的 `entries: NavEntry[]`、`collapsedCount`），`subscribeNav` 接收增量。`status` 为 `idle` / `scanning` / `indexing` / `ready` / `error`；扫描与索引期间如实显示 `Indexing <indexed> of <total> files`，不把未完成的索引显示成“没有结果”。
- 打开工作区不等待整目录扫描，也不等待任何文档的 `WatchFiles`；`incomplete` 在索引就绪前一直为 `true`，`navSearchReady` 随后补齐结果。
- 文件树按深度优先排列：文件夹的展开子项紧跟在它后面，再排同级的文件与文件夹；文件夹行显示该目录下递归的 Markdown 文件数（`markdownCount`，与工作区根的总数分开），行内还可显示标题、预览、修改时间、批注数与 edited / conflict / deleted 状态。
- 过滤：`setFilter(query)` 80ms 防抖；`NavSearchResult` 的命中带标题、预览与章节命中（`hits[].headings`），点击章节命中经 `onJumpHeading` 激活文档并 `editor.reveal(offset)`。已打开文档的未保存文本也参与索引（`openTexts`），所以未保存的新标题同样可搜。
- 左栏可拖拽分栏（`SIDEBAR_DEFAULT` 起，带最小宽度），也可用键盘调宽；顶部 `PathCrumbs` 显示当前文档相对工作区根的路径，点击祖先目录展开、点击文件名打开。
- 文件树与标签栏的键盘：标签栏 `role="tablist"`，方向键在标签间移动，`Delete` / `⌘W` 关闭当前标签；树行可获得焦点并展开/折叠。
- Welcome（`data-testid="welcome"`）：没有文件夹工作区且没有标签时显示 `Open Folder`、`New Document` 与 `Recent Workspaces`（`RecentWorkspaces` 最多 12 条，经 `recentWorkspaces()` 读取）。已绑定文件夹但没有标签时显示 `Open a Markdown file from this folder.`（`workspace-empty`），窗口保留。
- 路由（Finder / File > Open / 文件树）见 IPC 的 `Open`：同一文件全应用一个标签，重复打开只定位；`RevealPath` 在树中展开并选中路径，不新建文档。

## 侧边栏、右侧栏与大纲插件（#4，#16 三栏布局）

- `web/src/components/SidePane.tsx`：渲染 `location: 'left'` 的注册（`aside.side-pane`，测试钩子 `left-pane`）；`web/src/components/RightPane.tsx`：渲染 `location: 'right'` 的注册（`aside.right-pane`，测试钩子 `right-pane`，顶部分段标签 `right-tab-<id>`，支持 `rightPane.show(viewId)` 命令与 `< 1180px` 窄窗浮层开关 `right-pane-toggle`）。视图挂载/卸载沿用 `mount(el)` / `Disposable` 语义，由 `ViewHost.tsx` 的 `ViewHost` 完成（中央视图同用）。
- Outline 插件（`createOutlinePlugin()`）注册视图 `outline.panel`（`location: 'right'`，标题 `Outline`），与 `genui.panel` 并列在右侧栏顶部分段标签中切换。
- 条目来自 `extractHeadings(ctx.documents.snapshot().text)`，按 `level` 缩进；文档变更经 `OUTLINE_DEBOUNCE_MS`（300ms，导出自 `OutlinePanel.tsx`）防抖后重新解析。
- 当前章节 = 最后一个 `offset ≤ 光标偏移` 的标题；光标在第一个标题之前时不高亮任何条目；光标偏移来自 `ctx.editor.onSelectionChange`（WYSIWYG 为顶层块起始偏移，源码为精确偏移），高亮条目标 `aria-current="true"`。
- 点击条目执行 `editor.reveal(该条目自身 offset)`，所以同文标题各跳各的位置；两种模式行为一致。
- 无标题时显示 `No headings in this document.`（`data-testid="outline-empty"`）。
- GenUI 不自带标题列表；在 GenUI 模式下点大纲条目会切回 Rich text 并跳转。
- `App({ workspace | host, plugins })` 在 GenUI / Outline 之后为每个 `DocumentPane` 追加注册插件（自动化测试注入异步激活插件用），注册与启用逐项检查 `disposed`。

## GenUI 视图模式（#9，取代 #6 的右侧栏与 v1 格式）

- 位置：文档 `<dir>/<name>.md` 的视图文件是同目录隐藏文件 `<dir>/.<name>.md.genui.json`；未保存的新文档没有视图文件。页面不直接读文件，只经 `ctx.viewFile.watch`；超过 1 MiB 或读失败作为 `error` 送达。
- 按需：插件注册两个视图：中央视图 `genui`（`location: 'center'`，纯只读 A2UI 组件卡片与来源引用底座）与右栏管理面板 `genui.panel`（`location: 'right'`，附带 `status: ViewStatusSource` 供右栏 `GenUI` 标签圆点提示）；任一视图首次 `mount` 时才 `import('./GenUIView')`（独立代码块，含 A2UI 与 zod）并开始引用计数的 `ctx.viewFile.watch`，两处视图都卸载时停止跟随文件。未进入 GenUI 时不加载该代码块，也没有 `WatchView` 调用。每份文档的 GenUI 状态、`GenerationController` 与 `ReadingStore` 各自独立，标签切换不共享。
- 视图文件跟随（#29）：`WatchView(documentId)` 先读并送出第一份快照（不等目录监听），再 `WatchFiles` 建监听，随后再送一份追赶快照；中间没有“已读但未监听”的空档，监听无法启动时错误仍由 `WatchView` 返回，已送出的第一份快照不撤回。首次进入 GenUI 早于视图文件可读时显示加载状态而不是空白；视图文件被写入、替换或删除时以 `viewChanged` 更新。
- 阅读状态（#27）：`ReadingStore` 按文档保存 KeyPoints 折叠、ComparisonTable 排序与横向滚动位置、Tabs 选中页；离开 GenUI 再回来、或替换其中一张卡片都不会重置其余卡片。保存的 Tabs 选中页只在首次渲染后恢复一次，之后的用户点击不会被覆盖。它从不写文档。
- 生成槽（#24）：每份文档最多一个任务，不同文档可并发生成；设置、文本、路径与生成要求在开始时捕获，结束的运行若文档已关闭或路径已变则拒绝写入。生成中标签显示 `doc-tab-generating-<id>` 圆点，关闭该标签取消该任务，切换标签不取消。
- 中央 `genui` 视图仅渲染只读 A2UI 组件表面（每张内容卡片带 `data-component-id` 与 `SourceFrame` 底座：`From`、`Source changed` / `Source not found`、`Edit in source`、`Note`），不包含生成按钮、生成要求输入框、模型选择或错误横幅；无视图或视图文件不可用时显示安静的空状态/说明并提供按钮经 `rightPane.show('genui.panel')` 打开右栏 `GenUI` 面板。
- 右栏 `genui.panel` 管理面板分为滚动区（`.genui-panel-scroll`）与底部固定区（`.genui-panel-footer`）：
  - 顶部状态与唯一主操作：五种状态互斥显示（无视图 `No GenUI view yet` + `Generate`；全部最新 `Up to date` + 次要 `Regenerate all`；存在原文已改组件 `N components out of date` + `.changed-list` 点击使中央滚动定位并高亮对应 `[data-component-id]` 卡片 + 主按钮 `Regenerate changed (N)` + 次要 `Regenerate all`；生成中进度 + `Cancel`；生成失败错误框 + `Retry` / `Dismiss`）。
  - 生成要求（`Focus`）：有视图时默认只读显示（空时 `No focus set`），点击 `Edit` 原地展开 `.composer` 白纸输入块（`<textarea aria-label="Generation request">`，支持 `⌘↩` / `Save` 保存、`Escape` / `Cancel` 取消）。
  - 底部固定区（钉在滚动容器外）：常驻只读 `Agent: Antigravity (agy)`、`ModelMenu`（只写 `{ model }`）、生成时间 `Generated <generatedAt>` 以及 `canUndo` 时按需显示的 `Undo` 按钮。`agent-not-found` 失败时错误块另给 `Locate agent…`，原地展开 `.composer` 的 `<input aria-label="Agent command">`（`Enter` / `Save` 只写 `{ agentCommand }`，空值恢复默认 `agy`，`Escape` / `Cancel` 取消）。两个控件都只提交自己那一个字段，不覆盖另一个或握手表里的模型列表。
  - 设置传播：面板先订阅 `onSettings` 再首次 `getSettings`，异步读取被版本守卫，只接受有序的 `settingsChanged` 公告。任何窗口的写入都会让空闲面板显示当前全局值，无需重新挂载；本文档正在运行的任务仍使用开始时捕获的值。常规界面没有字体、字号、主题或系统提示词偏好设置——这些不是缺失的控件，而是本产品不提供的设置（浅色界面与固定字体由 ADR 0006 确定）。
  - 右栏顶部分段标签状态圆点（`.rdot`）：生成失败时 `tone: 'danger'`，存在 `Source changed` / `Source not found` 组件时 `tone: 'warn'`，其余为 `null`。
- 格式唯一说明：`web/src/plugins/genui/view-format.md`（`version: 2`；`generatedAt?`、`model?`、`request?`、`sources{ id: { heading, occurrence?, hash? } }`、`a2ui[]` A2UI 1.0 消息）。测试直接渲染其中的 JSON 示例，文档与实现不会分叉。
- 外层解析（`viewFile.ts` 的 `parseViewFile`）：非 JSON、非对象、版本不是 2、缺 `a2ui` 数组、`sources` 条目不合法 → 一条 `role="alert"` 消息；`version: 1` → `This view was made by an older version of Mark Go Go. Generate it again.`（不迁移）。
- A2UI 适配（`a2ui.tsx`，唯一 import `@a2ui/*` 的模块，版本精确锁定 0.15.0）：组件目录 `markgogo.dev:genui/v1` = 基础目录的 `Column` / `Row` / `Tabs` / `Card` / `Text` + 内容组件（见 #11）。`MessageProcessor` 以严格校验运行（必须有 `root`、引用必须存在、属性按 zod schema 严格校验、嵌套约束、只允许 `createSurface` / `updateComponents`、只能一个 surface）；任一失败 → 整个视图一条错误，写明组件和原因（嵌套违规以 `UNALLOWED_PARENT:` / `UNALLOWED_CHILD:` 开头）。目录外的组件名在加载前替换为占位组件，只在该处显示 `“<名字>” is not a component this app can show.`，其余照常。
- `Text` 的 Markdown 由 `SafeMarkdown.tsx` 用应用自己的 mdast 解析器渲染为 React 元素：粗体、斜体、删除线、行内代码、代码块、列表、引用；链接只显示文字（`.genui-link`），图片和原始 HTML 不显示。没有 `innerHTML`。（`@a2ui/markdown-it` 0.2.2 的 peer 依赖是 web_core ^0.14，与 0.15 冲突，未采用。）
- 视图里的交互（切换标签页等）只改变显示，不调用 `host.update`。
- 章节（`sections.ts`）与 `fnv1a32` 哈希不变。
- 有视图文件时右栏底部显示 `Generated <generatedAt>`；`Model` 控件显示当前设置里的模型（视图文件自带的 `model` 字段不另外呈现，它记录的是生成当时的模型）。

## GenUI 来源状态与“去原文修改”（#10）

- `sources.ts` 的 `SourceStore`：视图文件的来源表对照当前文档。文档变化后防抖 300ms（与大纲相同）重新提取章节；状态 `fresh` / `changed`（哈希不同）/ `missing`（找不到 heading + occurrence，优先于 changed）/ `unknown`（无记录哈希的手写视图，或组件没有来源）。
- `SourceFrame.tsx`：所有内容组件都画在同一个框里：底部 `From <标题…>`、状态标记 `Source changed` / `Source not found`（只在这两种状态显示）、`Edit in source` 按钮与 `Note` 入口。状态写在 `data-source-status`。组件经 `useComponentSource(sources)` 取状态与 `edit(text?)`。
- 部分来源丢失（#28）：卡片仍有来源存在时保留 `Edit in source` 与 `Note`，用第一个仍存在的来源；全部来源都失效时才禁用这两个动作。显示的状态仍如实写出缺失的来源，不用不存在的来源静默替代。
- `Edit in source`：取组件第一个仍在文档里的来源，先立即刷新章节（不用防抖中的旧偏移），再执行命令 `editor.select(start, end)`。带 `text` 时（Checklist 项）在各来源节里找这段原文，找到就只选它，否则选第一个来源整节。App 切到 Rich text、选中，并在编辑器区显示 `← Back to GenUI`（`data-testid="return-to-view"`）。点它回到 GenUI；在头部选任一模式后这个按钮消失。
- 滚动位置：GenUI 视图的挂载元素 `.center-view` 就是滚动区。位置按视图文件文本记在插件里（`ScrollMemory`），跨挂载保留；同一份视图文件回来时恢复，换了视图文件从顶部开始。A2UI 首次提交后才渲染组件树，所以恢复时跟随内容增高（`ResizeObserver`，最多 1s）。
- 生成器（#7）按页面给的章节范围写入每个来源的 `hash`；手写的视图文件不带 `hash` 时不显示状态。

## GenUI 组件目录（#11）

- 内容组件（`components.tsx`，共用 `common` 属性 `sources?` / `weight?` 与 `title?`，`.strict()`，不含子组件）：`KeyPoints`（`items`）、`MetricGrid`（`metrics[{label, value, unit?, change?, trend?: up|down|flat}]`）、`ComparisonTable`（`columns` ≥ 2、`rows` 每行与列数相同；列头是真正的按钮，可点击也可用键盘操作，升序 → 降序 → 原顺序，数字按值比较，`aria-sort` 跟随当前列，只改显示）、`Timeline`（`events[{when, title, detail?}]`）、`Checklist`（`items[{text, done?, quote?}]`；`role="checkbox"` + `aria-readonly`，点一项经 `edit(quote ?? text)` 选中原文，从不改原文；全部来源缺失时禁用）。
- 嵌套约束（布局组件上声明）：`Row` 只能在顶层、`Column`、`Tabs` 里；`Tabs` 只能在顶层或 `Column` 里；`Card` 只能装 `Column`、`Text` 或内容组件。占位组件 `MgoUnknown` 被所有约束接纳，并从错误信息里去掉。
- `catalogDefinition()`：A2UI 1.0 目录文档（`Catalog.toJson()`），列出 10 个组件的 JSON Schema（含字段说明）与 `allowedParents` / `allowedChildren`，不含占位组件和函数；约 9 KB。#7 把它和格式文档一起交给 Agent。
- 排版：`Row` 在 GenUI 里换行，每项至少 260px；`.genui-block` `min-width: 0` 与 `overflow-wrap: anywhere`；宽表在自己的框里横向滚动。
- 主题（ADR 0006）：本轮只有浅色界面，`styles.css` 的 `:root` 声明 `color-scheme: light`，没有外观切换入口；应用也没有读取 `prefers-color-scheme`。GenUI 的 `--genui-*` 映射到应用 Token（`--paper` / `--code` / `--ink*` / `--accent` / `--content-warning` 等），因此 GenUI 与外壳、编辑器共用同一套浅色材质。样式表里仍保留一份未被任何界面启用的 `.dark` Token 块，它不是受支持的外观，不要据此声称深色模式。字体（IBM Plex Sans / IBM Plex Sans SC、Barlow Semi Condensed、IBM Plex Mono）与图标都是仓库内 `.woff2`，经 `web/src/assets/fonts.css` 离线加载，不请求外部来源。
- 格式文档 `view-format.md`：每个组件一节，示例用到全部 10 个组件（合约测试：示例无错误渲染，且组件集合等于目录）。

## GenUI 生成（#7，ADR 0003）

- 页面（`generation.ts` 的 `GenerationController`，每份文档一个，随 `DocumentPane` 生命周期，跨 GenUI 挂载保留）：点 Generate 时取该文档的 `documents.snapshot().text`（含未保存修改）和它的章节（共享章节模块，UTF-16 偏移），`format` = `view-format.md` + 「Catalog definition」JSON（`catalogDefinition()`），`request` = 生成要求（去首尾空白，空则不发）。同一文档一次只跑一个，不同文档互不阻塞；运行中 GenUI 显示进度（`role="status"` 最新一步 + 已完成的步骤列表）和骨架，旧视图暂时隐藏；只剩 Cancel。生成期间可以离开 GenUI 或切换标签继续编辑。
- 生成要求输入框预填当前视图文件的 `request`；未保存的文档 Generate 禁用并提示 `Save the document first: the view is stored next to it.`。失败时 `role="alert"` 显示 `message`、`hint`（该做什么）、`model-invalid` 的可用模型列表、`detail`（「The agent said:」下原样的 `<pre>`），按钮：`Retry`（用失败那次的请求重跑）、`Locate agent…`（仅 `agent-not-found`，原地打开 `.composer` 的命令输入框，`Enter` / `Save` 写入 `{ agentCommand }`，即 ADR 0004 的定位动作）、`Dismiss`；旧视图不变。`Cancel` 只在运行中出现；取消不算失败、不提示。
- 失败分类（Go `failure.go`，`GenerateFailure.kind`）：`agent-not-found`（找不到或启动不了命令）、`auth-required`（任一调用的 JSON-RPC -32000 或含 “authentication required”，包括 stderr；提示 ``Run `<命令> acp --login` in Terminal, then retry.``）、`model-invalid`（`set_config_option` 失败且 `session/new` 列出了模型）、`agent-message`（这一轮结束但没有 `out.json` 而有回复，或 `session/prompt` 返回错误；`detail` 为原话，最多 4000 字符）、`invalid-output`（缺失、过大、符号链接、非 JSON、`a2ui` / `sources` 不合格；有回复时附为 `detail`）、`agent-error`（Agent 提前退出附 stderr、其他 RPC 错误、越界工具调用、内部错误）、`timeout`（默认 5 分钟，`generationJob.Timeout`）、`cancelled`、`unsaved`、`busy`。超时与取消同样 `session/cancel` → 2 秒 → 杀进程 → 删临时目录。
- 退出：关窗口取消调用的 ctx；退出 app 时 `main` 在 `App.Run` 返回后 `waitForGenerations(5s)`，等每个运行结束 Agent、删除临时目录再退出进程。
- 模型选择与设置（#17/#24，ADR 0004）：面板底部固定区展示只读 `Agent: Antigravity (agy)` 与向上弹出菜单 `ModelMenu`（`.sel-btn` 26px 凸起触发钮 + `.mmenu.pop` 10px 圆角白纸弹层 + `Agent’s choice` 与缓存的 `models` 列表 + 右侧勾号 `.chk`）；点选模型后只提交 `{ model }` 并立即经 `setSettings` 持久化、关闭弹层，支持点击外部或按 `Escape`（`document` 级监听）关闭、生成中禁用。`agent-not-found` 错误里的 `Locate agent…` 只提交 `{ agentCommand }`。常规界面不暴露自由文本 `Model` 输入框，`Agent command` 输入框只在定位失败时原地出现。
- 宿主（Go）：临时目录里只有 `source.md`；`<agentCommand> acp`，无其他参数；`initialize` → `session/new` → `set_config_option(model)`（模型非空时）→ `session/prompt`。权限与工具调用按同一条路径规则检查（读 `source.md`、写 `out.json`、思考），越界即取消本次运行。`out.json` ≤ 1 MiB、普通文件、JSON 对象、非空 `a2ui`、合法 `sources`；宿主写 `version: 2`、`generatedAt`、`model`、`request`、带 `hash` 的 `sources`、原样的 `a2ui`，丢弃其他字段，原子替换视图文件；临时目录总是删除。进度：`starting`（启动、Agent 就绪）、`thinking`、`reading` / `writing`（工具标题）、`checking`。
- 共享测试向量 `web/test/fixtures/section-vectors.json` 增加了章节的 UTF-16 `start` / `end` 与标题规范化（`normalize`），TS 与 Go 两边都用它验证。

## 生成设置（进程级，`settings.go`）

- 唯一存储：`settings.json` 在 `PathUserData`，属于进程而不是某份文档或某个窗口。默认 `{ model: "gemini-3.8-flash", agentCommand: "agy", models: [gemini-3.8-flash, gemini-4-argon, claude-opus-5-5] }`；每次 ACP `session/new` 握手成功后把 `configOptions` 的模型列表去空去重后写入 `models`。
- `SetSettings(p SettingsPatch) (Settings, error)` 是原子意图补丁：只有补丁里出现的字段被写入；空 `model` 表示让 Agent 自选，空 `agentCommand` 恢复默认命令；缓存的 `models` 永远不能由补丁写入（只属于握手）。成功写入返回提交后的 `Settings`，并在存储锁内按提交顺序向每个窗口广播 `settingsChanged`。
- 两个窗口同时改不同字段时两处都保留：Model 选择不会覆盖自定义命令，定位 Agent 不会覆盖模型。一次运行在开始时捕获一次设置，之后的写入不影响该次运行。
- 面板先订阅 `onSettings`、再 `getSettings`，异步读取带版本守卫，只应用有序公告；因此空闲面板在别的文档改设置后也显示当前全局值，不需要重新挂载，也不会把写入的回声当作新状态。
- 常规界面没有字体、字号、主题、系统提示词一类的通用偏好设置；这些能力本产品不提供（ADR 0006 固定浅色与离线字体），不计为未实现的控件。

## 原生菜单、撤销重做与会话恢复（#21、#23、#30）

- File 菜单的 Save / Save As / Duplicate / Rename / Move / Revert 作用于**聚焦窗口的活动文档**（MyGo 文档组），点击时捕获该窗口的 `documentId`；`Cmd+W` / File > Close 关闭当前文档标签而不是窗口；窗口关闭按钮 / File > Close Window 调 `CloseWorkspace`（逐个询问脏文档，取消保留窗口与全部标签）；退出向每个工作区窗口发 `flushQuit`，每个窗口送完最后一次编辑并调 `PrepareQuit`，全部确认后才写 `session.json` 并继续退出。`AskOnQuit` 保持未设置，未保存字节仍由 MyGo 恢复副本保护。
- Edit > Undo / Redo（`nativeCommand` 的 `undo` / `redo`）按点击时的活动 `documentId` 应用到**该文档编辑器的历史**，与 `UndoView`（视图文件撤回）无关。`nativeEdit.ts` 维护每文档的目标（ProseMirror 或 CodeMirror），聚焦的原地 Composer `textarea` 保留原生字段撤销；只读 GenUI / ComparisonTable 的焦点不改写文档。`SetNativeEditState(documentId, canUndo, canRedo)` 只为聚焦窗口的活动文档更新菜单可用性。
- 会话文件：`session.json` 在 `PathUserData`，记录未主动关闭的工作区（`id`、`root`、按顺序的 `tabs[{id, path, viewMode}]`、`activeId`），不复制未保存字节。布局变化（打开、移动标签、激活、关闭、切换视图模式）时写入，退出确认后再写一次；恢复期间跳过写入，避免用半恢复的窗口覆盖已存布局。空的无标题窗口（Welcome）不存；关闭的标签或工作区不复活。
- `flushQuit` / Save / Close 的文本来源是**该面板的核心文档状态**：宿主载入的字节，加上编辑器随后送达的每一次编辑。未挂载编辑器的面板不注册 flush，所以退出或保存不会把未编辑的文档变成空缓冲；用户确实清空文档时，按普通编辑送达 `""`。
- 恢复时从磁盘打开的文档会得到新的 MyGo `documentId`，因此会话里存的 active tab 要映射到该文档**现在真实**的 id 再设为活动标签；绝不使用没有任何打开文档的 id。恢复后当前标签应立即有活动编辑器与选中状态。
- 恢复：`restoreAfterStart` 在 MyGo 打开恢复副本与启动参数文件之后、创建未命名窗口之前执行；复用恢复副本而不是再次按路径打开，从不重放 Agent 任务。缺失的根或文件变成 `WorkspaceSnapshot.restoreError` / `DocumentInfo.restoreError`，其余工作区照常恢复；本次启动显式打开的文件在布局放好后获得焦点。

## GenUI 重新生成与撤回（#12）

- 页面：有视图时按钮是 `Regenerate changed (N)` 与 `Regenerate all`；没有视图时是 `Generate`。N = 表面上 `sources` 状态为「原文已改」的组件数（随来源刷新）；N = 0 时 `Regenerate changed` 禁用。`Regenerate changed` 发 `mode: 'update'`、`replaceIds` = 这些组件 id；两种都带生成要求，失败后 `Retry` 按同一种方式重跑。有备份时显示 `Undo last generation`（生成中禁用）。
- 宿主（`update.go`）：局部生成前先读当前视图（版本 2、有 surface、`replaceIds` 非空且都是视图里的组件），否则不启动 Agent，`agent-error`「The current view cannot be updated: …」。临时目录里放 `source.md` 和 `view.json`（当前视图原样）；权限策略允许读 `view.json`，只允许写 `out.json`。提示词列出要替换的 id 和 surface。`out.json` 的 `a2ui` 必须是**恰好一条** `updateComponents`（只有 `version` 与 `updateComponents` 两个键），是视图的 surface，组件 id 只能是要替换的或视图里没有的，不重复；`sources` 只能新增，或与视图里同名的完全一致。任何一条不符合 → `invalid-output`，视图和备份不变。通过后这条消息追加到 `a2ui` 末尾，所有来源按本次文本重算哈希，`generatedAt` / `model` / `request` 取本次。写入前再读一次视图文件，如果 Agent 工作期间被改过，不写入（`agent-error`）。
- 备份：每次成功写入新视图（全部或局部）前，把当前视图原子写到 `.<name>.genui.prev.json`，权限与视图相同；写视图失败时把备份恢复原样。生成失败不动备份。撤回 = `rename(prev, view)`，只有一份。

## 批注（#13，#19 原地 Composer）

- 格式：一个单独的顶层 HTML 注释块 `<!-- note: 文字 -->`。`note` 后面可以有属性 `key="value"`，然后是 `:`、文字、`-->`。语法只在 `web/src/markdown/notes.ts` 里。同一行还有别的内容、不符合这个语法的注释、代码块或列表里的注释都不是批注。本阶段只写文字，不写属性。
- 转义：文字里的 `-->` 写成 `--\>`，`--!>` 写成 `--\!>`（浏览器也把它当注释结束）；已经带反斜杠的再加一个，读回时去掉一个，所以任何文字都读回不变。文字可以多行，读写时去掉首尾空白。
- 编辑器：批注是锁定块（`kind: 'note'`），按原字节写回。只有改过的批注才重新序列化，只改文字部分，冒号之前（包括不认识的属性）原样保留，并沿用这一块原来的间隔。
- 原地白纸输入块（`.composer.mgo-note-composer`，无 `<dialog>` 模态弹窗）：
  - Rich text：块菜单（`+` 或 `/`）里的 `Add note` 或已有批注上的 `Edit` 按钮通过 `noteDialog.ts` 的 `askForNote` 在目标顶层块紧下方原地展开 `<div class="composer mgo-note-composer" role="region" aria-label="Note composer">`（含随内容增高的 `<textarea aria-label="Note">`、底部 `.composer-foot` 提示 `⌘↩ to save · Esc to cancel`、`Cancel` 与 `Save` 按钮）；保存或 `Delete` 都是普通事务，可以撤销。只读文档不能操作。Markdown 模式显示原始文本。
  - GenUI：组件底座 `SourceFrame.tsx` 的 `Note` 按钮在卡片底座下方原地展开 React `.composer.mgo-note-composer`；`⌘↩` / `Ctrl+↩` 或点击 `Save` 时通过命令 `editor.addNote(offset, text)` 把批注加到第一个来源那一节的末尾，停留在 GenUI 且不使任何组件变为 `Source changed`；`Escape` 或 `Cancel` 关闭输入块不写入。宿主编辑器处在 Rich text 或 Markdown 都写出相同的字节。组件旁显示它所有来源节里的批注。只读文档会提示无法添加。
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

## 特殊块（可视呈现与原地 Markdown 编辑，ADR 0006）

- 种类（`segment.ts` 的 `LockKind`）：`yaml-frontmatter`、`github-alert`、`footnote-definition`、`link-reference-definition`、`code-fence-meta`、`raw-html`、`note`（批注，见「批注」）、`math`（`$$` 块）、`unsupported`（单片段解析结果 ≠ 1 个顶层节点或解析失败）。
- 呈现（`lockedView.ts` + `blocks/present.ts`）：Frontmatter 显示为键值表；GitHub alert 显示为带语义图标与类型文字（Note / Tip / Important / Warning / Caution）的提示块；脚注定义显示为编号与正文，并有返回引用的 ↩；带语言/元信息的围栏代码块显示语言标签、meta 与高亮代码；数学块显示 `Math` 与源码。`raw-html`、引用式链接定义与 `unsupported` 显示为 `Kept as written` 保真块，HTML 不执行。每个可视块仍是一个原子顶层节点，`data-kind` / `data-source` / `data-label` 保留原文。
- 原地编辑：非只读时，可视块上的 `Edit`（`data-block-action="edit"`，批注另有 `Edit` / `Delete`）在**该块原地**展开 `.composer.mgo-block-composer`（`<textarea aria-label="Markdown">`，内容是该块的原始 Markdown，`⌘↩` / `Save` 保存、`Escape` / `Cancel` 取消），不切换整份原文的视图模式、不弹 `<dialog>`。保存是一次普通编辑器事务，可以撤销；只读原文不显示编辑入口，程序化点击也被拒绝。
- 字节规则：呈现不改任何字节；保存只重写被编辑块自己的 `source`，未触碰块、块间空白与换行保持原样（textarea 的 LF 会按原块的行尾映射回去，CR / CRLF 不被改写）；取消不调用 `host.update`。编辑过的块按 Milkdown 规则重写其内部文本（例如 setext → ATX），这与“未触碰块逐字节”的既有规则一致。

## 编辑器装配

CrepeBuilder features：`cursor, listItem, placeholder, linkTooltip, blockEdit, table, imageBlock, toolbar, codeMirror`（语言：TS/JS/JSON/Go/Python/Bash/Markdown）。未启用：`topBar, latex, ai`。源码模式用 CodeMirror 6 独立装配，不经过 Crepe。直接依赖锁定的 CodeMirror 包：`@codemirror/lang-markdown 6.5.2`、`@codemirror/language 6.13.1`、`@codemirror/state 6.7.6`、`@codemirror/view 6.43.15`、`@codemirror/commands 6.11.1`。

已移除的 Milkdown 内置插件（会破坏未改动块的字节一致）：`MILKDOWN_HEADING_ID`、`MILKDOWN_KEEP_LIST_ORDER`、`MILKDOWN_TRAILING`。

## 尚未实现（后续阶段）

搜索替换；同一原文的多个同步编辑视图；正式深色主题与外观切换（本轮只有浅色，ADR 0006）；字体、字号和系统提示词等通用偏好设置（本产品不提供，见「生成设置」）。
