# ADR 0019 · 统一导出架构（Markdown 与 GenUI）

Status: accepted

## Context

Mark Go Go 是纯 Go + MyGo 原生绘制应用，无主界面 WebView。Markdown 解析使用 `third_party/mygo/ui/markdown/parser`，公式（`go-tex/math`）与 Mermaid（`go-mermaid`，含外链安全过滤）均在 Go 侧本地输出 SVG，代码高亮复用 `mygo/ui/editor` 的 `Tokenizer` 与 Portrait `CodeTheme`；GenUI 视图由 `openuilang.Session.Render()` 输出确定性 `*openuilang.Element` 树；Reader Notes 以 `<!-- note: ... -->` 存在源 Markdown 中并由 `internal/markdown/sections.go` 提取。

导出需求分两类：
- **Markdown 导出（全局文档能力）**：Standalone HTML、PDF，以及可选的 DOCX / PPTX / EPUB / RTF / LaTeX。
- **GenUI 导出（局部视图能力）**：当前视图状态的 Standalone HTML 与 PDF。

目标是在不引入 `goldmark`、不内置重型外部二进制、不新建动态插件运行时的前提下，提供统一、轻量、可离线打开且打印可靠的导出管线。

## Decision

- **统一纯 Go 注册表（`internal/export`）**：建立单一格式注册表，按 `Scope`（`Markdown` / `GenUI`）与 `Engine`（`Builtin` / `Pandoc`）注册格式元数据与渲染器，分包为：
  - `internal/export`：格式元数据、`Options`、注册表与错误类型。
  - `internal/export/theme`：将当前 Wu Xing `portrait.Space` 与明暗 `Appearance` 序列化为静态 CSS 变量与 `@media print` 样式表。
  - `internal/export/assets`：HTML 与 PDF 共用的资源解析器（严格限定在文档目录 `os.Root` 内的本地图片内联 `data:` URI 及单图/总量上限、缺失/越界/超限图片占位；Phosphor 图标在 GenUI 实际用到图标时按需内联一次 `Phosphor.ttf` `@font-face`）。Standalone HTML 中远程图片保留原 URL（导出进程绝不发起外网抓取）；PDF 临时打印 HTML 中屏蔽远程图片并输出离线占位（同时收紧 CSP 至 `img-src data:`），避免隐藏 `WKWebView` 联网或被慢图拖超时；不内联系统字体。
  - `internal/export/markdown`：`parser.Document` → 零 JS Standalone HTML，复用 `go-tex/math`、`go-mermaid`（含 `externalDiagram` 过滤）与 `editor.Tokenizer` + `portrait.CodeTheme`。
  - `internal/export/genui`：`*openuilang.Element`（v3，50 个组件）→ 零 JS Standalone HTML，只取一次当前 `Session.Render()` + `Reading.states` 快照。16 个交互/表单组件静态化为只读态视觉快照；`Flashcard` 同时展开正反面；`Carousel` 展开全部项；`Popover` 附轻注释块；`Chart` 在 Go 侧直出内联 SVG；`Celebration` 忽略。
  - `internal/export/pdf`：通过临时 `WindowOptions{Hidden: true}`（无 `Content`，走 `WKWebView`）加载 Standalone HTML 并调用 `mygo` 的 `(*Page).PrintToPDF`（自动复用 `fixPrintedPDF` 的 macOS CJK CMap 修复）。
  - `internal/export/pandoc`：外部可选 Pandoc 探测、版本校验、Markdown 预处理与子进程执行。
- **入口与零弹窗交互**：
  - Markdown 导出放在 macOS 顶部菜单 `File → Export ▸` 与命令栏（`⌘K`），不增加阅读页顶栏按钮。
  - GenUI 导出放在底部悬浮胶囊 `⋯` 菜单，不放入全局菜单。
  - 采用零弹窗快捷路径：选定格式即弹出原生保存框（`DialogModule.Save`，带 `Parent` sheet 与扩展名过滤）。导出偏好统一持久化到 `portrait.json`（`mgo.export.*`）：高频偏好（`Include Reader Notes`、`Keep Dark Theme for PDF`）放在导出菜单底部勾选项；低频配置（纸张大小、`mgo.export.pandocPath`）放在 Settings / Export 页面。
  - **前置拦截**：缺少 Pandoc 或当前 GenUI 为 v1/v2 遗留视图时，在打开保存框前即提示并终止，不让用户选完路径才失败。
- **主题与打印策略**：
  - Standalone HTML 固化导出瞬间的 Portrait Space 与明暗外观，不使用 `prefers-color-scheme`，保证跨机所见即所得。
  - `@media print` 与 PDF 默认切到同一 Space 的 Light 纸面，并共享同一套 Light token 与静态 print override（`break-inside: avoid`、代码块轻边框等）。
  - 公式、Mermaid 与 `Chart` 输出的 SVG 通过 `currentColor` / CSS 变量驱动前景色与描边，切换 Light 纸面时同步切换代码高亮与矢量颜色，避免深色模式打印出现浅色字。
  - `Keep Dark Theme for PDF` 菜单项在 macOS 13.3+（支持 `PDFOptions.Background`）保持可勾选，并在导出窗口为深色外观时生效；低版本禁用该勾选项并说明原因。
- **Reader Notes（`<!-- note: ... -->`）**：
  - Markdown 与 GenUI **两端均默认关闭**，勾选后记住偏好；关闭时单测强制断言导出产物源码中不残留任何笔记正文。
  - 开启时：Markdown 渲染为行内卡片（紧跟所属段落；仅关联章节时按规范章节身份 `SectionHeading + SectionOccurrence` 放节末，避免同哈希重名章节合并）；GenUI 在文末按规范章节身份（`SectionHeading + SectionOccurrence`）聚合为 `Reader Notes` 附录，并与正文 `Source` 有笔记的目标角标做纯 HTML `#anchor` 双向跳转（无笔记来源渲染为静态标签，避免断链）；Pandoc 通道先移除原 HTML 注释，开启时转为 `::: {.reader-note custom-style="Reader Note"}` fenced div，避免与正文引用混淆。
- **GenUI v1/v2 遗留视图**：Phase 1 仅支持 v3（`openui-lang/1.0`）；v1/v2 保留可见导出入口，点击时在保存框前轻提示仅支持新版 GenUI（v3），不维护第二套布局或摘要降级渲染器。
- **PDF 打印可靠性硬约束**：
  - `PrintToPDF` 在 goroutine 中执行，禁止在主线程菜单回调直接阻塞调用（规避 `mygo` 内部向主线程派发并 `await` 导致的死锁）。
  - 进程级互斥锁串行执行 PDF 打印；隐藏窗口设固定尺寸并带超时与 `defer Close()` 销毁兜底。
  - 打印前由 Go 侧探针确认 `document.fonts.ready` 与图片解码完成；探针仅作用于临时打印流程，绝不写入保存给用户的零 JS HTML。
- **Pandoc 探测、安全与 Mermaid 预处理**：
  - 探测顺序：`portrait.json` 的 `mgo.export.pandocPath` → `exec.LookPath("pandoc")` → `/opt/homebrew/bin/pandoc`、`/usr/local/bin/pandoc`、`~/.nix-profile/bin/pandoc`。校验最低版本 `2.15.0` 以支持 `--sandbox` 与 `custom-style`。
  - 未安装时在 `File → Export ▸` 保持可见并标注需 Pandoc，点击弹安装指引与 Settings 入口。
  - 严禁走 shell，统一 `exec.CommandContext` + 参数切片 + 超时 + `--sandbox`。由于 Pandoc `--sandbox` 阻断 reader/writer 读取外部资源文件，本地图片与预渲染 Mermaid 图一律由 Go 侧预内联为 `data:` URI。
  - Mermaid 在 Pandoc 通道预渲染：EPUB 优先用 SVG，DOCX/PPTX/RTF/LaTeX 用 2x PNG（复用 `mygo` 纯 Go `ui.ParseSVG` → `(*ui.SVG).Image(2x)` → `png.Encode`，不引入 CGo 或外部 `rsvg-convert`；栅格化时必须向 Mermaid 传入具体 Portrait Light 固定 hex 色板，不得使用 `var(--...)` 或 `currentColor`，并通过尺寸属性避免 DOCX/PPTX 双倍放大）；单图失败降级保留带语言标记的原代码块并在结果中指明失败图表。

## Consequences

- `internal/export/genui` 需配齐穷尽性单元测试（遍历 `internal/genuikit/catalog.json` 全部 50 个组件，新增组件漏配渲染器即测试失败）与 Golden HTML 快照测试。
- 导出验收包含离线自包含检查：将生成的 `.html` 移出原工作区并在断网环境打开，验证无破图、无外部依赖、无未勾选的 Reader Notes 残留。
- 分三期推进：Phase 1 交付统一注册表与 Markdown/GenUI 的零 JS Standalone HTML；Phase 2 交付内置 PDF 闭环；Phase 3 交付外部可选 Pandoc 扩展。
