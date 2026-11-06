# ADR 0001：架构基线（MyGo + CrepeBuilder + 块级保真保存）

- 状态：已采纳（2026-11-06 原型验收通过，见 `docs/acceptance.md`）
- 日期：2026-11-06

## 背景

交接 brief 选定 MyGo 宿主、Milkdown/Crepe 编辑器，但多项适用性未经验证。2026-11-06 的调研（源码阅读 + Chromium 实测 + MyGo 构建）得到以下事实，本 ADR 据此修正 brief。

| 事实 | 证据 |
|---|---|
| MyGo 建仓 2026-09-26，v0.7.4，79 个 tag/6 周，提交主要来自单一维护者；支持 macOS/Linux/Windows/wasm；无 cgo（purego） | `github.com/egoist/mygo` git 历史 |
| MyGo `NewDocuments[T]` 提供脏状态、自动保存、恢复副本、外部修改冲突（mtime+size）、文件监听、`OnReload`、File 菜单、原子替换写入 | `docs/documents.md`；原型中 `doc.Set`/`SaveAs` 实测 |
| MyGo typed IPC 有真实 TS 代码生成（`Bind`、`Event[T]`、`Channel[T]`） | `GenerateTypeScript()` 实际输出 |
| 文档 + Web 页面窗口组合未有官方示例；DOM ready 前 `Page.Eval` 失败 | `examples/documents` 仅原生 UI；实测报错 |
| Milkdown 整篇序列化：21 个刁钻用例中 8 个语义损坏、11 个文本规范化；二次往返幂等 | `/tmp/mdws-rt` 往返实测（Milkdown 7.23.0） |
| 完整 Crepe 2.80MB min / 936KB gzip；Builder + 常用 UI ≈ 1.2–1.5MB；`latex` 依赖 CodeMirror 且把 `$5` 转义为 `\$5` | esbuild 打包；实测 |
| Crepe AI feature 在 7.23.0 发布包中，可用：流式、Diff、Accept/Reject/Retry、取消；仅单轮选区改写 | `feature/ai/types.ts:23`；mock provider 实测 |
| Crepe 无源码模式、无 frontmatter 支持 | 源码；实测 frontmatter 被破坏 |
| PerMark（Milkdown + CodeMirror）不用 Crepe，保存时从不整篇序列化：未改块原字节、改动块单独序列化 | PerMark `visual/compose.ts`、`edit/lines.rs` |

## 决定

### D1 编辑器装配：CrepeBuilder

启用：`listItem, cursor, placeholder, linkTooltip, blockEdit, table, imageBlock, toolbar, codeMirror`（显式小语言列表，不引入 `language-data`）。
不启用：`topBar`（应用自有外框）、`latex`（`$` 误转义、强依赖 CodeMirror）、`ai`（第一阶段无 Agent）。
自行实现：源码模式（CodeMirror + `replaceAll`）、大纲、搜索替换、frontmatter 处理。

代价：放弃 Crepe 默认的 LaTeX；Vue 运行时仍随 Crepe 组件打包。

### D2 单一文档权威

| 层 | 职责 | 不负责 |
|---|---|---|
| MyGo `Document[string]`（Go） | 已保存/待保存文本、文件读写、脏状态、自动保存、恢复副本、外部修改冲突、File 菜单、退出流程 | Markdown 语义 |
| Application Core（TS） | 当前文本快照与版本、命令/视图/插件注册 | 文件 I/O、编辑事务 |
| Editor Adapter（TS） | Milkdown 实例、块级合成（compose）、位置与事务 | 文件状态 |

数据流：打开/外部重载时 Go → 页面（`Load` / `docReloaded`）；编辑后页面 compose 出全文 → `Update` → `doc.Set`。页面不保存第二份脏状态，只显示 Go 推送的 `DocState`。

### D3 Markdown 保真定义

- 未触碰的块：**字节保真**（含 CRLF、空行、标记风格）。
- 编辑过的块：允许 Milkdown 文本规范化（标记风格等）。
- 任何语义损坏：缺陷。
- Milkdown 无法语义往返的块（frontmatter、GitHub alert、脚注定义、带 meta 的围栏代码、引用式链接定义等）作为**锁定块**按原文显示与写回，不可视化编辑。

实现：保存路径永不整篇序列化；按顶层块合成（originNode → 原文片段）。

### D4 插件契约与 GenUI

插件只通过 `PluginContext` 获得能力（文档快照/变更、视图注册、命令注册）；注册表追踪所有经 ctx 产生的注册，`disable` 时统一释放。生命周期管理 ≠ 安全隔离：第一阶段只加载第一方可信插件。

GenUI 使用内部小型判别联合 schema + 组件白名单（借鉴 A2UI catalog），未知组件渲染为错误提示，不执行任意代码。第一阶段使用固定 JSON。A2UI v1.0 协议暂不采用。

### D5 Agent 接入边界

第一阶段不实现。顺序：

1. 文件即接口：外部 Agent 直接修改 `.md`，由 MyGo 监听；无本地修改时自动重载，有修改时进入冲突。
2. 只读 MCP/CLI 适配器：文档、大纲、范围读取、当前选区（独立进程/适配器，不进入编辑器内核）。
3. 写入以建议（suggestion）为默认，经 Editor Adapter 做版本校验与 Markdown 偏移 → ProseMirror 位置转换。
4. Crepe AI feature 仅作选区内改写；provider 经 Go 代理，密钥不进入 WebView。

## 风险与失败条件

| 风险 | 缓解 | 触发重新评估的结果 |
|---|---|---|
| MyGo 早期、API 变动快、单维护者 | go.mod 固定 v0.7.4；Host Adapter 薄层；Wails v2 作为退路 [INFERENCE：未在 Wails 上验证] | WKWebView 中 IME/快捷键/保存不可靠，或升级需大面积改动 |
| 块对齐错误（列表合并、HTML 块） | 每片段单独解析；无法 1:1 的片段锁定 | 未修改文档 compose 后非字节一致 |
| 锁定块过多影响编辑体验 | 逐类补 remark 插件解锁（frontmatter 面板、alert 节点） | 日常文档中锁定块比例高到无法接受 |
| 外部重载丢失撤销历史 | 接受（第一阶段） | 用户反馈 |

## 未决

- 源码模式与 WYSIWYG 共享同一 compose 基线的方式。
- 打字延迟与超长文档（>1MB）的增量策略。
- 多窗口/标签页模型。
