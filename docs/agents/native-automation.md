# macOS 原生验证

从项目根目录运行与 MyGo 同版本的官方 CLI 和原生 smoke。

```sh
go tool mygo-automation skills
scripts/native-automation-smoke.sh
```

脚本从 Go 源码和应用资源打包 QA 应用，创建临时文稿、独立应用标识和私有 Unix socket。它从 Finder 的系统打开入口送入第二篇文稿，通过 Automation 确认窗口为 Content、可访问树包含 Markdown 源码编辑器，再用键盘输入和文件菜单保存。脚本比较含混合换行与 Unicode 的完整文件字节，检查编辑菜单的撤销、重做，并在退出后启动新进程核对活动文稿。设置 `MYGO_AUTOMATION_REPORT` 可选择截图路径。[原生 smoke](../../scripts/native-automation-smoke.sh)

手动检查时，在具有独立应用标识的测试副本中，用 `GOFLAGS=-tags=e2e` 和 `MYGO_AUTOMATION=<私有 socket 路径>` 启动 `go tool mygo dev`。通过 `go tool mygo-automation` 的 `windows`、`snapshot --full`、`key`、`type` 和 `menu` 观察和驱动真实控件。系统文件对话框、输入法和 VoiceOver 使用对应的 macOS 服务实测，记录构建标识和操作结果。[测试构建入口](../../automation_e2e.go) · [MyGo 发布守卫](../../third_party/mygo/plugins/automation/off.go)

2026 年 11 月 20 日在 macOS 27.2（26B123）arm64、Go 1.27.3 和 MyGo 0.10.0 上，首个原生窗口通过上述 smoke。Automation 读到 `[content] "Discussion.md"` 和 `[textField] "Markdown source"`。在含 CRLF 与 LF 的夹具开头输入 `前🙂` 后，菜单保存写入预期字节，撤销和重做改变控件文本。退出后，新进程重新显示相同文稿，完整文件比较通过。同一源码的默认构建产出 macOS app 和 DMG，Linux amd64 与 Windows amd64 完成交叉编译。
