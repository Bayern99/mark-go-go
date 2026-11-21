# ADR 0001 · 本地宿主与原文保真

本 ADR 记录原生迁移前的架构，代码固定在 [81522cb48ec9718bffbda0255df1039692a2f6cd](https://github.com/Bayern99/mark-go-go/tree/81522cb48ec9718bffbda0255df1039692a2f6cd)。迁移后的窗口、Preview、Markdown 和 GenUI 均由 MyGo Native UI 绘制，当前入口见 [架构图](../../ARCHITECTURE.md)。

当时按块组合保存，未编辑的块及其间隔保持原字节。回归依据为固定提交中的 [往返用例](https://github.com/Bayern99/mark-go-go/blob/81522cb48ec9718bffbda0255df1039692a2f6cd/web/test/roundtrip.test.ts) 和 [撤销保真用例](https://github.com/Bayern99/mark-go-go/blob/81522cb48ec9718bffbda0255df1039692a2f6cd/web/test/undo-fidelity.test.ts)。现行编辑保真由 [原生编辑器及测试](../../internal/nativeeditor/) 覆盖。
