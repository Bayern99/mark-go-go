# 验收

## 可复验入口

CI 以 [工作流](../.github/workflows/ci.yml) 为准。原生窗口、菜单和重启在 macOS 上按 [Automation 指南](agents/native-automation.md) 验证。提交前选择覆盖实际改动的检查。

macOS 正式构建的窗口、编辑、滚动、切换与内存对照见 [Portrait 性能基线](agents/portrait-performance.md)。

## Feynman 与 Git 插件

2026-11-19 在隔离的 macOS arm64 应用中，使用 Feynman 0.5.71、pi-acp 0.0.33 和本地固定响应模型验证了工作区对话。空文件夹可以发起研究；同一窗口内两个工作区各自保留对话和产物。生成文件可打开为文档标签，重启后可恢复文档和对话。

480 px 宽窗口中，收到修改建议后继续编辑正文会使旧比较失效。重新比较、应用和原生 Undo 均通过。删除一份已完成报告后，从右栏显式恢复取得相同字节。改名期间的修改建议由迁移前 [固定版本的浏览器回归](https://github.com/Bayern99/mark-go-go/blob/81522cb48ec9718bffbda0255df1039692a2f6cd/web/test/feynman-edit.test.tsx) 覆盖。单文档 Git 检查点在已有暂存内容的仓库中完成创建、比较与恢复。实测 HEAD、当前分支和暂存条目前后一致。现行回归入口：[Research](../internal/agent/feynman/research_test.go) · [产物](../internal/review/artifact_test.go) · [Git](../internal/review/checkpoints_test.go) · [原生编辑器](../internal/nativeeditor/)。

### 研究交互与原生 CLI

Feynman 0.5.71 与 pi-acp 0.0.33 在隔离的本地模型上公布了 45 个命令。供应端实际收到了切换后的模型、推理级别、图片字节与所选文件引用。1280 × 820 和 480 × 360 的 macOS 原生构建验证了命令检索、Enter 发送、Markdown 表格和引用、研究文件、确认改文与撤销、会话切换和终端重连。原生 CLI 的 `status` 和交互模式中的 `/help` 使用真实可执行文件。

该记录早于 [Direct Feynman RPC](adr/0008-direct-feynman-rpc-and-gui-shell.md)。[旧 ACP 测试](https://github.com/Bayern99/mark-go-go/blob/d7ab3257a14960480629b320549ba0c4d3b1b1ec/research_acp_test.go)与[旧终端测试](https://github.com/Bayern99/mark-go-go/blob/d7ab3257a14960480629b320549ba0c4d3b1b1ec/research_console_test.go)保留在历史提交。现行回归入口：[附件](../internal/agent/feynman/research_sources_test.go) · [会话](../internal/agent/feynman/research_sessions_test.go) · [CLI](../internal/agent/feynman/cli_test.go)。

### Feynman 原生研究栏迁移（#66）

2026-11-20 在 macOS 27.2（26B123）arm64、Go 1.27.3、MyGo 0.10.0、1280 × 820 窗口中运行 [原生 Feynman QA](../scripts/native-feynman-qa.sh)。脚本在 `mktemp` 目录创建 HOME、工作区和原文，并从 `internal/agent/feynman` 的 `TestMain` 构建 RPC 与 CLI 夹具；不读取用户凭据或改动正式配置。工作区通过系统打开文件夹对话框载入，插件在原生应用中启用。Chat、Files、Settings 与 alphaXiv 标签可见，设置页显示 Doctor Healthy 及模型、账号和搜索控件。

连接和研究使用仓库内可复现的 Feynman RPC 供应端。发送 `stream full lifecycle` 后，原生 Markdown 对话显示流式回复，并可展开 `read · notes.md · completed` 工具查看输出和改动前后文本。第二次发送后，服务发布 `late.md`；Files 中打开为原生文档标签。删除该文件后刷新显示 Restore，恢复后文件内容为 `# Late artifact\n`，SHA-256 为 `46aa7a389515e7912bb52bc9ebc2b1acf519dc92e0bafc604c55b100b4a324ff`。扩展问题的 input 和 editor 回答提交成功，供应端收到 `Native Fixture` 与 `Final native fixture abstract`；状态、进度小组件和研究标题也更新到界面。

首轮通过系统文件对话框和界面导航后，夹具的 select 与 confirm 提问分别在 10 秒及 5 秒超时。原生控件的选择和确认入口由 [无窗口界面测试](../internal/nativefeynman/chat_test.go)覆盖，回答协议由 [Feynman 服务测试](../internal/agent/feynman/research_test.go)覆盖。

随后在实机补验 480 × 360 窗口、附件选择、会话切换与跨根隔离，以及提案审阅、重启恢复、系统拼音和 VoiceOver。alphaXiv 面板在实机检查了入口，查询行为以 Go 测试为依据。 [窄窗研究](agents/native-automation.md#feynman-窄窗) · [原生提案和检查点](agents/native-automation.md#agent-提案和检查点) · [系统输入和会话](agents/native-automation.md#系统输入文件菜单和会话)

## 迁移前历史记录

- MyGo v0.10.0 升级合并在 `9559625`。当时的 Go、Web 构建、两组 Web 测试和 Windows 交叉编译通过。 [main CI](https://github.com/Bayern99/mark-go-go/actions/runs/44240578323)
- `5b68b5c` 的隔离应用在 macOS 完整跑了两次原生 smoke，均退出 `0`。四个文档同窗、切换、磁盘保存、原生撤销重做和重启恢复通过。空正文退出 `1`，应用退出 `23` 时脚本返回 `23`。QA 数据与 Vite 临时缓存随运行目录清理。 [升级 PR](https://github.com/Bayern99/mark-go-go/pull/41)
- 升级后的 macOS 包另行验证了 Save As、Rename、Move To、未命名原文关闭确认、外部冲突取消，以及真实 `Ctrl+Tab`。 [升级 PR](https://github.com/Bayern99/mark-go-go/pull/41)

## 窄窗键盘操作

[PR #43](https://github.com/Bayern99/mark-go-go/pull/43) 的 macOS arm64 隔离 QA 构建在 480 px 窗口宽度下确认文件栏可展开和收起。正文内按 `⌥⌘0` 后焦点进入右栏，输入字母时正文保持原样，`Esc` 关闭浮层并将焦点还给开关。扩宽窗口后文件栏恢复停靠。保存、撤销、重做和重启恢复均通过。

## 原生覆盖缺口

本轮原生界面验收在 macOS arm64 进行。精确光标与滚动位置的 macOS 实机复验仍有缺口；跨重启滚动位置的会话序列化、恢复和非活动标签保留由 [会话测试](../internal/workspace/session_test.go) 覆盖。Linux 和 Windows 的覆盖来自自动化和构建。ComparisonTable 的键盘排序由 [原生 GenUI 阅读器测试](../internal/nativegenui/reading_test.go) 覆盖。迁移前的 [Chromium 用例](https://github.com/Bayern99/mark-go-go/blob/81522cb48ec9718bffbda0255df1039692a2f6cd/web/test/genui-catalog.test.tsx) 与 [工作区集成 #32](https://github.com/Bayern99/mark-go-go/issues/32) 保留作历史参照。
