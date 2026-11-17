# UI source and credits / 界面源码与致谢

## Portrait UI / Portrait 界面

The Portrait design boards, tokens, and implementation are in this repository. The browser preview runs the interface with sample documents and an in-memory host. See [development setup](README.md#development).

Portrait 的设计画板、样式变量和界面实现都在仓库里。浏览器预览使用样例原文和内存宿主，运行方式见[开发说明](README.md#development)。

- [Design boards / 设计画板](docs/design/Mark%20Go%20Go%20界面设计.html) · [UI rules / 界面规则](docs/agents/ui-tasks.md)
- [Portrait tokens / Portrait 样式变量](web/src/theme/portrait-tokens.ts) · [App styles / 应用样式](web/src/styles.css) · [Rich text styles / Rich text 样式](web/src/editor/crepe-theme.css)
- [Workspace and document tabs / 工作区与文档标签](web/src/App.tsx) · [Shared components / 公用组件](web/src/components/) · [GenUI views and components / GenUI 视图与组件](web/src/plugins/genui/)

## Design and interaction references / 设计与交互参考

- **Tactile Design** informed the review of selection, focus, borders, and controls. Its authors also credit Emil Kowalski, Radix Colors, and Apple's Human Interface Guidelines. Mark Go Go includes a copy of the skill under MIT. / Tactile Design 为选中、焦点、边线与控件的审查提供了参考。其作者也感谢 Emil Kowalski、Radix Colors 和 Apple Human Interface Guidelines。仓库保留这份技能及其 MIT 许可证。[Upstream / 上游](https://github.com/kosta4a/tactile-design) · [Local license / 随附许可证](.devin/skills/tactile-design/LICENSE)
- **SoloMD** provided an interaction reference for in-place Markdown editing and for keeping rendering separate from saving the document. / SoloMD 为原地 Markdown 编辑和分开展示与保存提供了交互参考。[Reference and revision / 参考与版本](docs/adr/0006-preserved-blocks-and-semantic-colors.md)
- **Awesome MyGo** helped us explore the MyGo ecosystem. The UI planning notes also record the component libraries evaluated. / Awesome MyGo 帮我们了解 MyGo 生态。界面规划记录了评估过的组件库。[Awesome MyGo](https://github.com/yldm-tech/awesome-mygo) · [UI planning notes / 界面规划](docs/design/ui-component-gaps.md)

## Software and assets / 软件与资源

- **MyGo** by Egoist and contributors provides the native desktop host. The repository includes v0.7.4 with document group and workspace lifecycle patches under MIT. Its CLI's pinyin data comes from Unicode Unihan 16.0.0. / Egoist 与贡献者的 MyGo 提供原生桌面宿主。仓库包含 MIT 许可的 v0.7.4 代码，以及文档组和工作区生命周期补丁。CLI 的拼音数据来自 Unicode Unihan 16.0.0。[Upstream / 上游](https://github.com/egoist/mygo/tree/v0.7.4) · [MyGo license / MyGo 许可证](third_party/mygo/LICENSE) · [Unicode notice / Unicode 声明](third_party/mygo/cmd/mygo/LICENSE-Unicode.txt) · [Patch notes / 补丁说明](docs/mygo-document-groups.md)
- **Milkdown Crepe and ProseMirror** power Rich text, and **CodeMirror** powers the Markdown view. **Radix Primitives** provides controls such as tooltips and menus, **OverlayScrollbars** provides scroll areas, **Mermaid** renders diagrams locally, and **A2UI** provides the GenUI component protocol. / Milkdown Crepe 与 ProseMirror 提供 Rich text 编辑器，CodeMirror 提供 Markdown 视图。Radix Primitives 提供提示和菜单等基础控件，OverlayScrollbars 提供滚动区域，Mermaid 在本地呈现图表，A2UI 提供 GenUI 组件协议。[Frontend dependencies / 前端依赖](web/package.json)
- **ProseMirror Tables** by Marijn Haverbeke and contributors provides the Rich text table model and selection. / Marijn Haverbeke 与贡献者的 ProseMirror Tables 提供 Rich text 的表格模型和选区。[Upstream / 上游](https://github.com/ProseMirror/prosemirror-tables) · [Local license / 随附许可证](web/public/licenses/LICENSE-ProseMirror-Tables.txt)
- **IBM Plex Sans, Sans SC and Mono** and **Barlow Semi Condensed** ship under the SIL Open Font License. `MGG Condensed` is the CSS name used for the bundled Barlow files with adjusted display metrics. / 随应用提供的 IBM Plex Sans、Sans SC、Mono 和 Barlow Semi Condensed 采用 SIL Open Font License。`MGG Condensed` 是同一组 Barlow 字体在 CSS 中应用尺寸调整后的名称。[IBM Plex](https://github.com/IBM/plex) · [Barlow](https://github.com/jpt/barlow) · [Font licenses / 字体许可证](web/src/assets/licenses/)
- **Phosphor Icons** by Helena Zhang and Tobias Fried provides the bundled UI icon font under MIT. / 随应用提供的 UI 图标字体来自 Helena Zhang 和 Tobias Fried 的 Phosphor Icons，采用 MIT 许可。[Phosphor Icons](https://phosphoricons.com/) · [Icon license / 图标许可证](web/src/assets/licenses/LICENSE-Phosphor.txt)

A production build generates `licenses/Frontend-Licenses.md` alongside the bundled asset notices. The desktop app also carries the MyGo, purego, Go library, and Unicode CLDR notices in its application resources. / 生产构建会生成 `licenses/Frontend-Licenses.md`，与随包资源的许可声明一起分发。桌面应用的资源目录也包含 MyGo、purego、Go 库和 Unicode CLDR 的许可声明。[Frontend notices / 前端声明](web/public/licenses/README.md) · [Desktop notices / 桌面端声明](resources/licenses/)

MyGo uses localization tables generated from **Unicode CLDR 48.2.0**. / MyGo 的本地化表由 Unicode CLDR 48.2.0 生成。[CLDR data / CLDR 数据](https://github.com/unicode-org/cldr-json/tree/48.2.0) · [Local license / 随附许可证](resources/licenses/Unicode-3.0-CLDR.txt)
