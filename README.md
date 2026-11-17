# Mark Go Go

[English](README.en.md) · [开发](#开发) · [界面源码与致谢](ACKNOWLEDGMENTS.md)

Mark Go Go 是 macOS 上的本地 Markdown 编辑器。打开文件夹即可建立工作区，在 Markdown 预览中直接编辑渲染后的文档，或在 Markdown 模式中编辑原文。也可以切换到由原文派生的 GenUI 阅读视图。

- 工作区窗口默认大小为 1280 × 820，包含文件树和文档标签。可搜索路径、标题、摘要和章节标题，搜索也会反映已打开原文中的改动。重启后恢复上次的工作区、标签和视图模式。
- 按 `⌘/` 在 `Preview` 和 `Markdown` 之间切换。预览逐字节保留未编辑的块，本地呈现 Mermaid 图表，并支持原地编辑特殊块。
- GenUI 是从 Markdown 原文派生的只读视图。卡片可跳转到对应的原文位置。右栏放置大纲和 GenUI 生成控件，生成由外部 ACP 代理执行。
- 三栏统一使用 Portrait 浅色界面，字体与图标随应用提供。

## 开发

开发需要 Go 1.27.3 和 Node 24。

```sh
npm --prefix web ci
npx --prefix web playwright install chromium-headless-shell

# 桌面开发
go tool mygo dev

# 使用内存宿主预览界面
npm --prefix web run dev

# Go 接口变更后更新绑定
go tool mygo generate
```

## 验证与打包

日常运行覆盖改动的用例。涉及文件、进程通信或窗口生命周期时，扩大验证范围。

```sh
# 定向检查
go test . -run 'Test.*(Close|Quit)'
npm --prefix web test -- test/workspace-restore.test.tsx

# Go 绑定变更后
go tool mygo generate
npm --prefix web run typecheck

# 集成检查
npm --prefix web test
go vet .
go test -race .

# 发布构建
go tool mygo build
```

`go tool mygo doctor` 检查开发环境，可在初次配置或排查构建问题时运行。`go tool mygo build` 生成绑定，检查并打包前端，然后将应用和 DMG 写入 `build/`。

修改仓库内 MyGo 的文档组时，运行 `(cd third_party/mygo && go test -race . -run TestGroupClose)`。

原生检查使用 `go tool mygo dev` 启动具有独立标识的开发版。可从 `web/test/fixtures/` 复制样文到临时工作区。验证打包版时，给临时项目副本单独的应用名和包标识，再运行 `go tool mygo build -skip-dmg`。浏览器测试覆盖页面行为，菜单、保存对话框和输入法通过 macOS 应用验证。

仓库内的 MyGo 省略了自动化插件。做定向检查时，可以将 v0.7.4 插件复制到临时项目的依赖目录，在带 `e2e` 构建标签的 Go 文件中导入。插件可以列出窗口、检查网页视图和调用应用菜单。系统对话框通过原生界面操作。[MyGo 自动化指南](https://github.com/egoist/mygo/blob/v0.7.4/docs/plugins/automation.md)

## 项目文档

- [更新日志](CHANGELOG.md)
- [验收记录](docs/acceptance.md)
- [架构](ARCHITECTURE.md)与[架构决策](docs/adr/)
- [接口](docs/interfaces.md)与[工作区契约](docs/workspace-contract.md)
- [MyGo 文档组补丁](docs/mygo-document-groups.md)
- [Portrait 设计画板](docs/design/Mark%20Go%20Go%20界面设计.html)
- [界面组件缺口与排版规划](docs/design/ui-component-gaps.md)

## 许可证

Mark Go Go 的原创代码和 Portrait 界面设计文件采用 [MIT 许可证](LICENSE)。随附的第三方代码和资源采用各自的许可，见[界面源码与致谢](ACKNOWLEDGMENTS.md)。
