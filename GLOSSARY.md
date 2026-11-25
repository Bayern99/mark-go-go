# Mark Go Go 术语

| 术语 | 含义 |
|---|---|
| 原文 · Document | 用户编辑的 Markdown。 |
| 未命名原文 · Untitled document | 尚未首次保存的原文。 |
| 工作区 · Workspace | 一个文件夹根目录及其文件导航。 |
| 工作区窗口 · Workspace window | 承载一组文档标签与一个或多个工作区的窗口。 |
| 当前工作区 · Selected workspace | 左栏正在显示文件树与过滤结果的工作区。 |
| 文档标签 · Document tab | 顶部切换原文的控件。 |
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
| 外部通道 · External IPC | 应用在用户数据目录里的 `ipc.sock` 上按行接收 JSON-RPC 的本机通道，默认关闭；外部 Agent 与 `markgogo` CLI 经它读写当前工作区。 |
| stdio 垫片 · stdio shim | `markgogo mcp`：把 MCP 客户端的标准输入输出按行接到外部通道，客户端不需要非标准字段。 |
| 变更提案 · Change proposal | 由任意 Agent 提交、尚未写入原文、等待在编辑器中审阅后接受或拒绝的改动草稿。 |
| 检查点 · Checkpoint | 原文在某一时刻的本地版本快照（可附带标签与来源），可与编辑中的文字比较或恢复。 |
| Portrait | Mark Go Go 的界面设计语言，统一规定字体、颜色、材质与动效。 |
| 色调 · Tone | Portrait 的一组语义配色：Neutral、Accent（即 Success）、Info、Warning 或 Danger。 |
| 托盘 · Frame | 放在纸面上、比内容外扩一圈的浅色衬底，用来把一组卡片或控件收在一起。 |
| 卡片 · Card | 承载一块独立内容的白色纸片容器，可放在纸面或玻璃面上。 |
| 抽屉 · Drawer | 窄窗时从窗口边缘滑出、暂时覆盖内容的侧栏或研究面板。 |
| 输入框 · InputFrame | 文字输入控件的外框与焦点环，不含输入行为本身。 |
