# 工作区 UI 组件缺口与文字布局规划

本次范围：默认窗口、文件树、文档标签、Rich text 的 Mermaid 呈现，以及它们在现有三栏工作台中的关系。组件缺口先盘点，再只为已确认的交互缺口引入轻量 primitives；不引入完整 UI 套件，不扩大 GenUI Catalog。滚动区域维持现有 OverlayScrollbars 实现，不在本次范围。

依据：[界面任务规范](../agents/ui-tasks.md)、[领域词汇](../../GLOSSARY.md)、[三栏职责](../adr/0004-three-column-layout-and-genui-panel.md)、[特殊块保真与编辑](../adr/0006-preserved-blocks-and-semantic-colors.md)和 [Portrait 设计画板](Mark%20Go%20Go%20界面设计.html)。设计画板的工作区与组件板均规定标签文字与关闭动作间隔 6px，关闭命中区 20px；文档标签的滚动方式按后续已采纳的溢出菜单规则执行。

## 已有能力，优先复用

| 用途 | 现有实现 | 判断 |
| --- | --- | --- |
| 按钮与图标动作 | `web/src/styles.css` 的 `.pill` / `.pill.icon` / `.quiet` | 已有视觉基底；本轮不另建按钮库 |
| 文件树与搜索结果 | `web/src/components/SidePane.tsx` 的 `FileTree`、`FolderRow`、`FileRow`、`FilterResults` | 已有组件；文字挤压属于布局与元信息优先级问题 |
| 文档标签与右栏选择 | `web/src/App.tsx` 文档标签；`RightPane.tsx` 分段标签 | 两者语义不同，不能仅因都叫 tab 就合为一个组件 |
| 原地编辑 | `.composer`、`noteDialog.ts`、特殊块编辑动作 | Mermaid 复用此路径；不新增图表编辑器或弹窗 |
| 持续状态和失败 | `App.tsx` 的冲突、删除、`actionError` / `restoreError` 状态条；右栏 GenUI 状态 | 已有明确位置；不能用自动消失的通知替换 |
| 滚动区 | `web/src/components/ScrollArea.tsx` | 已统一；不再增加一套滚动机制 |
| GenUI 阅读组件 | Column、Row、Tabs、Card、Text、KeyPoints、MetricGrid、ComparisonTable、Timeline、Checklist | 当前 Catalog 为 10 项；原文特殊块与 GenUI Catalog 是不同契约 |

## 确认的缺口与候选组件

P1 表示下一轮优先；P2 表示有实际使用需求时再落地。下列项目是规划，不代表已经实现。

| 优先级 | 缺口 | 当前证据 | 最小职责与验收 |
| --- | --- | --- | --- |
| 已落地 | Tooltip / 截断文字完整提示 | `components/Tooltip.tsx` + `components/ui.css`；文件树、标签文字和状态标记接入 | Radix 提供 hover/焦点提示、Escape、Portal 和碰撞定位；完整路径不占行内宽度；标签按钮的测量 ref 不经过 Tooltip 包装 |
| 已落地 | 弹出菜单的共享交互底座 | 文档溢出菜单与 `ModelMenu.tsx` 共用 `radix-ui` DropdownMenu；Crepe 块菜单仍由编辑器拥有 | 统一焦点进入、Escape、外部点击、关闭后返回触发钮及定位；文档菜单保留搜索与焦点恢复，模型菜单保留 listbox/option 和选中勾号 |
| P1（GenUI 后续需求） | 只读 Diagram 组件 | 当前 Catalog 没有 Diagram；`SafeMarkdown.tsx` 对代码块只输出 `<pre><code>`，不会把 Mermaid 变成图 | 若生成视图需要保留图表，新增受校验的声明式组件，带来源引用并复用原文的图表渲染能力；修改仍走 `Edit in source`；不允许生成任意 HTML/JS。此次不扩 Catalog |
| 已落地 | 短时复制通知 | `components/feedback.ts` 调用 Sonner；`App.tsx` 根部一个 Toaster；Crepe 代码块与链接复制成功回调接入 | `sonner@2.0.8`，浅色 Portrait 样式；相同 ID 更新而非叠加；持续错误、冲突、恢复失败和生成状态仍原位显示 |
| P2 | 数学公式可视呈现 | `lockedView.ts` 的数学块目前展示源码 | 作为特殊块呈现能力单独评估；原文字节、原地编辑与撤销约束不变，不新增独立文档模型 |

## 组件参考的实际取舍（2026-11-11）

| 参考 | 结论 | 依据与边界 |
| --- | --- | --- |
| [Awesome MyGo UI](https://awesomemygo.dev/ui) / [Awesome MyGo](https://github.com/yldm-tech/awesome-mygo) | 用作 MyGo 生态索引，不直接等同于现成 Web 组件 | 官方索引区分 native UI 和 web frontend；本项目是 Go/MyGo 窗口承载 WebView，React 组件仍运行在同一页面，不需要为 Tooltip、菜单和通知另写 Go 原生 UI |
| [Radix Tooltip](https://www.radix-ui.com/primitives/docs/components/tooltip) / [Dropdown Menu](https://www.radix-ui.com/primitives/docs/components/dropdown-menu) | 采用 primitives，经项目薄适配层使用 | 提供 hover/焦点提示、Escape、焦点管理、键盘导航、typeahead、Portal 与碰撞定位。锁定 `radix-ui@1.8.1`；不使用其默认外观，样式继续由 Portrait Token 定义 |
| [Lobe UI](https://github.com/lobehub/lobe-ui) | 只作后续 AI 阅读组件参考，本轮不引入 | v6 虽改为 Base UI + StyleX，但仍要求 `@lobehub/icons`、`@lobehub/fluent-emoji`、`motion` 和 `--lobe-*` 主题体系；其 Markdown/聊天组件不能替代 A2UI Catalog、来源引用和原文保真约束。见[官方迁移说明](https://ui.lobehub.com/migration-v6) |
| [Sonner](https://sonner.emilkowal.ski) | 只承担短时操作通知 | 根部一个 `<Toaster theme="light">`，固定 ID 更新复制反馈，样式使用项目 Token。冲突、删除、恢复失败和 GenUI 生成状态仍留在既有位置，不变成会自动消失的通知 |

决策：补齐**交互底座**而不是购买整套外观。`AppTooltip` 是项目唯一提示入口；应用自有的文档溢出菜单与 `ModelMenu` 使用同一 Radix 菜单机制，Crepe 内部菜单保持其所有权。`notifyCopied` 只在 Crepe 复制成功回调后触发。

图标按钮已经有 `.pill.icon` 视觉基底；空、加载和错误状态已经有局部实现。缺少统一 React 包装不自动构成缺功能，不将它们列入必须新增的组件。

## 已选定的导航视觉方向（2026-11-11）

用户实际比较 A / B / C 后选定：**B 的结构与信息层级 + A 的拟物材质 + C 的 `+N` 溢出计数**。

- **文档标签**：保留文件图标与 20px 关闭命中区，活动标签使用 `--raised` / `--raised-shadow`，不是单纯的平面文字按钮；隐藏文档数直接写成 `+N`。
- **文档菜单**：340px 纸面命令菜单，含 `OPEN DOCUMENTS` 标题、搜索图标、24px 文件图标、文件名与路径次级行、当前项勾号。搜索聚焦用细内描边，禁止全局 3px 双层绿色焦点环。
- **文件树**：Phosphor folder-open/folder/file-text 是必要的信息图标，不用 Unicode 黑色三角形；保留深度优先的目录顺序和现有深度缩进边界。普通文件次级行显示父目录，过滤结果仍显示标题/摘要，避免把 Markdown 标记当成树布局。
- **Model 菜单**：26px 凸起触发钮 + 纸面弹层，增加 `MODEL` 标题、Agent/模型图标、当前项勾号与细焦点态；仍只写 `{ model }`，不改变业务接口。
- **完整交互**：Radix 继续负责 Portal、碰撞定位、键盘导航和焦点恢复，Portrait Token 与 Phosphor 图标负责实际视觉；架构约束与视觉完成度不作为互相排斥的目标。

三个方案及切换器作为 primary source 保存在分支 `prototype/workspace-navigation`（提交 `ccc8c71`），生产入口已移除原型。

## 文字与排版的具体规则

| 场景 | Before | After / 目标 | 原因 |
| --- | --- | --- | --- |
| 默认窗口 | 940×720，宽度低于 1180px 右栏断点 | 1280×820，沿唯一工作区窗口创建入口配置，不请求全屏或最大化 | 默认能容纳三栏，用户仍可调窄 |
| 文档标签 | 文件名与 `×` 相邻，没有动作间隔或稳定按钮宽度 | 文字独立省略，关闭动作固定命中区与间隔；状态徽标不挤出关闭动作；活动标签保留，其他标签进入搜索溢出菜单 | 区分导航文字与关闭动作，防止误点和标签越界 |
| 文件树深层行 | 缩进不断占用标题空间，日期和批注数继续占用摘要宽度 | 深层缩进有可用空间边界；元信息随实际剩余宽度让位；文件名与摘要保持独立省略，完整路径可查询 | 首先看清原文身份和内容，再读辅助元信息 |
| 摘要的 Markdown 标记 | 截图中出现 `- [ ]`、`_emphasis_`、原始链接语法 | 后续对齐桌面与浏览器的“纯文本摘要”语义，保留可读文本；不在 CSS 中遮盖标记，不修改原文 | `navmeta.go` 的 `paragraphText` 目前返回行文本，而浏览器 `workspace/nav.ts` 从 mdast 取 `plainText`；这是提取语义差异，新增组件不会自动修复 |
| 字体层级 | 文字用途容易被局部尺寸掩盖 | 复用 `styles.css` 的 `--text-ui-*`（13/18）、`--text-meta-*`（12/16）、`--text-reading-*`（16/28）、标题与代码 Token；中文与英文均使用已打包字体 | 保持工作台文字、辅助元信息与正文的既定层级，不再增加第二套字号 |

## 后续验收边界

- 文件树检查 190、248、420px 侧栏，深度 0–8、长中文/英文名称、日期和批注数；检查过滤结果和键盘调宽。
- 文档标签检查长名称、同名消歧、多个状态徽标、窄窗、溢出搜索、键盘切换和关闭；标签不得滚动或越出标签带。
- 原文 Mermaid 检查流程图和时序图、无效语法、多个图、原地保存/取消/撤销、只读原文、视图与文档切换、LF/CRLF 字节保真；普通代码块保留原路径。
- 新增 GenUI 组件时必须同步 Catalog 校验、模型契约、SourceFrame 和只读交互；本轮只记录缺口。
- 默认窗口的原生实际尺寸与屏幕适配需要桌面检查；浏览器视口检查不能替代原生窗口几何验证。
