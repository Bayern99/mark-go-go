# ADR 0011 · A2UI 信封内使用 OpenUI Lang 语句

Status: accepted

版本 3 视图文件保留 A2UI 信封、`sources` 和局部更新，`components` 的每一项改为一条 OpenUI Lang 1.0 语句。组件目录由少量语义块改为布局、文本等原语，模型自己排版。解析、求值和渲染全部用 Go 和 MyGo Native UI 完成，版本 2 文件继续可读。格式见 [视图格式](../genui/view-format.md)；实现见 [语言模块](../../internal/openuilang/) 和 [原生阅读器](../../internal/nativegenui/)。

- 语言固定为 1.0。上游一致性用例拷入仓库作为测试，来源见 [SOURCE.md](../../internal/openuilang/testdata/SOURCE.md)。升级语言版本时替换用例并修到通过。
- [组件目录](../../internal/genuikit/catalog.json) 以 Open Intelligent UI 的组件签名为起点，之后由本仓库维护。属性只在末尾追加，已保存的视图因此保持可读。
- 来源归属用 `Source(sources, children)` 包裹组件表达。来源匹配规则不变。哈希仍是章节原文（不含读者批注）的 fnv1a32；一直到文档末尾的章节按“以一个换行结尾”计算（文档用 CRLF 时是 CRLF），文件末尾有多少空白都不改变哈希。新写入的哈希都按这条规则计算。已保存的哈希等于当前章节按旧规则（末尾空白原样计入）算出的值时，同样视为未变，升级前保存的视图因此不会显示“Source changed”。 [哈希计算](../../internal/genui/sectiontext.go) · [共享用例](../../internal/testdata/markdown/section-vectors.json)
- 图标取自应用已打包的 Phosphor 字体（MIT，[许可证](../../internal/portrait/fonts/LICENSE-Phosphor.txt)），[名称表](../../internal/nativegenui/icons.tsv) 由字体的连字表导出，并补了常见 lucide 名称的对应。
- `Image` 只显示原文引用的本地图片文件，生成和阅读都不联网取图。
- 文件记录 `language`，`catalogId` 带目录版本。

**Considered Options**
- 留在 A2UI 的 JSON 组件：没有表达式和状态语法，原语目录下输出冗长。
- 用 WebView 内嵌上游 React 渲染器：与原生界面方向相反。
- 依赖上游 `@openuidev/a2ui`：该包为实验性的 TypeScript 实现，这里只沿用“components 里放语句”的约定。
