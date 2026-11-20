# macOS 原生验证

从项目根目录运行与 MyGo 同版本的官方 CLI 和原生 smoke。

```sh
go tool mygo-automation skills
scripts/native-automation-smoke.sh
```

脚本从 Go 源码和应用资源打包 QA 应用，创建临时文稿、独立应用标识和私有 Unix socket。它从 Finder 的系统打开入口送入第二篇文稿，通过 Automation 确认窗口为 Content、可访问树包含 Markdown 源码编辑器，再用键盘输入和文件菜单保存。脚本比较含混合换行与 Unicode 的完整文件字节，检查编辑菜单的撤销、重做，并在退出后启动新进程核对活动文稿。设置 `MYGO_AUTOMATION_REPORT` 可选择截图路径。[原生 smoke](../../scripts/native-automation-smoke.sh)

手动检查时，在具有独立应用标识的测试副本中，用 `GOFLAGS=-tags=e2e` 和 `MYGO_AUTOMATION=<私有 socket 路径>` 启动 `go tool mygo dev`。通过 `go tool mygo-automation` 的 `windows`、`snapshot --full`、`key`、`type` 和 `menu` 观察和驱动真实控件。系统文件对话框、输入法和 VoiceOver 使用对应的 macOS 服务实测，记录构建标识和操作结果。[测试构建入口](../../automation_e2e.go) · [MyGo 发布守卫](../../third_party/mygo/plugins/automation/off.go)

2026 年 11 月 20 日在 macOS 27.2（26B123）arm64、Go 1.27.3 和 MyGo 0.10.0 上，首个原生窗口通过上述 smoke。Automation 读到 `[content] "Discussion.md"` 和 `[textField] "Markdown source"`。在含 CRLF 与 LF 的夹具开头输入 `前🙂` 后，菜单保存写入预期字节，撤销和重做改变控件文本。退出后，新进程重新显示相同文稿，完整文件比较通过。同一源码的默认构建产出 macOS app 和 DMG，Linux amd64 与 Windows amd64 完成交叉编译。

## Portrait 工作区迁移验收

2026 年 11 月 20 日在 macOS arm64 实机使用独立 app、数据目录和合成文档。原生窗口通过 Finder 打开第二份原文，在 Preview 和 Markdown 间切换，并通过文件菜单保存。混合换行、中文和 emoji 的磁盘字节比较通过。Undo、Redo 和重启后恢复活动标签及 Markdown 模式均通过。运行 `scripts/native-automation-smoke.sh` 可复跑这一组操作。

使用与 [#60](https://github.com/Bayern99/mark-go-go/issues/60) 相同的 256 节长文，实机截图与可访问树覆盖 1280 × 820、1180 × 820、1179 × 820、641 × 500、640 × 500 和 480 × 360。实测 640 以下导航浮层、641 起左栏停靠、1179 与 1280 的研究栏切换，以及浮层 Esc 返回入口焦点。通过截图发现选中胶囊曾覆盖文字，修正后复拍六档。最小窗的 Preview 标题和首段距顶仍大于旧版，对照截图已交编辑器验收。 [原生控件与像素回归](../../internal/nativeui/shell_test.go)

另一组实机验收从此 QA app 自己的 WebKit LocalStorage 载入插件开关、研究栏宽度、视图模式、折叠与列宽数据。验证了 Markdown 恢复、研究开关与旧键值迁入原生配置。折叠与列宽的视觉恢复由对应编辑功能验证。大纲跳转、文件和标题搜索、打开结果及 Ctrl+Tab 返回原文通过。对合成文件制造外部冲突和删除后，告警及未保存草稿均保留。

[#68](https://github.com/Bayern99/mark-go-go/issues/68) 须在最终集成构建复跑 Save As、Duplicate 后续存盘、Rename、Move To、未命名关闭、取消与多窗口退出失败重试，另以真实 Finder 拖入及单根目录关闭核对范围。旧版的实机操作与结果见 [PR #41](https://github.com/Bayern99/mark-go-go/pull/41)、[PR #43](https://github.com/Bayern99/mark-go-go/pull/43) 和[既有验收记录](../acceptance.md)。当前的退出失败与重试服务回归见 [session_test.go](../../internal/workspace/session_test.go)。

## GenUI 阅读和生成

2026 年 11 月 20 日，在独立应用标识的 macOS 调试包中打开带两处同名标题、CRLF 和 emoji 的文稿。通过原生控件打开已有旁车，核对缺失与已改来源，点击清单进入第二个标题的原文。Preview 的可访问选区为 36–50，对应引用的完整文字。返回 GenUI、切换内页并对金额排序后，原地添加含评论结束符和 emoji 的批注。保存后比对整份原文，只有转义后的批注和分隔换行增加。

生成验收使用实际工作区服务、文件观察器和独立 ACP 测试进程。完整生成保存了所选模型、生成要求与对应章节的哈希。局部更新保留另一页的表格，恢复操作逐字节还原前一份旁车。无效输出、已接收 prompt 后取消、重试及外部创建、损坏、删除和重建旁车均通过。两个文档同时生成时，挂起文档的要求、取消操作和结果均归属该文档。切换模型后再次生成，新旁车记录了新模型。本次使用测试进程提供可复现的 ACP 回复。

## Agent 提案和检查点

2026 年 11 月 20 日，在隔离的 macOS arm64 调试应用中，用真实 loopback MCP 会话读出 Markdown 编辑器里尚未保存的字节、章节和批注。通过 MCP 创建研究产物并提交两份文稿的提案。在原生比较面板打开第一份后继续输入，原比较随即失效。重新比较，接受第一块，第二块仍待审阅。切到 Preview 用 Edit → Undo，整份实时缓冲与应用提案前的字节一致。拒绝余下的块后，第二份文稿的提案仍在队列。

在有既存暂存内容的 Git 工作区，创建带标签的检查点、比较和恢复，再切换视图撤销恢复。前后比对 HEAD 与完整暂存差异，结果相同。在普通目录以同一界面创建 Local snapshot，比较、恢复、保存并比对完整文件，随后跨视图一步撤销恢复。提案与检查点实测复用正常文档和编辑服务，测试启动时注入的是隔离 MCP 会话注册。
