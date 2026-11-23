# Portrait 界面设计

Mark Go Go 的正式界面由 MyGo Native UI 绘制。原文居中显示，导航和可选工作区面板围绕编辑与阅读任务排列。实现入口见 [原生窗口外壳](internal/nativeui/shell.go) 和 [原生编辑器](internal/nativeeditor/)。

## 视觉基准

- 已有变量的视觉基准是退役 WebUI 固定提交 [c6504af 的 web/src](https://github.com/Bayern99/mark-go-go/tree/c6504af20b2129486bc05ee4db314f8e5fb555d2/web/src) 中的 Portrait 浅色变量。目标是观感与节奏等价，允许像素差异；深色主题不在范围内。决策见 [ADR 0010](docs/adr/0010-portrait-on-mygo-base-widgets.md) 和 [ADR 0006](docs/adr/0006-preserved-blocks-and-semantic-colors.md)。
- WebUI 没有的组件参考 coss ui（[cosscom/ui@b2336c4](https://github.com/cosscom/ui/tree/b2336c45f2b7355c42678ce786b6f14d62b819be)，MIT）的组件语义与状态模型，不引入 React、Tailwind 或 WebView。每个设计变量登记 `webui@c6504af`、`coss@b2336c4` 或 `portrait-derived` 之一；Tone、圆角、尺寸、材质与交互边界见 [ADR 0012](docs/adr/0012-portrait-2-coss-reference-and-contracts.md)。
- 框架做不到的效果使用最接近的做法，并逐项记成带 `needs-triage` 标签的 issue，写明 WebUI 效果、替代做法、缺少的能力和自行实现的成本与收益。

## Portrait 模块

[internal/portrait](internal/portrait/) 是设计变量的唯一来源，原生各包只从这里取字体、字号、颜色、材质和动效。 [守护测试](internal/portrait/contract_test.go) 在原生包中发现字号数字、颜色字面量或字体名时失败。

- 字体栈：正文 IBM Plex Sans 与 IBM Plex Sans SC，展示 Barlow Semi Condensed（字号乘以 1.05，对应 WebUI 的 `size-adjust: 105%`），代码 IBM Plex Mono，图标 Phosphor。MyGo 按字形回退，汉字自动落到 IBM Plex Sans SC。字体和许可证随应用打包。 [字号与颜色](internal/portrait/portrait.go) · [字体资源](internal/portrait/fonts/)
- 字号阶梯是封闭的：display 28/36、title 20/28、reading 16/28（字间距 0.01em）、small 14/22、ui 13/18、control 13/20、meta 12/16、code 13/22；没有小于 12 的字。Feynman 的 Markdown 主题把代码块语言名与复制按钮、有序列表序号和图片占位显式设为 meta 12；未配置这些次要文字字号的 `ui/markdown` 调用方仍保留框架按正文比例计算的默认外观。
- 颜色沿用 WebUI 的表面、墨色、线条、强调与状态色；MyGo 基础组件的主题按 WebUI 的 `--mgo-*` 桥接变量取值。 [主题](internal/portrait/theme.go)
- 材质：凸起、按下、行、凹面、轨道、页面与浮层的渐变和多层阴影，以及左右栏和浮层的半透明玻璃。 [材质](internal/portrait/material.go)
- 动效：按下、切换、展开、进入、菜单和跳转的时长与曲线；键盘操作或减少动态效果偏好下，过渡直接落位。Preview 的标题折叠与展开直接到位：编辑器把后文的块放在上一帧的高度上，高度过渡会让文字叠画，见 issue #96。 [动效](internal/portrait/motion.go)
- 共用控件：凸起、强调和安静按钮，图标按钮，分段控件，输入框，浮层，工具提示，加载指示与 Phosphor 图标。 [控件](internal/portrait/controls.go) · [图标](internal/portrait/icons.go)

## 玻璃材质

半透明材质使用与 vendored 框架同一上游提交的官方 [glass 插件](third_party/mygo/plugins/glass/)：Portrait 的玻璃先由插件模糊背后已绘制的内容，再叠加白色染色、径向高光和边缘阴影。系统要求减少透明度或提高对比度时改用不透明纸面。WebUI 的 `saturate()` 和 `brightness()` 暂无对应能力，见 issue #91。

## 窗口与阅读

- 中央区域显示当前文档的 Preview、Markdown 或 GenUI 视图。顶部标签切换文档；标签过多时，溢出菜单提供搜索和键盘导航。左栏在工作区文件与当前文档大纲间切换。 [标签](internal/nativeui/tabs.go) · [导航](internal/nativeui/navigation.go) · [文档视图](internal/nativeui/document.go)
- 641 点及以上宽度时左栏停靠，较窄时通过抽屉打开。启用 Feynman 后，研究面板在 1180 点及以上宽度时停靠，较窄时通过抽屉打开；分隔线可调整宽度。抽屉关闭或按 Esc 后，焦点返回其入口。 [窗口外壳](internal/nativeui/shell.go) · [键盘与窄窗用例](internal/nativeui/shell_test.go)
- Preview、Markdown 和就地块编辑共用原文缓冲及 Undo/Redo。段落、表格、公式、Mermaid、脚注和批注在原生视图中呈现；表格列宽可在任意行的列边界拖拽调整，横向滚动留在表格区域。测试检查编辑前后原文的字节保真。 [原文状态](internal/nativeeditor/source.go) · [特殊块](internal/nativeeditor/special.go) · [表格](internal/nativeeditor/table.go) · [编辑测试](internal/nativeeditor/)
- GenUI 是只读的衍生视图。来源动作定位回原文，未知组件保持隔离并给出可读反馈。版本 3 视图由布局与文本原语组成，网格按可用宽度减少列数，末行只剩一张卡时这张卡占满整行；并排的卡片等高；行里的标题只在词间换行，放不下时其余项换到下一行。视图格式和完整示例见 [GenUI 格式](docs/genui/view-format.md)。 [原生阅读器](internal/nativegenui/) · [格式校验](internal/genui/)
- 版本 3 的 `Title` 按声明的字号绘制，字重 600：sm 15、md 17、lg 21、xl 25、2xl 32、3xl 40 点；卡片、网格格子、列表项等紧凑位置里声明的字号保持顺序，"2xl"、"3xl" 画成 "xl" 的 25 点。卡片开头的 "sm"、"md" 标题不缩小，不小于同卡的列表项标题（15 点）和正文（15 点）。`Text` 正文 16 点，紧凑位置 15 点；声明字号时 xs 12、sm 14、md 16（紧凑位置 15）、lg 18 点。紧跟卡片开头标题的纯文字 `Text`，按副标题字号在所在宽度里排得下一行时是副标题：13 点（声明的字号更小时不放大）、次要色，与标题相距 2 点；排不下时按正文的字号和颜色绘制。同样的文字在同样的宽度下样式相同，与窗口此前的宽度无关。`Caption` 12 点、次要色，紧跟卡片开头标题时 13 点。 [字号表](internal/nativegenui/statements.go) · [字号用例](internal/nativegenui/statements_test.go)
- Review 差异视图按段显示行号、增删符号和增删底色，行号与符号对辅助技术隐藏，每行的内容节点自述其变更与行号（Added / Removed / Unchanged）；每段可接受、拒绝或清除选择。 [差异视图](internal/nativereview/diff.go)

## 状态与可访问性

状态同时使用可读文字或控件名称表达。标签报告未保存、冲突、删除、只读和生成状态；可操作的标签、菜单、来源和输入控件提供原生角色与辅助说明。 [标签状态](internal/nativeui/tabs.go) · [可访问控件](internal/nativeui/)

## 界面改动如何验收

1. 先从目标交互找到原生窗口、状态与绘制代码。按需检查空态、错误、禁用、键盘焦点和窗口缩放行为。
2. 新的设计变量加入 Portrait 模块并登记来源，通过 [ADR 0012](docs/adr/0012-portrait-2-coss-reference-and-contracts.md) 的五类契约：来源登记、取值锁定（WebUI 来源仍与 c6504af 比较）、WCAG AA 实算、封闭阶梯和派生不变量；原生包的存量字面量只减不增。组件外观在 Portrait Visual Gallery 中以结构与关系型像素断言验收，界面行为通过 `ui.Tester` 在界面的公开绘制入口测试。
3. 运行覆盖该功能的 Go 测试；Markdown 编辑和 Preview 保真还由 [26 份原文语料](internal/testdata/markdown/) 覆盖。
4. 涉及原生菜单、Finder、输入法或辅助技术时，在 macOS 应用中执行 [原生 Automation 流程](docs/agents/native-automation.md)，并记录对应构建和实测结果。
