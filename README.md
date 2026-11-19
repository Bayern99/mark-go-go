# Mark Go Go

[English](README.en.md) · [致谢与许可](ACKNOWLEDGMENTS.md)

Mark Go Go 是 macOS 本地 Markdown 编辑器。多个文件夹工作区共享窗口和文档标签，`Ctrl+Tab` 在最近的文档间往返。按 `⌘/` 在可编辑的预览和 Markdown 间切换。右栏可以查看大纲，或通过已配置的 ACP Agent 生成只读 GenUI 视图。

## 可选插件

桌面端在文档标签栏的 **Plugins** 菜单中分别启用 Feynman 和 Git。两项初始均关闭。

**Feynman** 使用本机的 Feynman CLI、Node.js 和 `pi-acp`。通过 `npm install -g pi-acp` 安装适配器，按 Feynman 的设置指南配置模型和登录，再打开右栏的 **Feynman**。同一工作区共用研究对话，重开后可续接。在 **Files** 中打开报告、计划、草稿或来源记录，它们会进入普通文档标签。勾选 **Edit this document** 会把当前编辑内容交给 Feynman。查看差异后，**Apply in Markdown** 将改动作为一次可撤销编辑应用到 Markdown 原文。[Feynman 设置指南](https://feynman.is/docs/getting-started/setup)

**Git** 使用现有本地仓库。先保存当前文档，再到右栏 **Git** 创建 Checkpoint。选取版本即可与当前缓冲区比较并恢复。Checkpoint 按仓库内的文件路径保存，使用独立的本地 Git 引用；改名后的文档从新路径开始记录。

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

[架构与代码入口](ARCHITECTURE.md) · [界面设计](DESIGN.md) · [更新日志](CHANGELOG.md) · [验收记录](docs/acceptance.md) · [MIT 许可证](LICENSE)
