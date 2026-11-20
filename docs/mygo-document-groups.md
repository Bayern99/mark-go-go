# MyGo 受控补丁

应用通过 `go.mod` 的 `replace` 使用仓库内 MyGo。版本及上游提交见 [SOURCE.txt](../third_party/mygo/SOURCE.txt)。上游按一窗一文档管理原生文档。补丁让多个根目录共享窗口，同时让原生菜单命中实际活动文档。产品取舍见 [ADR 0005](adr/0005-workspace-document-window-model.md)。

| 升级时检查 | 入口 |
|---|---|
| 文档身份、活动槽、关闭事务 | [document_group.go](../third_party/mygo/document_group.go) · [用例](../third_party/mygo/document_group_test.go) |
| 保存目标、文档打开与会话 | [document.go](../third_party/mygo/document.go) · [document_file.go](../third_party/mygo/document_file.go) · [document_flow.go](../third_party/mygo/document_flow.go) |
| 菜单路由和原生校验 | [document_menu.go](../third_party/mygo/document_menu.go) · [menu.go](../third_party/mygo/menu.go) · [Darwin WebKit](../third_party/mygo/internal/darwin/webview.go) · [Darwin window](../third_party/mygo/internal/darwin/window.go) · [Linux menu](../third_party/mygo/internal/linux/menu.go) · [Windows menu](../third_party/mygo/internal/windows/menu.go) |
| 原生 Content 与 WebKit 的按需初始化 | [Darwin 启动](../third_party/mygo/internal/darwin/app.go) · [窗口创建](../third_party/mygo/internal/darwin/window.go) · [原生界面](../third_party/mygo/ui/) · [Markdown 源码编辑器](../third_party/mygo/ui/editor/) |
| macOS 最近使用标签切换 | [AppKit 窗口的 Ctrl+Tab 转发](../third_party/mygo/internal/darwin/window.go) · [工作区快捷键](../internal/nativeui/shell.go) |
| 可编辑 Markdown Preview、精确换行、选区与共用撤销 | [编辑端](../internal/nativeeditor/) · [原生缓冲](../third_party/mygo/ui/editor/buffer.go) · [上游编辑器差异](https://github.com/egoist/mygo/compare/ebde9399baf2734b3361856a53f4036859973d1d...f098a223fcbccfb2ce772f8d12f171f1b79c5c7a) |

升级前对照 `SOURCE.txt` 指定的上游提交核对补丁，跑文档组用例与三平台编译，再做 [macOS 原生 smoke](agents/native-automation.md)。
