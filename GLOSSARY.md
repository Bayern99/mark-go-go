# Mark Go Go 术语

| 术语 | 含义 |
|---|---|
| 原文 · Document | 用户编辑的 Markdown。 |
| 未命名原文 · Untitled document | 尚未首次保存的原文。 |
| 工作区 · Workspace | 一个文件夹根目录及其文件导航。 |
| 工作区窗口 · Workspace window | 承载一组文档标签与一个或多个工作区的窗口。 |
| 当前工作区 · Selected workspace | 左栏正在显示文件树与过滤结果的工作区。 |
| 文档标签 · Document tab | 侧栏里切换原文的竖排控件。 |
| 视图模式 · View mode | 同一原文在中央的 Preview、Markdown 或 GenUI。 |
| Markdown 预览 · Markdown Preview | 渲染后的可编辑视图，按钮显示 `Preview`。 |
| Markdown | 直接编辑原文文本的视图。 |
| 原地 Markdown 编辑 · In-place Markdown editing | 在特殊块内直接编辑该块的 Markdown。 |
| GenUI 视图 · GenUI view | 从原文派生的只读阅读视图。 |
| 组件目录 · Catalog | GenUI 接受的组件及规则。 |
| 语句 · Statement | 版本 3 视图里的一行 OpenUI Lang：`名称 = 组件(参数)`。 |
| 原语 · Primitive | 版本 3 组件目录中的布局、文本等基础组件。 |
| 来源引用 · Source reference | 组件所依据的原文章节。 |
| 原文已改 · Source changed | 来源章节的内容已有变化。仅修改批注不触发。 |
| 生成要求 · Generation request | 读者随 GenUI 保存并可沿用的生成偏好。 |
| 批注 · Note | 与原文一同保存的读者文字意见。 |
| 研究会话 · Research session | 同一文件夹工作区内跨文档续接的研究对话（内置支持 Feynman，不绑死核心编辑模型）。 |
| 外部 Agent · External agent | 用户在自己的终端里运行、经 `markgogo` 命令或 stdio 垫片读写当前工作区的 Harness（Codex、Pi 等；不接入 Claude Code）；它不生成阅读视图，与 GenUI 后端是两件事。 |
| 外部通道 · External IPC | 应用在用户数据目录里的 `ipc.sock` 上按行接收 JSON-RPC 的本机通道，默认关闭；外部 Agent 与 `markgogo` CLI 经它读写当前工作区。 |
| stdio 垫片 · stdio shim | `markgogo mcp`：把 MCP 客户端的标准输入输出按行接到外部通道，客户端不需要非标准字段。 |
| 守卫扩展 · Guard extension | 随应用分发、由 GenUI 的 Pi 后端在每轮生成时以 `-e` 显式加载的扩展；把读取限制在本轮隔离目录、写入限制在本轮 `out.json`，未确认加载时不生成。 |
| 变更提案 · Change proposal | 由任意 Agent 提交、尚未写入原文、等待在编辑器中审阅后接受或拒绝的改动草稿。 |
| 检查点 · Checkpoint | 原文在某一时刻的本地版本快照（可附带标签与来源），可与编辑中的文字比较或恢复。 |
| Portrait | Mark Go Go 的界面设计语言，统一规定字体、颜色、材质与动效。 |
| 布景 · Scenery | 窗口的彩色底面，侧栏直接坐在上面；取自所在 Space 当前外观的两端颜色。 |
| Space | 一个工作区的颜色身份：八个五行 Space 之一，浅色与深色下各有布景、纸页、强调色和元素色。 |
| 外观 · Appearance | System、Light 或 Dark；System 跟随系统的浅色或深色外观。 |
| 纸页 · Page | 浮在布景上的不透明圆角纸面，承载当前原文及其顶行；颜色随 Space 与外观。 |
| 侧栏 · Sidebar | 布景左侧的导航：命令入口、文档标签、工作区、文件或大纲。 |
| 设置中心 · Settings Center | `⌘,` 打开的统一设置 sheet；分类导航、实时搜索、重置与深链都从这里进入。 |
| 设置页 · Settings page | 通过 `SettingsPage` 注册的一个设置分类；拥有标签、图标、关键词、绘制、可见性与可选重置行为。 |
| 插件 · Plugin | 可在 `Settings / Plugins` 单独开关的功能模块（Feynman、GenUI、Git Checkpoints）。GenUI 缺省开启，其余缺省关闭；开关保存在 `mgo.plugin.*`。 |
| 命令栏 · Command Bar | `⌘K` 打开的浮层，搜索原文、文件、标题并执行命令。 |
| 色调 · Tone | Portrait 的一组语义配色：Neutral、Accent（即 Success）、Info、Warning 或 Danger。 |
| 托盘 · Frame | 放在纸面上、比内容外扩一圈的浅色衬底，用来把一组卡片或控件收在一起。 |
| 卡片 · Card | 承载一块独立内容的白色纸片容器，可放在纸面或玻璃面上。 |
| 抽屉 · Drawer | 窄窗时从窗口边缘滑出、暂时覆盖内容的玻璃侧栏或研究面板。 |
| 输入框 · InputFrame | 文字输入控件的外框与焦点环，不含输入行为本身。 |
