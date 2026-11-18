# ADR 0006 · 特殊块与语义色

Preview 用可视形式呈现支持的特殊块。无法呈现的块保留可见的原始 Markdown。修改只写回当前块，沿用逐块保真和撤销路径。原地编辑的交互参考 [SoloMD](https://github.com/zhitongblog/solomd/commit/219a6baafccbe7d50776a1a41909189141dfea4b)。

Portrait 使用浅色。原文中的提示类型与应用的冲突、错误等状态使用各自的语义色，两者都配可辨认的文字或图标。部分来源丢失时，内容动作仍可使用有效来源。实现见 [预览块](../../web/src/editor/blocks/) 与 [来源组件](../../web/src/plugins/genui/SourceFrame.tsx)。
