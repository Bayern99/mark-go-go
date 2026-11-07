# 界面任务执行规范

## 目标与范围

在用户给定的项目和界面任务内工作。使视觉、交互与真实业务状态一致，而不只是生成漂亮截图。

- 只检查和修改目标页面及必要依赖。
- 不默认全库审计、重构、建设完整组件库或重写项目规则。
- 沿用现有架构、设计体系和文档位置，不建立平行体系。
- 缺少项目、目标页面或关键业务定义时，说明缺口，不编造。
- 发布、部署、删除数据等操作需有对应授权。

## 决策原则

1. 先找证据，再写实现；不把猜测说成现有约定。
2. 按用途复用 Token、组件和区块，不按外观随意凑值。
3. 先有真实需求和重复，再做抽象。
4. 界面状态真实，比截图漂亮更重要。
5. 遵循已确定的产品判断，不擅自扩大产品范围。
6. 规则必须有理由，检查必须有证据。

## 执行流程

### 1. 有界盘点

检查目标页面及其必要依赖，找到：
- 实际适用的项目规则；
- 基础设计值和语义 Token；
- 可复用组件、真实 props 和已有变体；
- 相似页面与已有区块；
- 数据来源、状态转换及测试方式。

记录必要的文件路径、符号名或行号。
文档与实现不一致时明确指出，不使用尚未实现的接口。
说明哪些要求适用，哪些不适用；不要扩大为全库整改。

### 2. 确定最小实现

简短列出目标行为、拟修改范围和验收方式，然后执行。
有充分上下文的可逆改动直接推进，不把普通计划变成审批关卡。

#### 基础值与语义 Token

- 复用颜色、字体、字号、字重、行高、间距、圆角、阴影、动效。
- 优先使用按用途命名的语义 Token，而不是直接挑基础颜色或写裸值。
- 保持主要/次要文字、操作、状态、层级等语义一致。
- 不随意用成功、警告、错误色作装饰。
- 确需新增值时，沿用现有命名和位置，说明缺口与理由。
- 不机械照搬其他项目的字体、尺寸、颜色或组件库。

#### 组件

- 优先复用实际存在的组件和变体。
- 核对适用场景、默认变体及替代选择。
- 从真实代码识别好例与反例；没有现成例子就说明，不伪造。
- 覆盖本任务适用的悬停、按下、焦点、禁用、加载、空、错误状态。
- 新增或更新说明时，只描述已实现能力。

#### 区块

- 优先使用真实页面已有的组件组合。
- 仅在当前任务出现真实重复、且抽取确实更简单时提炼区块。
- 区块应有清楚的用途、职责、变体和状态。
- 区块可以是一行，也可以是组合区域；不要把每个容器都抽象。
- 名称表达用途，例如工作区切换器、导航项、导航分组、审批面板。
- 不只记录默认外观，也记录实际存在的状态和变体。
- 超出现有区块时，先判断确有需要还是重复造轮子；产品范围变化需确认。

### 3. 处理未决事项

需暂停相关部分：
- 缺少授权；
- 不可逆操作；
- 高影响产品判断；
- 无法安全推断的数据或状态语义。

暂停时说明具体问题、影响及最小待确认事项。
不受影响的工作继续推进。

可直接推进：
- 小范围、可逆且已有项目惯例的细节。
- 沿用最近的合适实现，并在交付中列明必要假设。

不要为每个尺寸反复询问，也不要让猜测悄悄固化为标准。

### 4. 必要时沉淀规则

- 长期规则：写“要求＋理由”。
- 按需 Skill：写“触发条件＋执行步骤＋验收方式”。
- 只有任务需要或用户授权时，才修改相关规则、文档或项目级 Skill。
- 沿用现有位置；不自动安装全局 Skill。
- 超出范围的改进只列为建议，不顺手扩建体系。

### 5. 验证真实行为

按风险使用项目现有的类型检查、构建、测试与界面验证方式。
不要为形式另建复杂流程。

验收 Checklist：
- [ ] 使用的 Token、组件、props 和变体实际存在。
- [ ] 已检查绕过既有设计值的裸值，必要例外有理由。
- [ ] 适用的交互、加载、空、错误和禁用状态已检查。
- [ ] 运行期间有准确可见的状态。
- [ ] 审批完成后不再显示“待审批”。
- [ ] 运行结束后不残留“运行中”等过期提示。
- [ ] 主操作层级符合本项目规则；不机械强加“一屏一个主操作”。
- [ ] 实际走过关键状态变化，不只审静态截图。
- [ ] 目标流程及必要关联流程已检查回归。
- [ ] 文档与实际实现一致，没有新增虚构能力。

区分静态阅读、截图检查与运行测试，不能互相冒充。
骨架屏只是加载状态的一种实现，不要求所有页面使用。
设计系统检查不能替代适用的无障碍、安全、权限、隐私和性能检查。

## 验收状态

对每项适用检查报告状态与证据：

- PASS：已验证，符合预期。
- FAIL：已验证，不符合；说明现象。
- BLOCKED：无法验证或尚未运行；说明原因与缺口。
- N/A：本任务不适用；说明原因。

证据可用文件位置、命令结果、测试用例、操作路径或截图。
没有执行的检查不得标为 PASS，不得虚构验证结果。

## 最终交付

简短报告：
1. 完成：改了什么，涉及哪些文件。
2. 依据：复用了哪些规则、Token、组件或区块及其出处。
3. 验证：检查项｜状态｜证据或原因。
4. 剩余：假设、未决事项、未运行检查及必要后续。

只有任务已完成且适用验收通过，才能声称完成。
否则明确报告部分完成或受阻，以及剩余范围。

---

## 本项目长期界面规则（要求＋理由）

1. **三栏职责严格分离（左栏工作区、中央只放内容、右栏放 Outline 与 GenUI 管理）**
   - **要求**：左栏仅放工作区文件导航；中央仅渲染当前文档内容（Rich text / Markdown / GenUI 视图）及紧贴正文的动作（卡片底部来源章节、`Source changed`、`Edit in source`、`Note` 与正文批注操作）；生成触发、进度、失败和模型选择全部放在右栏 GenUI 面板（见 `docs/adr/0004-three-column-layout-and-genui-panel.md`）。
   - **理由**：避免把“读视图”和“管生成”混在一起，消除中央阅读区上方常驻表单对阅读体验的干扰，并为后续围绕当前文档工作的 Agent 预留自然栏位。

2. **状态语义色严格限定用途**
   - **要求**：`--warn`（`#9a4d12` / 暗色 `#f0a560`）**仅**用于标记 `Source changed`（原文已改）；`--danger`（`#a83a3a` / 暗色 `#ef8a80`）**仅**用于文件外部冲突、文件被删、生成失败或删除动作；`--accent`（`#2f6f62` / 暗色 `#7cc4b2`）用于主操作高亮、活动指示与菜单当前选中勾号（见 `docs/design/Mark Go Go 界面设计.html` 画板 3 `3-language.html`）。
   - **理由**：防止将警告、错误或成功色当作装饰色滥用，确保用户在冷灰纸面界面中一眼识别哪些组件因原文改动而过期、哪些发生真实错误。

3. **批注与生成要求统一使用原地 Composer（禁止模态弹窗）**
   - **要求**：添加/编辑批注（Rich text 与 GenUI 卡片内）及编辑生成要求（Focus）均在最终落位处原地展开为单张白纸输入块（`border-radius: 8px`，随内容增高，`⌘↩` 保存、`Esc` 取消），不使用 `<dialog>` 或居中弹窗（见 `docs/adr/0004-three-column-layout-and-genui-panel.md`）。
   - **理由**：居中弹窗会遮挡正在批注的原文或卡片，造成框套框和上下文割裂。

4. **Model 选择器复用块菜单材质的向上弹出菜单（禁止原生 `<select>` 与自由手输框）**
   - **要求**：右栏底部固定的 `Model` 选择使用 26px 凸起触发钮（`.sel-btn`）+ 向上弹出的纸面菜单（`.mmenu.pop`，`10px` 圆角、`var(--pop-shadow)`、行 hover `6px` 圆角、右侧当前项勾号），点击外部或按 `Esc` 关闭，生成中禁用；放弃原生 `<select>` 与自由文本输入框（见 `docs/adr/0004-three-column-layout-and-genui-panel.md`）。
   - **理由**：A2UI 与 Crepe 均无通用表单下拉组件；原生 `<select>` 弹层由操作系统绘制，破坏 Portrait 设计语言；手输框易输错模型 ID 触发 `model-invalid`。

5. **领域词汇与原文单向派生约束**
   - **要求**：界面文案、组件命名、测试与报告严守 `GLOSSARY.md`（使用 Document/原文、View mode/视图模式、GenUI view/GenUI 视图、Catalog/组件目录、Source reference/来源引用、Source changed/原文已改、Generation request/生成要求、Note/批注）；GenUI 视图只读，修改原文统一走 `Edit in source` 跳转，批注以 `<!-- note: ... -->` 写回原文（见 `docs/adr/0002-genui-derived-center-view.md`）。
   - **理由**：保证“一个文件唯一权威来源”，避免双向同步与概念漂移。

---

## 按需 Skill：Mark Go Go 界面任务落地

- **触发条件**：在 Mark Go Go 中新增、重构或修复任何前端界面、插件视图（Outline / GenUI）、编辑器交互（Rich text / Markdown / 批注）或落地 `docs/design/Mark Go Go 界面设计.html` 与 `docs/adr/0004-three-column-layout-and-genui-panel.md`。
- **执行步骤**：
  1. **有界盘点**：阅读 `GLOSSARY.md`、`docs/adr/0002-genui-derived-center-view.md`、`docs/adr/0004-three-column-layout-and-genui-panel.md`、`docs/design/Mark Go Go 界面设计.html` 及下方“当前代码与设计规范对照基准”，核对目标文件、Token、组件 props 与 Go ↔ Web `HostAdapter` 接口（`docs/interfaces.md`），列出适用项与缺口。
  2. **确定最小实现**：复用既有语义 Token、组件与区块；对未实现的后端接口（如 `GetSettings` 模型列表缓存）先对齐契约并运行 `go tool mygo generate`（若改 Go 绑定签名），再改前端视图与样式。
  3. **覆盖真实状态**：逐一连接空闲、生成中（步骤进度 + `Cancel` + 结束清理）、过期（`Source changed` 列表 + `Regenerate changed`）、错误（分类失败原因 + `Retry` / `Dismiss`）、禁用与撤回（`Undo`）状态。
  4. **验证与交付**：执行类型检查、单测、构建与真实交互状态走查，按《界面任务执行规范》四段式报告输出 `PASS` / `FAIL` / `BLOCKED` / `N/A` 及证据。
- **验收方式**：
  - `cd web && npm run typecheck && npm run test && npm run build` 全部通过；
  - `go test ./...` 全部通过；
  - 对照本文档“验收 Checklist”逐项给出真实证据，确认文档描述与实际已实现代码一致。

---

## 附：Mark Go Go 当前代码与 ADR 0004 / 设计稿对照基准（仅描述真实状态）

> 说明：以下区分**当前代码已实现能力**与 **ADR 0004 / 设计稿待落地缺口**，落地完成前不得将“待落地项”当作已实现接口或现成组件引用。

### 1. 基础值与语义 Token

- **当前已实现（旧暖米色体系，待升级）**：
  - 全局变量定义于 `web/src/styles.css`（`#L1-L26`）：`--mgo-bg`、`--mgo-bg-elevated`、`--mgo-panel`、`--mgo-border`、`--mgo-text`、`--mgo-muted`、`--mgo-accent` 等。
  - GenUI 局部变量定义于 `web/src/plugins/genui/genui.css`（`#L1-L32`）：`--genui-text`、`--genui-muted`、`--genui-border`、`--genui-surface`、`--genui-card`、`--genui-accent`、`--genui-stale-fg`、`--genui-missing-fg`。
- **设计板定义的目标 Token（位于 `docs/design/Mark Go Go 界面设计.html` 画板 3 `3-language.html`，尚未写入 `web/src/styles.css`）**：
  - 字体：`--font-sans`（IBM Plex Sans / SC）、`--font-display`（MGG Condensed / Barlow Semi Condensed）、`--font-mono`（IBM Plex Mono）。
  - 表面与墨色（含 Dark 模式）：`--chrome`、`--paper`、`--raised`、`--code`、`--ink`、`--ink-2`、`--ink-3`、`--ink-4`、`--accent`、`--warn`、`--danger`、`--fill`、`--fill-hover`、`--fill-strong`、`--line`、`--line-strong`。
  - 触觉阴影：`--raised-shadow`、`--row-shadow`、`--sunken-shadow`、`--sheet-shadow`、`--pop-shadow`。
  - 网格与尺寸：左右栏内边距 `8px`、图文对齐线 `16px`、树/大纲缩进 `16px/级`、顶栏 `46px`、次级栏 `44px`、窄窗右栏收起断点 `1180px`。

### 2. 组件与区块现状（好例、反例与缺口）

- **真实代码中已实现的好例（可直接复用）**：
  - **插件与视图宿主机制**：`web/src/core/ViewRegistry.ts`（`#L1-L43`）、`web/src/components/ViewHost.tsx`（`#L1-L38`）。
  - **A2UI 1.0 基础与高级只读组件目录**：`web/src/plugins/genui/a2ui.tsx`（`#L1-L211`，`Column`/`Row`/`Tabs`/`Card`/`Text`）与 `web/src/plugins/genui/components.tsx`（`#L1-L349`，`KeyPoints`/`MetricGrid`/`ComparisonTable`/`Timeline`/`Checklist`）。
  - **来源引用底座区块**：`web/src/plugins/genui/SourceFrame.tsx`（`#L15-L67`）与 `web/src/plugins/genui/sources.ts`（`#L1-L101`，`SourceStore` 状态计算）。
  - **生成状态机**：`web/src/plugins/genui/generation.ts`（`#L1-L113`，`GenerationController` 维护生成进度、取消、局部重生成与单步 `Undo`）。
- **真实代码中待改造的反例（违反 ADR 0004）**：
  - `web/src/plugins/genui/GenUIView.tsx`（`#L234-L393`）：将 `<GeneratePanel>` 嵌在中央 GenUI 阅读视图顶部。
  - `web/src/plugins/genui/GenUIView.tsx`（`#L475-L483`）：在折叠 `<SettingsForm>` 中用自由文本 `<input>` 输入 `Model` 并常驻 `Agent command` 输入框。
  - `web/src/editor/noteDialog.ts`（`#L5-L47`）：`askForNote` 使用居中模态 `<dialog class="mgo-note-dialog">` 输入批注。
  - `web/src/plugins/outline/index.tsx`（`#L18-L22`）：`outline.panel` 仍注册在 `location: 'left'`，顶栏 `web/src/App.tsx`（`#L375-L384`）仍保留 `Outline: On/Off` 开关。
- **当前代码尚无现成实现、需按设计板 `4-components.html` 新建的组件/区块**：
  - 右栏宿主容器（`ViewLocation` 扩展 `'right'`，顶部切换 `Outline` / `GenUI`，`< 1180px` 自动收起为浮层）；
  - 右栏 `GenUI` 管理面板区块（顶部状态与唯一主按钮、中部生成要求 Focus、底部固定区）；
  - `ModelMenu` 向上弹出菜单组件（`.sel-btn` 触发钮 + `.mmenu.pop` 弹层，`document` 级 `Escape` 与外部点击关闭）；
  - 原地 `Composer` 输入组件（供正文批注、GenUI 卡片批注与右栏 Focus 共用）。

### 3. 数据来源与接口缺口（文档 vs 实现）

- **缺口说明**：`docs/adr/0004-three-column-layout-and-genui-panel.md` 要求 `ModelMenu` 展示 ACP `session/new` 返回的 `configOptions` 模型列表加 `Agent’s choice`。但当前 Go 端 `generate.go`（`#L283-L312`、`#L620-L649`）仅在执行生成的临时子进程中调用 `session/new`，而 `settings.go`（`#L47-L74`）的 `GetSettings` 与前端 `web/src/host/types.ts`（`#L110-L113`）的 `GenerationSettings` 仅返回 `{ model, agentCommand }`（模型列表仅在 `model-invalid` 失败时通过 `GenerateFailure.models` 带回）。
- **已确认实施方案**：扩展 Go `Settings` / `Host`，缓存每次 ACP `session/new` 握手拿到的 `models` 列表（未握手前支持探测或默认回退列表），随 `getSettings` 一并返回给前端 `ModelMenu`。
