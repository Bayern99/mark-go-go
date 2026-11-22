# Mark Go Go

[English](README.en.md) · [致谢与许可](ACKNOWLEDGMENTS.md)

Mark Go Go 是 macOS 本地 Markdown 编辑器。多个文件夹工作区共享窗口和文档标签，`Ctrl+Tab` 在最近的文档间往返。按 `⌘/` 在可编辑的预览和 Markdown 间切换。切到中央的 **GenUI**，在视图右上角打开 **Generation settings**，即可选择模型、生成或更新阅读视图。

## 可选插件

左栏的 **Workspace / Outline** 可切换工作区文件和当前文档大纲。桌面端在文档标签栏的 **Plugins** 菜单中分别启用 Feynman 和 Git。两项初始均关闭。

**Feynman** 直接使用本机 Feynman CLI 的 RPC 模式。安装 Feynman 并启用插件，如需登录可在应用的 Feynman 设置中操作。右侧为英文 Portrait 面板，同一工作区切换文档时对话和输入草稿会保留。拖动分隔线可调整宽度，窄窗使用抽屉。[Feynman 设置指南](https://feynman.is/docs/getting-started/setup)

Mark Go Go 将 `~/.feynman` 中已有的登录凭据、模型、记忆、MCP 配置、技能、提示、扩展、包和会话等文件与目录链接到应用配置。写入已链接的会话或在设置中修改已链接的全局 MCP 配置，普通 CLI 也能看到。每次启动 Feynman 前都会补充缺失的链接，不覆盖应用配置中已有的内容。因此，新建的状态文件以及与已有私有文件同名的内容可能与全局分开。Feynman 仍可使用用户级 Pi 资源，但应用的项目信任文件独立、默认拒绝，只批准应用托管的工作目录，不修改全局信任。Windows 使用符号链接，需开启开发者模式或具备创建符号链接的权限；链接失败时不会改用复制。Windows 运行时尚未实测。

- **Chat**。点 **Actions** 按名称查找工作流、Skills、扩展和会话操作。**Browse research tools** 列出本机工具，选择后可以查看参数或把工具带入当前问题。扩展要求的选择、确认和文字输入直接在对话中完成。选择后填写主题或指令并发送，也支持输入 `/` 搜索。Enter 发送，Shift+Enter 换行。可以选择模型和思考级别、附上论文或图片、展开工具记录、打开引用链接，以及从历史菜单新建或恢复本工作区的对话。
- **Review**。审阅提案差异后可将改动应用到 Markdown；这次修改可一次撤销。已完成的报告、计划和草稿可从 **Research files** 在普通文档标签中打开。
- **Feynman settings**。从齿轮按钮查看状态并管理模型与 MCP 服务。交互式设置和登录在系统终端中打开。

**Git** 检查点按仓库内的文件路径保存为独立的本地 Git 引用。不在 Git 仓库中时，应用为每篇文档保留最多 50 个本地快照；之后运行 `git init` 仍可访问这些快照。保存文档后，从文档工具栏的 **Checkpoints** 创建快照，或比较并恢复旧版本。改名后的文档从新路径开始记录。

## 开发

原生应用使用 [go.mod](go.mod) 指定的 Go 版本。

```sh
go tool mygo dev
go tool mygo build
```

预览、Markdown、GenUI 和工作区界面由 MyGo Native UI 绘制。26 份 Markdown 语料和共享章节向量由 Go 回归测试直接载入；格式与协议示例见 [GenUI 格式](docs/genui/view-format.md)。

## 验证

```sh
scripts/verify.sh
```

脚本运行 Go 格式、vet、MyGo vet、竞态测试及 Windows amd64 交叉编译。macOS 原生窗口、菜单和重启使用 [原生 Automation 与 macOS 验证流程](docs/agents/native-automation.md)。历史 WebKit 基线及同机对照方法见 [Portrait 性能记录](docs/agents/portrait-performance.md)。

[架构与代码入口](ARCHITECTURE.md) · [界面设计](DESIGN.md) · [更新日志](CHANGELOG.md) · [验收记录](docs/acceptance.md) · [MIT 许可证](LICENSE)
