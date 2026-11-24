# Portrait 界面设计 · Arc 实验

这是一次隔离的重设计实验：假设由 Arc 浏览器的设计团队从头设计 Mark Go Go。界面仍由 MyGo Native UI 绘制，[internal/portrait](internal/portrait/) 仍是设计变量的唯一来源；变的是窗口构成、导航方式、材质和字体。取舍与来源见 [ADR 0013](docs/adr/0013-arc-experiment-scenery-page-command-bar.md)，实现入口见 [原生窗口外壳](internal/nativeui/shell.go) 和 [原生编辑器](internal/nativeeditor/)。

## 设计立场

- **窗口是一个房间，纸页是其中唯一的实体。** 窗口本身是带颜色的布景，侧栏直接坐在布景上，不加卡片和边线；原文是浮在布景上的一张不透明白纸。界面里其余的东西都比这张纸轻。
- **导航在侧面，不在顶上。** 打开的原文是侧栏里的竖排标签，文件名不会被挤短。原来的标签行、文档工具行和状态行三行界面合成纸页顶部的一行。
- **其余一切交给命令栏。** `⌘K` 或 `⌘T` 打开命令栏，搜索打开的原文、工作区文件、标题和全部命令，不必记住东西放在哪里。
- **工作区即 Space。** 每个文件夹工作区有自己的布景颜色，一眼认出身在哪个工作区。
- **不照搬的部分。** Arc 的强调色随主题变；这里强调色固定为一种蓝，保证正文链接、焦点环和主按钮在每种布景上都通过对比度检查。Arc 的文字灰度达不到正文对比度，墨色沿用原 Portrait 的取值。字体不用衬线展示体，产品内只有无衬线。

## 窗口构成

- **布景 Scenery**：窗口底色，一条 160° 的浅色渐变，共六种：Mist、Sky、Dawn、Twilight、Aurora、Sand。工作区按根路径固定分到一种彩色布景，没有文件夹的窗口用 Sky；工作区菜单的 **Scenery** 或命令栏可以改选，选择按工作区根路径记在偏好文件里。macOS 上窗口带系统侧栏材质，布景以 78% 不透明度叠在上面，桌面隐约透出；系统要求减少透明度或提高对比度时画成不透明。布景上的文字只用 Ink 与 Ink2。 [布景](internal/portrait/space.go) · [布景与对比度用例](internal/portrait/tone_test.go)
- **侧栏**：自上而下是窗口控件行、命令入口、打开的原文、工作区名（点开是切换工作区、打开文件夹、关闭工作区和 Scenery 的原生菜单）、Files / Outline 切换、文件过滤与文件树，底部是窗口内各工作区的色点、Plugins 菜单和新建原文。641 点及以上宽度时停靠，`⌘0` 或侧栏右上的按钮把它收起，纸页独占窗口；较窄时通过抽屉打开。 [侧栏](internal/nativeui/sidebar.go) · [竖排标签](internal/nativeui/tabs.go)
- **纸页 Page**：白色、圆角 12、带一圈发丝线和柔和投影，距窗口边缘 8 点。顶部一行依次是侧栏开关（侧栏未停靠时）、可点击定位的路径、编辑状态、版本、Preview / Markdown / GenUI 切换、检查点和研究面板开关；其下只有内容。完整路径在文件名的提示里，不再有底部状态行。 [纸页](internal/nativeui/document.go)
- **研究面板**：启用 Feynman 后，1180 点及以上宽度时是并排的第二张纸，像 Arc 的分屏；两张纸之间的空隙可拖动调宽。较窄时是从右侧滑出的玻璃抽屉。抽屉关闭或按 Esc 后，焦点返回其入口。 [窗口外壳](internal/nativeui/shell.go) · [键盘与窄窗用例](internal/nativeui/shell_test.go)
- **窗口控件**：macOS 隐藏系统标题栏，红绿灯落在侧栏顶部；侧栏开关、路径和视图切换与红绿灯同处一条水平线，这一行可以拖动窗口。侧栏收起时纸页顶行让出红绿灯的位置。Linux 与 Windows 保留系统标题栏。 [窗口适配](internal/nativeui/window.go) · [窗口控件用例](internal/nativeui/sidebar_test.go)

## 命令栏

命令栏是居中偏上的玻璃浮层：一行输入，下面是结果。空白时先列其他打开的原文（按最近使用排序，当前原文排在最后），再列命令。输入后同时匹配打开的原文、当前原文的标题、工作区里的文件与标题，以及命令：新建原文、打开文件夹、切换视图、检查点、显示或收起侧栏、显示 Outline 或 Files、研究面板、启停插件、关闭原文、切换工作区、打开最近的工作区和更换布景。↑↓ 移动选中项，回车执行，Esc 关闭；选中项用强调色整行填充。命令栏借用工作区搜索，关闭时把侧栏自己的过滤词还回去。 [命令栏](internal/nativeui/command.go) · [命令栏用例](internal/nativeui/command_test.go)

## Portrait 模块

原生各包只从 Portrait 取字体、字号、颜色、材质和动效。 [守护测试](internal/portrait/contract_test.go) 在原生包中发现字号数字、颜色字面量或字体名时失败。每个设计变量登记 `webui@c6504af`、`coss@b2336c4`、`arc@adr-0013` 或 `portrait-derived` 之一。

- **字体**：界面与正文 Inter，一级和二级标题 Inter Display，代码 IBM Plex Mono，图标 Phosphor；汉字按字形回退到 IBM Plex Sans SC。字体和许可证随应用打包。 [字号与颜色](internal/portrait/portrait.go) · [字体资源](internal/portrait/fonts/)
- **字号阶梯**仍是封闭的：display 28/36、title 20/28、reading 16/28、small 14/22、ui 13/18、control 13/20、meta 12/16、code 13/22；没有小于 12 的字。字间距随字号收紧：display −0.02em，title −0.012em，正文不调。编辑器的行样式还不能设字间距，Preview 的标题靠 Inter Display 自身较紧的间距。Feynman 的 Markdown 主题把代码块语言名与复制按钮、有序列表序号和图片占位显式设为 meta 12。
- **颜色**：表面与墨色沿用原 Portrait。强调色是 Arc 的品牌蓝 `#3139fb`，白字对比度 6.75；焦点环是它的 28% 不透明度。`Success` 是独立的绿色 `#197a49`，`Info` 是强调色的别名；警告与危险色不变。差异视图的新增行、复制成功的对勾和上升趋势用 `Success`。 [色调](internal/portrait/tone.go)
- **材质**是平的：没有渐变面和多层内阴影。凸起是白纸加发丝线和一层浅投影，悬停与按下只改变投影和底色；强调按钮是纯色加同色光晕；行悬停、凹面和分段控件的轨道是 5% 左右的黑色薄层；`Sheet`、`Pop`、`Page` 是白纸加由浅到深的三档投影。 [材质](internal/portrait/material.go) · [控件](internal/portrait/controls.go)
- **圆角**只有 4、6、8、12、16 与胶囊形，外加嵌套的轨道 10 与分段 8。行与按钮 8，纸页、卡片与弹层 12，抽屉与命令栏 16。控件高度仍是 22、28、32；侧栏的行高 32。 [阶梯](internal/portrait/scale.go) · [外壳尺寸](internal/portrait/theme.go)
- **动效**沿用原有时长与曲线：按下、切换、展开、进入、菜单和跳转；键盘操作或减少动态效果偏好下，过渡直接落位。选中的标签是一张在行间滑动的白色胶囊。Preview 的标题折叠与展开直接到位，见 issue #96。 [动效](internal/portrait/motion.go)
- **共用控件**：凸起、强调、安静、危险和链接按钮，图标按钮，分段控件，输入框，浮层，工具提示，Alert、Badge、Toast、Confirm、Card、Frame、Skeleton 与 Phosphor 图标。工具条与侧栏内一律用安静按钮。 [图标](internal/portrait/icons.go)

## 玻璃材质

玻璃只留给盖在内容上面的东西：抽屉、弹层和命令栏。它使用与 vendored 框架同一上游提交的官方 [glass 插件](third_party/mygo/plugins/glass/)，先模糊背后已绘制的内容，再叠白色染色和一圈发丝线，不再有径向高光。停靠的侧栏不用玻璃，它就是布景；停靠的研究面板是纸。系统要求减少透明度或提高对比度时改用不透明纸面。`saturate()` 和 `brightness()` 暂无对应能力，见 issue #91。 [玻璃用例](internal/nativeui/glass_test.go)

## 阅读与编辑

- Preview、Markdown 和就地块编辑共用原文缓冲及 Undo/Redo。段落、表格、公式、Mermaid、脚注和批注在原生视图中呈现；表格列宽可在任意行的列边界拖拽调整，横向滚动留在表格区域。测试检查编辑前后原文的字节保真。 [原文状态](internal/nativeeditor/source.go) · [特殊块](internal/nativeeditor/special.go) · [表格](internal/nativeeditor/table.go) · [编辑测试](internal/nativeeditor/)
- GenUI 是只读的衍生视图。来源动作定位回原文，未知组件保持隔离并给出可读反馈。还没有视图时，纸页中央是一个空状态和一个强调按钮。版本 3 视图由布局与文本原语组成，网格按可用宽度减少列数，末行只剩一张卡时这张卡占满整行；并排的卡片等高；行里的标题只在词间换行，放不下时其余项换到下一行。卡片是圆角 12 的纸。视图格式和完整示例见 [GenUI 格式](docs/genui/view-format.md)。 [原生阅读器](internal/nativegenui/) · [格式校验](internal/genui/)
- 版本 3 的 `Title` 按声明的字号绘制，字重 600：sm 15、md 17、lg 21、xl 25、2xl 32、3xl 40 点；卡片、网格格子、列表项等紧凑位置里声明的字号保持顺序，"2xl"、"3xl" 画成 "xl" 的 25 点。卡片开头的 "sm"、"md" 标题不缩小，不小于同卡的列表项标题（15 点）和正文（15 点）。`Text` 正文 16 点，紧凑位置 15 点；声明字号时 xs 12、sm 14、md 16（紧凑位置 15）、lg 18 点。紧跟卡片开头标题的纯文字 `Text`，按副标题字号在所在宽度里排得下一行时是副标题：13 点（声明的字号更小时不放大）、次要色，与标题相距 2 点；排不下时按正文的字号和颜色绘制。同样的文字在同样的宽度下样式相同，与窗口此前的宽度无关。`Caption` 12 点、次要色，紧跟卡片开头标题时 13 点。 [字号表](internal/nativegenui/statements.go) · [字号用例](internal/nativegenui/statements_test.go)
- Review 差异视图按段显示行号、增删符号和增删底色，行号与符号对辅助技术隐藏，每行的内容节点自述其变更与行号（Added / Removed / Unchanged）；每段可接受、拒绝或清除选择。 [差异视图](internal/nativereview/diff.go)

## 状态与可访问性

状态同时使用可读文字或控件名称表达。侧栏的标签报告未保存、冲突、删除、只读和生成状态，窄位置用 6 点的色调圆点加提示，完整状态在无障碍说明里；纸页顶行用文字写出 Edited、Conflict、Read only 和 Generating。打开的原文是标签列表里的标签，方向键、Home 和 End 在其间移动；命令栏的结果是列表框里的选项；工作区色点是标签。可操作的标签、菜单、来源和输入控件提供原生角色与辅助说明。 [标签状态](internal/nativeui/tabs.go) · [可访问控件](internal/nativeui/)

## 界面改动如何验收

1. 先从目标交互找到原生窗口、状态与绘制代码。按需检查空态、错误、禁用、键盘焦点和窗口缩放行为。
2. 新的设计变量加入 Portrait 模块并登记来源，通过 [ADR 0012](docs/adr/0012-portrait-2-coss-reference-and-contracts.md) 的五类契约（取值与不变量按 [ADR 0013](docs/adr/0013-arc-experiment-scenery-page-command-bar.md) 更新）：来源登记、取值锁定、WCAG AA 实算、封闭阶梯和派生不变量；原生包的存量字面量只减不增。组件外观在 Portrait Visual Gallery 中以结构与关系型像素断言验收，界面行为通过 `ui.Tester` 在界面的公开绘制入口测试。
3. 运行覆盖该功能的 Go 测试；Markdown 编辑和 Preview 保真还由 [26 份原文语料](internal/testdata/markdown/) 覆盖。
4. 要看整窗效果时导出场景图：`MGG_SCENES_DIR=/tmp go test ./internal/nativefeynman -run TestShellScenes`，它用真实的外壳、编辑器和研究面板按 2 倍像素渲染十个状态。 [场景](internal/nativefeynman/scenes_list_test.go)
5. 涉及原生菜单、Finder、输入法或辅助技术时，在 macOS 应用中执行 [原生 Automation 流程](docs/agents/native-automation.md)，并记录对应构建和实测结果。

## 实验的已知缺口

- 没有 Arc 的悬停滑出侧栏、两篇原文并排的分屏和标签自动归档。
- 布景只染窗口底色；强调色不随工作区变化。
- 深色主题仍不在范围内。
- Inter 的五个字重文件比原来的 IBM Plex Sans 与 Barlow 多约 1.5 MB。
- 隐藏标题栏与系统材质只在 macOS 上启用，Windows 与 Linux 的原生窗口尚未实测。
