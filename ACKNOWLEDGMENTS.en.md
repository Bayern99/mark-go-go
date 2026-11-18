# UI source and credits

[中文](ACKNOWLEDGMENTS.md)

## Portrait UI

The Portrait design boards, tokens, and implementation are in this repository. The browser preview runs the interface with sample documents and an in-memory host. See [development setup](README.en.md#development).

- [Design boards](docs/design/Mark%20Go%20Go%20界面设计.html) · [UI rules](docs/agents/ui-tasks.md)
- [Portrait tokens](web/src/theme/portrait-tokens.ts) · [App styles](web/src/styles.css) · [Markdown Preview styles](web/src/editor/crepe-theme.css)
- [Workspace and document tabs](web/src/App.tsx) · [Shared components](web/src/components/) · [GenUI views and components](web/src/plugins/genui/)

## Design and interaction references

- **Tactile Design** informed the review of selection, focus, borders, and controls. Its authors also credit Emil Kowalski, Radix Colors, and Apple's Human Interface Guidelines. Mark Go Go includes a copy of the skill under MIT. [Upstream](https://github.com/kosta4a/tactile-design) · [Local license](.devin/skills/tactile-design/LICENSE)
- **SoloMD** provided an interaction reference for in-place Markdown editing and for keeping rendering separate from saving the document. [Reference and revision](docs/adr/0006-preserved-blocks-and-semantic-colors.md)
- **Awesome MyGo** helped us explore the MyGo ecosystem. The UI planning notes also record the component libraries evaluated. [Awesome MyGo](https://github.com/yldm-tech/awesome-mygo) · [UI planning notes](docs/design/ui-component-gaps.md)

## Software and assets

- **MyGo** by Egoist and contributors provides the native desktop host. The repository includes v0.10.0 with document group and workspace lifecycle patches under MIT. Its CLI's pinyin data comes from Unicode Unihan 16.0.0. [Upstream](https://github.com/egoist/mygo/tree/v0.10.0) · [MyGo license](third_party/mygo/LICENSE) · [Unicode notice](third_party/mygo/cmd/mygo/LICENSE-Unicode.txt) · [Patch notes](docs/mygo-document-groups.md)
- **Milkdown Crepe and ProseMirror** power Markdown Preview, and **CodeMirror** powers the Markdown view. **Radix Primitives** provides controls such as tooltips and menus, **OverlayScrollbars** provides scroll areas, **Mermaid** renders diagrams locally, and **A2UI** provides the GenUI component protocol. [Frontend dependencies](web/package.json)
- **ProseMirror Tables** by Marijn Haverbeke and contributors provides the table model in Preview and selection. [Upstream](https://github.com/ProseMirror/prosemirror-tables) · [Local license](web/public/licenses/LICENSE-ProseMirror-Tables.txt)
- **IBM Plex Sans, Sans SC and Mono** and **Barlow Semi Condensed** ship under the SIL Open Font License. `MGG Condensed` is the CSS name used for the bundled Barlow files with adjusted display metrics. [IBM Plex](https://github.com/IBM/plex) · [Barlow](https://github.com/jpt/barlow) · [Font licenses](web/src/assets/licenses/)
- **Phosphor Icons** by Helena Zhang and Tobias Fried provides the bundled UI icon font under MIT. [Phosphor Icons](https://phosphoricons.com/) · [Icon license](web/src/assets/licenses/LICENSE-Phosphor.txt)

A production build generates `licenses/Frontend-Licenses.md` alongside the bundled asset notices. The desktop app also carries the MyGo, purego, Go library, and Unicode CLDR notices in its application resources. [Frontend notices](web/public/licenses/README.md) · [Desktop notices](resources/licenses/)

MyGo uses localization tables generated from **Unicode CLDR 48.2.0**. [CLDR data](https://github.com/unicode-org/cldr-json/tree/48.2.0) · [Local license](resources/licenses/Unicode-3.0-CLDR.txt)
