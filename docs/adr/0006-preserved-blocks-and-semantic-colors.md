# ADR 0006：特殊块的保真呈现、原地编辑与语义色边界

- 状态：已采纳（目标行为已确认，实现纳入统一规格）。
- 日期：2026-11-08
- 补充：ADR 0001 D3 的特殊块展示与编辑策略，以及 ADR 0004 的原地 Composer。

设计稿要求 Frontmatter 键值表、GitHub alert 提示块、脚注和代码块的可视呈现，现有实现则把这些特殊语法显示成锁定源码。用户确认借鉴 SoloMD 的“原文权威、渲染与保存分离”思路，保留现有块级字节保真和编辑器内核。

## 特殊块呈现与编辑

特殊块平时按设计可视呈现；点击编辑时，在该块原地展开 Markdown 编辑区，完成后回到可视块，不切换整份原文的视图模式。沿用原地 Composer 的保存、取消与键盘约定；只读原文不能进入编辑。

Frontmatter、GitHub alert、脚注及带元信息的代码块使用对应的可视呈现。原始 HTML 和暂不支持的语法继续显示为 `Kept as written` 保真块，不执行其中的 HTML。不能可靠呈现的内容保留原文可见和可编辑，不能静默丢弃。

渲染不改原文字节，保存不经过展示用的规范化或全篇重新序列化。未触碰块、块间空白和换行保持原样；原地 Markdown 编辑仅写回用户修改的片段，并支持撤销。暂不引入 Frontmatter 字段表单或独立属性侧栏。

SoloMD 的参考实现固定于 [219a6ba](https://github.com/zhitongblog/solomd/commit/219a6baafccbe7d50776a1a41909189141dfea4b)：其 [实时编辑扩展](https://github.com/zhitongblog/solomd/blob/219a6baafccbe7d50776a1a41909189141dfea4b/app/src/lib/cm-live-render.ts)、[展示渲染器](https://github.com/zhitongblog/solomd/blob/219a6baafccbe7d50776a1a41909189141dfea4b/app/src/lib/markdown.ts)与[直接写出文本的保存路径](https://github.com/zhitongblog/solomd/blob/219a6baafccbe7d50776a1a41909189141dfea4b/app/src/composables/useFiles.ts)提供交互参考；其换行规范化和属性面板写入方式不替代本项目的逐块保真约束。

### Mermaid 图表（2026-11-10）

预览将 `mermaid` 围栏作为保真特殊块显示为图表，复用现有锁定节点与原地 Composer。流程图、时序图等由本地打包的 Mermaid 渲染，展示结果不写入原文；`Edit` 仅修改该围栏，保存、取消、撤销与只读规则沿用现有特殊块。无效图保留错误说明和原始 Markdown，不让一张图的失败阻断其他图或正文。

Markdown 视图继续展示原文；GenUI Catalog 本次不增加 Diagram 组件，`Text` 中的代码围栏仍按代码显示。GenUI 的图表能力缺口及工作台组件规划见 [UI 组件缺口与文字规划](../design/ui-component-gaps.md)。

## 部分来源丢失

多来源 GenUI 卡片只丢失部分来源时，仍显示来源缺失提示，但 `Edit in source` 与 `Note` 可以使用第一个仍存在的来源。全部来源都失效时才禁用这些动作。保留只读 GenUI 与修改原文的单向关系，不用不存在的来源静默替代实际依据。

## 颜色与本轮主题范围

应用状态色与原文内容语义色分开定义。应用中的 `Source changed`、文件冲突、删除与生成失败继续采用各自状态语义；原文内的 GitHub Warning、Caution 等使用独立内容语义 Token，并有类型文字和图标，不能只靠颜色区分。

本轮统一完成浅色界面，不新增正式深色主题或外观切换入口。字体按设计离线打包，编辑器、GenUI 与外框共用规定的字体、字号、间距和材质。

## 取舍

原地编辑该块 Markdown，保留了编辑能力与上下文，同时避免为每种特殊语法维护新的结构化写回模型。代价是编辑这些块时仍需接触 Markdown。内容语义色与应用状态色分别定义，允许文档的 Warning／Caution 按设计表达含义，而不改变工作台状态色的职责。
