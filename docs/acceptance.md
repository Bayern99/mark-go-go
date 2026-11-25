# 验收

## 可复验入口

CI 以 [工作流](../.github/workflows/ci.yml) 为准。原生窗口、菜单和重启在 macOS 上按 [Automation 指南](agents/native-automation.md) 验证。提交前选择覆盖实际改动的检查。

macOS 正式构建的窗口、编辑、滚动、切换与内存对照见 [Portrait 性能基线](agents/portrait-performance.md)。

## 0.6.0

2026-11-24 至 25 日在 macOS 27.2（26B123）arm64、Go 1.27.3、MyGo 0.10.0 上验收，应用与 `markgogo` 均为 0.6.0。测试包使用独立的应用标识、临时用户数据目录和合成文稿，不读取正式配置。发现的问题由 [PR #203](https://github.com/Bayern99/mark-go-go/pull/203) 修复后复验，汇总见 [#191](https://github.com/Bayern99/mark-go-go/issues/191)。

### GenUI 后端

四种后端都在真实账户下完成了模型刷新、思考等级选择、一次完整生成与一次局部更新，最终视图都通过校验并保存为 v3 文件。

| 后端 | 版本 | 说明 |
|---|---|---|
| Codex | codex-acp 1.13.1，Codex CLI 0.172.0 | 读到 7 个模型和六档思考等级。生成期间只在本轮临时目录写入 `out.json`，工作区原文哈希不变 |
| Antigravity | agy 1.5.9 | 读到 11 个模型。生成中卡片逐张出现；局部更新只改一张卡片，退回操作逐字节还原更新前的视图 |
| Pi | Pi 1.6.0 | 读到 42 个模型，每轮生成前完成守卫握手 |
| Custom ACP | 以 codex-acp 1.13.1 配置 | 与 Codex 使用同一适配器，经自定义命令入口验收 |

0.5 的 v1 设置迁移到 `schemaVersion: 2` 后保留了原有的非默认模型和模型列表。

### Pi 守卫

在 `ba0601b` 上用真实 Pi 1.6.0 验证守卫扩展。

- **正常生成**：把诱导越界的文字分别放进文稿正文和生成请求，经产品生成路径运行 7 次，7 次都保存了视图。模型只写 `out.json`，视图中没有诱饵内容。
- **直接驱动**：使用相同参数和守卫环境，明确要求模型越界调用工具。9 种越界读写全部被拦截：用绝对路径、`~/`、`@`、`file://`、`..` 读取目录外文件，写目录外文件，写 HOME，写本轮目录内 `out.json` 以外的文件，以及编辑 `source.md`。RPC 记录中没有诱饵内容，诱饵写入路径没有生成，`source.md` 未变。

### 外部 Agent 与 MCP

- **本机通道**：默认显示 Not listening。开启后目录为 `0700`、socket 为 `0600`，页面显示最近一次调用的来源和时间。关闭后 socket 被删除。
- **CLI**：`status`、`read --sections` 和 `propose` 返回正确的工作区与章节，提案出现在原生审阅队列中，接受前原文不变。
- **MCP Inspector 2.13.0**：在 `2026-07-28`（`server/discover`）和 `2025-11-25`（`initialize`）两个协议版本下都列出 7 个工具，读取文档和提交提案成功。
- **Pi 1.6.0（shell 型 Agent）**：加载 `markgogo agent-prompt` 生成的说明后，用 shell 调用 `markgogo` 读到正确的工作区、标题、行范围和章节 ID。
- **Codex CLI 0.172.0（沙箱型 Agent）**：在 `read-only` 和 `workspace-write` 两种沙箱下，由 Codex 启动的 stdio MCP 都取得正确的工作区；从沙箱内直接运行 `markgogo status` 无法连接 socket，返回 `permission_denied`。因此对 Codex 推荐 stdio 方式。
- **安装包中的 CLI**：arm64、x86_64 与 Universal 安装包的 `Contents/Resources/markgogo` 架构正确，严格验签通过。在独立副本上实测了 Install、升级后重新安装和 Uninstall。

### 界面

- 设置中心的 Plugins、GenUI、AI Integration 三页在 1280 × 820 窗口中的浅色与深色外观下均检查通过。
- 关闭 GenUI 插件后，活动文档回到 Preview，GenUI 入口和 macOS View 菜单中的 GenUI 项被禁用，两个窗口状态一致；重新开启后恢复。
- 从 GenUI 来源按钮跳到原文，再返回 GenUI，插件关闭与重开后均正确。
- 命令栏与 Feynman Actions 的键盘行为（[#175](https://github.com/Bayern99/mark-go-go/issues/175)）和 GenUI 来源摘录预览（[#177](https://github.com/Bayern99/mark-go-go/issues/177)）由无窗口的 `ui.Tester` 用例覆盖，未做真机检查。

### 未覆盖

- Codex 生成期间每一次写入与宿主权限回调的逐一对应，没有做全机文件事件跟踪。
- Windows 运行时，包括 Feynman 配置的符号链接，只有交叉编译和单元测试。

以下为 0.5.0 及更早版本的记录。

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

本轮原生界面验收在 macOS arm64 进行。2026-11-20 在 `cb6deb2` 的 macOS 27.2 arm64 隔离应用上补验四项：Feynman 连接失败后 Reconnect 与会话、根切换状态一致；重启后活动和非活动标签的滚动位置精确恢复；系统拼音组字中 `Esc` 只取消预编辑、`Cmd+Enter` 不提交；光标位于 EOF 时折叠最后一个 Section，展开后光标回到可见标题且后续输入不改动隐藏原文。预览表格单元格内拼音上屏后一次 Undo 会回到未上屏文本，因 MyGo 编辑器未导出组字状态，记为非阻塞后续项（#64）。跨重启滚动位置的会话序列化、恢复和非活动标签保留由 [会话测试](../internal/workspace/session_test.go) 覆盖。Linux 和 Windows 的覆盖来自自动化和构建。ComparisonTable 的键盘排序由 [原生 GenUI 阅读器测试](../internal/nativegenui/reading_test.go) 覆盖。迁移前的 [Chromium 用例](https://github.com/Bayern99/mark-go-go/blob/81522cb48ec9718bffbda0255df1039692a2f6cd/web/test/genui-catalog.test.tsx) 与 [工作区集成 #32](https://github.com/Bayern99/mark-go-go/issues/32) 保留作历史参照。
