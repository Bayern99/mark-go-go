# UI source, style references and credits

[中文](ACKNOWLEDGMENTS.md)

## Native Portrait UI

MyGo Native UI provides the native window host and widget foundation. Mark Go Go draws its workspace shell, Markdown editor, GenUI reader and Feynman panel in pure Go.

- [Portrait design system](internal/portrait/) · [Window shell, Settings Center, tabs and navigation](internal/nativeui/) · [Markdown Preview and source editing](internal/nativeeditor/) · [GenUI reader](internal/nativegenui/) · [Feynman research panel](internal/nativefeynman/) · [Proposal review](internal/nativereview/)
- Inter, Inter Display, IBM Plex Sans SC and IBM Plex Mono, together with Phosphor icons, ship bundled with the native app. [Fonts and their licences](internal/portrait/fonts/)

## Design and style references

The following projects and design systems directly shaped Mark Go Go's visual direction, spatial layout and interaction details:

- **Arc Browser**: Inspired the coloured Scenery framing an opaque paper Page, vertical document tabs in the sidebar, per-workspace Spaces, the `⌘K`/`⌘T` Command Bar, split-screen second paper sheet and hidden macOS title bar, informed as well by community Arc design guides (`nexu-io/open-design`, `shadcn.io`, and Blake Crosley's Arc design analysis). [Decision record](docs/adr/0013-arc-experiment-scenery-page-command-bar.md)
- **coss ui (`cosscom/ui`)** (MIT): Provided the component semantics, state matrix and three-tier desktop control scale (22 / 28 / 32 pt) blueprint for Portrait 2.0, reimplemented in pure Go native widgets. [Upstream](https://github.com/cosscom/ui) · [Decision record](docs/adr/0012-portrait-2-coss-reference-and-contracts.md)
- **Chinese traditional colors & Wu Xing 742 palette**: Named colours for the eight Wu Xing Spaces (16 Light/Dark palettes) are drawn from the Wu Xing 742 traditional colour reference (reorganized from traditional colour names in `nevertoday/zhongguo-traditional-colors`; no GPL-3.0 code or assets are imported), rendered with Oklab gradients and verified automatically against WCAG AA contrast. [Decision record](docs/adr/0014-wuxing-spaces-per-space-accent-and-dark-mode.md)
- **Emil Kowalski, Radix Colors and Apple's Human Interface Guidelines**: Informed motion restraint, accessible focus rings, hairline borders, Light/Dark surface elevation and native macOS window conventions.
- **SoloMD**: Provided an interaction reference for in-place Markdown editing and separating rendered presentation from byte-preserving saves. [Decision record](docs/adr/0006-preserved-blocks-and-semantic-colors.md)
- **OpenUI Lang (`thesysdev/openui`)** (MIT): Informed the GenUI v3 statement syntax and conformance test vectors. [Upstream](https://github.com/thesysdev/openui) · [Decision record](docs/adr/0011-openui-lang-statements-in-a2ui.md) · [Test vector provenance](internal/openuilang/testdata/SOURCE.md)
- **Onevra Chat**, the **MyGo Markdown chat** example, **Pi P2** and **GoRex**: Informed the research panel, composer, tool details, sidebar actions and terminal interactions. [Onevra Chat](https://github.com/oneo-me/onevra/tree/main/source/components/chat) · [MyGo example](https://github.com/egoist/mygo/tree/v0.10.0/examples/markdown-chat) · [Pi sidebar](https://github.com/earendil-works/pi/blob/main/apps/p2/src/web/SidePanel.tsx) · [GoRex](https://github.com/egoist/gorex)
- **Awesome MyGo**: Helped us explore the MyGo ecosystem. [Awesome MyGo](https://github.com/yldm-tech/awesome-mygo)

## Software and assets

- **MyGo** by Egoist and contributors provides the native desktop host and UI toolkit. The repository includes its MIT-licensed v0.10.0 source with document group and workspace lifecycle patches. [Upstream](https://github.com/egoist/mygo/tree/v0.10.0) · [MyGo licence](third_party/mygo/LICENSE) · [Patch notes](docs/mygo-document-groups.md)
- Native Markdown, math and Mermaid diagram features use the Go modules declared in [go.mod](go.mod). The Feynman plugin connects directly to the local **Feynman** CLI (an MIT-licensed AI research agent by Companion, built on Pi and alphaXiv, [feynman.is](https://feynman.is)). GenUI files use an A2UI 1.0 envelope with OpenUI Lang 1.0 statements as documented in the [format guide](docs/genui/view-format.md).
- **Inter** and **IBM Plex** use the SIL Open Font License. **Phosphor Icons** uses the MIT licence. [Font and icon licences](internal/portrait/fonts/) · [Application MIT licence](LICENSE)
- Licence records for bundled Go dependencies, math fonts and Unicode CLDR live in [resources/licenses](resources/licenses/). MyGo's locale tables are generated from **Unicode CLDR 48.2.0**. [CLDR data](https://github.com/unicode-org/cldr-json/tree/48.2.0) · [Local licence](resources/licenses/Unicode-3.0-CLDR.txt)
