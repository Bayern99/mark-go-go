# 界面源码、风格参考与致谢

[English](ACKNOWLEDGMENTS.en.md)

## 原生 Portrait 界面

MyGo Native UI 提供窗口与控件基础，应用以纯 Go 代码绘制工作区、编辑器、GenUI 与 Feynman 面板。

- [Portrait 设计系统](internal/portrait/) · [窗口外壳、设置中心、标签与导航](internal/nativeui/) · [Markdown Preview 与源码编辑](internal/nativeeditor/) · [GenUI 阅读视图](internal/nativegenui/) · [Feynman 研究面板](internal/nativefeynman/) · [提案审阅](internal/nativereview/)
- Inter、Inter Display、IBM Plex Sans SC 与 IBM Plex Mono 字体，以及 Phosphor 图标随原生应用打包。[字体与随附许可证](internal/portrait/fonts/)

## 设计与风格参考

以下开源项目与设计体系为 Mark Go Go 的视觉风格、空间结构与交互细节提供了直接参考：

- **Arc Browser**：启发了彩色布景（Scenery）承载不透明纸页（Page）、侧栏竖排文档标签、按工作区区分的 Space、全局 `⌘K`/`⌘T` 命令栏、宽窗双纸页分屏以及 macOS 隐藏标题栏布局，同时参考了社区整理的 Arc 设计指南（`nexu-io/open-design`、`shadcn.io` 与 Blake Crosley 的设计解析）。[决策记录](docs/adr/0013-arc-experiment-scenery-page-command-bar.md)
- **coss ui（`cosscom/ui`）**：采用 MIT 许可，为 Portrait 2.0 提供了组件语义、状态矩阵与桌面三档控件阶梯（22 / 28 / 32 pt）蓝图，由本项目以纯 Go 原生控件重新实现。[上游](https://github.com/cosscom/ui) · [决策记录](docs/adr/0012-portrait-2-coss-reference-and-contracts.md)
- **中国传统色与五行色系**：八个五行 Space（浅色与深色共 16 套配色）的具名色值整理自五行 742 色表（源自 `nevertoday/zhongguo-traditional-colors` 的传统色名整理；未引入任何 GPL-3.0 源码或资源），全部经由 Oklab 渐变与 WCAG AA 对比度自动化验算。[决策记录](docs/adr/0014-wuxing-spaces-per-space-accent-and-dark-mode.md)
- **Emil Kowalski、Radix Colors 与 Apple Human Interface Guidelines**：为动效克制、无障碍焦点环、发丝线层级、浅深色材质关系与 macOS 原生窗口习惯提供了参考。
- **SoloMD**：为原地 Markdown 编辑以及将渲染展示与磁盘保存解耦提供了交互参考。[决策记录](docs/adr/0006-preserved-blocks-and-semantic-colors.md)
- **OpenUI Lang（`thesysdev/openui`）**：采用 MIT 许可，为 GenUI v3 视图语句语法与一致性测试向量提供了基础。[上游](https://github.com/thesysdev/openui) · [决策记录](docs/adr/0011-openui-lang-statements-in-a2ui.md) · [用例来源](internal/openuilang/testdata/SOURCE.md)
- **Onevra Chat**、**MyGo Markdown chat**、**Pi P2** 与 **GoRex**：为研究面板、输入编排器、工具详情展示、侧栏操作与终端交互提供了参考。[Onevra Chat](https://github.com/oneo-me/onevra/tree/main/source/components/chat) · [MyGo 示例](https://github.com/egoist/mygo/tree/v0.10.0/examples/markdown-chat) · [Pi 侧栏](https://github.com/earendil-works/pi/blob/main/apps/p2/src/web/SidePanel.tsx) · [GoRex](https://github.com/egoist/gorex)
- **Awesome MyGo**：帮助我们了解和探索 MyGo 生态。[Awesome MyGo](https://github.com/yldm-tech/awesome-mygo)

## 软件与资源

- Egoist 与贡献者的 **MyGo** 提供原生桌面宿主和 UI 工具包。仓库包含 MIT 许可的 v0.10.0 源码，以及文档组与工作区生命周期补丁。[上游](https://github.com/egoist/mygo/tree/v0.10.0) · [MyGo 许可证](third_party/mygo/LICENSE) · [补丁说明](docs/mygo-document-groups.md)
- 原生 Markdown、公式与 Mermaid 图表功能使用 [go.mod](go.mod) 中声明的 Go 模块。Feynman 插件直接连接本机安装的 **Feynman** CLI（Companion 开发的 MIT 开源研究代理，基于 Pi 与 alphaXiv 构建，[官网](https://feynman.is)）；GenUI 视图文件采用 A2UI 1.0 信封与 OpenUI Lang 1.0 语句，见 [格式说明](docs/genui/view-format.md)。
- **Inter** 与 **IBM Plex** 采用 SIL Open Font License，**Phosphor Icons** 采用 MIT 许可证。[字体和图标许可证](internal/portrait/fonts/) · [应用 MIT 许可证](LICENSE)
- 随附资源中的 Go 依赖、数学字体与 Unicode CLDR 许可记录位于 [resources/licenses](resources/licenses/)。MyGo 的本地化表由 **Unicode CLDR 48.2.0** 生成。[CLDR 数据](https://github.com/unicode-org/cldr-json/tree/48.2.0) · [随附许可证](resources/licenses/Unicode-3.0-CLDR.txt)
