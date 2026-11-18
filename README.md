# Mark Go Go

[English](README.en.md) · [致谢与许可](ACKNOWLEDGMENTS.md)

Mark Go Go 是 macOS 本地 Markdown 编辑器。多个文件夹工作区共享窗口和文档标签，`Ctrl+Tab` 在最近的文档间往返。按 `⌘/` 在可编辑的预览和 Markdown 间切换。右栏可以查看大纲，或通过已配置的 ACP Agent 生成只读 GenUI 视图。

## 开发

Go 和 Node 版本见 [CI 配置](.github/workflows/ci.yml)。

```sh
npm --prefix web ci
go tool mygo dev
```

浏览器预览运行 `npm --prefix web run dev`。绑定变更后运行 `go tool mygo generate`。正式构建运行 `go tool mygo build`。

## 验证

```sh
npx --prefix web playwright install chromium-headless-shell
npm --prefix web test
go test -race .
```

菜单、保存对话框和重启使用 [原生 Automation 与 macOS 验证流程](docs/agents/native-automation.md)。

[架构与代码入口](ARCHITECTURE.md) · [更新日志](CHANGELOG.md) · [验收记录](docs/acceptance.md) · [MIT 许可证](LICENSE)
