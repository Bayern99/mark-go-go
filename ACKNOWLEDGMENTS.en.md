# UI source and credits

[中文](ACKNOWLEDGMENTS.md)

## Native Portrait UI

MyGo Native UI provides the window and controls. Go code draws the workspace, editor, GenUI reader and Feynman panel.

- [Window shell, tabs and navigation](internal/nativeui/) · [Markdown Preview and Markdown editing](internal/nativeeditor/) · [GenUI reader](internal/nativegenui/)
- IBM Plex Sans, Sans SC and Mono, Barlow Semi Condensed, and Phosphor icons ship with the native app. [Fonts and their licenses](internal/nativeui/fonts/) · [Portrait styling](internal/nativeui/portrait.go)

## Design and interaction references

- **Tactile Design** informed the review of selection, focus, borders and controls. Its authors also credit Emil Kowalski, Radix Colors and Apple's Human Interface Guidelines. The skill is licensed under MIT. [Upstream](https://github.com/kosta4a/tactile-design) · [Local licence](.devin/skills/tactile-design/LICENSE)
- **SoloMD** provided an interaction reference for in-place Markdown editing and keeping rendering separate from saving. [Decision record](docs/adr/0006-preserved-blocks-and-semantic-colors.md)
- **Onevra Chat**, the **MyGo Markdown chat** example, **Pi P2** and **GoRex** informed the research panel, composer, tool details, sidebar actions and terminal interactions. [Onevra Chat](https://github.com/oneo-me/onevra/tree/main/source/components/chat) · [MyGo example](https://github.com/egoist/mygo/tree/v0.10.0/examples/markdown-chat) · [Pi sidebar](https://github.com/earendil-works/pi/blob/main/apps/p2/src/web/SidePanel.tsx) · [GoRex](https://github.com/egoist/gorex)
- **Awesome MyGo** helped us explore the MyGo ecosystem. [Awesome MyGo](https://github.com/yldm-tech/awesome-mygo)

## Software and assets

- **MyGo** by Egoist and contributors provides the native desktop host and UI toolkit. The repository includes its MIT-licensed v0.10.0 source with document group and workspace lifecycle patches. [Upstream](https://github.com/egoist/mygo/tree/v0.10.0) · [MyGo licence](third_party/mygo/LICENSE) · [Patch notes](docs/mygo-document-groups.md)
- Native Markdown and diagram features use the Go modules listed in [go.mod](go.mod). The Feynman plugin connects directly to the installed **Feynman** CLI. GenUI files follow the A2UI protocol documented in the [format guide](docs/genui/view-format.md).
- **IBM Plex** and **Barlow Semi Condensed** use the SIL Open Font License. **Phosphor Icons** uses the MIT licence. [Font and icon licences](internal/nativeui/fonts/) · [Application MIT licence](LICENSE)
- Licence records for bundled Go dependencies, math fonts and Unicode CLDR are in [resources/licenses](resources/licenses/). MyGo's locale tables are generated from **Unicode CLDR 48.2.0**. [CLDR data](https://github.com/unicode-org/cldr-json/tree/48.2.0) · [Local licence](resources/licenses/Unicode-3.0-CLDR.txt)
