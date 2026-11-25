# Portrait 2.0 界面设计

Mark Go Go 的原生桌面界面由 MyGo Native UI 绘制，[internal/portrait](internal/portrait/) 是全部设计变量与共用控件的唯一来源。窗口采用受 Arc 启发的彩色布景与悬浮纸页结构，结合中国传统色五行八境与统一设置中心：组件语义与契约见 [ADR 0012](docs/adr/0012-portrait-2-coss-reference-and-contracts.md)；布景、纸页与命令栏见 [ADR 0013](docs/adr/0013-arc-experiment-scenery-page-command-bar.md)；五行 Space、每个 Space 自有的强调色与深色外观见 [ADR 0014](docs/adr/0014-wuxing-spaces-per-space-accent-and-dark-mode.md)；统一设置中心见 [ADR 0015](docs/adr/0015-unified-settings-center.md)；实现入口见 [原生窗口外壳](internal/nativeui/shell.go) 和 [原生编辑器](internal/nativeeditor/)。

## 设计立场

- **窗口是一个房间，纸页是其中唯一的实体。** 窗口本身是带颜色的布景，侧栏直接坐在布景上，不加卡片和边线；原文是浮在布景上的一张不透明白纸。界面里其余的东西都比这张纸轻。
- **导航在侧面，不在顶上。** 打开的原文是侧栏里的竖排标签，文件名不会被挤短。原来的标签行、文档工具行和状态行三行界面合成纸页顶部的一行。
- **其余一切交给命令栏。** `⌘K` 或 `⌘T` 打开命令栏，搜索打开的原文、工作区文件、标题和全部命令，不必记住东西放在哪里。
- **工作区即 Space。** 每个文件夹工作区是八个五行 Space 之一，布景、纸页、强调色和元素色都随它变，一眼认出身在哪个工作区。浅色与深色外观同样完整。
- **不照搬的部分。** 强调色属于 Space 而不随系统强调色变，16 套配色逐套验算对比度。Arc 的文字灰度达不到正文对比度，墨色是五行表中的墨色与霜白。字体不用衬线展示体，产品内只有无衬线。

## 窗口构成

- **布景 Scenery**：窗口底色是取自所在 Space 当前外观的 160° Oklab 三段渐变。八种 Space 按工作区根路径保存；侧栏底栏色点是唯一高频快速切换，完整选择在 `Settings / Appearance`，命令栏保留 `Space: …` 命令。外观 System / Light / Dark 同样由侧栏昼夜按钮、Settings 或命令栏切换。工作区菜单和 macOS View 不再维护第二套编辑器，View 的 `Appearance…` 直接深链。主窗口布景保持不透明传统色，玻璃只用于浮层。 [布景](internal/portrait/space.go) · [设置中心](internal/nativeui/settings.go)
- **侧栏**：自上而下是窗口控件行、工作区切换器（应用内主题菜单，仅含工作区、Open Folder、Close Workspace 与 `Settings…`）、命令入口（`⌘K` 键帽）、打开的原文、`Files / Outline` 与按需 SearchField、零缩进目录钻取与祖先栈。底部保留八境五行色点、昼夜切换、多工作区色点、Settings 与新建原文；Plugins 菜单已迁入 `Settings / Plugins`。641 点及以上停靠，较窄时使用抽屉。 [侧栏](internal/nativeui/sidebar.go) · [设置页注册](internal/nativeui/settings_pages.go)
- **纸页 Page**：所在 Space 的纸色（浅色为白，深色为各 Space 的深色纸）、圆角 12、带一圈发丝线和柔和投影，距窗口边缘 8 点。顶部一行依次是侧栏开关（侧栏未停靠时）、可点击定位的祖先路径与当前文件名、编辑状态、Preview / Markdown / GenUI 切换、检查点和研究面板开关；其下只有内容，不再有底部状态行。侧栏文件行、文档标签与纸页路径不弹出遮挡内容的完整路径悬浮提示。 [纸页](internal/nativeui/document.go)
- **研究面板**：启用 Feynman 后，1180 点及以上宽度时是并排的第二张纸，像 Arc 的分屏；两张纸之间的空隙可拖动调宽。较窄时是从右侧滑出的玻璃抽屉。抽屉关闭或按 Esc 后，焦点返回其入口。 [窗口外壳](internal/nativeui/shell.go) · [键盘与窄窗用例](internal/nativeui/shell_test.go)
- **窗口控件**：macOS 隐藏系统标题栏，红绿灯落在侧栏顶部；侧栏开关、路径和视图切换与红绿灯同处一条水平线，这一行可以拖动窗口。侧栏收起时纸页顶行让出红绿灯的位置。Linux 与 Windows 保留系统标题栏。 [窗口适配](internal/nativeui/window.go) · [窗口控件用例](internal/nativeui/sidebar_test.go)

## 命令栏

命令栏是居中偏上的玻璃浮层：一行输入，下面是结果。空白时先列其他打开的原文（按最近使用排序，当前原文排在最后），再列命令。输入后同时匹配打开的原文、当前原文的标题、工作区里的文件与标题，以及命令：新建原文、打开文件夹、切换视图、检查点、显示或收起侧栏、显示 Outline 或 Files、研究面板、启停插件、关闭原文、切换工作区、打开最近的工作区、更换 Space 和外观（`Toggle Dark Mode` 总在列表里，`Appearance: …` 与 `Space: …` 在输入后出现）。↑↓ 移动选中项，回车执行，Esc 关闭；选中项用当前 Space 的柔和强调色整行标记，文字保持墨色与辅助色的层级。命令栏借用工作区搜索，关闭时把侧栏自己的过滤词还回去。 [命令栏](internal/nativeui/command.go) · [命令栏用例](internal/nativeui/command_test.go)

## 设置中心

`⌘,` 打开统一设置中心。默认 sheet 为 1040×720、圆角 16，左侧 224 点分类栏带 `/` 搜索，右侧为 `Settings / Category` 面包屑、按需 Reset 与 Close；内容列最大 720 点，组标题下是一张带发丝行分隔的圆角卡片。宽度不足 720 点时分类栏变成 Select。第一次 Esc 清空搜索，第二次关闭，搜索结果仍是可立即操作的真实设置行。

设置页由 `SettingsPage` 注册而不是硬编码：General、Appearance、Editor、Shortcuts、Plugins、Diagnostics 属于外壳；Feynman 与 GenUI 各自注册研究和工作区页面。`OpenSettings` 是唯一深链入口，空值恢复上次分类，`tools` 兼容到 `mcp`。每个偏好只有 Settings Center 一个编辑器；chrome 仅保留高频一键切换或深链。General 持久化导航与研究可见性，Editor 持久化阅读和源码偏好，Plugins 保存各插件的 `mgo.plugin.*` 开关并深链到插件自己的设置页，Diagnostics 可定位偏好文件或经确认重置全部设置。GenUI 缺省开启，关闭时文档回到 `Preview` 且不再监听或生成，已保存的视图留在磁盘上。 [设置中心](internal/nativeui/settings.go) · [外壳设置页](internal/nativeui/settings_native.go) · [ADR 0015](docs/adr/0015-unified-settings-center.md)

## Portrait 模块

原生各包只从 Portrait 取字体、字号、颜色、材质和动效。 [守护测试](internal/portrait/contract_test.go) 在原生包中发现字号数字、颜色字面量或字体名时失败。每个设计变量登记 `webui@c6504af`、`coss@b2336c4`、`arc@adr-0013`、`wuxing@adr-0014` 或 `portrait-derived` 之一。颜色、材质、色调和主题由当前 `(Space, 深浅)` 推导：窗口外壳每帧先调用 `portrait.Use`，原生各包照旧读取 `portrait.Colors`、`portrait.Materials`、`portrait.Tones` 和 `portrait.Theme()`。 [外观](internal/portrait/appearance.go)

- **字体**：界面与正文 Inter，一级和二级标题 Inter Display，代码 IBM Plex Mono，图标 Phosphor；汉字按字形回退到 IBM Plex Sans SC。字体和许可证随应用打包。 [字号与颜色](internal/portrait/portrait.go) · [字体资源](internal/portrait/fonts/)
- **字号阶梯**保持封闭：display 28/36、title 20/28、reading 16/28、small 14/22、ui 13/18、control 13/20、meta 12/16、code 13/22，新增 H3 18/28 与 mono CodeSmall 12/18；没有小于 12 的界面文字。Preview 标题为 H1 28、H2 20、H3 18、H4 16、H5 14、H6 13。编辑器阅读字号 15–18、源码字号 12–14 与行宽 560/624/720 由独立 `EditorTokens` 派生，不改变界面 Type 阶梯。 [字号与颜色](internal/portrait/portrait.go) · [编辑器变量](internal/portrait/editor.go)
- **颜色**：每个 Space 在浅色与深色下各有布景两端、纸页、代码面、强调色、标记色和 Success / Warn / Danger，全部取自五行 742 色表的具名颜色（纸页与代码面为派生中性面）。浅色墨色 `#1D1B1C` / `#48484D` / `#6E6E73`，强调色上的字为白；深色墨色 `#F2F3F4` / `#CDD1D3` / `#959DA2`，强调色上的字为玄黑 `#0E100F`。焦点环是强调色的 28%（深色 40%）；`Info` 与 `Note` 是强调色的别名。深色下发丝线与填充是白色薄层，凸起与弹层由 `lift(...)` 略亮于纸面，凹面比纸暗；多工作区色点选中态使用 `GlassSelected`（85% 纸色），文档标签选中胶囊使用 `Materials.Raised`。差异视图的新增行、复制成功的对勾和上升趋势用 `Success`。 [Space](internal/portrait/space.go) · [色调](internal/portrait/tone.go) · [五行契约](internal/portrait/wuxing_test.go)
- **材质**是平的：没有渐变面和多层内阴影。浅色下凸起是白纸加发丝线和一层浅投影，深色下以纸面提亮并配深色投影，悬停与按下只改变投影和底色；强调按钮是纯色加同色光晕；行悬停、凹面和分段控件的轨道是 5% 左右的薄层；`Sheet`、`Pop`、`Page` 是纸面加由浅到深的三档投影。 [材质](internal/portrait/material.go) · [控件](internal/portrait/controls.go)
- **圆角**只有 4、6、8、12、16 与胶囊形，外加嵌套的轨道 10 与分段 8。行与按钮 8，纸页、卡片与弹层 12，抽屉与命令栏 16。控件高度仍是 22、28、32；侧栏的行高是阶梯的：打开的标签行 32，文件树行 28。字号同样分层，工作区名 14、打开的标签 13、文件树与大纲行 12，只有选中的标签或文件行加粗到 500。 [阶梯](internal/portrait/scale.go) · [外壳尺寸](internal/portrait/theme.go) · [文件树](internal/nativeui/sidebar.go)
- **动效**沿用原有时长与曲线：按下、切换、展开、进入、菜单和跳转；键盘操作或减少动态效果偏好下，过渡直接落位。选中的文档标签、文件行与大纲行通过共享胶囊在行间平滑滑动；指针点击收放停靠侧栏平滑过渡宽度并避让标题栏窗口控件，`⌘0` 则单帧瞬切；大纲点击将目标标题定位到编辑区顶部附近并同步原生选中态；Preview 的标题折叠与展开直接到位，见 issue #96。 [动效](internal/portrait/motion.go)
- **共用控件**：六种按钮、图标按钮、分段控件、TextField、SearchField、TextArea、Select、Stepper、Slider、Radio、DateField、Menu、ContextMenu、Popover、DialogShell、SettingsGroup/Row、Disclosure、OutputBlock、Chip、ListRow、Tooltip、Alert、Badge、Toast、Decision、Prompt、Confirm、Card、Frame、Skeleton 与 Phosphor 图标。图标尺寸只取 XS 12、S 14、M 16、L 20；除 GenUI 文档内容阅读器外，原生表面不直接调用框架的 Button/TextInput/TextArea/Select/Popover/Collapsible。 [控件](internal/portrait/controls.go) · [表单](internal/portrait/fields.go) · [设置行](internal/portrait/settings_controls.go)

## 玻璃材质

玻璃只留给盖在内容上面的东西：抽屉、弹层和命令栏。它使用与 vendored 框架同一上游提交的官方 [glass 插件](third_party/mygo/plugins/glass/)，先模糊背后已绘制的内容，再叠白色染色和一圈发丝线，不再有径向高光。停靠的侧栏不用玻璃，它就是布景；停靠的研究面板是纸。系统要求减少透明度或提高对比度时改用不透明纸面。`saturate()` 和 `brightness()` 暂无对应能力，见 issue #91。 [玻璃用例](internal/nativeui/glass_test.go)

## 阅读与编辑

- Preview、Markdown 和就地块编辑共用原文缓冲及 Undo/Redo。段落、表格、公式、Mermaid、脚注和批注在原生视图中呈现；表格列宽可在任意行的列边界拖拽调整，横向滚动留在表格区域；列宽按字形估算，汉字等全角字按一个字号宽计，长单元格在列内换行，行高随最高的单元格增长，不再裁切。测试检查编辑前后原文的字节保真。 [原文状态](internal/nativeeditor/source.go) · [特殊块](internal/nativeeditor/special.go) · [表格](internal/nativeeditor/table.go) · [编辑测试](internal/nativeeditor/)
- GenUI 是只读的衍生视图。来源动作定位回原文，未知组件保持隔离并给出可读反馈。还没有视图时，纸页中央是一个空状态和一个强调按钮。版本 3 视图由布局与文本原语组成，网格按可用宽度减少列数，末行只剩一张卡时这张卡占满整行；并排的卡片等高；行里的标题只在词间换行，放不下时其余项换到下一行。卡片是圆角 12 的纸。视图格式和完整示例见 [GenUI 格式](docs/genui/view-format.md)。 [原生阅读器](internal/nativegenui/) · [格式校验](internal/genui/)
- 版本 3 的 `Title` 按声明的字号绘制，字重 600：sm 15、md 17、lg 21、xl 25、2xl 32、3xl 40 点；卡片、网格格子、列表项等紧凑位置里声明的字号保持顺序，"2xl"、"3xl" 画成 "xl" 的 25 点。卡片开头的 "sm"、"md" 标题不缩小，不小于同卡的列表项标题（15 点）和正文（15 点）。`Text` 正文 16 点，紧凑位置 15 点；声明字号时 xs 12、sm 14、md 16（紧凑位置 15）、lg 18 点。紧跟卡片开头标题的纯文字 `Text`，按副标题字号在所在宽度里排得下一行时是副标题：13 点（声明的字号更小时不放大）、次要色，与标题相距 2 点；排不下时按正文的字号和颜色绘制。同样的文字在同样的宽度下样式相同，与窗口此前的宽度无关。`Caption` 12 点、次要色，紧跟卡片开头标题时 13 点。 [字号表](internal/nativegenui/statements.go) · [字号用例](internal/nativegenui/statements_test.go)
- Feynman 研究面板聚焦 `Chat / Files / alphaXiv` 三栏。History、Actions、Model / Thinking 使用 Portrait 弹层、SearchField 与 ListRow；问题卡、工具输出、文件列表和 alphaXiv 分组共用 Card、Disclosure、Alert 与 OutputBlock。低频配置统一进入 Settings Center 的 Models & Providers、Search & Papers、MCP Servers、Packages 与 Diagnostics；研究齿轮、模型选择器底部和 alphaXiv Account 都是深链，不再存在独立设置标签。 [研究面板](internal/nativefeynman/extension.go) · [设置](internal/nativefeynman/settings.go) · [设置中心](internal/nativeui/settings.go)
- Review 差异视图按段显示行号、增删符号和增删底色，行号与符号对辅助技术隐藏，每行的内容节点自述其变更与行号（Added / Removed / Unchanged）；每段可接受、拒绝或清除选择。 [差异视图](internal/nativereview/diff.go)

## 状态与可访问性

状态同时使用可读文字或控件名称表达。侧栏的标签报告未保存、冲突、删除、只读和生成状态，窄位置用 6 点的色调圆点加提示，完整状态在无障碍说明里；纸页顶行用文字写出 Edited、Conflict、Read only 和 Generating。打开的原文是标签列表里的标签，方向键、Home 和 End 在其间移动；命令栏的结果是列表框里的选项；工作区色点是标签。可操作的标签、菜单、来源和输入控件提供原生角色与辅助说明。 [标签状态](internal/nativeui/tabs.go) · [可访问控件](internal/nativeui/)

## 界面改动如何验收

1. 先从目标交互找到原生窗口、状态与绘制代码。按需检查空态、错误、禁用、键盘焦点和窗口缩放行为。
2. 新的设计变量加入 Portrait 模块并登记来源，通过 [ADR 0012](docs/adr/0012-portrait-2-coss-reference-and-contracts.md) 的五类契约（取值与不变量按 [ADR 0013](docs/adr/0013-arc-experiment-scenery-page-command-bar.md) 与 [ADR 0014](docs/adr/0014-wuxing-spaces-per-space-accent-and-dark-mode.md) 更新，C3 覆盖全部 16 套配色）：来源登记、取值锁定、WCAG AA 实算、封闭阶梯和派生不变量；原生包的存量字面量只减不增。组件外观在 Portrait Visual Gallery 中以结构与关系型像素断言验收，界面行为通过 `ui.Tester` 在界面的公开绘制入口测试。
3. 运行覆盖该功能的 Go 测试；Markdown 编辑和 Preview 保真还由 [26 份原文语料](internal/testdata/markdown/) 覆盖。
4. 要看整窗效果时导出 2× 场景图：`MGG_SCENES_DIR=/tmp/mgg MGG_SCENES_PREFIX=v2_ MGG_SCENES_SET=wuxing go test ./internal/nativefeynman -run TestShellScenes -v`。除八个 Space 的浅色/深色回归外，矩阵覆盖设置分类、搜索与窄窗、工作区菜单、研究弹层、检查点、提案、链接 hover、查找栏和 GenUI request。 [场景](internal/nativefeynman/scenes_list_test.go)
5. 涉及原生菜单、Finder、输入法或辅助技术时，在 macOS 应用中执行 [原生 Automation 流程](docs/agents/native-automation.md)，并记录对应构建和实测结果。

## 已知边界

- 没有 Arc 的悬停滑出侧栏、两篇原文并排的分屏和标签自动归档。
- macOS 的系统侧栏材质跟随系统外观；所选外观与系统不同时布景改为不透明，桌面不再透出。
- macOS 菜单栏、窗口 chrome 与任意路径的文件打开/保存面板仍由系统绘制；应用内菜单、关闭决策、重命名、移动确认、恢复确认和可定位到窗口的错误提示使用 Portrait，并跟随 Space 与深浅外观。
- Inter 的五个字重文件比原来的 IBM Plex Sans 与 Barlow 多约 1.5 MB。
- 隐藏标题栏与系统材质只在 macOS 上启用，Windows 与 Linux 的原生窗口尚未实测。
