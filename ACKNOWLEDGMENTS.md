# 界面源码与致谢

[English](ACKNOWLEDGMENTS.en.md)

## 原生 Portrait 界面

MyGo Native UI 提供窗口和控件基础，应用以 Go 代码绘制工作区、编辑器、GenUI 与 Feynman 面板。

- [窗口外壳、标签与导航](internal/nativeui/) · [Markdown Preview 与 Markdown 编辑](internal/nativeeditor/) · [GenUI 阅读视图](internal/nativegenui/)
- Inter、Inter Display、IBM Plex Sans SC 与 IBM Plex Mono 字体，以及 Phosphor 图标随原生应用打包。[字体与随附许可证](internal/portrait/fonts/) · [Portrait 样式](internal/nativeui/portrait.go)

## 设计与交互参考

- **Tactile Design** 为选中、焦点、边线与控件的审查提供了参考。其作者也感谢 Emil Kowalski、Radix Colors 和 Apple Human Interface Guidelines。该技能采用 MIT 许可。[上游](https://github.com/kosta4a/tactile-design) · [随附许可证](.devin/skills/tactile-design/LICENSE)
- **SoloMD** 为原地 Markdown 编辑和分开展示与保存提供了交互参考。[决策记录](docs/adr/0006-preserved-blocks-and-semantic-colors.md)
- **Onevra Chat**、**MyGo Markdown chat**、**Pi P2** 与 **GoRex** 为研究面板、输入、工具详情、侧栏操作和终端交互提供了参考。[Onevra Chat](https://github.com/oneo-me/onevra/tree/main/source/components/chat) · [MyGo 示例](https://github.com/egoist/mygo/tree/v0.10.0/examples/markdown-chat) · [Pi 侧栏](https://github.com/earendil-works/pi/blob/main/apps/p2/src/web/SidePanel.tsx) · [GoRex](https://github.com/egoist/gorex)
- **Awesome MyGo** 帮我们了解 MyGo 生态。[Awesome MyGo](https://github.com/yldm-tech/awesome-mygo)

## 软件与资源

- Egoist 与贡献者的 **MyGo** 提供原生桌面宿主和 UI 工具包。仓库包含 MIT 许可的 v0.10.0 代码，以及文档组和工作区生命周期补丁。[上游](https://github.com/egoist/mygo/tree/v0.10.0) · [MyGo 许可证](third_party/mygo/LICENSE) · [补丁说明](docs/mygo-document-groups.md)
- 原生 Markdown 与图表功能使用 [go.mod](go.mod) 中声明的 Go 模块。Feynman 插件直接连接本机的 **Feynman** CLI；GenUI 文件遵循保存于 [格式说明](docs/genui/view-format.md) 的 A2UI 协议。
- **Inter** 与 **IBM Plex** 采用 SIL Open Font License，**Phosphor Icons** 采用 MIT 许可证。[字体和图标许可证](internal/portrait/fonts/) · [应用 MIT 许可证](LICENSE)
- 随附资源中的 Go 依赖、数学字体与 Unicode CLDR 许可记录位于 [resources/licenses](resources/licenses/)。MyGo 的本地化表由 **Unicode CLDR 48.2.0** 生成。[CLDR 数据](https://github.com/unicode-org/cldr-json/tree/48.2.0) · [随附许可证](resources/licenses/Unicode-3.0-CLDR.txt)
