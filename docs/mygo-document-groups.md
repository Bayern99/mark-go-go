# MyGo 受控补丁

应用通过 `go.mod` 的 `replace` 使用仓库内 MyGo。版本及上游提交见 [SOURCE.txt](../third_party/mygo/SOURCE.txt)。上游按一窗一文档管理原生文档。补丁让多个根目录共享窗口，同时让原生菜单命中实际活动文档。产品取舍见 [ADR 0005](adr/0005-workspace-document-window-model.md)。

| 升级时检查 | 入口 |
|---|---|
| 文档身份、活动槽、关闭事务 | [document_group.go](../third_party/mygo/document_group.go) · [用例](../third_party/mygo/document_group_test.go) |
| 保存目标、文档打开与会话 | [document.go](../third_party/mygo/document.go) · [document_file.go](../third_party/mygo/document_file.go) · [document_flow.go](../third_party/mygo/document_flow.go) |
| 菜单路由和原生校验 | [document_menu.go](../third_party/mygo/document_menu.go) · [menu.go](../third_party/mygo/menu.go) · [Darwin WebKit](../third_party/mygo/internal/darwin/webview.go) · [Darwin window](../third_party/mygo/internal/darwin/window.go) · [Windows menu](../third_party/mygo/internal/windows/menu.go) |

升级前对照 `SOURCE.txt` 指定的上游版本核对补丁。同步 `mygo-runtime` 和生成绑定，跑文档组用例与三平台编译，再做 [macOS 原生 smoke](agents/native-automation.md)。
