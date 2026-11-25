# 全局命令栏与 Feynman Actions 的键盘契约

本文记录 issue #175 在基线 `c01e413` 上对照 `internal/nativeui/command.go` 与 `internal/nativefeynman/chat.go` 得到的差异，以及修正后的共同契约。

## 原实现差异

| 行为 | 全局命令栏 | Feynman `/` Actions | 判定 |
| --- | --- | --- | --- |
| 移动 | 首项初始选中；↑↓ 循环移动并滚动到可见位置；查询改变后回到首项。 | 没有列表选中状态，也不处理方向键。 | **缺陷**：补齐同一套循环选中、滚动和查询重置行为。 |
| 执行 | Enter 执行当前选中项；无结果时不执行。 | Actions 弹层不能按 Enter 选择；斜杠草稿按 Enter 时另取首个名称前缀匹配，可能与可见过滤结果不一致。 | **缺陷**：Enter 只选择当前项，无结果时不发送草稿。执行结果保持不同：命令栏运行命令，Actions 插入草稿。 |
| 取消 | Esc 关闭；⌘K / ⌘T 也可切换。 | Esc 关闭；斜杠入口记住本次取消，继续编辑同一草稿时不自动重开，Actions 按钮可明确重开。 | 基本取消契约相同；斜杠取消后的抑制与全局快捷键切换是**有意不同**，不共用。 |
| 焦点归还 | 关闭后归还编辑器、侧栏过滤器或命令入口。 | 关闭后由弹层归还 Research prompt 或 Actions 按钮。 | 原有契约相同，**不是缺陷**；各入口保留自己的焦点来源，不进入共用选择状态机。 |
| 空态 | 保持打开并显示 `No matching documents or commands`。 | 手动过滤显示领域空态；斜杠列表在后续查询无匹配时直接关闭。 | **缺陷**：已打开的斜杠列表也保持打开并显示空态。两处文案继续按各自内容命名。 |
| 过滤 | 忽略大小写与首尾空白；匹配原文、标题、工作区文件和命令。 | 列表忽略大小写并匹配名称、说明与来源；斜杠触发只认命令名称前缀，查询变化没有可见选中可重置。 | 过滤算法与重置选中是**共同契约**；搜索内容域及斜杠触发规则是**有意不同**。 |

## 修正后的共同契约

- 结果是 `ListBox`，每行是带选中状态的 `Option`；首项默认选中。
- ↑↓ 在结果中循环移动，必要时滚动到可见位置；过滤词变化后回到首项。
- Enter 执行当前选中项。无结果时 Enter 不关闭列表、不执行命令，也不发送研究草稿。
- Esc 取消并关闭，焦点返回触发前的位置。
- 过滤忽略大小写与首尾空白；已打开的列表在无匹配时显示各自领域空态。
- `internal/nativekit.CommandSelection` 只承载两处共用的选中索引、循环移动、越界收敛、重置与输入框提交。overlay 开关、取消记忆、焦点归还、过滤内容域和执行动作仍属于各原生包。

保留命令栏的工作区搜索与侧栏过滤词恢复，以及 Actions 的草稿插入和路径不误认。一次性把未知 `/unknown` 放入草稿不会自动打开 Actions；逐字输入时，第一个 `/` 会先打开 Actions，继续输入到无匹配时列表保留空态，此时 Esc 关闭列表并把焦点还给 Research prompt，随后 Enter 发送未改动的原文（例如逐字输入 `/tmp` → Esc → Enter 发送 `/tmp`）。`/tmp/input.md` 等包含后续 `/` 的草稿不触发 Actions。模型选择器、会话管理与 MCP 设置不在此契约内。

## ui.Tester 对照用例

| 用例 | 全局命令栏 | Feynman Actions |
| --- | --- | --- |
| 移动、循环与 Enter | `TestCommandBarRunsTheResultChosenWithTheArrows` | `TestResearchActionsChooseWithTheSameArrowsAndEnterAsCommandBar` |
| 取消与焦点归还 | `TestCommandBarRestoresEditorFocusOnEscapeDismissAndRun`、`TestCommandBarRestoresSidebarFilterFocus` | `TestResearchActionsEscapeReturnsFocusToItsOrigin` |
| 空态与空结果 Enter | `TestCommandBarEmptyResultStaysOpenAndEnterDoesNothing` | `TestResearchActionsEmptyResultStaysOpenAndEnterDoesNotSend` |
| 过滤大小写、首尾空白与结果 | `TestCommandBarFilterIsCaseInsensitiveAndIgnoresOuterSpace` | `TestResearchActionsFilterIsCaseInsensitiveAndIgnoresOuterSpace` |
| 按钮焦点与补全光标 | 不适用 | `TestResearchActionsButtonSupportsImmediateKeyboardChoiceAndEscape`、`TestResearchActionsCompletionPlacesCaretAfterCommand` |
| 斜杠取消记忆、路径与空态回归 | 不适用 | `TestSlashPopoverStaysAboveComposerAndRemembersEscape`、`TestSlashCommandsRejectPathsAndMissingCommands`、`TestSlashPopoverShowsEmptyStateWhenDraftStopsMatching`、`TestSlashUnknownTypedDraftSendsVerbatimAfterEscape` |
