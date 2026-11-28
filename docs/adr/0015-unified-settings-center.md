# ADR 0015 · Unified Settings Center and deep-surface controls

Status: experimental (branch on top of ADR 0014, pending evaluation)

> 随着 [ADR 0021](0021-wuxing-16-spaces-and-threshold-plates.md) 将五行 Space 扩充至十六境，侧栏底栏的高频切换例外由「八颗色点横排」修订为「单颗当前境色块按钮 + `1 + 3×5` 语义网格 Popover 弹层」，完整的摄影画廊与重置仍唯一归属 `Settings / Appearance`。

## Context

ADR 0013/0014 的 Settings 只有外观与少量 Feynman 配置，Space、Appearance、插件与 GenUI 仍同时存在于工作区菜单、macOS View 菜单、侧栏底栏和研究页。各深层表面还分别搭建输入框、弹层、折叠区与按钮，导致同一个偏好有多个编辑器，浅色/深色与键盘行为不一致。

## Decision

采用 **Unified Settings Center · 设置中心**：

- `⌘,` 打开 1040×720 的圆角 sheet，窗口边缘保留 32 点；左侧 224 点分类栏带 `/` 与 `⌘F` 搜索，右侧是 `Settings / Category` 面包屑、按需出现的 Reset 与 Close。内容列最大 720 点，设置按圆角卡片分组，行间使用发丝线。sheet 小于 720 点时分类栏折叠成 Select。
- 设置页通过 `nativeui.SettingsPage` 注册。`nativeui` 提供 General、Appearance、Editor、Shortcuts、Plugins、Diagnostics；`nativefeynman` 提供 Models & Providers、Search & Papers、MCP Servers、Packages 并扩展 Diagnostics；`nativegenui` 提供 GenUI。`OpenSettings` 保留单一深链入口，空字符串回到上次分类，`tools` 兼容映射到 `mcp`。
- 搜索复用真实设置行，不维护另一套搜索结果 UI。第一次 Esc 清空查询，第二次关闭；Down 进入结果，Enter 打开第一个匹配分类。
- 一个偏好只有一个编辑器，即设置中心。窗口 chrome 只保留高频快捷切换（昼夜按钮，以及由 [ADR 0021](0021-wuxing-16-spaces-and-threshold-plates.md) 拟定的侧栏当前境色块弹层，替代原八境色点横排）或深链。工作区菜单不再包含 Space/Appearance 子菜单，侧栏不再包含 Plugins 菜单，macOS View 以 `Appearance…` 深链替代两个子菜单。
- General 持久化侧栏、默认 Files/Outline、研究面板与默认文档视图；Editor 增加阅读字号、行宽、源码字号与行号偏好，并提供按文档重置折叠和表格列宽。空偏好值始终表示默认值。
- Portrait 增加 Form、Settings、Editor 与 IconSizes 变量，以及 TextField、SearchField、TextArea、Select、Stepper、Menu、DialogShell、SettingsGroup/Row、Disclosure、OutputBlock、Chip 和 ListRow。现有 RadiusScale、SizeScale 和既有 Type 字号保持封闭；编辑器字号在独立 EditorTokens 中变化。
- Feynman、Review、Checkpoints、Editor hover/find 与 GenUI 全局设置迁到这些原语。GenUI 文档内容阅读器保留独立 Reader tokens；编辑器上下文菜单保留原生实现，以承载系统文本服务与输入法项目。

## Consequences

- 所有设置类别都可从命令栏按 `Settings: <Category>` 到达；帮助菜单提供 Keyboard Shortcuts。
- 原生控件 allowlist 只减不增；除 `nativegenui/controls.go` 与 `reading.go` 的文档内容控件外，原生包不再直接调用 `ui.Button`、`ui.TextInput`、`ui.TextArea`、`ui.Select`、`ui.Popover` 或 `ui.Collapsible`。
- 新 Phosphor 图标用随包字体的 cmap 做自动化存在性检查。新增文本角色、控件和 AccentSoft 选中态在 16 套 Space/外观组合中执行 WCAG AA 实算（待 [ADR 0021](0021-wuxing-16-spaces-and-threshold-plates.md) 落地后扩至 32 套）。
- 2× 场景集覆盖设置分类、搜索、窄窗、工作区菜单、研究历史/动作/问题/文件、alphaXiv、检查点、提案、链接 hover、查找栏与 GenUI request，作为视觉回归基线。

## Alternatives considered

- 保留各 chrome 菜单作为同步副本：入口看似方便，但持久化、选中态、外观与测试都要维护多份。
- 为搜索做独立索引和静态结果页：实现更快，但结果不能就地操作，容易与真实页面漂移。
- 扩大既有 RadiusScale/SizeScale 或改变界面密度：会破坏封闭阶梯；新尺寸只进入 Form、Settings、Editor 与 IconSizes。
