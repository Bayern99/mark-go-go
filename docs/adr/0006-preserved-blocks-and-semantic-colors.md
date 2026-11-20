# ADR 0006 · 特殊块与语义色

Preview 用可视形式呈现支持的特殊块。无法呈现的块保留可见的原始 Markdown。修改只写回当前块，沿用逐块保真和撤销路径。原地编辑的交互参考 [SoloMD](https://github.com/zhitongblog/solomd/commit/219a6baafccbe7d50776a1a41909189141dfea4b)。

Portrait 使用浅色。原文中的提示类型与应用的冲突、错误等状态使用各自的语义色，两者都配可辨认的文字或图标。部分来源丢失时，内容动作仍可使用有效来源。迁移前实现见固定提交 [81522cb 的预览块](https://github.com/Bayern99/mark-go-go/tree/81522cb48ec9718bffbda0255df1039692a2f6cd/web/src/editor/blocks) 和 [来源组件](https://github.com/Bayern99/mark-go-go/blob/81522cb48ec9718bffbda0255df1039692a2f6cd/web/src/plugins/genui/SourceFrame.tsx)；当前实现见 [原生编辑器](../../internal/nativeeditor/) 与 [GenUI 阅读器](../../internal/nativegenui/)。
