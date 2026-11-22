# Portrait 界面设计

Mark Go Go 的正式界面由 MyGo Native UI 绘制。原文居中显示，导航和可选工作区面板围绕编辑与阅读任务排列。实现入口见 [原生窗口外壳](internal/nativeui/shell.go) 和 [原生编辑器](internal/nativeeditor/)。

## 窗口与阅读

- 中央区域显示当前文档的 Preview、Markdown 或 GenUI 视图。顶部标签切换文档；标签过多时，溢出菜单提供搜索和键盘导航。左栏在工作区文件与当前文档大纲间切换。 [标签](internal/nativeui/tabs.go) · [导航](internal/nativeui/navigation.go) · [文档视图](internal/nativeui/document.go)
- 641 点及以上宽度时左栏停靠，较窄时通过抽屉打开。启用 Feynman 后，研究面板在 1180 点及以上宽度时停靠，较窄时通过抽屉打开；分隔线可调整宽度。抽屉关闭或按 Esc 后，焦点返回其入口。 [窗口外壳](internal/nativeui/shell.go) · [键盘与窄窗用例](internal/nativeui/shell_test.go)
- Preview、Markdown 和就地块编辑共用原文缓冲及 Undo/Redo。段落、表格、公式、Mermaid、脚注和批注在原生视图中呈现；表格列宽和横向滚动留在表格区域。测试检查编辑前后原文的字节保真。 [原文状态](internal/nativeeditor/source.go) · [特殊块](internal/nativeeditor/special.go) · [表格](internal/nativeeditor/table.go) · [编辑测试](internal/nativeeditor/)
- GenUI 是只读的衍生视图。来源动作定位回原文，未知组件保持隔离并给出可读反馈。版本 3 视图由布局与文本原语组成，网格按可用宽度减少列数，末行只剩一张卡时这张卡占满整行；并排的卡片等高；行里的标题只在词间换行，放不下时其余项换到下一行。视图格式和完整示例见 [GenUI 格式](docs/genui/view-format.md)。 [原生阅读器](internal/nativegenui/) · [格式校验](internal/genui/)

## 样式与交互

- Portrait 使用连续白色正文、IBM Plex Sans / Sans SC 阅读字体、IBM Plex Mono 代码字体、Barlow Semi Condensed 展示字体和 Phosphor 图标。字体和许可证随 Go 应用资源打包。 [主题与材料](internal/nativeui/portrait.go) · [字体资源](internal/nativeui/fonts/)
- 按钮和选中项使用轻凸起，分段控件使用浅凹面。导航胶囊和浮层使用半透明材料；减少透明度或高对比度偏好下使用不透明表面。键盘操作及减少动态效果偏好下，选中状态直接落位。 [Portrait 样式](internal/nativeui/portrait.go)
- 状态同时使用可读文字或控件名称表达。标签报告未保存、冲突、删除、只读和生成状态；可操作的标签、菜单、来源和输入控件提供原生角色与辅助说明。 [标签状态](internal/nativeui/tabs.go) · [可访问控件](internal/nativeui/)

## 界面改动如何验收

1. 先从目标交互找到原生窗口、状态与绘制代码。按需检查空态、错误、禁用、键盘焦点和窗口缩放行为。
2. 运行覆盖该功能的 Go 测试；Markdown 编辑和 Preview 保真还由 [26 份原文语料](internal/testdata/markdown/) 覆盖。
3. 涉及原生菜单、Finder、输入法或辅助技术时，在 macOS 应用中执行 [原生 Automation 流程](docs/agents/native-automation.md)，并记录对应构建和实测结果。
