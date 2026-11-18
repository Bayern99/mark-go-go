# 界面源码与致谢

[English](ACKNOWLEDGMENTS.en.md)

## Portrait 界面

Portrait 的设计画板、样式变量和界面实现都在仓库里。浏览器预览使用样例原文和内存宿主，运行方式见[开发说明](README.md#开发)。

- [设计画板](docs/design/Mark%20Go%20Go%20界面设计.html) · [界面规则](docs/agents/ui-tasks.md)
- [Portrait 样式变量](web/src/theme/portrait-tokens.ts) · [应用样式](web/src/styles.css) · [Markdown 预览样式](web/src/editor/crepe-theme.css)
- [工作区与文档标签](web/src/App.tsx) · [公用组件](web/src/components/) · [GenUI 视图与组件](web/src/plugins/genui/)

## 设计与交互参考

- **Tactile Design** 为选中、焦点、边线与控件的审查提供了参考。其作者也感谢 Emil Kowalski、Radix Colors 和 Apple Human Interface Guidelines。仓库保留这份技能及其 MIT 许可证。[上游](https://github.com/kosta4a/tactile-design) · [随附许可证](.devin/skills/tactile-design/LICENSE)
- **SoloMD** 为原地 Markdown 编辑和分开展示与保存提供了交互参考。[参考与版本](docs/adr/0006-preserved-blocks-and-semantic-colors.md)
- **Awesome MyGo** 帮我们了解 MyGo 生态。界面规划记录了评估过的组件库。[Awesome MyGo](https://github.com/yldm-tech/awesome-mygo) · [界面规划](docs/design/ui-component-gaps.md)

## 软件与资源

- Egoist 与贡献者的 **MyGo** 提供原生桌面宿主。仓库包含 MIT 许可的 v0.10.0 代码，以及文档组和工作区生命周期补丁。命令行工具的拼音数据来自 Unicode Unihan 16.0.0。[上游](https://github.com/egoist/mygo/tree/v0.10.0) · [MyGo 许可证](third_party/mygo/LICENSE) · [Unicode 声明](third_party/mygo/cmd/mygo/LICENSE-Unicode.txt) · [补丁说明](docs/mygo-document-groups.md)
- **Milkdown Crepe** 与 **ProseMirror** 提供可编辑的 Markdown 预览，**CodeMirror** 提供 Markdown 视图。**Radix Primitives** 提供提示和菜单等基础控件，**OverlayScrollbars** 提供滚动区域，**Mermaid** 在本地呈现图表，**A2UI** 提供 GenUI 组件协议。[前端依赖](web/package.json)
- Marijn Haverbeke 与贡献者的 **ProseMirror Tables** 提供预览中的表格模型和选区。[上游](https://github.com/ProseMirror/prosemirror-tables) · [随附许可证](web/public/licenses/LICENSE-ProseMirror-Tables.txt)
- 随应用提供的 **IBM Plex Sans、Sans SC、Mono** 和 **Barlow Semi Condensed** 采用 SIL 开放字体许可证。`MGG Condensed` 是同一组 Barlow 字体在 CSS 中应用尺寸调整后的名称。[IBM Plex](https://github.com/IBM/plex) · [Barlow](https://github.com/jpt/barlow) · [字体许可证](web/src/assets/licenses/)
- 随应用提供的界面图标字体来自 Helena Zhang 和 Tobias Fried 的 **Phosphor Icons**，采用 MIT 许可。[Phosphor Icons](https://phosphoricons.com/) · [图标许可证](web/src/assets/licenses/LICENSE-Phosphor.txt)

生产构建会生成 `licenses/Frontend-Licenses.md`，与随包资源的许可声明一起分发。桌面应用的资源目录也包含 MyGo、purego、Go 库和 Unicode CLDR 的许可声明。[前端声明](web/public/licenses/README.md) · [桌面端声明](resources/licenses/)

MyGo 的本地化表由 **Unicode CLDR 48.2.0** 生成。[CLDR 数据](https://github.com/unicode-org/cldr-json/tree/48.2.0) · [随附许可证](resources/licenses/Unicode-3.0-CLDR.txt)
