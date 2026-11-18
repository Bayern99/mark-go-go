# 验收结果

本文件按阶段记录实际执行的检查。PR 合并前检查见下节。其余各节保留各自版本的原型与集成证据，被后续规格取代的条目已就地标注。

## MyGo v0.10.0 与官方 Automation（2026-11-18）

基线：多 Workspace 提交 `d729c17`；升级在独立分支进行。上游 v0.10.0 仍为一份文档一个窗口，因此保留并迁移受控 Document Group 补丁。

| 检查 | 判定 | 证据 |
|---|---|---|
| 上游与依赖迁移 | PASS | 标签 `v0.10.0` / 提交 `ebde939`；`go.mod`、`third_party/mygo`、`mygo-runtime` 均升级。Go API 无破坏性变更；新版拆分源码已补齐，两个三方合并冲突已解决。 |
| Go / MyGo 回归 | PASS | `go vet .`、`go test -race .`、`go test -tags e2e -race .`、`(cd third_party/mygo && go test -race . -run TestGroupClose)` 均通过。 |
| Web 回归 | PASS | 真实 Chromium 36 个文件、359 项通过；`npm --prefix web run typecheck` 与生产构建通过。 |
| 官方 CLI | PASS | `go tool mygo-automation version` 为 v0.10.0；`skills` 输出当前版本官方指南。 |
| 原生多 Workspace | PASS（macOS WebKit，非浏览器替身） | `scripts/native-automation-smoke.sh` 实际 Build → Launch → Inspect → Interact → Assert → Restart → Cleanup：三个 Workspace 共用一个窗口、Tab 往返、重复打开去重、视图和当前 Workspace 恢复。 |
| 原生编辑与菜单 | PASS（macOS） | 同一 Automation 脚本在 Markdown 视图插入文本，File > Save 实际写盘，Edit > Undo / Redo 改变真实编辑器历史，重启后保存内容仍在。 |
| 截图与三栏 | PASS | 官方 `screenshot` 生成 `/tmp/mgo-automation-smoke.png`；三个文档 Tab、当前 Workspace、Markdown 视图、中央正文与右栏正常。 |
| 正式版隔离 | PASS | 默认 `go list -deps .` 不含 Automation；普通 `mygo build` 成功。隔离正式 QA 应用设置 `MYGO_AUTOMATION` 后正常运行但未创建 socket。 |
| Save As / Rename / Move / 冲突系统对话框 | BLOCKED | 本轮没有用官方 Automation 完成这些系统面板内的原生操作；既有 Go 路径/冲突测试通过，但不据此宣称系统对话框验收通过。 |

## 单窗口多 Workspace（2026-11-17）

Workspace 菜单、Save As 和 Move To 的原生记录来自 `Mark Go Go Native Verify.app`（`dev.markgogo.editor.nativeverify`）。通过 Computer 在 macOS 操作应用菜单和系统文件面板，测试使用独立会话与临时 Markdown 文件。

| 检查 | 判定 | 证据 |
|---|---|---|
| 跨 Workspace 打开与往返 | PASS（独立 QA 应用，macOS WebKit） | 从 LaunchServices 依次打开 `Research/Discussion.md`、`Music Department/CC129.md`、`Personal Notes/Ideas.md`，始终只有一个主窗口；三个文档共享顶部 Tab，Tab 点击与真实 `Ctrl+Tab` 均切换正文和左栏。 |
| 重复路径与同一会话 | PASS（原生） | 以 `/tmp/...` 别名再次打开已恢复的 `/private/tmp/.../Discussion.md`，没有第二个标签；原生标题、页面与左栏均选择 Research。为此给受控 MyGo 增加 `DocumentFocused`。 |
| 重启与视图模式 | PASS（原生） | 两次退出并重开 QA 应用，三个根目录与标签恢复，退出前的 Personal Notes / Ideas.md 保持活动；Markdown 视图及编辑后的 `# EditedIdeas` 内容恢复。`session.json` 根目录记录含 `active` 标记。 |
| 窄窗 | PASS（原生，修复后） | 640×700 首次检查发现 WebKit 后台 resize 后右栏仍占位；改用文档真实 `clientWidth` + `ResizeObserver` 后复测，右栏收起、正文正常、三个 Tab 保留且溢出为 `+1`。 |
| 大量 Markdown 与资源过滤 | PASS | `TestLargeWorkspaceFiltersFiveHundredMarkdownFiles` 实际扫描 500 个嵌套 Markdown 与 JSON 资源；总数 500、目标搜索唯一命中、导航无非 Markdown 文件。 |
| Workspace 菜单与关闭范围 | PASS（原生 WebKit、Chromium） | 用 Computer 在原生应用内点击根目录切换、Open Folder… 和 Close Workspace。Atlas 有未保存副本时，取消关闭保留了全部 Tab。保存副本后再次关闭 Atlas，只移除了 Atlas 的导航和 Tab，Birch 与 Cedar 仍在同一窗口。通过 Open Folder… 重新打开 Atlas 后显示空工作区。Chromium 覆盖见 `multi-workspace.test.tsx`。 |
| Save As 原生保存面板 | PASS（原生） | 从 File → Save As… 将 `Atlas/Atlas.md` 另存到已打开的 Birch 根目录。活动 Tab 留在同一窗口，Tab 和窗口标题显示另存的文件名，左栏切到 Birch。源文件仍在，`cmp` 确认副本与源文件逐字节一致。 |
| Move To 原生文件夹面板 | PASS（原生） | 从 File → Move To… 将 `Atlas/MoveMe.md` 移到已打开的 Cedar 根目录。Tab 留在同一窗口，左栏切到 Cedar。磁盘上旧路径消失，目标文件的 SHA-256 与移动前一致。 |
| 自动回归 | PASS | Go 全量、vet、race 与 MyGo GroupClose race；Web typecheck、全量 Chromium、生产构建均通过。 |

### 原生回归修复复验（2026-11-17）

从当前源码构建 `Mark Go Go Regression QA.app`（`dev.markgogo.editor.regressionqa`），应用包通过签名校验。通过 Computer 操作 macOS 菜单与应用界面，测试使用独立会话和临时 Markdown 文件。

| 检查 | 判定 | 原生复现与结果 |
|---|---|---|
| 后台文件重载保留当前文档 | PASS | 打开 Birch、Cedar 和 Atlas，在 Atlas 活动时从磁盘修改 `Birch.md`。窗口标题、选中标签、正文和左栏均保持 Atlas。点击 Birch 标签后看到新增的 `Final native background check.`。 |
| 关闭标签后同步标题与 File 菜单 | PASS | 通过 `⌘W` 关闭 `Cedar-Second.md`，标题和选中标签都切到同根目录的 `Cedar.md`。关闭 Atlas 的最后一个标签后，标题和左栏显示 Atlas，正文显示空目录提示，Birch 和 Cedar 标签仍在。File 菜单的 Close、Save、Save As、Duplicate、Rename、Move To、Revert 全部禁用，再按 `⌘W` 窗口保留。切回 Cedar 后，可用的文档命令恢复启用。新启动的 Welcome 窗口也通过空状态菜单与 `⌘W` 检查。 |
| 带恢复副本的会话重启 | PASS | Birch 排在会话首位。暂时移走 `Cedar.md` 的磁盘文件，在 Markdown 模式输入 `Recovery copy single-window check.` 后退出，确认磁盘上有恢复副本。退出前分别选中 Birch 和 Cedar，两次重启后 macOS Window 菜单均只有一个文档窗口。三个根目录恢复，Birch 和 Cedar 标签同窗，活动根目录与退出前一致，Cedar 的未保存文字和 Markdown 视图完整恢复。 |
| 自动回归 | PASS | 应用 `go vet ./...` 和 `go test -race -count=1 ./...`、MyGo 包的 race 测试和模块全量测试均通过。Web 36 个文件、359 项测试通过。生产构建内含类型检查，macOS 应用包签名校验通过。Vite 仍报告既有的大 chunk 警告。 |

## 窄窗口、触控与节流网络 QA（2026-11-11）

本节记录发布前针对本轮 UI 改动的浏览器实测：宽度 1280 / 1024 / 768 / 620 / 560 / 480（应用最小窗口），Chromium CDP 触控仿真与约 400kbps、400ms 延迟的网络节流。发现两处实际问题并就地修复。

| 检查 | 判定 | 证据 |
|---|---|---|
| 侧栏在窄窗口挤压正文 | 已修复 | 480px 窗口下 248px 侧栏使 sheet 仅剩约 218px：头栏溢出、右栏开关不可点、正文逐字换行，Markdown 与 GenUI 视图同样挤坏。修复后侧栏让位至 170px、sheet 296px、头栏零溢出，各视图可用；700px 窗口内 420px 侧栏同样被压至 390px。 |
| 分隔条触控热区 | 已修复 | 6px 透明分隔条的外缘被中央栏覆盖（elementFromPoint 实测），鼠标与触控均无法从边缘拖动；热区扩至 14px（负边距 + `z-index` + `touch-action: none`）后，鼠标从外缘拖至 383px、CDP 触控拖至 420px。 |
| 节流 3G 加载 | PASS | 约 400kbps、400ms 延迟下 <800ms 完成渲染，无 FOUC 或破碎中间态；字体与图标随包加载，无外部请求。 |
| 其余宽度回归 | PASS | 1024 / 768 / 620 / 560 宽度下右栏按 `< 1180px` 断点浮层化，头栏与文档无横向溢出；1280 默认布局与基线一致。 |
| 触控可操作性 | PASS | 三选一分段、标签关闭、文件树行、触控拖动分隔条均可操作；仅分隔条热区需要修复（见上）。 |
| 自动检查 | PASS | `npm --prefix web run typecheck`；`npm --prefix web test` 真实 Chromium 31 个文件、304 项通过；`go vet ./...`；`go test -race .`。 |
| 原生 WebView 窗口几何 | PASS（`go tool mygo dev` 实际窗口） | 新窗口实测 1280×820；请求 400×300 被窗口最小尺寸钳制至约 482×362（`minWindowWidth/Height` 生效）。482×362 下侧栏让位、docbar 三选一切换与右栏开关完整可用、保存态退至底部状态栏；620×600 布局无溢出。 |

## 导航视觉方向与原生验收（2026-11-11）

| 检查 | 判定 | 证据 |
|---|---|---|
| 选定视觉方向 | PASS | 用户比较 A/B/C 后选定 B 的结构与图标、A 的拟物材质、C 的 `+N`；完整方案见[组件规划](design/ui-component-gaps.md)。 |
| 原生文件树与 Model 菜单 | PASS（macOS WebView 实际截图） | 桌面 App 显示 Phosphor folder/folder-open/file-text、凸起活动标签、`MODEL` 标题、Agent/模型图标、当前项勾号与细焦点态；没有 Unicode 黑色三角形。 |
| 真实文档命令菜单 | PASS（App + BrowserWorkspace 实际截图） | 12 文档时溢出按钮显示 `+8`；菜单有 8 条文件图标＋路径行，搜索框为细内描边，末项选择后正确激活。 |
| 全量前端验证 | PASS | 移除原型入口后执行 `npm --prefix web run typecheck && npm --prefix web test && npm --prefix web run build`；31 个文件、304 项通过。 |
| Go 集成 | PASS | `go test ./...` 通过；启动期导航索引 map 并发问题另以 `go test -race .` 验证通过。 |
| 原型清理 | PASS | `prototype/workspace-navigation` 分支提交 `ccc8c71` 保存三个方案；`main` 工作区没有原型目录或入口。 |

## UI primitives 与短时通知（2026-11-11）

| 检查 | 判定 | 证据 |
|---|---|---|
| 类型、全量测试与构建 | PASS | `npm --prefix web run typecheck && npm --prefix web test && npm --prefix web run build`；Chromium 31 个文件、304 项通过。构建完成，保留大于 500kB chunk 警告。 |
| Tooltip、文档菜单与 Model 菜单 | PASS（Chromium + 实际页面） | `workspace-tabs`、`genui-generate`、`genui-isolation` 覆盖溢出搜索、焦点恢复、键盘切换、Escape、外部点击、模型选择和跨文档设置。实际页面显示文件树完整路径、向上 Model 菜单，并成功切换模型；Portal 不影响标签测量。 |
| 复制通知 | PASS（实际页面） | 点击代码块 `Copy` 显示 `Code copied`；连续复制后仍为一个 `data-testid="copy-toast"`，页面只有一个 `data-sonner-toaster`。持续错误与生成状态未迁移。 |
| 键盘 Tooltip | PASS（实际页面） | 文件树行获得键盘焦点后显示唯一完整路径 `/demo/welcome.md`，无重复摘要文本。 |
| Go 集成 | PASS | `go test ./...` 通过。 |
| 滚动区域 | N/A | 用户明确排除；现有 `ScrollArea` 与 OverlayScrollbars 未改。 |
| 原生 WebView 检查 | BLOCKED | 本轮验证在真实 Chromium App/BrowserHost 完成，未运行原生 macOS WebView。 |

## 窗口、Mermaid 与工作区文字（2026-11-10）

本节记录本轮工作区改动；不以浏览器结果代替原生窗口几何验收。

| 检查 | 判定 | 证据 |
|---|---|---|
| 前端类型、测试、生产构建 | PASS | `npm --prefix web run typecheck && npm --prefix web test && npm --prefix web run build`；真实 Chromium 31 个文件、303 项通过。构建保留大于 500kB 的 chunk 提示，未调整阈值或压制警告。 |
| Mermaid 的原文保真与编辑 | PASS（Chromium） | `mermaid.test.ts` 覆盖有效图、无效图原文与错误、CRLF 原地修改与撤销、外部资源拒绝及只读入口；既有 roundtrip 与特殊块套件一并通过。 |
| Mermaid 实际工作台 | PASS（浏览器） | 在 App + BrowserWorkspace 接缝同时显示中文流程图和时序图；Save 只改目标块，Undo 恢复全文，Cancel 不写入，Markdown / 预览切换保留全文；最终串行渲染版再次显示两图、原文字节一致，资源记录无外部地址。 |
| 深层文件树与标签 | PASS（浏览器） | 16 文档、深度 8 文件树；248/190/420px 侧栏的深层摘要实测约 95/74/175px，日期先让位、批注数随后让位；标签关闭区均 20px，溢出菜单搜索末项并激活、方向键切换与 Delete 关闭后剩余 15 文档。 |
| Go 集成 | PASS | `go test ./...`，应用包通过；未执行本轮原生打包。 |
| 默认窗口实际启动尺寸与小屏适配 | BLOCKED | 唯一工作区窗口创建入口已设 1280×820，未请求全屏/最大化；1280×820 浏览器布局有真实截图。原生工具按名称/路径选择应用时超时，Dev 应用随后被自动审批拒绝（`Computer Use was not approved`），未读取原生启动 bounds，也未验证较小屏幕工作区。 |
| 组件盘点 | PASS（静态） | [UI 组件缺口与文字规划](design/ui-component-gaps.md)区分已有组件、Tooltip/菜单底座、未来 GenUI Diagram 和可选临时通知；桌面摘要残留 Markdown 标记作为提取语义差异记录。 |

## PR #33 合并前检查（2026-11-10）

源码候选为 `695e882`。下列结果覆盖本次合并检查，历史原生结果仍按各自的源码版本记录。

| 检查 | 判定 | 证据 |
|---|---|---|
| 前端类型、全量测试与构建 | PASS | `npm --prefix web run typecheck`、`npm --prefix web test`、`npm --prefix web run build`。真实 Chromium 28 个文件、280 项通过。 |
| 文档标签与滚动区域 | PASS（Chromium） | 全量套件覆盖标签随窗口缩窄和放宽进入及退出溢出菜单、通过菜单和方向键切换后的焦点、编辑器滚动位置恢复、每文档 GenUI 跳转和 ComparisonTable 的键盘横向滚动。 |
| Go 与随包 MyGo | PASS | 应用 `go build ./...`、`go vet ./...`、`go test -race ./...`。`third_party/mygo` 的构建、`vet` 和 `go test ./...` 退出码为 0，该模块当前没有测试文件。 |
| macOS 应用包 | PASS（构建与签名） | 从 `695e882` 导出的源码运行 `go tool mygo build -skip-dmg`，并用 `codesign --verify --deep --strict` 校验 arm64 应用包。 |
| 本次滚动和标签改动的原生交互 | BLOCKED | 桌面工具打开独立测试包时超时，按 bundle ID 查询也未解析到应用。已有原生证据仍见下文。 |

#20、#25、#30、#31 和 #32 保持开放，原生验收缺口按对应场景记录。

环境（原型阶段）：macOS arm64，Go 1.27.3，MyGo v0.7.4，Milkdown 7.23.0，Node 24。

## 自动化（2026-11-06/07 原型）

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
- 源码模式 undo/redo；切换保留/重建预览的边界（未改动保留撤销历史，改动后以新文本重建）；切换不触发 change。
- 源码中 `reveal` / `onSelectionChange` 为精确偏移；切换模式后在新模式 reveal 旧光标偏移；源码 reload 会报告光标重置为新位置，切换回预览只报告最终块偏移、不报告加载中间态。
- 源码中外部 reload 替换文本、保持模式并重置历史；即使 reload 文本等于进入源码前的 compose 文本，回到预览仍强制重建（撤销不会复活 reload 前状态）；只读文档在源码中只读。
- App：头部按钮与 `⌘/`、`Ctrl+/` 均可切换；进入/退出无编辑时 `host.update` 从未调用；源码中改 frontmatter 并去掉 `[!NOTE]` 后 host 收到期望字节，回到预览该块是可编辑引用而非锁定块，随后预览编辑仍保留源码改动的字节；源码 undo；源码中 reload 保持模式；只读 host。

打包应用手动检查（2026-11-06，`go tool mygo build` 产物）：

| 检查 | 状态 |
|---|---|
| `⌘/` 在打包应用中切换源码 / 预览，焦点与滚动位置正确；不编辑来回切换后文件 sha256 不变 | 通过（自动化 UI 操作） |
| 源码模式内 `⌘Z` / `⇧⌘Z` 撤销重做（真实键盘） | 通过（人工） |
| 源码模式内中文输入法输入正常（真实 IME） | 通过（人工） |
| 源码模式编辑后自动保存：状态栏 Edited → Saved | 通过（人工） |

## 大纲（#4）

> 历史说明（2026-11-08）：本节记录 #4 当时的实现——视图只有 `'left' | 'center'`，大纲在左栏，“没有右栏”。该布局已被 #16 与 ADR 0004 取代：现在 `ViewLocation` 有 `'left' | 'center' | 'right'`，大纲注册在右栏（`outline.panel`），左栏是工作区文件树。以下旧事实保留原样，不再描述当前界面。

自动化（`web/test/outline.test.tsx` 在 App + `createBrowserHost` 接缝；`web/test/plugin-registry.test.ts` 在注册表接缝）：

- 视图注册的 `location` 是 `'left' | 'center'`：左侧视图进左栏，中央视图成为视图模式（见 #9）；没有右栏。
- 大纲对 `test/fixtures/outline.md`（ATX H1–H3、setext H2、代码围栏内的伪标题、重复标题）只列真实标题且层级正确。
- 点击每个条目（含第二个重复标题）把光标移到该标题：预览检查光标所在标题，源码模式检查 CodeMirror 精确偏移。
- 新增 / 重命名 / 删除标题在防抖（300ms）后更新列表。
- 条目缩进按标题层级递增（比较文字 `getBoundingClientRect`，不依赖内部属性）；标题内 `**粗体**` / `` `code` `` 显示为纯文本。
- frontmatter 内的 `# x` 与原始 HTML 块内的 `<h2>` / `# y` 不会出现在大纲。
- 点击条目会把标题滚动进视口并把焦点交给编辑器（预览与源码模式都验证），焦点不停留在大纲按钮上。
- 源码模式中把光标移到后续章节时，对应条目高亮；光标在第一个标题之前时没有高亮。
- 关闭后重新打开大纲：左栏与条目恢复，高亮继续跟随光标。
- 预注册左侧与中央视图后挂载侧栏：左侧视图只挂载一次，中央视图不会被侧栏挂载；只有中央视图时不渲染左栏。
- App 在插件激活未完成时卸载：无 unhandled rejection，激活完成后才产生的注册被丢弃。
- 移动光标高亮所在章节（`aria-current="true"`）。
- 无标题文档显示空状态。
- 关闭大纲插件：左栏消失、编辑仍写入 host；`PluginContext.editor.onSelectionChange` 监听随插件禁用被释放。
- GenUI 不包含自己的标题列表；在 GenUI 模式下大纲仍在左栏，点条目切回预览并把光标放到该标题。

## 视图模式与 GenUI（#9，取代 #6 的右侧栏与 v1 格式）

自动化（`web/test/genui.test.tsx`、`web/test/app-source.test.tsx`、`web/test/outline.test.tsx` 在 App + `createBrowserHost` 接缝；`web/test/sections.test.ts` 用共享向量；`web/test/side-pane.test.tsx`、`web/test/plugin-registry.test.ts`）：

- 头部三选一 `Preview` / `Markdown` / `GenUI`；`⌘/`、`Ctrl+/` 在预览与 Markdown 之间切换。
- 没有视图文件：显示 `No view yet.` 与 `Generate`（#7 起可用）。
- `view-format.md` 的 v2 JSON 示例直接加载：替代编辑器占满中央，无错误，Tabs / Row / Card / Column / Text / KeyPoints 全部渲染，`Text` 里的 `**粗体**` 是 `<strong>`；切到 `Details` 标签页显示第二页。
- 在视图里切换标签页不调用 `host.update`，文档不变为 edited。
- 目录外组件（`Chart`）：只在其位置显示错误，其余照常渲染。
- `Text` 里的链接、图片、`<b>`：不产生 `a` / `img` / `b` 元素；链接只显示文字。
- 非 JSON、`version: 1`（提示「请重新生成」）、`version: 3`、缺 `a2ui`、引用不存在的组件：整个视图一条错误；写入、替换、删除视图文件时视图随之更新。
- 只在 GenUI 模式显示时跟随视图文件：启动时 0 个跟随者，进入 GenUI 1 个，离开归 0；离开期间写入的视图在再次进入时显示。
- Markdown 模式里的修改经过一次 GenUI 后仍在，并已写入 host；预览的滚动位置在往返 GenUI 后恢复。
- 视图模式按文档记忆：重新挂载 App 后回到 GenUI。
- 章节范围与 `fnv1a32` 哈希与独立计算的向量一致（含 CJK、emoji、CRLF、重复标题、setext、代码围栏伪标题）。

前端体积（`npm run build`）：初始代码块 `index-*.js` 562.86 KB gzip（#6 时 563.5 KB）；GenUI 代码块 `GenUIView-*.js` 136.34 KB gzip + CSS 0.50 KB，只在进入 GenUI 时加载。

打包应用手动检查（2026-11-06，`go tool mygo build` 产物，`/tmp/mdws-smoke/view6/report.md`）：

| 检查 | 状态 |
|---|---|
| 头部三选一，切到 GenUI：格式文档示例占满中央，粗体、标签页、分栏、卡片、KeyPoints 正确 | 通过（自动化 UI 操作） |
| 切到 `Details` 标签页 | 通过（自动化 UI 操作） |
| GenUI 模式下点大纲 `Metrics`：切回预览，光标在 Metrics 标题 | 通过（自动化 UI 操作） |
| 退出后重新打开：直接回到 GenUI | 通过（自动化 UI 操作） |
| 在磁盘上改写视图文件：未知组件只在其位置报错；写成 v1 显示「请重新生成」 | 通过（自动化 UI 操作） |
| 以上全部操作后 `report.md` sha1 不变 | 通过 |

## GenUI 来源状态与“去原文修改”（#10）

自动化（`web/test/genui.test.tsx` 在 App + `createBrowserHost` 接缝；`web/test/editor-offsets.test.ts`、`web/test/source-mode.test.ts` 在编辑器接缝）：

- 视图哈希由共享章节模块算出（与宿主将写入的一致）。外部改动一个章节：只有引用它的组件（含多来源组件）标 `Source changed`，其他不变；改掉标题：引用它的组件标 `Source not found`（优先于 changed），内容照常显示，`Edit in source` 禁用；恢复原文后标记全部消失。
- 无 `hash` 的手写视图：改动来源后仍不显示任何状态。
- `Edit in source` 引用第二个同名 `Summary`：切到预览，选中的正是第二个章节（标题 + 正文），而不是第一个。
- 视图滚到下方点 `Edit in source`、在预览修改、点 `← Back to GenUI`：回到 GenUI，滚动位置误差 < 2px，被改章节的组件标 `Source changed`，其余不变；之后返回按钮消失。在头部选模式后返回按钮也消失。
- 编辑器 `select`：预览按整块选中区间（重复标题各选各的），越界返回 `false`；源码模式为精确选区并报告起始偏移，`from > to` 返回 `false`。

打包应用手动检查（2026-11-06，`go tool mygo build` 产物，`/tmp/mdws-smoke/view10/report.md`，视图哈希按格式文档独立计算）：

| 检查 | 状态 |
|---|---|
| 进入 GenUI：所有组件无标记，底部显示 `From <标题>` 与 `Edit in source` | 通过（自动化 UI 操作） |
| 滚到底部点 LATE 的 `Edit in source`：切到预览，第二个 `Summary` 章节被选中，大纲高亮该条，显示 `← Back to GenUI` | 通过（自动化 UI 操作） |
| 在该段末尾输入 ` Revised.`：自动保存，文件末尾为 `Late summary. Revised.` | 通过（自动化 UI 操作） |
| 点 `← Back to GenUI`：回到原滚动位置，只有 LATE 标 `Source changed` | 通过（自动化 UI 操作） |
| 外部进程把 `# Metrics` 改成 `# Numbers`：版本 v2，METRICS 与 BOTH 标 `Source not found`，LATE 仍为 `Source changed` | 通过（自动化 UI 操作） |

## GenUI 组件目录（#11）

自动化（`web/test/genui-catalog.test.tsx`、`web/test/genui.test.tsx`，App + `createBrowserHost` 接缝）：

- 格式文档示例用到目录里全部 10 个组件，在 App 里无错误渲染（Summary：KeyPoints、Checklist、MetricGrid、粗体 Text；Details：ComparisonTable、Timeline）。
- ComparisonTable：点 `Size` 升序（9、12、180，按数值）→ 降序 → 原顺序；点 `Option` 按文字排序；`aria-sort` 跟随；`host.update` 从未调用。
- Checklist：`aria-checked` / `aria-readonly`；点 `Generation`（`quote: "Ship generation"`）切到预览并选中原文那一项，原文不变、未 edited；来源缺失时各项禁用。
- 属性写错：错误写明组件与 id（`'MetricGrid' (root)`）和原因（缺 `value`、`trend` 枚举、表格某行单元格数不对、空 `events`、多余属性 `checked`）。
- 嵌套违规：`Card` 里放 `Tabs` → `UNALLOWED_CHILD: …`；`Row` 里放 `Row` → `UNALLOWED_PARENT: …`；信息里没有占位组件名。
- 560px 与 1600px 宽：中央区与每个组件都不横向溢出，超长单词换行，宽表只在自己框内滚动。
- ~~`color-scheme` 切到 dark：组件背景变暗、文字变亮。~~ **已被 ADR 0006 取代**：本轮仅浅色，该深色用例已从套件删除（套件 267 → 266 项），不再断言深色外观。
- `catalogDefinition()`：组件列表等于目录、不含占位组件；带 schema（如 `MetricGrid.metrics` 的 `minItems: 1`、字段说明）与嵌套约束。

前端体积：初始代码块 563.26 KB gzip（未变）；GenUI 代码块 139.69 KB gzip（#9 时 136.34）+ CSS 1.67 KB。

打包应用手动检查（2026-11-06，`go tool mygo build` 产物，`/tmp/mdws-smoke/catalog11/report.md`，视图文件就是格式文档的示例）：

| 检查 | 状态 |
|---|---|
| GenUI Summary：两张卡片并排（KeyPoints、Checklist 一项已勾），下方 MetricGrid 带单位与 ▲ +4%；`Edit in source` 按钮样式正确 | 通过（自动化 UI 操作 + 截图） |
| Details：ComparisonTable 与 Timeline；点 `Size (MB)` 列头后为 Tauri 9 → MyGo 12 → Electron 180 | 通过（自动化 UI 操作 + 截图） |
| 点 Checklist 项：切到预览，`Next steps` 一节被选中（该项文字不在原文，回退到整节），显示 `← Back to GenUI` | 通过（自动化 UI 操作 + 截图） |
| ~~WebKit 深色：临时把 `:root` 改成 `color-scheme: dark` 打包，组件为深色背景、浅色文字（只验证了 GenUI；其余界面还没有深色样式）~~ | 曾通过（截图），已改回；**已被 ADR 0006 取代**：本轮不提供深色外观 |
| 以上全部操作后 `report.md` sha1 不变 | 通过 |

## GenUI 生成（#7，ADR 0003）

自动化（`generate_test.go`：测试二进制自己充当假 agent，走真实 ACP 管道；`acp_process_cache_test.go` 覆盖空闲进程复用、过期、并发与关闭；`web/test/genui-generate.test.tsx` 在 App + `createBrowserHost` 接缝；`web/test/sections.test.ts` 与 Go 共用 `section-vectors.json`）：

- 运行：agent 只带 `acp` 参数；进程在缓存私有目录启动，`initialize` 每个进程一次；每次运行新建运行目录，开始时只有 `source.md`，`session/new` 的 `cwd` 指向它；提示里有格式文档与读者的请求；模型经 `session/set_config_option` 选择；进度按阶段变化逐步上报。
- 进程缓存（`acp_process_cache_test.go`）：空闲两分钟过期，环境变化不复用，并发运行结束后只保留一个空闲进程；`session/close` 未确认、空或重复 Session ID 或握手中取消都丢弃进程；关闭缓存会停止空闲进程并删除进程目录。
- 成功：视图文件键顺序为 `version` / `generatedAt` / `model` / `request` / `sources` / `a2ui`；`model` 只写 Agent 配置应答确认的实际值；`a2ui` 原样、多余字段丢弃；`sources` 哈希由宿主按原文计算（与页面共享向量一致），找不到的标题不带哈希；新文件为 0644，替换时保留原权限。
- 权限请求：读 `source.md` / 写 `out.json` 允许，其余按既有类型、路径与符号链接规则拒绝。通知按 toolCallId 合并已知类型与路径；pending、仅状态、缺少 kind/locations、尚未知 id 或 `other` 的观测不凭信息缺失中止，后续补全后继续检查；已报告的越界路径、execute 等不支持操作立即停止，视图不变。通知兼容性不放宽权限批准规则。
- 失败（#8，每类断言 `kind` / `message` / `hint`，并检查视图不变、临时目录删除）：命令找不到、完整路径不存在 → `agent-not-found`；`session/new` 返回 -32000、提示里含 “authentication required”、stderr 里含 → `auth-required`（登录提示带配置的命令）；未知模型 → `model-invalid` 带模型列表；这一轮无 `out.json` 但有回复、`session/prompt` 返回错误 → `agent-message`，`detail` 原样（含换行的额度提示）；`out.json` 缺失 / 非 JSON / 超大 / 符号链接 / 缺 `a2ui` 或为空 / `sources` 格式错误 → `invalid-output`；agent 启动即退出 → `agent-error` 附 stderr；`reportFailure` 发出 `failed` 步骤并以 message + hint 拒绝。“同一窗口只跑一次”与“未保存文档”的 Go 端检查依赖窗口会话，没有单元测试（页面在运行中和未保存时本就禁用 `Generate`）。
- 取消与超时（`hangingRun`、`TestGenerateCancelAcknowledgedWithoutProcessExit`、`TestACPCancelDoesNotWaitForDescendantPipesOrBlockedWrite`、`TestAgentCacheCancelAcknowledgedClosesSessionAndReusesProcess`、`TestGenerateTimeout`）：页面关闭通道（ctx 取消，关窗口与退出走同一条路）/ 300ms 超时 → agent 收到 `session/cancel`；提示轮应答即完成，不再等固定 2 秒或进程退出，1 秒内无应答则杀死进程。运行目录删除、视图文件字节不变；缓存路径在会话关闭被证明后复用进程，否则丢弃。超时文字 `Generation timed out after 5 minutes.`（默认值）。
- 页面：生成中显示步骤列表与骨架、隐藏旧视图、请求与设置禁用；生成中继续编辑，视图按发起时的文本生成；失败保留旧视图，显示 message、hint、原样的 agent 原话、模型列表，`Retry` 用同一请求重跑并可成功、`Open agent settings` 打开设置、`Dismiss` 关闭；失败时没有 `Cancel`；非 `GenerationError` 的错误显示为 agent-error；未保存文档禁用 `Generate`；`Agent settings` 改模型与命令并持久化。

> 2026-11-10 收尾实测见 [ACP 与 GenUI 报告](acp-genui-completion.md)：12 次初始生成均通过 Go 校验与提交，11 次即时页面观测通过、1 次观测器选错活动文档（原失败与 TTU 缺口保留；同一已提交视图补观测通过）；最终修正提交另跑三题，3/3 生成与渲染通过。真实取消、重生成、跨文档/模型隔离、崩溃恢复与退出清理均有原始记录；真实未确认取消走 1 秒丢弃退路，不记作热复用成功。

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

> 说明（2026-11-08）：上面的 `Open agent settings` 按钮与「Agent settings」自由文本输入属于 2026-11-07 的实现。ADR 0004 与 #24 之后，常规界面不再有该设置面板：改模型用右栏底部 `ModelMenu`；命令只在 `agent-not-found` 失败里经 `Locate agent…` 原地修改。历史行保留当时的实测结论，不再描述当前控件。

> 说明（2026-11-10）：以上 2026-11-06/07 表格记录当时的进程策略；当前的空闲进程复用与取消语义已取代旧的“生成成功或取消后一定没有 `agy` 进程”期望。历史观察保留原样；新的真实 cohort 另见[完成报告](acp-genui-completion.md)。

## GenUI 重新生成与撤回（#12）

自动化（`update_test.go` 用同一个假 agent；`web/test/genui-generate.test.tsx` 的「GenUI regeneration」）：

- 局部：agent 目录开始时只有 `source.md` 与 `view.json`；读 `view.json` 允许、写它拒绝；提示里有要替换的 id、surface、生成要求；结果是原消息 + agent 的一条 `updateComponents`，来源全部按本次文本重算哈希并加入新来源；被替换的视图成为备份（权限与视图相同）。
- 越界整次拒绝（`invalid-output`，视图和备份不变）：改了未点名的组件、改了 `root`、两条消息、`createSurface`、另一个 surface、改动已有来源。视图没有那个 id、没有 id、没有视图 → 不启动 agent。agent 工作期间视图被改写 → 不写入、不留备份。
- 备份与撤回：全部重新生成也留备份；生成失败备份不变；撤回后视图 = 备份、不再有备份，再次撤回被拒绝。
- 页面：没有组件「原文已改」时 `Regenerate changed` 禁用；改了一节后变为 `Regenerate changed (1)`，发出 `mode: 'update'`、`replaceIds: ['metrics']` 和生成要求；更新后另一个组件保持原样；`Undo last generation` 回到旧视图后按钮消失；`Regenerate all` 发 `mode: 'full'`；失败保留备份，生成中撤回禁用。

打包应用手动检查（2026-11-07，真实 `agy acp`，`gemini-3.8-flash`，`/tmp/mdws-smoke/gen12/report.md` + #7 生成的视图）：

| 检查 | 状态 |
|---|---|
| 预览里把 Options 表格 Tauri 的 9 改成 0 → GenUI：`From Options` 标 Source changed，按钮 `Regenerate changed (1)` | 通过（自动化 UI 操作） |
| `Regenerate changed (1)`：进度 Starting / Agent ready / Thinking / File source.md / File view.json / File out.json，约 1 分钟后表格显示 Tauri 0、无 Source changed、`Regenerate changed` 禁用、出现 `Undo last generation` | 通过（自动化 UI 操作） |
| 视图文件：`a2ui` = 原 `createSurface` + 一条只含 `compare` 的 `updateComponents`；`sources.options.hash` 重算为 `fnv1a32:6a329a8c`；`.report.md.genui.prev.json` 与原视图逐字节一致；无 `agy` 进程与临时目录 | 通过 |
| `Undo last generation`：视图文件与原视图逐字节一致、备份删除；页面回到 Tauri 9 + Source changed + `Regenerate changed (1)`，撤回按钮消失 | 通过 |
| 备份权限：这次实测发现备份是 0644、视图是 0600，已改为备份沿用视图权限（Go 测试覆盖） | 修复后未在打包应用里重测 |

## 批注（#13）

自动化：`web/test/notes.test.ts`（格式、转义、预览添加 / 编辑 / 删除 / 撤销、源码模式、只读）、`web/test/genui-notes.test.tsx`（从组件添加、显示、不标「原文已改」、`exclude` 传给宿主）、`web/test/compose-gaps.test.ts`（增删块后其余间隔不变）、语料 `22-notes.md`（逐字节往返）、`section-vectors.json` 的 `notes`（TS 和 Go）。

打包应用实测（2026-11-07，真实 `agy acp`，`/tmp/mdws-smoke/notes/report.md`：CRLF、不规则空行、无结尾换行、带属性的批注、代码块里的 `<!-- note -->`）：

| 检查 | 结果 |
|---|---|
| 预览显示 | 批注是标记，带 Edit / Delete；代码块里的注释仍是代码 |
| 编辑（写入 `Edited --> note`） | 磁盘上只有文字变成 `Edited --\> note`，`author` / `id`、CRLF、空行不变；显示读回原文 |
| ⌘Z | 文件回到编辑前的字节 |
| 块菜单 `Add note` | 批注落在打开菜单的块后面，其余字节（含 `\n\n\n`）不变 |
| 带批注的文档 `Generate` | 4 个来源的哈希与 Python 手工去掉批注后计算的一致；没有组件标 Source changed；Agent 没有把批注写进内容 |
| GenUI 组件 `Note` | 带子节的来源：批注落在子节之后；末节：文件仍无结尾换行；两次都只多出批注；没有组件标 Source changed；停留在 GenUI |
| Markdown 模式 | 显示 `<!-- note: … -->` 原始文本 |
| 回预览按两次 ⌘Z | GenUI 加的两条都撤销，文件回到添加前的字节 |
| Delete | 批注和它前面的间隔一起去掉，其余字节不变；GenUI 没有标 Source changed |
| #12 遗留：备份权限 | 视图 0600 时 `Regenerate all`：备份 0600、视图 0600，备份与旧视图逐字节一致；Undo 后视图逐字节恢复，备份删除 |
| 退出 | 没有 `agy` 进程，没有 `markgogo-genui-*` 临时目录 |

自动化说明（2026-11-07）：网页内容第一次读取时可能不在辅助功能树里，重读一次就出现。屏幕锁定时 macOS 隐藏所有窗口的辅助功能树，窗口也显示空白，驱动报告 `screen_locked`，解锁后才能继续（#12 时「连不上窗口」可能是这两个原因之一，未确认）。像素点击和键入不一定到达编辑器；AX 按大纲条目会把焦点交给编辑器，之后 ⌘Z、`/` 和方向键可用。`--` 经键盘注入会被系统智能破折号替换成 `—`，测转义时用 AX 直接设值。

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
  - 全局样式引入 Portrait 语义 Token（`--chrome`、`--paper`、`--raised`、`--code`、`--ink`~`--ink-4`、5 级触觉阴影）。当时还带一份深色 Token 块；**ADR 0006 之后本轮仅浅色**，该块不被任何界面启用。
- **ModelMenu 与模型列表持久化缓存（#17）**：
  - Go `GetSettings` 在未握手前回退默认模型列表（`gemini-3.8-flash`、`gemini-4-argon`、`claude-opus-5-5`）；ACP `session/new` 握手拿到非空 `configOptions` 时自动更新并持久化 `models`。
  - `ModelMenu` 实现 26px 凸起药丸按钮与向上展开的纸面菜单（`.mmenu.pop`），支持 `Agent’s choice`、当前项勾号，点击外部或按 `Escape`（`document` 级监听）均可关闭，生成运行中自动禁用。常规界面移除自由文本 `Model` 和 `Agent command` 输入框。
  - **已被 #24 扩展**：`SetSettings` 改为原子意图补丁并返回提交值，成功写入向每个窗口广播 `settingsChanged`；Model 与 Locate agent 只写各自字段；`Agent command` 输入框只在 `agent-not-found` 失败里的 `Locate agent…` 中原地出现。见上文设置审计。
- **GenUI 视图拆分（#18）**：
  - 中央视图（`location: 'center'`）仅渲染纯净只读 A2UI 组件卡片与来源底座，彻底剥离生成工具栏与错误提示框。
  - 右栏 `genui.panel`（`location: 'right'`）承载 5 种顶部状态与唯一主操作按钮、点击 `Source changed` 列表项平滑滚动并高亮定位中央卡片、Focus 生成要求安静只读与原地编辑、以及固定底部元数据（`Antigravity (agy)`、`ModelMenu`、生成时间、`Undo`）。
- **原地白纸 Composer（#19）**：
  - 彻底移除居中模态 `<dialog class="mgo-note-dialog">`，预览与 GenUI 卡片内写批注均原地展开自适应增高白纸卡片（`.composer`，`⌘↩` 保存，`Esc` 取消），保持原文逐字节保真，且不触发“原文已改”。

## 工作区、标签与 Portrait 集成（#20/#21–#32，2026-11-08）

审查基线 `4f80bbf094b5cda0cfa642090f5255e050ade587`。本节记录从 `f9237f2` / `7b90f1e` 到 `47e3b4a`、`6478ceb` 的实际证据；各 SHA 的角色见下表。本节区分**已执行的自动化检查**、**已在真实打包应用里实际观察到的原生证据**、**已观察到的原生失败**，以及**仍未运行、等待证据的行**；未运行的行不得当作通过。

各 SHA 的角色不同，不要混用：

| SHA | 角色 | 证据状态 |
|---|---|---|
| `f9237f2`、`7b90f1e` | 历史原生包 | `7b90f1e` 的 FAIL 红证据保留（退出清零、恢复空白/错标题、Save As 0 字节、自动保存清零）；`f9237f2` 有早期部分观察 |
| `47e3b4a` | 原生部分验收基线 | 下列原生表实际 PASS 的场景；其余仍未运行或受策略限制 |
| `6478ceb` | 源码冻结（64 行为） | 全量自动化 27 文件/275 项等 PASS；原生 focused matrix 已运行，确认项见下；字体诊断完成；该 SHA 上有 Close→Delete 的窗口/会话丢失 FAIL（历史证据保留；`c442` 已修复并在 normal release 实机 PASS 原场景） |
| `c44239c` | 最终源代码（`1d76` RoleClose 路由 + `9c3` 无文档 fallback + `c442` test-only 清理） | 受影响 Go 检查 PASS（root build/vet/race、nested MyGo、changed Go gofmt）；两轴 focused 复查 PASS（Standards 0.99、Spec 0.98、0 新 hard）；normal release 的关闭回归实机 PASS（Cancel/Delete/quit-relaunch/tab close/0tabs no-op/红关整窗）；前端 275 未作为该 SHA 重跑；standalone untitled 新 route 变体复验中 |
| docs-only 提交（`6f3dba2`、`3a78f31`、`3569cb6`、`ccb09a3`、`8fd09de` 及本次） | 文档修订 | 不改变运行时代码，不产生新的 runtime 证据；文档提交自身的 hash 不作为验收证据，只引用实际运行过的包与源码 SHA |

### 自动化全量检查（PASS，实际执行）

执行目录 `/private/tmp/mark-go-go-spec20-checks-32`，提交 `7b90f1e`：

| 检查 | 命令 | 结果 |
|---|---|---|
| 依赖 | `npm ci`（`web/`） | 301 个包 |
| 类型 | `npm run typecheck` | 通过 |
| 前端测试 | `npx vitest run`（真实 Chromium） | **26 个文件、266 项通过**（修正前为 10 个文件、11 项失败） |
| 构建 | `npm run build` | 通过（仅既有 chunk 体积警告） |
| Go 静态检查 | `go vet ./...` | 通过 |
| Go 测试 | `go test -race ./...` | 通过（修正前 3 项失败） |
| 文档组接缝 | `go test -race -run 'TestDeepestRoot\|TestFolderContains\|TestSameFile\|TestConfirmClose\|TestDocumentInfo\|TestHostSnapshot' -v .` | 12/12 通过 |
| vendored MyGo | `cd third_party/mygo && go build ./... && go vet ./... && go test ./...` | 通过（嵌套模块无测试文件，补丁由根测试覆盖） |
| SDK | `go tool mygo generate` | `web/src/host/generated/mygo.ts` 已是最新（确定性，未手改） |
| 格式 | `gofmt -l session.go index_test.go` | 无输出 |

这些结果证明自动化层面在当前源码上通过；它们不证明原生保存表、Edit 菜单目标归属、WebKit 辅助功能、IME 或退出/恢复行为。

### 最终候选 `47e3b4a` 全量复检（PASS，实际执行）

同一检查负责人（`FinalCheck32`）在精确 `47e3b4a` 上复跑：

| 检查 | 结果 |
|---|---|
| 前端类型 | `npm run typecheck` 通过 |
| 前端全量测试 | 真实 Chromium，**26 个文件、270 项通过** |
| 前端构建 | `npm run build` 通过 |
| Go 树 | 与 `6e7e395` 字节相同，沿用该提交的 `go vet ./...` 与 `go test -race ./...` PASS |
| Go 变更文件格式 | `gofmt` 无输出 |

未重复无关的 vendored MyGo 检查。这些仍是自动化结果；原生保存表、Edit 菜单归属与退出/恢复行为见下文原生行。

### Source freeze `6478ceb` 全量自动检查（PASS，实际执行）

检查负责人 `FinalCheck32` 在精确 `6478ceb6d48a16d3a9326c4469f5a1c93411d944`（源冻结）上实际执行，这是**运行时验证**，不只是静态阅读：真实 Chromium 浏览器测试与 Go `-race` 都真正运行。

| 检查 | 结果 |
|---|---|
| 前端类型 | `npm run typecheck` 通过 |
| 前端全量测试 | 真实 Chromium，**27 个文件、275 项通过** |
| 前端构建 | `npm run build` 通过 |
| Go | `go build ./...`、`go vet ./...`、`go test -race ./...` 通过 |
| vendored MyGo | `build`、`vet`、`test` 通过（嵌套模块本身无测试文件，补丁由根测试覆盖） |
| 格式 | 所有变更 Go 文件 `gofmt` 干净 |
| SDK | `go tool mygo generate` 报告已是最新 |

这些结果证明 `6478ceb` 的自动化层面通过；它们不证明原生保存表、Edit 菜单目标归属、WebKit 辅助功能、IME 或退出/恢复行为，也不表示 `6478ceb` 已被原生包运行。

> 区分：上述 27 文件/275 项属于 `6478ceb`（64 源行为）。最终源候选 `c44239c` 的受影响 Go 检查已 PASS：root `go build ./...` / `go vet ./...` / `go test -race ./...`、nested MyGo build/vet/test、变更 Go 文件 `gofmt` 干净。前端 275 未作为 `c44239c` 的结果重跑。`c442` 只做 test-only 清理（删掉无效的 fakeWindow 断言），`9c3` 修无文档时先解析 focused window 再关闭；该提交新增的 `close_group_test.go` 锁生命周期，但平台**没有能按原生菜单的 headless seam**，Go 测试不能当作 CmdW 的真实证明。新 normal release 的原生结果尚未观察。

### 两轴 focused 复查（`6478ceb` → `c44239c`，PASS）

`Standards` 与 `Spec` 两轴先对 `6f3dba2..6478ceb` 复查 PASS，随后对最终源候选 `c44239c` 的 close 修复再次 focused 复查并 PASS；这是代码与规格轴结论，不替代原生运行证据。

| 轴 | 结论 | 关键内容 |
|---|---|---|
| Standards | **PASS**（`correct`，置信度 0.99 于 `c442`） | `6478` 时 3 项硬违规与 2 项可操作启发式 resolved、P3 接受；对 `1d76`/`9c3`/`c442` 的 close 路由复查 **0 新违规、0 新 smell** |
| Spec | **PASS**（代码符合规格，置信度 0.98 于 `c442`） | `6478` 时 0 未解决代码缺陷；close 修复复查 **0 新硬缺陷**；剩余为原生与文档门 |

持久记录：`local://spec20-standards-review.md`、`local://spec20-spec-review.md`（均已更新）。

### Spec 短裁定（`6478ceb`）

- **Save As 的 OS 内建 Replace**：原生 Save As 对已打开目标时，OS `NSSavePanel` 在选择阶段先显示内建 Replace 确认；用户确认后应用仍以 already-open 拒绝，**目标不被写入，也不产生第二个会话**。这符合原 #20/#21 与 ADR 0005——原条款只禁止静默覆盖与双会话，并未禁止 OS 选择阶段的确认。`ReservePath` 的承诺边界是：文件/文件夹选择器返回目标之后、任何 write / `os.Rename` / 路径变更 / MyGo 自己的选择后 Move To 覆盖对话框之前；它不承诺屏蔽 OS Save 面板的确认。无需代码修复，也不降低原有的“绝不覆盖、绝无第二会话”要求。
- **取消**：Cancel 不产生失败、不提示，属于规范行为（接口明确取消不算失败），旧视图与视图文件哈希保持不变；不作为 FAIL 候选。

### 真实打包应用的部分原生证据（f9237f2，实际观察）

同一已批准的 Smoke 身份 `dev.markgogo.editor.coresmoke21`（仅验证工作树的 `mygo.json` 使用该身份），构建自 `f9237f2`，通过 `cua_repl` 操作。以下是实际观察，不是推断：

| 场景 | 观察 | 判定 |
|---|---|---|
| 冷启动 Welcome | 0 个文档标签，真实的 Open Folder / New Document 与最近工作区 | 通过（该场景） |
| Open Folder 原生面板 | Go To `/private/tmp/.../native32/workspace` 后得到规范化根，文件数 12 | 通过（该场景） |
| 文件树 | `bytes` 文件夹可访问名为 `bytes, 8 files`，计数右对齐；8 个子孙连续排在根级 alpha/beta/comparison/partial-sources 之前；折叠只隐藏这 8 行并保留根级行 | 通过（该场景） |
| 同窗多标签 | 从树打开 Alpha 再打开 Beta，同一原生窗口两个标签，两个正文都显示 | 通过（该场景） |
| 视图模式 | `⌘/` 在预览 / Markdown 间切换并把焦点交给源码编辑器；切换标签各自保留模式 | 通过（该场景） |
| 输入归属 | `typeText` 分别写入 Alpha `NATIVE-A-INPUTxy`、Beta `NATIVE-B-INPUTwz`，未观察到串写；短暂 Edited 后自动 Saved（v1 不是编辑计数） | 通过（该场景） |
| 标签键盘 | 标签获得焦点后方向键 Right 激活 Beta | 通过（该场景；CUA 焦点报告仍留在前一个标签，不构成 VoiceOver 结论） |
| 路由修复（更早 SHA） | `0b226e99` 构建上，同一工作区先开 Alpha 再开 Beta 得到同窗两个标签；此前 `ca95333` 因 `/tmp` 与 `/private/tmp` 别名分裂为两个窗口（已由符号链接前缀规范化修复） | 通过（该场景，非最终验收） |

原生构建来源：`/private/tmp/mark-go-go-spec20-verify-21/build/darwin-arm64/Mark Go Go CoreSmoke21.app`，构建日志 `/private/tmp/mgo-spec20-evidence/coresmoke21-build-7b90f1e.log`。上述会话以正常 Cmd+Q 结束，应用已停止。

### 已观察的原生失败（7b90f1e，FAIL；47e 已复验修复）

`7b90f1e` 上实际观察到的红证据保留在此，不因后续修复删除。最后一列是 `47e3b4a` 新包上的复验结果；未复验的项目不因“同名文件已修复”而转 PASS。

| 场景 | 7b90f1e 实际观察 | 7b 判定 | 47e3b4a 复验 |
|---|---|---|---|
| 正常退出后的内容保护 | 正常 Quit 后两份未编辑文档被写成 0 字节（2/2），mtime 等于 CmdQ 时刻，无对话框，会话已保存。证据：`native32/quit-zerodocs-7b90f1e/` | **FAIL** | **PASS（已观察）**：无编辑 CmdQ 前后 alpha 172 B / beta 171 B 的 sha256 与 mtime 完全不变，进程退出，`session.json` 合理 |
| 退出后重启恢复的呈现 | 重启后活动面板空白；窗口标题 / represented URL 是 `beta.md`，而会话 `activeId` 是 alpha；没有选中标签与编辑器子树，点击标签后才出现内容（2/2）。证据：同目录 + AX transcript | **FAIL** | **PASS（已观察）**：重启后标题与 URL 为 `alpha.md`，alpha 标签选中，alpha 编辑器已挂载并显示 `NATIVE-A-INPUTxy`，视图模式 `Markdown`，`Saved v1`；无空白、无错标签标题 |
| 未命名 Save As 写入 0 字节并丢内容/历史 | 可见 584 字符、撤销重做正常的未命名文档 Save As 到工作区后为 0 字节，迁移标签只剩占位，历史消失。证据：`native32/quit-zerodocs-7b90f1e/untitled-migrate-0byte.md`、`evidence-7b90f1e/session-midtest.json` | **FAIL** | **PASS（已观察）**：typed untitled 经 Cmd+Z / ⇧⌘Z 后 Save As 为精确 50 B（`SAVEAS47-TYPED-LINE\n\nsecond-line-only-in-untitled\n`，sha256 `f100a808…ec30`），标签迁移到工作区且内容完整；迁移后撤销重做仍有效 |
| 自动保存把两份打开文档清零（12:30:22） | `comparison.md`、`partial-sources.md` 在 7b 运行期间变为 0 字节（保留于 `evidence-7b90f1e/empty-autosave-1230/` 并从备份恢复） | **FAIL** | 同一交付修复覆盖；这两份文件没有逐一复跑。47e 上的普通编辑自动保存（alpha 181 B）与随后 CmdQ 字节保持已观察 |
| 期望（不变） | 未编辑文档在退出、恢复、重启后逐字节不变；恢复后当前标签立即可见并可编辑；保存不得丢失内容与历史 | 规范要求 | 规范要求 |

修复根因与自动检查见下一小节；未列入“47e 复验”的场景仍按“未运行”处理。

### 修复 `47e3b4a`：自动检查 PASS

最终原生候选提交 `47e3b4a`（`Flush the text on screen, not a change-only buffer (#23)`，父提交 `6e7e395`，已进入集成分支）。

| 项 | 内容 | 判定 |
|---|---|---|
| 自动回归（目标用例） | 目标回归由 2 项失败转为 **11 项通过**，`tsc` 通过（源码负责人执行并报告） | **PASS（自动）** |
| 全量自动检查（`47e3b4a`） | 前端 `typecheck`、真实 Chromium 全量 **26 个文件、270 项通过**、`build` 通过；Go 树与 `6e7e395` 字节相同，沿用其 `go vet` + `go test -race` PASS；变更 Go 文件 `gofmt` 干净 | **PASS（自动）** |
| 退出/保存的空缓冲成因与修复 | 面板此前用一份只记录 `onChange` 的占位缓冲参与 flush，重载或重新挂载后会交付假的 `''`。现在只有编辑器已 ready 才注册 reader；交付时取 `editor.getMarkdown()`，已注册 reader 执行时若 editor ref 短暂为空则回落到宿主载入的核心文本；从未挂载编辑器的面板从不注册、也从不交付 `''`。用户主动清空则按普通编辑送达 `""` | 已修复（代码，47e/6478） |
| 恢复后无活动编辑器：成因与修复 | 从磁盘打开的文档在恢复时得到新的 MyGo id，旧实现按会话里存的 id 设置活动标签，活动标签因此指向不存在的 id。现在把存储的 active tab 映射到该文档真实的新 id，再设为活动（6e） | 已修复（代码，6e） |
| 文档契约 | `docs/workspace-contract.md` 同步为：flush 交付屏幕上的文本（否则核心文档状态）且每入口只写回一次，恢复把存储的 active tab 映射到真实 id，路径变更先锁后写、目标预留至操作结束、失败记入 `actionError` | 已同步（47e + `6478ceb` 文本） |

### `47e3b4a` 原生复验（部分 PASS，实际观察）

构建：`/private/tmp/mark-go-go-spec20-verify-21/build/darwin-arm64/Mark Go Go CoreSmoke21.app`（`dev.markgogo.editor.coresmoke21`，构建日志 `coresmoke21-build-47e3b4a.log`），fixtures 仅用 `native32/…`。报告仍为**进行中**：本表只列实际观察过的场景，表后另列未运行项。

| 场景 | 实际观察 | 判定 |
|---|---|---|
| 无编辑 CmdQ 的字节与 mtime | alpha 172 B sha256 `1dc63b4b…9264`、beta 171 B `51e40b57…9f82`，退出后哈希与 mtime 均不变；进程退出，`session.json` 合理 | **PASS** |
| 编辑与保存后的正常 CmdQ | alpha 181 B `c797c639…7306`、新的 `saveas-typed-47e.md` 50 B `f100a808…ec30`、beta 不变；退出后全部不变，无对话框 | **PASS** |
| 输入归属与快速切换 | 在 alpha 输入 `R47-ALPHA` 后 119 ms 点击 beta（输入开始后 155 ms）：beta 无该文本，alpha 自动保存为 `NATIVE-A-INPUTxyR47-ALPHA`（181 B），beta 哈希与 mtime 不变 | **PASS** |
| 首次输入的显式保存 | alpha 上 Cmd+S 写入同一 181 B 内容，编辑器内容保持，无空写入 | **PASS** |
| 未命名 Save As 与迁移 | 见上表：精确 50 B、标签迁移、无占位、`Saved v1` | **PASS** |
| 迁移后的撤销/重做 | 迁移文档内 Cmd+Z 移除输入（Edited）、⇧⌘Z 恢复；alpha/beta 字节不变，历史属于迁移后的文档 | **PASS** |
| 特殊块与原地 Composer 字节保真（47e 复验保存/撤销；Esc 取消在 7b 观察） | 打开原始样文时未写盘（169 B `5b377f55…75e6`，mtime 12:30:06）；composer 追加 ` CHK47` 后自动保存为 175 B `1d0f0c44…db28`，diff 只有 alert 行，其余字节一致；Cmd+Z + 自动保存恢复到精确原始 169 B `5b377f55…75e6`。7b 上的 Esc 取消保持文档与文件不变 | **PASS（47e）** |
| ComparisonTable 键盘排序与不写原文（7b 构建） | 表头 `Option/Size/Notes` 可聚焦，Space 排序 180/12/9（数值降序），再次 Space 恢复；`comparison.md` 哈希与 mtime 不变。注意：6478 本轮只以鼠标激活取得一次升序，键盘/AT 未复验（见下） | **PASS（7b 构建，路径未改动）** |
| 部分来源与全部缺失的实时跟随（7b 构建） | mixed 视图：FRESH、STALE `Source changed`、MISSING `Source not found` 且 `Edit in source`/`Note` 禁用并提示“None of this part's sources are in the document now.”、无哈希不标记；面板 `2 components out of date`、`Regenerate changed (2)`；`state.sh all-missing` 实时变为 MISSING A/B 且均禁用，`state.sh partial` 恢复；原文哈希与 mtime 不变 | **PASS（7b 构建，路径未改动）** |
| 窄窗右栏浮层（7b 构建） | 约 750 pt 窗口下 `Show right panel` 以浮层盖在文档上，含 Outline/GenUI 分段；GenUI 显示过期摘要；`Hide` 收起；宽窗布局已在同名标签场景观察到 | **PASS（7b 构建，路径未改动）** |
| 未命名脏关闭 Cancel（47e） | 原生保存表“Do you want to keep this new document “Untitled”?…” 含 Save…/Delete/Cancel；Cancel 保留标签与文本 `DIRTY-CLOSE-TEST`，header `Edited`（Delete 分支未执行） | **PASS（Cancel 分支）** |
| 两个未命名 A/B，A 首次保存进已有文件夹（47e） | 新建 Untitled A 与 Untitled 2 B；`File > Save As` 后 A 成为 `first-save-A.md` 且内容完整，B 仍是独立的 `Untitled 2` 标签；保存后 A 内 Cmd+Z/⇧⌘Z 有效，B 未受影响 | **PASS** |
| 只读文档（47e） | `readonly-special-blocks.md` 以 `Read only` 打开并渲染块；输入 `RO-EDIT` 不改变文档；Cmd+S 显示原生提示“The document “readonly-special-blocks.md” can’t be saved because its file is read-only. Save it as another file to keep your changes.”（Cancel / Save As…），Cancel 关闭；文件仍为 0444、字节不变 | **PASS** |
| 同名标签与导航（47e） | workspace-same 显示 `notes.md — alpha` 与 `notes.md — beta` 两个标签且内容各自不同；面包屑 `workspace-same / beta / notes.md`；过滤 `notes` 显示 “2 of 3 files”；同时观察到宽窗三栏布局 | **PASS** |
| 主动清空（47e） | 预览中 Cmd+A/Backspace 后 Cmd+S：文件为 1 字节 `01ba4719…546b`（单个 `\n`），退出重启后文档恢复为空白、Saved，文件仍 1 字节；Markdown 模式清空后 Cmd+S 为 0 字节 `e3b0c442…`，两种形式都不被退出覆盖 | **PASS** |
| 并发 A/B 生成（fake ACP，47e） | alpha（paced，pace 15）与 beta（仓库 helper）在 alpha 运行期间先后生成：alpha `ALPHA VIEW` / 请求 `ALPHA-ONLY focus` / `gemini-3.8-flash` / 04:47:44Z；beta `BETA VIEW` / `BETA-ONLY focus` / 同一模型 / 04:47:52Z；两者 `Up to date`，磁盘视图与日志分别匹配各自请求；alpha 运行中 Model 弹层禁用 | **PASS** |
| 取消生成（47e） | alpha `Regenerate all` 后 Cancel：回到原有 `ALPHA VIEW`，面板 `Up to date`，请求与生成时间保持；视图文件哈希 `f98d39cd…3bb1` 不变；日志以 `session/prompt → session/cancel` 结束。取消不产生失败、不提示（接口规则），属规范行为，不是 FAIL 候选 | **PASS（规范行为）** |
| 大目录与监听竞态（#14/#29，47e） | `large-workspace` 只列 `· 1 file`（`audit.md`；514 个无关目录与 16k txt 不列出）；切换 GenUI 到首个含视图内容的 AX 为 **460 ms**（含单次 getAXState 等待，方法已记录）；运行中原子替换视图文件后运行中的视图更新为新文本（约 1.5 s 内，观察上界），恢复原文件后视图回退；`audit.md` 字节不变 | **PASS** |
| 全部编辑后的正常 CmdQ（47e） | 退出后会话保留 3 个工作区（workspace 10 个标签、workspace-same 两个 notes、large-workspace 的 audit + `genui` 模式）；重启恢复 audit 的 GenUI 与其视图、以及清空后的 `first-save-A copy`；退出时没有 Agent 运行 | **PASS** |
| 部分来源的真实同卡片（#28，47e） | `state.sh mixed-card`：单张 `MIXED SOURCES` 卡片显示 `From Removed section, Overview Source not found` 且正文照常渲染，`Edit in source` 与 `Note` 可用；`Edit in source` 落在 Overview 并选中其正文；`Note` 写入 `MIX47-NOTE` 到 Overview 内（`## Details` 之前），卡片仍 `Source not found`、无误报 `Source changed`；对照 `all-missing` 时两者禁用并显示 “None of this part’s sources are in the document now.” | **PASS** |
| 缺失根恢复（47e） | 打开 `workspace-missing` 后退出，重命名根目录再重启：原生横幅 “The folder … is missing or unreadable.”，工作区名保留、`· 0 files`、`No Markdown files in this folder.`，没有恢复任何标签；其余工作区不受影响；目录随后改回 | **PASS** |
| 显式关闭的工作区不复活（47e） | 关闭缺失根窗口后退出，`session.json` 只列 workspace、workspace-same、large-workspace；重启 Window 菜单只有 alpha.md/audit.md/notes.md，没有 workspace-missing | **PASS** |
| Beta 失败告警（47e） | `Regenerate all` 以 `role=alert` 显示 “The agent is not signed in. Run `…/fake-acp-agent acp --login` …”，含 “The agent said:” 与原文 `FAKE-BETA-FAILURE…`，按钮 Retry/Dismiss；原有 `BETA VIEW` / `Up to date` 保留，视图文件 sha256 `39cdc260…d0c7fb` 不变 | **PASS** |
| 生成中退出、不重放（47e） | alpha 以 pace 60 生成到 “Thinking…” 时 CmdQ：应用退出，无 coresmoke/fake-agent 进程残留；日志以 `session/prompt → session/cancel → cancelled-before-write` 结束，视图不变（`f98d39cd…3bb1`）；重启后 alpha 为 Saved v1、无生成中状态、无 Agent 重放 | **PASS** |
| 冷启动首次 GenUI 与监听竞态（47e） | 全新进程、全新路径 `cold2.md` + 从未被监听的视图：点击 GenUI 到首个含视图内容的 AX 为 **499 ms**，无 “No view yet” 中间态；运行中把视图改写为 `COLD-RACE-47E first frame` 后视图更新，恢复原文件（sha `2ffb220b…a8ddf`）后视图回退 | **PASS** |
| Folder 最后标签保留（47e） | workspace2（0 标签）打开 b-doc.md 后，用标签焦点 + Cmd+W 关闭唯一标签：文件夹窗口保留、标签条为空、文件树完整、显示 “Open a Markdown file from this folder.”、标题 b-doc.md；会话随后同时列出 workspace 与 workspace2 | **PASS** |
| 空未命名窗口最后标签关闭（47e） | 关闭未命名窗口的唯一标签后窗口关闭，`session.json` 写为 `{}`（无残留工作区） | **PASS** |
| Agent command 文本字段（47e） | `Locate agent…` 文本字段保存绝对路径后，下一次生成的日志实际使用该绝对路径 | **PASS** |
| 打包字体（部分，47e） | bundle 只含二进制与图标；嵌入的 `web/dist` 含 IBM Plex Sans/SC、Barlow Semi Condensed、KaTeX、Phosphor `.woff2`，二进制含 Phosphor 字符串，UI 截图使用随包字体；Web Inspector 未启用，严格的离线 `FontFace`/网络证明未取得 | **部分证据**（不作严格离线 PASS） |
| Model 菜单可见性（47e） | 47e 时列表已显示 `Agent’s choice / gemini-4-argon / gemini-3.8-flash (selected) / claude-opus-5-5`，缓存也含 3 个模型，但弹层点击丢失 CUA 窗口，未能完成变更；6478 已用弹层选项坐标完成实际变更与持久化（见下） | **47e 未验证；6478 已 PASS** |
| Duplicate 与目标唯一性（47e） | `File > Duplicate` 对 cold2 生成独立的未保存副本 `cold2 copy`（Edited，内容相同），创建时没有新目标可预留，直到它自身首次保存才按路径规则认领；Save As 冲突未被触发（保存表按默认名 `cold2 copy.md` 完成），目标唯一性仍未在原生验证 | **部分（创建 PASS；冲突未验）** |

### `6478ceb` 原生 focused matrix（实际观察）

构建与身份：`/private/tmp/mark-go-go-spec20-verify-21/build/darwin-arm64/Mark Go Go CoreSmoke21.app`，**源码 `6478ceb6d48a16d3a9326c4469f5a1c93411d944`**（验证工作树 `git rev-parse HEAD`，运行标记均为 6478；`go version -m` 证实 `vcs.revision=6478ceb…`、tags `mygo_noinspector`，正常 release）。报告早先误写的 7b header 已更正；47e 的详细证据单独保留，不与本节混引。Settings 与 fixtures 仅用 `native32/`，结束后 `settings.json` 移除、应用正常退出。

| 场景 | 实际观察 | 判定 |
|---|---|---|
| 输入归属与约 250ms 切换 | A 保存 `F6478-A`，B 哈希不变（`51e40b57…`） | **PASS** |
| 首次输入后立即 Cmd+S | 键入到保存约 50ms，`Saved v1`，只有 alpha 含 `F6478-SAVE` | **PASS** |
| Markdown 清空为 0 字节 + Cmd+S | `workspace/Untitled.md` 0 B，sha256 `e3b0c442…b855` | **PASS** |
| typed untitled SaveAs 迁移 + Undo/Redo | 文件 15 B `F6478-UNTITLED\n`，标签迁移；Cmd+Z 移除、⇧⌘Z 恢复 | **PASS** |
| SaveAs 到已打开的 alpha | NSSavePanel 内建 Replace 确认后，应用以 “that file is already open in another document” 拒绝；alpha 不变。报告不声称“无 Replace 提示”，OS 面板提示与 SDK 覆盖守卫分属两层 | **PASS（拒绝）** |
| Rename open beta 到已打开的 alpha | 同样的应用层拒绝；alpha/beta 哈希与 mtime 不变 | **PASS（拒绝）** |
| Rename 成功 + 恢复 | beta→collide-notes→notes-unknown-attrs→beta；171 B `51e40b57…`，视图伴随文件重新找到 | **PASS** |
| Move 已打开根目录 notes 到 `bytes/`（已打开） | 写入前拒绝；根 171 B `51e40b57…`、bytes 578 B `7a1596f0…` 不变 | **PASS（拒绝）** |
| 只读 Save As | 副本 383 B `b2bdc493…` 与原文件一致；原文件仍 0444、mtime 不变 | **PASS** |
| 脏关闭 → Cancel | 标签与内容完好 | **PASS** |
| 关闭保存 untitled 到已打开的 alpha | 提示 `The document “Untitled” could not be saved. markgogo: that file is already open in another document`；untitled 保留、alpha 不变 | **PASS（拒绝）** |
| 带未保存 untitled 的正常 Cmd+Q → 重启 | 恢复 5 个标签与顺序（含无路径活动 untitled）；Recovery 96 B；`QUIT-RESTORE-6478` 存在；磁盘 `Untitled.md` 0 B | **PASS** |
| Model 控件与持久化 | UI `gemini-3.8-flash`→`gemini-4-argon`；`settings.json` model 4；fake agentCommand 与 3 个模型保留；重启后 B/alpha footer 读 `gemini-4-argon` | **PASS** |
| A 生成中的 File 菜单 | `Save` 与 `Duplicate` 可用；`Save As…` / `Rename…` / `Move To…` 禁用 | **PASS** |
| A 生成中切到空闲 B 的 File 菜单 | A 仍 `Saved · generating…`；B 的 `Save As…` / `Rename…` / `Move To…` 全部可用 | **PASS** |
| 已生成视图的记录 vs 新默认 model | `.alpha.md.genui.json` 的 model 仍为 `gemini-3.8-flash`（request `ALPHA-ONLY focus`、06:01:30Z），之后全局默认改为 model 4 并未重写这个已生成视图；这是“已生成视图 3.8 vs 新默认 4”的记录区分 | **PASS（记录区分）**；不据此声称 Native 的即时 settingsChanged 推送或 mid-run freeze，二者由自动化独立覆盖 |
| B 用 model 4 实际生成 | B `Regenerate all` 以 `Up to date`、`Generated 06:05:43Z`、Undo 结束；`.beta.md.genui.json` 为 `gemini-4-argon` / `BETA-ONLY focus` | **PASS** |
| 重启后两文档读取统一默认 model 4 | 持久化 model 4 并重启后，alpha 与 beta 的 footer 都读取 `gemini-4-argon`；A 的已生成视图 JSON 仍为 3.8，B 的新视图为 4。这证明“重启后读取持久化默认 4”，不证明 Native 的即时跨文档推送或 mid-run 捕获（无设置 4 早于 A 完成的时间证据；该点由自动化独立覆盖） | **PASS（重启读取默认 4）** |
| 全量 Regenerate all（真实 fake ACP 经 Go 边界） | 步骤 Starting/Thinking/Reading/Writing；最终 `Up to date`、Undo；`.alpha.md.genui.json` 的 model/request/来源哈希 `fnv1a32:1b6582ef` 更新 | **PASS** |
| Regenerate changed（fake agent 的 update 路径） | UI 如实显示 `createSurface` vs `updateComponents` 的错误与 `The agent said: Alpha turn finished.`，Retry/Dismiss；未覆盖视图 | **PASS（错误处理；fixture 不匹配）** |
| 可重复的取消（paced 60s） | Cancel 回到先前 `Up to date`；无写入；视图哈希不变。Cancel 不是失败、不提示（Spec 规则），旧视图 + 不写入才是要求 | **PASS（规范行为）** |
| 外部改写后台 beta | 出现 `WATCH-RACE-6478`，`Saved v2`；GenUI 标 `Source changed`/过期；Markdown 显示标记；fixture 已恢复 171 B `51e40b57…` | **PASS** |
| 部分来源 | fresh + 仅读者批注；stale `Details changed`；missing `Source not found` 且 Edit/Note 禁用；无哈希不标记 | **PASS** |
| 特殊块与只读呈现 | frontmatter、脚注/回引、warning、code fence、raw HTML、中英文/emoji；`Read only`；Save As 副本可编辑 | **PASS** |
| 根目录外的 Save As 路由 | `workspace2/route-test-6478.md` 16 B 精确；目标标签出现，源标签保留 | **PASS** |
| Note 草稿隔离（A 写 Note，B 关闭） | A 的 composer 草稿 `NOTE-DRAFT-6478`、Save 可用；切到 B 并关闭 B 标签后回到 A，草稿仍在且 Save 可用；点击 Save 后 composer 关闭，A 视图 Notes 出现 `03 NOTE-DRAFT-6478`，A Saved | **PASS** |
| 两个挂载的相同 id GenUI（g1/g2） | g2 变更后 `10 components out of date`、全部 `Source changed`、列表 Summary + Detail 1…9、`Regenerate changed (10)`；点击列表最后一项 `Detail 9`：g2 标签保持选中、列表滚动并高亮 Detail 9、主视图显示可见的 DETAIL 9 卡片（绿框）；g1 仍 `Up to date`、无 Source changed。fixture 恢复后 `verify.sh` 显示 **ALL FIXTURES MATCH MANIFEST** | **PASS** |
| ComparisonTable 数值排序（6478 补充） | 鼠标点击表头把 Size 标记切到 ↑，行变 9/12/180（按数值而非文字）；降序与第三个循环本轮未能可靠到达；键盘/AT 激活未暴露。7b 已有的完整键盘三态循环证据保留（该代码路径未被 6478 改动），本行只是 6478 的补充观察 | **PARTIAL（不标 PASS）** |
| Edit 菜单 Undo/Redo 可用状态 | Edit NSMenu 返回空 AX，菜单项 enabled 状态未取得；快捷键撤销/重做证据存在，但菜单状态不作声明 | **BLOCKED（菜单 AX）** |
| 打包应用本地字体加载（source64 debugInspector 包，明确与 release core 分开） | 实际观察到加载的 FontFace：IBM Plex Sans/SC 与 MGG Condensed 500/600；`baseURI=mygo://localhost/`；body 的计算字体为 Plex Sans/SC。冷启动网络面板显示 12 个资源、全部来自 localhost；本地 woff2 含 SC Medium/Regular、Latin 400/500/600、Barlow 500/600；嵌入的自定义 scheme 传输 0。**这是本地字体加载、无外部字体请求的实际 PASS**。严格“强制 offline / 阻断请求”的 Inspector 控件不存在，该方法 **BLOCKED / 未运行**，不能声称 network-off 已跑；也不能反过来说字体本地加载未验证。无系统网络改动、无 OS 权限操作 | **PASS（本地加载）；严格 offline 方法 BLOCKED** |
| 脏未命名 Close → 原生 Delete（debugInspector 构建，close 逻辑与 release 相同） | 原 FAIL：genui folder 窗口已有 g1/g2 两个标签时，新建 Untitled 输入 `DISCARD-ONLY-6478`，Cmd+W 选原生 Delete：Untitled 消失，同时整个 genui 窗口消失、`session.json` 不再含 genui；未报告原文件被写 | **已修并 c442 实机 PASS（原场景）**：c442 normal release 上同一路径的 Delete 只移除新的 Untitled，g1/g2、窗口内容与哈希都不变；历史 FAIL 证据保留不删。Standalone untitled 的新 route 变体另见未运行行 |

### `c44239c` 原生关闭回归（normal release，实际观察）

源码 `c44239c8ca5fdee802806dbf47e5bb2fd0ac1f9c`，normal release 构建，`Main` 已验签；Actor 只复验关闭相关路径，不重做 fonts/Model/OS。

| 场景 | 实际观察 | 判定 |
|---|---|---|
| 脏未命名 Close → Cancel | Cancel 保留 3 个标签与文本 | **PASS** |
| 脏未命名 Close → Delete（原 64 FAIL 场景） | Delete 只移除新的 Untitled；g1/g2、窗口内容与文件哈希不变 | **PASS（原场景修复）** |
| 正常 Quit 与重启 | `session.json` 含 genui 两个标签、active 为 g2；relaunch 恢复两个标签且均可选择 | **PASS** |
| 已保存的 g2 上 Cmd+W | 只关闭 g2，g1 仍在 | **PASS** |
| g1 也关闭后 | folder 工作区 0 标签，窗口仍保留 | **PASS** |
| 0 标签时 Cmd+W / File > Close | `File > Close` 由 `RefreshDocumentMenus` 禁用，原生 Cmd+W 为 no-op；窗口不关闭（既定规则） | **PASS（规则行为）** |
| 原生红色关闭 alpha 工作区 | 整个窗口关闭；alpha/beta 两个文件哈希不变 | **PASS** |
| Standalone untitled 最后 tab 关闭（新 route） | 红关剩余测试窗口 → 正常 quit，`session.json` 的 `workspaces` 为 `[]`；relaunch 显示 Welcome `Untitled window · 0 files`；File > New 得到唯一 Untitled，输入 `STANDALONE-C442`，Cmd+W 选 Delete 一次：文档消失且**整个空的 untitled 窗口被销毁**，之后 AX 不再有该窗口；再次正常 quit 后 `session.json` 仍为 0 个 workspace，哈希 `f3696db8…fb266`；settings absent、0 进程。47e 的旧“最后 tab 关闭窗口”PASS 是旧整窗 CmdW 路径的平凡结果、不是新 route 证明，但本行是现在的新证据 | **PASS（new route，非旧 window-close 平凡结果）** |

### 代码审查发现与修复状态

初始审查（head `6f`）中，`SpecReview32`（spec 轴）与 `Standards`（标准轴）提出硬缺陷与启发式；下表保留这些发现的解决状态。修复后两轴对 `6f3dba2..6478ceb` 的 focused 复查均 PASS（Standards 0 新违规、Spec 0 新硬缺陷），见上文“两轴 focused 复查”。“已修”指 `6478ceb` 上的修复与全量自动化验证；其中路径菜单、目标唯一性、GenUI/card 跳转与 Composer 草稿隔离已由 `6478ceb` 原生矩阵覆盖，索引性能等未做原生专项测量。

| 发现 | 来源 / 性质 | 现状 |
|---|---|---|
| 生成中的文档，其原生路径菜单与错误提示不完整（`Save As` / `Rename` / `Move` 的禁用与提示没有完整覆盖 `generating` 状态） | SpecReview 硬缺陷 | **已修，自动化通过；Native 6478 PASS**（A 生成中 Save/Duplicate 可用、SaveAs/Rename/Move 禁用；切空闲 B 后三者可用）：`6478ceb` 的 `PathChange` 锁 + `RefreshDocumentMenus` |
| `Save`（只读时回退另存为）、`Rename`、`MoveTo` 的目标唯一性检查发生在写盘、改 `doc.Path()` 并触发状态之后，冲突时可能覆盖已打开文档的磁盘文件并留下两个同路径会话 | Standards P1（同一问题族见 SpecReview） | **已修，自动化通过；Native 6478 PASS**（SaveAs/Rename/Move/close-save 到已打开目标均应用层拒绝，目标未写、哈希与 mtime 不变；OS 面板 Replace 边界见“Spec 短裁定”）：`ReservePath` 在任何写入/改名/移动之前认领目标 |
| `GenUIView.scrollToCard` 用全局 `document.querySelectorAll('[data-testid="genui-view"] [data-component-id]')` 而没有限定到本视图的挂载元素；隐藏的 A 标签 GenUI 与活动 B 标签组件 id 相同（如 `root`、`summary`）时，B 的 `.changed-list` 点击会命中 A 的卡片，跳过 B 的高亮与滚动 | Standards P2（Tab 与文档隔离） | **已修，自动化通过；Native 6478 PASS**（g1/g2 相同 id：Detail 9 命中 g2 本卡并高亮，g1 保持 fresh）：`8a6be09` 按文档挂载元素限定查询 |
| `noteDialog.ts` 的 `activeCancel` 是模块级全局：任何后台标签的 `load` / `onReload`（`setMarkdown`）/ `setReadonly` / `destroy` 都会调用 `cancelNoteComposer()`，关闭活动标签正在输入的 Note 或特殊块 `.composer` 并丢弃草稿 | Standards P2（后台面板独立状态） | **已修，自动化通过；Native 6478 PASS**（A 的 `NOTE-DRAFT-6478` 跨 B 关闭保留，保存后写入 A 的 Notes）：`8a6be09` 按编辑器限定取消 |
| 原生命令对同一文档重复 flush（`App.tsx` 的 `host.flush` 与 `mygoHost` 的 `flushDocument` 各发一次；`closeWorkspace` 路径每标签可达四次），每次都触发 `reindexOpenDocument` | Standards 启发式 P2（重复代码） | **已修，自动化通过**：`6478ceb` 让 `bindFlush` 成为唯一原始文本读取器，每个文档在每次入口调用中只写回一次；6478 的保存/切换/菜单行为与该实现一致，但 RPC 次数本身不可原生观察，故不单列 Native 次数结论 |
| `utf16Count` 每行分配 `[]rune` 与 `[]uint16`；`navRel`/`relOf` 在树排序与过滤时对每一项调用 `canonicalPath`（`EvalSymlinks`），而索引里已有相对键 | Standards 启发式 P2（Go 分配/系统调用） | **已修，自动化通过；Native 未验（未做专项测量）**：`3b0923e` 复用索引键、按 UTF-16 单元计数不再分配 |
| 四处相似的 tab 移除 + 活动回退压缩（`documenthost.go` 与 vendored MyGo 的 document group） | Standards P3 启发式 | **暂接受**：按现有 app/kernel authority 保持局部重复，避免为四处小逻辑引入跨模块抽象；不是缺陷关闭，只是不做抽象 |

这些发现与 `47e3b4a` 的实机 PASS 分属不同证据类型；修复状态按源 `6478ceb` 记为“已修，自动化通过；Native 未验”或“待修”。自动化通过指 `6478ceb` 上的真实运行时验证（真实 Chromium 与 Go race），不等于原生复验通过，也不等于总体验收完成。

### 仍未运行 / 受策略阻塞的原生行

以下按最新报告的实际缺口保留，未执行的不因相邻场景 PASS 而升级：

| 范围 | 状态与缺口 |
|---|---|
| 生成中绕过菜单直接触发路径变更 | 6478 已观察 A 生成中 `Save As…`/`Rename…`/`Move To…` 菜单禁用、B 空闲时可用；应用层对生成中直接调用的拒绝由自动测试覆盖，本轮未用原生绕过菜单单独触发 |
| ComparisonTable 完整排序与键盘/AT | 7b 已有完整键盘三态循环证据（该路径未被本轮代码改动），保留原 SHA 已跑结果；6478 只补充了鼠标一次升序（9/12/180 按数值）为 PARTIAL；辅助技术激活未取得，不标 PASS |
| 原生 Edit 菜单 enabled 状态 | 6478 的 Edit NSMenu 返回空 AX，未取得菜单项状态；快捷键撤销/重做证据存在，菜单状态不作声明 |
| 严格强制 offline / 请求阻断（字体） | 本地字体加载与“无外部字体请求”的实际 PASS 见上表；Inspector 没有强制 offline/阻断请求控件，因此该方法 **BLOCKED / 未运行**，不声称 network-off 已跑。narrow/wide 已在 47e 观察 PASS，6478 未重复 |
| 标准 release 包状态 | `c442` normal release 已由 Main 验签并用于关闭回归；字体结果来自 `source64 debugInspector` 诊断包（与 release core 分开），诊断结束已恢复正常 release |
| 辅助技术（#31） | **用户取消本轮系统级验收**：VoiceOver 验证不再进行；未验证、**不计 PASS**，原状态 0/0。历史事实：真实 Cmd+F5 已尝试仍为 VO 0，fn key 不支持，System Settings 与 VoiceOver Utility 两个 target 均被 CUA 拒绝（工具层）。 |
| reduced motion（#25） | **用户取消本轮系统级验收**：不再申请权限、不再验证；**不计 PASS**，`ReduceMotionEnabled` 原状态 0。 |

> 外部 saved file 经 Open 打开时会在父目录建立 `folder` root 并保留 0 个标签；这是 folder 工作区契约的预期行为（见 `docs/workspace-contract.md`），不是 temporary-window 关闭失败。

### 设置审计（2026-11-08 复核）

| 控件 | 状态 | 依据 |
|---|---|---|
| Model（右栏底部 `ModelMenu`） | 已实现；只提交 `{ model }`；跨文档的原生传播尚未验证（见未运行行的 Model 说明） | `settings.go` 的 `SettingsPatch`；`GenUIView.tsx` 的 `selectModel` |
| Locate agent…（`agent-not-found` 失败内） | 已实现；只提交 `{ agentCommand }`，空值恢复默认 | 同一补丁类型；`GenerationFailure` 的定位输入 |
| 生成要求 Focus（右栏，原地 Composer） | 已实现；随视图文件保存并在重新生成时沿用 | `view-format.md` 的 `request` |
| 视图模式（Preview / Markdown / GenUI） | 已实现；按 `documentId` 随会话保存 | `SetViewMode`、`session.json` |
| ACP 模型列表缓存 | 已实现；只由 `session/new` 握手写入 | `settings.go` 的 `setModels` |
| 字体、字号、主题、外观切换、系统提示词 | **N/A（按设计不提供）**，不是未实现控件 | ADR 0006：本轮固定浅色与离线字体；界面无此类入口 |
| 原生保存表 / Edit 菜单归属 / VoiceOver | 保存表（首次输入 Cmd+S、typed untitled Save As）已在 47e 原生观察；Edit 菜单点击与 VoiceOver 仍未验证，见上表 | 上文的原生证据与未运行行 |

### 完成门槛

- 只有当每一个**适用**的验收行都实际通过——自动化行有真实命令结果、原生行有打包应用上的实际观察——本规格才算完成。静态阅读、代码推断、文档修订或“应已修复”都不构成通过。
- 未运行的行保持“未运行”；用户取消或策略受阻的项按取消/阻塞记录。它们不会因为时间推移、文档完成或同源修复而自动变成 PASS。关闭 issue 也以此为前提。
- 适用范围：用户已取消本轮 VoiceOver 与 reduced motion 的系统级验收，因此它们排除在本轮必过条件之外，但仍**未验证、不计 PASS**（原状态 0/0）；不得据此声称原规格的全部辅助技术验收已通过。历史工具层拒绝（CUA 对 System Settings / VoiceOver Utility 的两个 target denied）仅作事实保留。
- 运行时验收基线是 `47e3b4a`；docs-only 修订不产生新的运行时证据，也不回填或改写已观察事实；`6478ceb` 的源代码冻结只由全量自动化与两轴复查覆盖，原生矩阵单独记录。

## 未验证 / 已知问题

以下条目记录 2026-11-07 及更早的行为；被本轮规格取代的条目已就地标注，未删除历史。

- Edit 菜单的 Undo/Redo 显示为禁用（WKWebView 自身 undo manager 为空，历史在 ProseMirror）。键盘 ⌘Z 不受影响；用鼠标点菜单撤销不可用。需要时让菜单经 IPC 调用编辑器命令。**已被 #30 取代**：原生 Edit 菜单现在作用于点击时捕获文档的真实编辑器历史，`SetNativeEditState` 报告可用性；原生复验见上文 BLOCKED 行。
- 自动化工具注入中文只落下首字，属注入方式限制；真实输入法已人工确认正常。
- 外部重载会重建编辑器：撤销历史和光标重置（已接受）。
- 编辑过的块按 Milkdown 规则重写（setext→ATX、表格补齐、块内 CRLF→LF）；新块前面的间隔是 `\n\n`，它后面的块保留自己的间隔（#13）。
- GenUI 视图由 `agy acp` 生成（#7），失败分类与清理见 #8，局部重新生成与撤回见 #12。A2UI 校验不过的视图显示问题列表（`role="alert"`）和 `Regenerate all`；有备份时也可以撤回。
- 撤回只有一份备份；视图里已经是局部更新的组件，下一次局部更新后 `a2ui` 继续变长（不压缩）。
- Agent 隔离只是尽力而为（ADR 0003）：agy 按用户自己的 `~/.gemini/antigravity-cli/settings.json` 运行，可能不请求权限就写目录外文件或运行 shell。宿主看到越界工具调用就停止运行、不写视图，但无法阻止已经发生的那一步。
- ~~只有 GenUI 有深色令牌；应用其余部分（头部、大纲、编辑器）还没有深色主题，`:root` 固定为 `color-scheme: light`。~~ **已被 ADR 0006 取代**：本轮统一为仅浅色界面，没有外观切换；GenUI 与外壳共用浅色 Token。样式表残留的 `.dark` 块不被任何界面启用，也不受支持。
- ~~ComparisonTable 的列头按钮在 macOS 辅助功能树里不出现（表格行可见），自动化只能用坐标点击。~~ **已由 #31 取代**：列头是真正的按钮，支持键盘操作并暴露 `aria-sort`；Chromium 测试覆盖键盘排序。真实 WebKit/VoiceOver 行为仍待原生复验（见上表 BLOCKED 行）。
- ~~多来源组件只要有一个来源 `missing` 就禁用 `Edit in source`，即使其他来源还在。~~ **已由 #28 取代**：只要还有来源存在，`Edit in source` 与 `Note` 就用第一个仍存在的来源；只有全部失效才禁用。
- 打开大目录工作区（同目录 516 项，含子目录约 1.6 万个文件）首次切到 GenUI 时，旧版本中央区域空白一段时间后才显示视图。**已由 #29 修正代码路径**：`WatchView` 先送出第一份快照再建目录监听，随后追赶快照，GenUI 也有明确的加载状态；真实原生首次视图就绪时间与监听竞态仍需在准备好的大目录上测量（见上表 `#14/#29` 行），不得沿用旧推测作为结论。
- 大纲在预览模式按顶层块粒度高亮章节：光标在同一顶级块内移动不会改变当前章节。
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

## 2026-11-10 可靠性复验

原生检查使用 macOS arm64 上的 MyGo v0.7.4 隔离调试构建。独立的包标识、数据目录和 Markdown 文件用于区分测试与日常使用。MyGo Automation 负责查询 WebView 和调用菜单，系统确认框通过实际窗口操作。

| 场景 | 实际观察 |
|---|---|
| 启动并保存 | 基线打开 Markdown 时，AppKit 将 `validateMenuItem:` 发给缺少该实现的 WebKit 父类，进程退出。修复后，真实 WebView 显示样本文字，File > Save 将编辑写入样本文件。 |
| 取消关闭后恢复 | 同窗打开一个已保存文件和两个未命名文档。基线依次选择 Delete、Cancel 后，三个标签仍显示，但第一份恢复副本消失。修复版保留三个标签和两份恢复副本，正常退出重启后，两份未保存文字都可见。 |
| 确认关闭 | 重启后再次关窗，依次对两份草稿选择 Delete。两份恢复副本被清理，已保存样本的磁盘字节一致。 |
| 退出写入失败与重试 | 在隔离目录中用同名目录阻断 `session.json` 写入。通过原生 Quit 菜单退出时出现错误提示，工作区和两份恢复副本仍在。恢复可写路径后再次退出，会话文件写入成功。重启后恢复三个标签和两份未保存文字。 |
| 旧退出回执未返回时重试 | Chromium 回归让首次 flush 一直等待，第二次退出尝试仍完成写入和回执。原生调试包拒绝了过期的退出写入，正常退出并重启后，两份草稿仍可读取。 |
| 关窗确认与应用退出 | Go 回归确认重复的关窗请求仍等待原有确认，已经准备好的应用退出可以关闭该窗口。单窗口原生确认框挡住了 Quit 操作。取消确认后，原生退出与草稿恢复通过。 |

自动化检查包括 Go 静态检查与竞态测试、TypeScript 检查，以及 Chromium 的 29 个测试文件、285 个用例。仓库内的 MyGo 在关闭 cgo 的情况下通过 macOS arm64、Linux amd64 和 Windows amd64 编译。常规 macOS arm64 `.app` 在隔离副本中构建成功并通过本地签名校验，常规构建的 Go 依赖图中没有 Automation。MyGo 的 `hello` 和 `counter-native` 示例在相同生产构建参数下与基线大小一致，分别为 9,578,386 和 9,406,290 字节。
