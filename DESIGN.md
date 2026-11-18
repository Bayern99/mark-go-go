# Portrait 界面设计

Mark Go Go 是围绕原文阅读和编辑的桌面工作台。原文占据视觉中心，Portrait 的浅色框架、纸面和控件给导航与操作提供清楚的层级。界面保留完成任务所需的信息和反馈。

## 窗口与阅读

- 左栏显示当前工作区的文件树和过滤，中央显示活动原文的 Preview、Markdown 或 GenUI，右栏放大纲与 GenUI 生成、进度和管理。内容动作贴着对应原文或卡片。 [三栏决策](docs/adr/0004-three-column-layout-and-genui-panel.md) · [窗口界面](web/src/App.tsx)
- 窗口顶部共用一组文档标签。标签、左栏和正文跟随活动原文。宽度不足时，活动标签保留入口，其余进入可搜索的溢出菜单。窄窗把右栏收为可展开的浮层，正文保持可用。未保存、冲突与删除状态留在相应文档的标签和导航内。 [工作区决策](docs/adr/0005-workspace-document-window-model.md) · [标签测试](web/test/workspace-tabs.test.tsx)
- 正文沿纸面的阅读边线排版，标题、段落与常规块共享阅读宽度。表格等原本需要宽度的块先使用可用空间，再在块内横滚。导航、正文和大纲各有自己的滚动区域，切换文档保留各自的阅读位置。 [预览排版](web/src/editor/crepe-theme.css) · [ScrollArea](web/src/components/ScrollArea.tsx)

## 样式与交互

- 用已有 Portrait Token 和组件组成同一套界面。灰色外框衬托白色原文纸面，标题、对齐和留白建立阅读顺序。IBM Plex Sans / SC 用于阅读和界面，Barlow Semi Condensed 用于展示类标题和标签，IBM Plex Mono 用于源码与代码。字体和 Phosphor 图标随应用打包。 [Token 名称](web/src/theme/portrait-tokens.ts) · [样式定义](web/src/styles.css)
- 浅凸起用于按钮和活动标签，搜索与分段控件使用凹槽，浮动菜单沿用现有边角和阴影。区分选中、悬停、按下、键盘焦点和禁用。仅有图标的控件提供可访问名称，交互反馈留在控件本身，避免带动相邻内容跳动。 [基础组件](web/src/components/) · [GenUI 样式](web/src/plugins/genui/genui.css)
- 用颜色配合文字或图标表达状态。选中和主操作使用强调色，原文已改使用警告色，文件冲突、删除与生成失败使用错误色。原文内部提示沿用独立的内容语义色。 [语义色决策](docs/adr/0006-preserved-blocks-and-semantic-colors.md)
- 批注、特殊块 Markdown 编辑与生成要求在目标位置展开输入区，沿用 `⌘↩` 保存和 `Esc` 取消。GenUI 为只读视图，内容动作通过来源引用回到原文。加载、空态、失败和禁用从实际状态派生。键盘焦点立即可见，减少动态效果时保留操作反馈。 [预览块](web/src/editor/blocks/) · [来源动作](web/src/plugins/genui/SourceFrame.tsx)

## 界面改动如何验收

1. 从目标交互定位组件、状态和样式。修改后核对真实触发和结果，并检查适用的空态、错误、禁用与焦点恢复。 [页面测试](web/test/)
2. 用真实 Markdown 试默认和最小窗口，确认文档内容、活动标签和主要操作仍可达。涉及阅读宽度或行距时，运行既有排版基准。 [窗口尺寸](workspace.go) · [排版基准](docs/editor-typeset/README.md)
3. 涉及原生菜单、文件面板、输入法或辅助技术时，在 macOS 应用中验证。记录实际操作的版本、结果和待验项。 [原生验证](docs/agents/native-automation.md) · [验收记录](docs/acceptance.md)
