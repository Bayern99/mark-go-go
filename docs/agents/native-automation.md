# macOS 原生验证

在项目根目录运行随 MyGo 锁定的官方 CLI。首次运行前先按 [README](../../README.md#开发) 安装 Web 依赖。

```sh
go tool mygo-automation skills
scripts/native-automation-smoke.sh
```

Smoke 创建独立的 QA 应用、用户数据目录、文档和私有 Unix socket。`MYGO_AUTOMATION_REPORT` 可指定截图路径。检查范围和清理以 [脚本](../../scripts/native-automation-smoke.sh) 为准。

手动驱动隔离的开发版时，用 `GOFLAGS=-tags=e2e` 和 `MYGO_AUTOMATION=<私有 socket 路径>` 启动 `go tool mygo dev`。连接、菜单、`eval` 和截图用 `go tool mygo-automation`，命令见 `skills`。每个运行使用独立的 session。正式构建的插件入口见 [automation_e2e.go](../../automation_e2e.go) 与 [上游发布守卫](../../third_party/mygo/plugins/automation/off.go)。

Smoke 的 `Ctrl+Tab` 使用页面合成事件。物理键盘、系统文件对话框、输入法和辅助技术用原生应用复验。将构建标识、实测结果和仍待验证的范围一起记录。
