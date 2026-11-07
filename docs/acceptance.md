# 验收结果（2026-11-07，原型）

环境：macOS arm64，Go 1.27.3，MyGo v0.7.4，Milkdown 7.23.0，Node 24。

## 自动化

| 检查 | 命令 | 结果 |
|---|---|---|
| Go | `go vet ./...`、`go test -race ./...` | 通过 |
| TS 类型 | `npm --prefix web run typecheck` | 通过 |
| 测试 | `npm --prefix web test`（真实 Chromium） | 17 个文件、189 项通过 |
| 打包 | `go tool mygo build` | `Mark Go Go.app` 12.9MB，dmg 4.2MB |

测试覆盖：23 个语料文件 + 编辑用例零编辑 compose **逐字节一致**；改一段只改变该块字节；编辑后 compose 幂等；插件禁用释放视图/命令/监听（含编辑器选择监听），重新启用正常，`activate` 失败回滚。

## 源码模式（#3）

自动化（`web/test/source-mode.test.ts`、`web/test/app-source.test.tsx`，真实 Chromium）：

- 23 个语料文件：进入源码 → 文本与 `getMarkdown` 逐字节一致 → 不编辑退出 → 逐字节一致，且无 change 事件。
- CRLF、混合换行、无末尾换行、单独 CR：源码中编辑无关行，其余字节不变。
- 源码模式 undo/redo；切换保留/重建 WYSIWYG 的边界（未改动保留撤销历史，改动后以新文本重建）；切换不触发 change。
- 源码中 `reveal` / `onSelectionChange` 为精确偏移；切换模式后在新模式 reveal 旧光标偏移；源码 reload 会报告光标重置为新位置，切换回 WYSIWYG 只报告最终块偏移、不报告加载中间态。
- 源码中外部 reload 替换文本、保持模式并重置历史；即使 reload 文本等于进入源码前的 compose 文本，回到 WYSIWYG 仍强制重建（撤销不会复活 reload 前状态）；只读文档在源码中只读。
- App：头部按钮与 `⌘/`、`Ctrl+/` 均可切换；进入/退出无编辑时 `host.update` 从未调用；源码中改 frontmatter 并去掉 `[!NOTE]` 后 host 收到期望字节，回到 WYSIWYG 该块是可编辑引用而非锁定块，随后 WYSIWYG 编辑仍保留源码改动的字节；源码 undo；源码中 reload 保持模式；只读 host。

打包应用手动检查（2026-11-06，`go tool mygo build` 产物）：

| 检查 | 状态 |
|---|---|
| `⌘/` 在打包应用中切换源码 / WYSIWYG，焦点与滚动位置正确；不编辑来回切换后文件 sha256 不变 | 通过（自动化 UI 操作） |
| 源码模式内 `⌘Z` / `⇧⌘Z` 撤销重做（真实键盘） | 通过（人工） |
| 源码模式内中文输入法输入正常（真实 IME） | 通过（人工） |
| 源码模式编辑后自动保存：状态栏 Edited → Saved | 通过（人工） |

## 大纲（#4）

自动化（`web/test/outline.test.tsx` 在 App + `createBrowserHost` 接缝；`web/test/plugin-registry.test.ts` 在注册表接缝）：

- 视图注册的 `location` 是 `'left' | 'center'`：左侧视图进左栏，中央视图成为视图模式（见 #9）；没有右栏。
- 大纲对 `test/fixtures/outline.md`（ATX H1–H3、setext H2、代码围栏内的伪标题、重复标题）只列真实标题且层级正确。
- 点击每个条目（含第二个重复标题）把光标移到该标题：WYSIWYG 检查光标所在标题，源码模式检查 CodeMirror 精确偏移。
- 新增 / 重命名 / 删除标题在防抖（300ms）后更新列表。
- 条目缩进按标题层级递增（比较文字 `getBoundingClientRect`，不依赖内部属性）；标题内 `**粗体**` / `` `code` `` 显示为纯文本。
- frontmatter 内的 `# x` 与原始 HTML 块内的 `<h2>` / `# y` 不会出现在大纲。
- 点击条目会把标题滚动进视口并把焦点交给编辑器（WYSIWYG 与源码模式都验证），焦点不停留在大纲按钮上。
- 源码模式中把光标移到后续章节时，对应条目高亮；光标在第一个标题之前时没有高亮。
- 关闭后重新打开大纲：左栏与条目恢复，高亮继续跟随光标。
- 预注册左侧与中央视图后挂载侧栏：左侧视图只挂载一次，中央视图不会被侧栏挂载；只有中央视图时不渲染左栏。
- App 在插件激活未完成时卸载：无 unhandled rejection，激活完成后才产生的注册被丢弃。
- 移动光标高亮所在章节（`aria-current="true"`）。
- 无标题文档显示空状态。
- 关闭大纲插件：左栏消失、编辑仍写入 host；`PluginContext.editor.onSelectionChange` 监听随插件禁用被释放。
- GenUI 不包含自己的标题列表；在 GenUI 模式下大纲仍在左栏，点条目切回 Rich text 并把光标放到该标题。

## 视图模式与 GenUI（#9，取代 #6 的右侧栏与 v1 格式）

自动化（`web/test/genui.test.tsx`、`web/test/app-source.test.tsx`、`web/test/outline.test.tsx` 在 App + `createBrowserHost` 接缝；`web/test/sections.test.ts` 用共享向量；`web/test/side-pane.test.tsx`、`web/test/plugin-registry.test.ts`）：

- 头部三选一 `Rich text` / `Markdown` / `GenUI`；`⌘/`、`Ctrl+/` 在 Rich text 与 Markdown 之间切换。
- 没有视图文件：显示 `No view yet.` 与 `Generate`（#7 起可用）。
- `view-format.md` 的 v2 JSON 示例直接加载：替代编辑器占满中央，无错误，Tabs / Row / Card / Column / Text / KeyPoints 全部渲染，`Text` 里的 `**粗体**` 是 `<strong>`；切到 `Details` 标签页显示第二页。
- 在视图里切换标签页不调用 `host.update`，文档不变为 edited。
- 目录外组件（`Chart`）：只在其位置显示错误，其余照常渲染。
- `Text` 里的链接、图片、`<b>`：不产生 `a` / `img` / `b` 元素；链接只显示文字。
- 非 JSON、`version: 1`（提示「请重新生成」）、`version: 3`、缺 `a2ui`、引用不存在的组件：整个视图一条错误；写入、替换、删除视图文件时视图随之更新。
- 只在 GenUI 模式显示时跟随视图文件：启动时 0 个跟随者，进入 GenUI 1 个，离开归 0；离开期间写入的视图在再次进入时显示。
- Markdown 模式里的修改经过一次 GenUI 后仍在，并已写入 host；Rich text 的滚动位置在往返 GenUI 后恢复。
- 视图模式按文档记忆：重新挂载 App 后回到 GenUI。
- 章节范围与 `fnv1a32` 哈希与独立计算的向量一致（含 CJK、emoji、CRLF、重复标题、setext、代码围栏伪标题）。

前端体积（`npm run build`）：初始代码块 `index-*.js` 562.86 KB gzip（#6 时 563.5 KB）；GenUI 代码块 `GenUIView-*.js` 136.34 KB gzip + CSS 0.50 KB，只在进入 GenUI 时加载。

打包应用手动检查（2026-11-06，`go tool mygo build` 产物，`/tmp/mdws-smoke/view6/report.md`）：

| 检查 | 状态 |
|---|---|
| 头部三选一，切到 GenUI：格式文档示例占满中央，粗体、标签页、分栏、卡片、KeyPoints 正确 | 通过（自动化 UI 操作） |
| 切到 `Details` 标签页 | 通过（自动化 UI 操作） |
| GenUI 模式下点大纲 `Metrics`：切回 Rich text，光标在 Metrics 标题 | 通过（自动化 UI 操作） |
| 退出后重新打开：直接回到 GenUI | 通过（自动化 UI 操作） |
| 在磁盘上改写视图文件：未知组件只在其位置报错；写成 v1 显示「请重新生成」 | 通过（自动化 UI 操作） |
| 以上全部操作后 `report.md` sha1 不变 | 通过 |

## GenUI 来源状态与“去原文修改”（#10）

自动化（`web/test/genui.test.tsx` 在 App + `createBrowserHost` 接缝；`web/test/editor-offsets.test.ts`、`web/test/source-mode.test.ts` 在编辑器接缝）：

- 视图哈希由共享章节模块算出（与宿主将写入的一致）。外部改动一个章节：只有引用它的组件（含多来源组件）标 `Source changed`，其他不变；改掉标题：引用它的组件标 `Source not found`（优先于 changed），内容照常显示，`Edit in source` 禁用；恢复原文后标记全部消失。
- 无 `hash` 的手写视图：改动来源后仍不显示任何状态。
- `Edit in source` 引用第二个同名 `Summary`：切到 Rich text，选中的正是第二个章节（标题 + 正文），而不是第一个。
- 视图滚到下方点 `Edit in source`、在 Rich text 修改、点 `← Back to GenUI`：回到 GenUI，滚动位置误差 < 2px，被改章节的组件标 `Source changed`，其余不变；之后返回按钮消失。在头部选模式后返回按钮也消失。
- 编辑器 `select`：WYSIWYG 按整块选中区间（重复标题各选各的），越界返回 `false`；源码模式为精确选区并报告起始偏移，`from > to` 返回 `false`。

打包应用手动检查（2026-11-06，`go tool mygo build` 产物，`/tmp/mdws-smoke/view10/report.md`，视图哈希按格式文档独立计算）：

| 检查 | 状态 |
|---|---|
| 进入 GenUI：所有组件无标记，底部显示 `From <标题>` 与 `Edit in source` | 通过（自动化 UI 操作） |
| 滚到底部点 LATE 的 `Edit in source`：切到 Rich text，第二个 `Summary` 章节被选中，大纲高亮该条，显示 `← Back to GenUI` | 通过（自动化 UI 操作） |
| 在该段末尾输入 ` Revised.`：自动保存，文件末尾为 `Late summary. Revised.` | 通过（自动化 UI 操作） |
| 点 `← Back to GenUI`：回到原滚动位置，只有 LATE 标 `Source changed` | 通过（自动化 UI 操作） |
| 外部进程把 `# Metrics` 改成 `# Numbers`：版本 v2，METRICS 与 BOTH 标 `Source not found`，LATE 仍为 `Source changed` | 通过（自动化 UI 操作） |

## GenUI 组件目录（#11）

自动化（`web/test/genui-catalog.test.tsx`、`web/test/genui.test.tsx`，App + `createBrowserHost` 接缝）：

- 格式文档示例用到目录里全部 10 个组件，在 App 里无错误渲染（Summary：KeyPoints、Checklist、MetricGrid、粗体 Text；Details：ComparisonTable、Timeline）。
- ComparisonTable：点 `Size` 升序（9、12、180，按数值）→ 降序 → 原顺序；点 `Option` 按文字排序；`aria-sort` 跟随；`host.update` 从未调用。
- Checklist：`aria-checked` / `aria-readonly`；点 `Generation`（`quote: "Ship generation"`）切到 Rich text 并选中原文那一项，原文不变、未 edited；来源缺失时各项禁用。
- 属性写错：错误写明组件与 id（`'MetricGrid' (root)`）和原因（缺 `value`、`trend` 枚举、表格某行单元格数不对、空 `events`、多余属性 `checked`）。
- 嵌套违规：`Card` 里放 `Tabs` → `UNALLOWED_CHILD: …`；`Row` 里放 `Row` → `UNALLOWED_PARENT: …`；信息里没有占位组件名。
- 560px 与 1600px 宽：中央区与每个组件都不横向溢出，超长单词换行，宽表只在自己框内滚动。
- `color-scheme` 切到 dark：组件背景变暗、文字变亮。
- `catalogDefinition()`：组件列表等于目录、不含占位组件；带 schema（如 `MetricGrid.metrics` 的 `minItems: 1`、字段说明）与嵌套约束。

前端体积：初始代码块 563.26 KB gzip（未变）；GenUI 代码块 139.69 KB gzip（#9 时 136.34）+ CSS 1.67 KB。

打包应用手动检查（2026-11-06，`go tool mygo build` 产物，`/tmp/mdws-smoke/catalog11/report.md`，视图文件就是格式文档的示例）：

| 检查 | 状态 |
|---|---|
| GenUI Summary：两张卡片并排（KeyPoints、Checklist 一项已勾），下方 MetricGrid 带单位与 ▲ +4%；`Edit in source` 按钮样式正确 | 通过（自动化 UI 操作 + 截图） |
| Details：ComparisonTable 与 Timeline；点 `Size (MB)` 列头后为 Tauri 9 → MyGo 12 → Electron 180 | 通过（自动化 UI 操作 + 截图） |
| 点 Checklist 项：切到 Rich text，`Next steps` 一节被选中（该项文字不在原文，回退到整节），显示 `← Back to GenUI` | 通过（自动化 UI 操作 + 截图） |
| WebKit 深色：临时把 `:root` 改成 `color-scheme: dark` 打包，组件为深色背景、浅色文字（只验证了 GenUI；其余界面还没有深色样式） | 通过（截图），已改回 |
| 以上全部操作后 `report.md` sha1 不变 | 通过 |

## GenUI 生成（#7，ADR 0003）

自动化（`generate_test.go`：测试二进制自己充当假 agent，走真实 ACP 管道；`web/test/genui-generate.test.tsx` 在 App + `createBrowserHost` 接缝；`web/test/sections.test.ts` 与 Go 共用 `section-vectors.json`）：

- 运行：agent 只带 `acp` 参数，临时目录开始时只有 `source.md`；提示里有格式文档与读者的请求；模型经 `session/set_config_option` 选择；进度按阶段变化逐步上报。
- 成功：视图文件键顺序为 `version` / `generatedAt` / `model` / `request` / `sources` / `a2ui`；`a2ui` 原样、多余字段丢弃；`sources` 哈希由宿主按原文计算（与页面共享向量一致），找不到的标题不带哈希；新文件为 0644，替换时保留原权限。
- 权限请求：读 `source.md` / 写 `out.json` 允许，其余拒绝；agent 汇报的工具调用越界（shell、未知的调用）→ 停止运行、视图不变；只带 id 的 `tool_call_update` 沿用其 `tool_call` 的类型与路径。
- 失败（#8，每类断言 `kind` / `message` / `hint`，并检查视图不变、临时目录删除）：命令找不到、完整路径不存在 → `agent-not-found`；`session/new` 返回 -32000、提示里含 “authentication required”、stderr 里含 → `auth-required`（登录提示带配置的命令）；未知模型 → `model-invalid` 带模型列表；这一轮无 `out.json` 但有回复、`session/prompt` 返回错误 → `agent-message`，`detail` 原样（含换行的额度提示）；`out.json` 缺失 / 非 JSON / 超大 / 符号链接 / 缺 `a2ui` 或为空 / `sources` 格式错误 → `invalid-output`；agent 启动即退出 → `agent-error` 附 stderr；`reportFailure` 发出 `failed` 步骤并以 message + hint 拒绝。“同一窗口只跑一次”与“未保存文档”的 Go 端检查依赖窗口会话，没有单元测试（页面在运行中和未保存时本就禁用 `Generate`）。
- 取消与超时（`hangingRun`）：页面关闭通道（ctx 取消，关窗口与退出走同一条路）/ 300ms 超时 → agent 收到 `session/cancel`，进程已退出，临时目录删除，视图文件字节不变；超时文字 `Generation timed out after 5 minutes.`（默认值）。
- 页面：生成中显示步骤列表与骨架、隐藏旧视图、请求与设置禁用；生成中继续编辑，视图按发起时的文本生成；失败保留旧视图，显示 message、hint、原样的 agent 原话、模型列表，`Retry` 用同一请求重跑并可成功、`Open agent settings` 打开设置、`Dismiss` 关闭；失败时没有 `Cancel`；非 `GenerationError` 的错误显示为 agent-error；未保存文档禁用 `Generate`；`Agent settings` 改模型与命令并持久化。

前端体积：初始代码块 563.80 KB gzip；GenUI 代码块 144.21 KB gzip（#11 时 139.69）+ CSS 2.05 KB。

打包应用手动检查（2026-11-06，`go tool mygo build` 产物，真实 `agy acp` 1.5.2，模型 `gemini-3.8-flash`，`/tmp/mdws-smoke/gen7/report.md`）：

| 检查 | 状态 |
|---|---|
| 请求 `Compare the options in a table` → `Generate`：依次显示 Starting the agent / Agent ready / Thinking / File source.md …，只有 `Cancel` 可点 | 通过（自动化 UI 操作） |
| 约 3 分钟后渲染新视图：`OPTIONS` ComparisonTable，`From Options` 无过期标记；按钮变为 `Generate again`，请求预填 | 通过（自动化 UI 操作） |
| 视图文件键为 `version` / `generatedAt`（UTC）/ `model` / `request` / `sources` / `a2ui`；`sources.options.hash` 为 `fnv1a32:cebf17b8` | 通过 |
| `Generate again` 后在 agent 读完 `source.md`、思考中时点 `Cancel`：回到旧视图、无错误；`agy` 进程退出、`markgogo-genui-*` 临时目录删除、视图文件字节不变、无残留 `.tmp-*` | 通过 |
| 以上全部操作后 `report.md` sha256 不变 | 通过 |

#8 打包应用手动检查（2026-11-07，同一 `agy acp`，`/tmp/mdws-smoke/gen8/report.md`）：

| 检查 | 状态 |
|---|---|
| 设置模型 `gemini-9-nope` → `Generate`：提示 `The agent does not know the model "gemini-9-nope".`、该做什么、agy 实际提供的 5 个模型，按钮 `Retry` / `Open agent settings` / `Dismiss`，无 `Cancel`；之后无 `agy` 进程与临时目录 | 通过（自动化 UI 操作） |
| `Open agent settings` 改为 `gemini-3.8-flash` 并保存 → `Retry`：真实运行到 Thinking / File source.md | 通过（自动化 UI 操作） |
| 运行中 Cmd+Q（AppleScript quit）：退出前 `agy acp` pid 27608 与 `markgogo-genui-*` 存在；退出后 app、`agy` 进程、临时目录都不在，目录里无 `.tmp-*`，`report.md` sha256 不变 | 通过 |
| 生成中途点 `Cancel` | #7 已检查（同一路径） |
| 额度耗尽的模型触发额度提示（原样显示） | 未检查：当前账户没有额度耗尽的模型；同一路径由假 agent 的多行额度提示覆盖 |

## GenUI 重新生成与撤回（#12）

自动化（`update_test.go` 用同一个假 agent；`web/test/genui-generate.test.tsx` 的「GenUI regeneration」）：

- 局部：agent 目录开始时只有 `source.md` 与 `view.json`；读 `view.json` 允许、写它拒绝；提示里有要替换的 id、surface、生成要求；结果是原消息 + agent 的一条 `updateComponents`，来源全部按本次文本重算哈希并加入新来源；被替换的视图成为备份（权限与视图相同）。
- 越界整次拒绝（`invalid-output`，视图和备份不变）：改了未点名的组件、改了 `root`、两条消息、`createSurface`、另一个 surface、改动已有来源。视图没有那个 id、没有 id、没有视图 → 不启动 agent。agent 工作期间视图被改写 → 不写入、不留备份。
- 备份与撤回：全部重新生成也留备份；生成失败备份不变；撤回后视图 = 备份、不再有备份，再次撤回被拒绝。
- 页面：没有组件「原文已改」时 `Regenerate changed` 禁用；改了一节后变为 `Regenerate changed (1)`，发出 `mode: 'update'`、`replaceIds: ['metrics']` 和生成要求；更新后另一个组件保持原样；`Undo last generation` 回到旧视图后按钮消失；`Regenerate all` 发 `mode: 'full'`；失败保留备份，生成中撤回禁用。

打包应用手动检查（2026-11-07，真实 `agy acp`，`gemini-3.8-flash`，`/tmp/mdws-smoke/gen12/report.md` + #7 生成的视图）：

| 检查 | 状态 |
|---|---|
| Rich text 里把 Options 表格 Tauri 的 9 改成 0 → GenUI：`From Options` 标 Source changed，按钮 `Regenerate changed (1)` | 通过（自动化 UI 操作） |
| `Regenerate changed (1)`：进度 Starting / Agent ready / Thinking / File source.md / File view.json / File out.json，约 1 分钟后表格显示 Tauri 0、无 Source changed、`Regenerate changed` 禁用、出现 `Undo last generation` | 通过（自动化 UI 操作） |
| 视图文件：`a2ui` = 原 `createSurface` + 一条只含 `compare` 的 `updateComponents`；`sources.options.hash` 重算为 `fnv1a32:6a329a8c`；`.report.md.genui.prev.json` 与原视图逐字节一致；无 `agy` 进程与临时目录 | 通过 |
| `Undo last generation`：视图文件与原视图逐字节一致、备份删除；页面回到 Tauri 9 + Source changed + `Regenerate changed (1)`，撤回按钮消失 | 通过 |
| 备份权限：这次实测发现备份是 0644、视图是 0600，已改为备份沿用视图权限（Go 测试覆盖） | 修复后未在打包应用里重测 |

## 批注（#13）

自动化：`web/test/notes.test.ts`（格式、转义、Rich text 添加 / 编辑 / 删除 / 撤销、源码模式、只读）、`web/test/genui-notes.test.tsx`（从组件添加、显示、不标「原文已改」、`exclude` 传给宿主）、`web/test/compose-gaps.test.ts`（增删块后其余间隔不变）、语料 `22-notes.md`（逐字节往返）、`section-vectors.json` 的 `notes`（TS 和 Go）。

打包应用实测（2026-11-07，真实 `agy acp`，`/tmp/mdws-smoke/notes/report.md`：CRLF、不规则空行、无结尾换行、带属性的批注、代码块里的 `<!-- note -->`）：

| 检查 | 结果 |
|---|---|
| Rich text 显示 | 批注是标记，带 Edit / Delete；代码块里的注释仍是代码 |
| 编辑（写入 `Edited --> note`） | 磁盘上只有文字变成 `Edited --\> note`，`author` / `id`、CRLF、空行不变；显示读回原文 |
| ⌘Z | 文件回到编辑前的字节 |
| 块菜单 `Add note` | 批注落在打开菜单的块后面，其余字节（含 `\n\n\n`）不变 |
| 带批注的文档 `Generate` | 4 个来源的哈希与 Python 手工去掉批注后计算的一致；没有组件标 Source changed；Agent 没有把批注写进内容 |
| GenUI 组件 `Note` | 带子节的来源：批注落在子节之后；末节：文件仍无结尾换行；两次都只多出批注；没有组件标 Source changed；停留在 GenUI |
| Markdown 模式 | 显示 `<!-- note: … -->` 原始文本 |
| 回 Rich text 按两次 ⌘Z | GenUI 加的两条都撤销，文件回到添加前的字节 |
| Delete | 批注和它前面的间隔一起去掉，其余字节不变；GenUI 没有标 Source changed |
| #12 遗留：备份权限 | 视图 0600 时 `Regenerate all`：备份 0600、视图 0600，备份与旧视图逐字节一致；Undo 后视图逐字节恢复，备份删除 |
| 退出 | 没有 `agy` 进程，没有 `markgogo-genui-*` 临时目录 |

自动化说明：网页内容第一次读取时可能不在辅助功能树里，重读一次就出现。屏幕锁定时 macOS 隐藏所有窗口的辅助功能树，窗口也显示空白，驱动报告 `screen_locked`，解锁后才能继续（#12 时「连不上窗口」可能是这两个原因之一，未确认）。像素点击和键入不一定到达编辑器；AX 按大纲条目会把焦点交给编辑器，之后 ⌘Z、`/` 和方向键可用。`--` 经键盘注入会被系统智能破折号替换成 `—`，测转义时用 AX 直接设值。

## 打包应用手动冒烟（`/tmp/mdws-smoke/report.md`：frontmatter、alert、setext 标题、`+` 列表、CJK 表格、带 meta 的代码块、引用式链接）

| 场景 | 结果 |
|---|---|
| 通过 `open -a` 打开 `.md` | 窗口标题 `report.md`，内容正确渲染；frontmatter / alert / 代码块 meta / 链接定义显示为锁定块 |
| 打开后不编辑 | 文件 sha1 不变 |
| 输入文字 → 自动保存 | 状态栏 Edited → Saved；`diff` 只有新增的一行，其余字节不变（`+` 列表、setext 标题、`~~~` 代码块均保留） |
| 外部进程改写文件（模拟 Agent） | 自动重载，版本 v1 → v2，状态 Saved，编辑器显示新内容 |
| 退出 | AppleScript quit 正常退出，文件为外部写入的版本 |
| GenUI 视图模式 | 见 #9 的打包检查 |
| Outline 开关（人工） | 左栏标题列表随文档与光标更新，点击条目跳转；关闭后左栏消失 |
| ⌘Z 撤销（人工，真实键盘） | 正常。Edit 菜单 Undo/Redo 显示禁用但不拦截按键，按键到达 ProseMirror |
| 中文输入法（人工） | 正常 |

## 三栏工作区与 Portrait 界面语言（#15, #16–#19, v0.1.0）

自动化（`web/test/layout.test.tsx`、`web/test/side-pane.test.tsx`、`web/test/genui.test.tsx`、`web/test/genui-generate.test.tsx`、`web/test/notes.test.ts`、`web/test/genui-notes.test.tsx`）：

- **三栏布局与 Outline 迁移（#16）**：
  - `ViewLocation` 支持 `'left' | 'center' | 'right'`，`Outline` 注册在 `'right'` 且默认首选高亮。
  - 顶栏移除旧 `Outline: On/Off` 独立按钮，改由右栏切换条与窄窗响应式开关控制。
  - `< 1180px` 窄窗自动收起右栏为 `.r-float` 浮层，点击中央内容画布自动收起浮层。
  - 全局样式引入 Portrait 语义 Token（浅色/深色模式 `--chrome`、`--paper`、`--raised`、`--code`、`--ink`~`--ink-4`、5 级触觉阴影）。
- **ModelMenu 与模型列表持久化缓存（#17）**：
  - Go `GetSettings` 在未握手前回退默认模型列表（`gemini-3.8-flash`、`gemini-4-argon`、`claude-opus-5-5`）；ACP `session/new` 握手拿到非空 `configOptions` 时自动更新并持久化 `models`。
  - `ModelMenu` 实现 26px 凸起药丸按钮与向上展开的纸面菜单（`.mmenu.pop`），支持 `Agent’s choice`、当前项勾号，点击外部或按 `Escape`（`document` 级监听）均可关闭，生成运行中自动禁用。常规界面移除自由文本 `Model` 和 `Agent command` 输入框。
- **GenUI 视图拆分（#18）**：
  - 中央视图（`location: 'center'`）仅渲染纯净只读 A2UI 组件卡片与来源底座，彻底剥离生成工具栏与错误提示框。
  - 右栏 `genui.panel`（`location: 'right'`）承载 5 种顶部状态与唯一主操作按钮、点击 `Source changed` 列表项平滑滚动并高亮定位中央卡片、Focus 生成要求安静只读与原地编辑、以及固定底部元数据（`Antigravity (agy)`、`ModelMenu`、生成时间、`Undo`）。
- **原地白纸 Composer（#19）**：
  - 彻底移除居中模态 `<dialog class="mgo-note-dialog">`，Rich text 与 GenUI 卡片内写批注均原地展开自适应增高白纸卡片（`.composer`，`⌘↩` 保存，`Esc` 取消），保持原文逐字节保真，且不触发“原文已改”。

## 未验证 / 已知问题

- Edit 菜单的 Undo/Redo 显示为禁用（WKWebView 自身 undo manager 为空，历史在 ProseMirror）。键盘 ⌘Z 不受影响；用鼠标点菜单撤销不可用。需要时让菜单经 IPC 调用编辑器命令。
- 自动化工具注入中文只落下首字，属注入方式限制；真实输入法已人工确认正常。
- 外部重载会重建编辑器：撤销历史和光标重置（已接受）。
- 编辑过的块按 Milkdown 规则重写（setext→ATX、表格补齐、块内 CRLF→LF）；新块前面的间隔是 `\n\n`，它后面的块保留自己的间隔（#13）。
- GenUI 视图由 `agy acp` 生成（#7），失败分类与清理见 #8，局部重新生成与撤回见 #12。A2UI 校验不过的视图显示问题列表（`role="alert"`）和 `Regenerate all`；有备份时也可以撤回。
- 撤回只有一份备份；视图里已经是局部更新的组件，下一次局部更新后 `a2ui` 继续变长（不压缩）。
- Agent 隔离只是尽力而为（ADR 0003）：agy 按用户自己的 `~/.gemini/antigravity-cli/settings.json` 运行，可能不请求权限就写目录外文件或运行 shell。宿主看到越界工具调用就停止运行、不写视图，但无法阻止已经发生的那一步。
- 只有 GenUI 有深色令牌；应用其余部分（头部、大纲、编辑器）还没有深色主题，`:root` 固定为 `color-scheme: light`。
- ComparisonTable 的列头按钮在 macOS 辅助功能树里不出现（表格行可见），自动化只能用坐标点击。
- 多来源组件只要有一个来源 `missing` 就禁用 `Edit in source`，即使其他来源还在。
- 打开 `~/Downloads/SuperCollider 深度分析报告.md`（同目录 516 项，含子目录约 1.6 万个文件）首次切到 GenUI 时，中央区域空白一段时间后才显示视图；浏览器测试中同一文档与视图文件立即渲染。推测（未验证）是宿主在 `WatchFiles` 建好目录监听后才发送第一份视图快照（`viewfile.go` 的 `WatchView` 先 `startViewWatch` 再 `emitView`）。另外 GenUI 在 `loading` 状态什么都不显示，没有“加载中”提示。待查（#14）。
- 大纲在 WYSIWYG 模式按顶层块粒度高亮章节：光标在同一顶级块内移动不会改变当前章节。
- `markdownUpdated` 每次编辑都会在内部整篇序列化（仅耗时，不影响保存内容）。
- 批注加到父节末尾时，如果父节以子节结束，它也属于这个子节，所以两个组件旁都会显示。
- 编辑批注时光标放到文字末尾，这个改动只经过 typecheck 和打包，没有在 app 里实测（当时屏幕已锁定）。
- 真实键盘在开启系统智能破折号时，对话框里的 `--` 会变成 `—`；粘贴的 `-->` 按转义写入。
- 中文通过自动化工具写入批注未测（注入方式限制，见上）；组件和往返测试覆盖了中文批注文字。
- 纯 `go build`/`go run` 不处理文件参数；须用 `go tool mygo dev|build`。

## 失败条件（触发 ADR 0001 重新评估）

- WebKit 升级或 MyGo 升级后 ⌘Z / 中文输入法回归。
- 任一未修改文档 compose 后非逐字节一致。
- 日常文档中锁定块比例过高。
