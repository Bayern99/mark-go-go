# 验收结果（2026-11-06，原型）

环境：macOS arm64，Go 1.27.3，MyGo v0.7.4，Milkdown 7.23.0，Node 24。

## 自动化

| 检查 | 命令 | 结果 |
|---|---|---|
| Go | `go vet ./...` | 通过 |
| TS 类型 | `npm --prefix web run typecheck` | 通过 |
| 测试 | `npm --prefix web test`（真实 Chromium） | 9 个文件、96 项通过 |
| 打包 | `go tool mygo build` | `Mark Go Go.app` 12.2MB，dmg 4.0MB |

测试覆盖：21 个刁钻语料文件 + 编辑用例零编辑 compose **逐字节一致**；改一段只改变该块字节；编辑后 compose 幂等；插件禁用释放视图/命令/监听，重新启用正常，`activate` 失败回滚。

## 源码模式（#3）

自动化（`web/test/source-mode.test.ts`、`web/test/app-source.test.tsx`，真实 Chromium）：

- 21 个语料文件：进入源码 → 文本与 `getMarkdown` 逐字节一致 → 不编辑退出 → 逐字节一致，且无 change 事件。
- CRLF、混合换行、无末尾换行、单独 CR：源码中编辑无关行，其余字节不变。
- 源码模式 undo/redo；切换保留/重建 WYSIWYG 的边界（未改动保留撤销历史，改动后以新文本重建）；切换不触发 change。
- 源码中 `reveal` / `onSelectionChange` 为精确偏移；切换模式后在新模式 reveal 旧光标偏移。
- 源码中外部 reload 替换文本、保持模式并重置历史；只读文档在源码中只读。
- App：头部按钮与 `⌘/`、`Ctrl+/` 均可切换；进入/退出无编辑时 `host.update` 从未调用；源码中改 frontmatter 并去掉 `[!NOTE]` 后 host 收到期望字节，回到 WYSIWYG 该块是可编辑引用而非锁定块，随后 WYSIWYG 编辑仍保留源码改动的字节；源码 undo；源码中 reload 保持模式；只读 host；GenUI chip 在源码模式按精确偏移跳转。

打包应用手动检查（**PENDING manual**，需在 `go tool mygo build` 产物中执行）：

| 检查 | 状态 |
|---|---|
| `⌘/` 在打包应用中切换源码 / WYSIWYG，焦点与滚动位置正确 | PENDING |
| 源码模式内 `⌘Z` / `⇧⌘Z` 撤销重做（真实键盘） | PENDING |
| 源码模式内中文输入法输入正常（真实 IME） | PENDING |
| 源码模式编辑后自动保存：状态栏 Edited → Saved，文件字节与源码一致 | PENDING |

## 打包应用手动冒烟（`/tmp/mdws-smoke/report.md`：frontmatter、alert、setext 标题、`+` 列表、CJK 表格、带 meta 的代码块、引用式链接）

| 场景 | 结果 |
|---|---|
| 通过 `open -a` 打开 `.md` | 窗口标题 `report.md`，内容正确渲染；frontmatter / alert / 代码块 meta / 链接定义显示为锁定块 |
| 打开后不编辑 | 文件 sha1 不变 |
| 输入文字 → 自动保存 | 状态栏 Edited → Saved；`diff` 只有新增的一行，其余字节不变（`+` 列表、setext 标题、`~~~` 代码块均保留） |
| 外部进程改写文件（模拟 Agent） | 自动重载，版本 v1 → v2，状态 Saved，编辑器显示新内容 |
| 退出 | AppleScript quit 正常退出，文件为外部写入的版本 |
| GenUI 开关 | 面板显示固定 JSON 组件 + 实时标题列表（中文标题正确） |
| ⌘Z 撤销（人工，真实键盘） | 正常。Edit 菜单 Undo/Redo 显示禁用但不拦截按键，按键到达 ProseMirror |
| 中文输入法（人工） | 正常 |

## 未验证 / 已知问题

- Edit 菜单的 Undo/Redo 显示为禁用（WKWebView 自身 undo manager 为空，历史在 ProseMirror）。键盘 ⌘Z 不受影响；用鼠标点菜单撤销不可用。需要时让菜单经 IPC 调用编辑器命令。
- 自动化工具注入中文只落下首字，属注入方式限制；真实输入法已人工确认正常。
- 外部重载会重建编辑器：撤销历史和光标重置（已接受）。
- 编辑过的块按 Milkdown 规则重写（setext→ATX、表格补齐、块内 CRLF→LF）；插入/删除块后的块间空白回退为 `\n\n`。
- GenUI 面板的组件数据是固定 fixture，与当前文档无关；仅标题列表是实时的。
- `markdownUpdated` 每次编辑都会在内部整篇序列化（仅耗时，不影响保存内容）。
- 纯 `go build`/`go run` 不处理文件参数；须用 `go tool mygo dev|build`。

## 失败条件（触发 ADR 0001 重新评估）

- WebKit 升级或 MyGo 升级后 ⌘Z / 中文输入法回归。
- 任一未修改文档 compose 后非逐字节一致。
- 日常文档中锁定块比例过高。
