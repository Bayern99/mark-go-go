# 验收结果（2026-11-06，原型）

环境：macOS arm64，Go 1.27.3，MyGo v0.7.4，Milkdown 7.23.0，Node 24。

## 自动化

| 检查 | 命令 | 结果 |
|---|---|---|
| Go | `go vet ./...` | 通过 |
| TS 类型 | `npm --prefix web run typecheck` | 通过 |
| 测试 | `npm --prefix web test`（真实 Chromium） | 12 个文件、131 项通过 |
| 打包 | `go tool mygo build` | `Mark Go Go.app` 12.3MB，dmg 4.0MB |

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
- 没有视图文件：显示 `No view yet.` 与禁用的 `Generate`。
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

## 未验证 / 已知问题

- Edit 菜单的 Undo/Redo 显示为禁用（WKWebView 自身 undo manager 为空，历史在 ProseMirror）。键盘 ⌘Z 不受影响；用鼠标点菜单撤销不可用。需要时让菜单经 IPC 调用编辑器命令。
- 自动化工具注入中文只落下首字，属注入方式限制；真实输入法已人工确认正常。
- 外部重载会重建编辑器：撤销历史和光标重置（已接受）。
- 编辑过的块按 Milkdown 规则重写（setext→ATX、表格补齐、块内 CRLF→LF）；插入/删除块后的块间空白回退为 `\n\n`。
- GenUI 视图目前只能手写或由其他工具写入；`Generate` 按钮在 #7 接入 Agent 生成。
- 大纲在 WYSIWYG 模式按顶层块粒度高亮章节：光标在同一顶级块内移动不会改变当前章节。
- `markdownUpdated` 每次编辑都会在内部整篇序列化（仅耗时，不影响保存内容）。
- 纯 `go build`/`go run` 不处理文件参数；须用 `go tool mygo dev|build`。

## 失败条件（触发 ADR 0001 重新评估）

- WebKit 升级或 MyGo 升级后 ⌘Z / 中文输入法回归。
- 任一未修改文档 compose 后非逐字节一致。
- 日常文档中锁定块比例过高。
