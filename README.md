# Mark Go Go

[English](README.en.md) · [致谢与许可](ACKNOWLEDGMENTS.md)

Mark Go Go 是 macOS 本地 Markdown 编辑器。多个文件夹工作区共享窗口和文档标签，`Ctrl+Tab` 在最近的文档间往返。按 `⌘/` 在可编辑的预览和 Markdown 间切换。切到中央的 **GenUI**，在视图右上角打开 **GenUI settings**，即可选择模型、生成或更新阅读视图。

## 可选插件

左栏的 **Workspace / Outline** 可切换工作区文件和当前文档大纲。桌面端在文档标签栏的 **Plugins** 菜单中分别启用 Feynman 和 Git。两项初始均关闭。

**Feynman** 使用本机的 Feynman CLI 和 Node.js。通过 `npm install -g pi-acp` 安装适配器，再启用插件。Feynman 占用整个右栏，同一工作区切换文档时对话和输入草稿会保留。拖动分隔线可调整宽度，窄窗使用抽屉。[Feynman 设置指南](https://feynman.is/docs/getting-started/setup)

- **Chat**。点 **Actions** 按名称查找工作流、Skills、扩展和会话操作。**Browse research tools** 列出本机工具，选择后可以查看参数或把工具带入当前问题。扩展要求的选择、确认和文字输入直接在对话中完成。选择后填写主题或指令并发送，也支持输入 `/` 搜索。Enter 发送，Shift+Enter 换行。可以选择模型和思考级别、附上论文或图片、展开工具记录、打开引用链接，以及从历史菜单新建或恢复本工作区的对话。
- **Files**。报告、计划、草稿和来源记录作为普通 Markdown 文档打开。文件夹按钮可查看研究过程中产生的其他格式。要修改当前文档，选中 **Edit document**，审阅差异后点击 **Apply in Markdown**；这次修改可一次撤销。
- **CLI**。在右栏直接运行原版 Feynman。留空参数并点 **Run** 进入交互界面，续接当前已保存的对话，也可以直接运行 `setup`、`doctor`、`model login` 或 `mcp list` 等命令。离开 CLI 页签后仍可返回同一个运行中的进程。正常退出时发布本轮完成的 Markdown，**Stop CLI** 中止运行。CLI 与 Chat 轮流使用同一研究工作目录。

**Git** 使用现有本地仓库。先保存当前文档，再点文档工具栏的 **Git checkpoints** 创建 Checkpoint。选取版本即可与当前缓冲区比较并恢复。Checkpoint 按仓库内的文件路径保存，使用独立的本地 Git 引用；改名后的文档从新路径开始记录。

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
