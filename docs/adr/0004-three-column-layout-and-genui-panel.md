# ADR 0004 · 三栏与原地编辑

左栏可切换工作区文件树与活动原文的大纲，中央是当前原文及其内容动作。GenUI 的模型、生成和管理操作从中央阅读视图右上角展开。启用 Feynman 时，整个右栏归当前工作区的研究会话。切换文档或中央视图时对话和草稿持续可用。启用 Git 时，当前原文工具栏提供检查点弹层。窄窗把两侧导航收为可分别打开的浮层。

批注、特殊块和生成要求在动作原地展开编辑区，保留关联的内容和位置。迁移前的界面入口见固定提交 [81522cb 的 App.tsx](https://github.com/Bayern99/mark-go-go/blob/81522cb48ec9718bffbda0255df1039692a2f6cd/web/src/App.tsx)；当前原生窗口从 [nativeui/shell.go](../../internal/nativeui/shell.go) 进入。
