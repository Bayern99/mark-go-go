# macOS 原生验证

从项目根目录运行与 MyGo 同版本的官方 CLI 和原生 smoke。

```sh
go tool mygo-automation skills
scripts/native-automation-smoke.sh
```

脚本从 Go 源码和应用资源打包 QA 应用，创建临时文稿、独立应用标识和私有 Unix socket。它从 Finder 的系统打开入口送入第二篇文稿，通过 Automation 确认窗口为 Content、可访问树包含 Markdown 源码编辑器，再用键盘输入和文件菜单保存。脚本比较含混合换行与 Unicode 的完整文件字节，检查编辑菜单的撤销、重做，并在退出后启动新进程核对活动文稿。设置 `MYGO_AUTOMATION_REPORT` 可选择截图路径。[原生 smoke](../../scripts/native-automation-smoke.sh)

手动检查时，在具有独立应用标识的测试副本中，用 `GOFLAGS=-tags=e2e` 和 `MYGO_AUTOMATION=<私有 socket 路径>` 启动 `go tool mygo dev`。通过 `go tool mygo-automation` 的 `windows`、`snapshot --full`、`key`、`type` 和 `menu` 观察和驱动真实控件。系统文件对话框、输入法和 VoiceOver 使用对应的 macOS 服务实测，记录构建标识和操作结果。[测试构建入口](../../automation_e2e.go) · [MyGo 发布守卫](../../third_party/mygo/plugins/automation/off.go)

2026 年 11 月 20 日在 macOS 27.2（26B123）arm64、Go 1.27.3 和 MyGo 0.10.0 上，首个原生窗口通过上述 smoke。Automation 读到 `[content] "Discussion.md"` 和 `[textField] "Markdown source"`。在含 CRLF 与 LF 的夹具开头输入 `前` 和 U+1F642 后，菜单保存写入预期字节，撤销和重做改变控件文本。退出后，新进程重新显示相同文稿，完整文件比较通过。同一源码的默认构建产出 macOS app 和 DMG，Linux amd64 与 Windows amd64 完成交叉编译。

## Portrait 工作区迁移验收

2026 年 11 月 20 日在 macOS arm64 实机使用独立 app、数据目录和合成文档。原生窗口通过 Finder 打开第二份原文，在 Preview 和 Markdown 间切换，并通过文件菜单保存。混合换行、中文和 emoji 的磁盘字节比较通过。Undo、Redo 和重启后恢复活动标签及 Markdown 模式均通过。运行 `scripts/native-automation-smoke.sh` 可复跑这一组操作。

使用与 [#60](https://github.com/Bayern99/mark-go-go/issues/60) 相同的 256 节长文，实机截图与可访问树覆盖 1280 × 820、1180 × 820、1179 × 820、641 × 500、640 × 500 和 480 × 360。实测 640 点及以下使用导航浮层、641 点起左栏停靠、1179 与 1280 点间切换研究栏，以及浮层按 Esc 返回入口焦点。通过截图发现选中胶囊曾覆盖文字，修正后复拍六档。最小窗 Preview 的标题顶距已按旧版位置调整。 [原生控件与像素回归](../../internal/nativeui/shell_test.go) · [同机截图流程](portrait-performance.md)

另一组迁移验收将旧版 WebKit LocalStorage 中的插件开关、研究栏宽度、视图模式、折叠与列宽数据导入原生偏好文件。它验证了 Markdown 恢复、研究开关与旧键值迁移。折叠与列宽的视觉恢复由对应编辑功能验证。大纲跳转、文件和标题搜索、打开结果及 Ctrl+Tab 返回原文通过。对合成文件制造外部冲突和删除后，告警及未保存草稿均保留。

原生文件菜单和恢复流程见[系统输入、文件菜单和会话](#系统输入文件菜单和会话)。旧版实机结果见 [PR #41](https://github.com/Bayern99/mark-go-go/pull/41)、[PR #43](https://github.com/Bayern99/mark-go-go/pull/43) 和[既有验收记录](../acceptance.md)。多窗口退出和重试的服务回归见 [session_test.go](../../internal/workspace/session_test.go)。

## Preview 复杂块

在 macOS arm64 的独立 Content 窗口中，含公式、Mermaid 和中英文表格的原文显示了两个原生图像和四个单元格文本框。通过“Edit math”打开原地编辑，输入新公式并用 ⌘↩ 提交。文件菜单保存后，磁盘出现新公式。菜单 Undo 后再次保存，磁盘恢复原公式。首次打开时的可访问树还暴露出高图形与下一块短暂重叠，编辑器现于测得实际高度后请求下一帧。

运行 `go test ./internal/nativeeditor` 可复核对齐、单元格编辑与撤销、矩形复制、相邻列拖动、宽表独立横滚、脚注往返、键盘折叠、批注冲突及混合换行保真。兼容用例载入旧版折叠和列宽签名。[原生编辑用例](../../internal/nativeeditor/complex_test.go)

在原生 Preview 切换任务复选框，保存后只有该任务的 Markdown 标记变化。通过链接浮层编辑目的地址，保存后完整文件只有 URL 变化，原有标题、标签格式和混合换行均保留。两项操作分别切到 Markdown 撤销及保存，原文件逐字节恢复。 [Preview 用例](../../internal/nativeeditor/preview_test.go)

用带 `title="demo.py" linenos` 的 Python 围栏复验特殊代码块。原生窗口分别显示语言和元数据，正文以等宽字体显示并保留缩进，Preview 和 GenUI 往返后保存文件与夹具完全一致。

## GenUI 阅读和生成

2026 年 11 月 20 日，在独立应用标识的 macOS 调试包中打开带两处同名标题、CRLF 和 emoji 的文稿。通过原生控件打开已有旁车，核对缺失与已改来源，点击清单进入第二个标题的原文。Preview 的可访问选区为 36–50，对应引用的完整文字。返回 GenUI、切换内页并对金额排序后，原地添加含评论结束符和 emoji 的批注。保存后比对整份原文，只有转义后的批注和分隔换行增加。

生成验收使用实际工作区服务、文件观察器和独立 ACP 测试进程。完整生成保存了所选模型、生成要求与对应章节的哈希。局部更新保留另一页的表格，恢复操作逐字节还原前一份旁车。无效输出、已接收 prompt 后取消、重试及外部创建、损坏、删除和重建旁车均通过。两个文档同时生成时，挂起文档的要求、取消操作和结果均归属该文档。切换模型后再次生成，新旁车记录了新模型。本次使用测试进程提供可复现的 ACP 回复。

在原生窗口打开版本 2 旁车，确认基于文档数据的标题在 GenUI 中显示为 `Read Native sidecar`，模板按数组渲染 `Alpha row` 和 `Beta row`。另以旧格式 `path` 和 `call` 读取标题和模板数据。切回 Preview 后确认本地图像按阅读宽度呈现，普通 Go 代码块有语言、语法配色与 Copy code，链接有可访问的打开控件。视图往返前后原文逐字节一致。[原生编辑用例](../../internal/nativeeditor/preview_test.go)

原生旁车支持 JSON Pointer、数据模板、字符串插值、逻辑与部分校验函数。`formatNumber`、`formatCurrency`、`formatDate` 和 `pluralize` 使用旧版的 `en-US` 默认格式。以 1234.5 为输入，原生引擎对 307 种货币的输出均与这台 Mac 的 JavaScriptCore `Intl` 一致。日期覆盖四位年份的 ISO 日期和时间。字符串插值里的嵌套函数仍有兼容差异。 [GenUI 读者用例](../../internal/nativegenui/surface_test.go)

## Agent 提案和检查点

2026 年 11 月 20 日，在隔离的 macOS arm64 调试应用中，用真实 loopback MCP 会话读出 Markdown 编辑器里尚未保存的字节、章节和批注。通过 MCP 创建研究产物并提交两份文稿的提案。在原生比较面板打开第一份后继续输入，原比较随即失效。重新比较，接受第一块，第二块仍待审阅。切到 Preview 用 Edit → Undo，整份实时缓冲与应用提案前的字节一致。拒绝余下的块后，第二份文稿的提案仍在队列。

在有既存暂存内容的 Git 工作区，创建带标签的检查点、比较和恢复，再切换视图撤销恢复。前后比对 HEAD 与完整暂存差异，结果相同。在普通目录以同一界面创建 Local snapshot，比较、恢复、保存并比对完整文件，随后跨视图一步撤销恢复。提案与检查点实测复用正常文档和编辑服务，测试启动时注入的是隔离 MCP 会话注册。

## 系统输入、文件菜单和会话

2026 年 11 月 20 日，在 macOS 27.2 arm64 上运行独立测试包。提交 `c6504af` 通过系统简体拼音输入法，在 Preview 以物理按键序列 n i h a o 和空格完成「你好」组字。文件菜单保存后，完整文件仅多这两个字的 UTF-8 字节。切换到 Markdown，执行一次 Edit → Undo 并保存，恢复到原始文件。

在集成提交 `5968525` 上，先在 Preview 修改标题，再切到 Markdown 在正文尾部追加文字，切回 Preview。第一次 Undo 保留标题修改，第二次还原完整原文。两次 Redo 恢复两次修改。每一步保存后与预期文件逐字节比较。

VoiceOver 系统字幕读出了 Preview 编辑区的名称、类型、插入位置和标题。启用 VoiceOver 后，使用投递到 macOS HID 层的系统键盘事件修改标题，以 ⌘S 保存，再用 ⌘Z 撤销并重新保存。逐字节检查改动和恢复。以方向键移动时，字幕跟随插入位置朗读正文。合入 AppKit 按键修复后，在 VoiceOver 启用的 480 × 360 窗口中通过 Ctrl+Tab 切换文稿，以 ⌥⌘0 打开研究抽屉，聚焦输入框并用系统键盘输入和发送。发送后输入框清空，消息出现在测试会话中。核对系统开关恢复关闭。

在系统设置开启 Reduce Motion 后，用 480 × 360 原生窗口打开导航抽屉，按 Esc 关闭并将焦点还给入口。使用 ⌘/ 往返 Preview 与 Markdown。测试后恢复原来的 Reduce Motion 值。

正式构建的性能校准在 AppKit 窗口层复现 Ctrl+Tab 未切换文稿。诊断包确认物理按键到达 NSWindow 后，没有进入原生内容视图。修正转发后，480 × 360 实测 Ctrl+Tab 与 Ctrl+Shift+Tab 在两篇文稿间切换，普通 Tab 仍可从 Preview 控件移至 Markdown。 [MyGo 补丁](../mygo-document-groups.md)

另存、改名及移动分别通过原生系统面板完成。复制创建了独立带修改状态的标签，保存后逐字节对照。关闭未保存的草稿时，Cancel 保留草稿，Don't Save 只关闭该标签。已保存文件的字节保持一致。

通过真实 Finder 拖放，将一份合成 Markdown 文件投放到 Preview 的编辑区，原生窗口打开文稿。切到另一篇文稿的 Markdown 源码界面，再投同一文件，应用定位到原来的标签。哈希一致。AppKit 诊断包也观察到 `public.file-url`，协商 Copy，解析出一个文件并完成打开。首次尝试的落点被系统权限弹窗遮挡，复测在无遮挡区域执行。

通过文件系统打开同一窗口的两个根目录。关闭第二个根后，原窗口保留第一个根的两篇文稿，第二个根的文件仍与基线字节一致。重新打开第二个根，在测试数据目录暂时用空目录占据 `session.json` 路径。真实应用菜单 Quit 显示保存会话失败的系统面板，并保留进程和文稿。关闭提示后切换活动文稿到 Markdown，移除测试目录，再执行 Quit。应用重新写出会话文件并退出。无参数启动新进程后，两根的三个标签、活动文稿和 Markdown 模式均恢复。

## Feynman 窄窗

在 480 × 360 窗口中以 ⌥⌘0 打开研究抽屉。第一次实测发现发送与附加控件被裁掉，后续验收也发现对话滚动区高为零。经过修正，最终窄窗实测滚动区高 73.3 点，输入框高 44 点，附加与发送均在抽屉内。发送测试请求后滚动到本轮回复，系统辅助功能树确认回复可见。

使用原生文件面板选择 `Attachment.md`，抽屉出现可移除的附件。在历史记录中创建对话，同根切换文稿保留本轮消息。切到另一根后读取另一工作区的对话，返回第一根时测试进程重放已选会话。按 Esc 关闭抽屉并还原入口焦点。 [可复跑的 Feynman 验收](../../scripts/native-feynman-qa.sh)

## 正式候选包

从提交 `9a22b9c` 导出干净源码，在 PATH 中无 Node 或 npm 的情况下构建 0.4.0。正式标签为 `mygo_nodownload,mygo_noinspector`，包内二进制为 Mach-O arm64。应用包通过严格验签，DMG 通过映像校验。只读挂载确认二进制、Info.plist 和 Applications 快捷方式，挂载的应用通过相同验签。包内资源检查覆盖 HTML、CSS、JavaScript 和 WebAssembly。此本地候选包经 ad hoc 签名，使用 `-skip-notarize`。

DMG 大小为 23,952,384 字节，SHA-256 为 `6409de8fc9239440269d1f1df9e6c571f57490268b1109df19ea08a975c7a7a2`。同一提交通过 `scripts/verify.sh`，包括 Go 与 MyGo vet、Go 竞态测试和 Windows amd64 构建。Linux amd64 构建也通过。构建和校验记录保存在采样 Mac 的 `~/Library/Application Support/Mark Go Go Benchmarks/ab-5k.c7XbVH/release-9a22b9c/`。
