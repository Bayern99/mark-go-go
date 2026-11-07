# Mark Go Go

本地优先的 macOS Markdown 工作台：先是可靠的 Markdown 编辑器，再在其上提供由 AI 生成、可交互的阅读视图。

## 文档

**原文（Document）**：
用户打开的那份 `.md` 文件，内容唯一的权威来源。
_Avoid_: 源文件、底稿、主版本

## 工作区与文档导航

**工作区（Workspace）**：
以一个文件夹为根的原文集合与导航范围。
_Avoid_: 项目（指工作区时）、工作目录（指工作区时）

**文档标签（Document tab）**：
窗口顶部代表一份已打开原文的导航项；与同一份原文的视图模式分开。
_Avoid_: 视图模式、GenUI 组件内的 Tabs

**未命名原文（Untitled document）**：
尚未首次保存、没有磁盘路径的原文。
_Avoid_: 临时文件

**临时窗口（Untitled window）**：
尚未绑定工作区、用于承载未命名原文的窗口。
_Avoid_: 空工作区、无根工作区

## 视图模式

**视图模式（View mode）**：
同一份原文在窗口中央的三种互斥呈现方式之一：Rich text、Markdown、GenUI。
_Avoid_: 面板、标签页、编辑模式

**Rich text**：
以所见即所得方式显示并编辑原文的视图模式。
_Avoid_: WYSIWYG 模式、富文本模式

**Markdown**：
以原始 Markdown 文本显示并编辑原文的视图模式。
_Avoid_: 源码模式、Source mode、MarkdownView

**GenUI**：
显示 GenUI 视图的视图模式。
_Avoid_: GenUI 面板、Generated UI 侧栏

**原地 Markdown 编辑（In-place Markdown editing）**：
在 Rich text 中只为一个特殊块展开原文编辑区，完成后回到该块的可视呈现；这是局部编辑操作，不是第四种视图模式。
_Avoid_: 源码模式（指该操作时）、结构化可视编辑

## GenUI

**GenUI 视图（GenUI view）**：
由 AI 从原文派生出的、只用组件目录中组件组成的可交互阅读界面；它不是原文，丢弃后可重新生成。
_Avoid_: 视图文件、生成结果、第二版本

**组件目录（Catalog）**：
GenUI 视图允许使用的全部组件的集合；目录之外的东西不会被渲染。
_Avoid_: 白名单、组件库

**来源引用（Source reference）**：
GenUI 视图中某个部分所依据的原文位置。
_Avoid_: 锚点、出处

**原文已改（Source changed）**：
来源引用所指的原文在 GenUI 视图生成后被改动过的状态；仅增改批注不算改动。
_Avoid_: 过期、Stale、失效、脏

**生成要求（Generation request）**：
用户为某份原文的 GenUI 视图写下的偏好说明，随该视图保存并在每次重新生成时沿用。
_Avoid_: 提示词、prompt、指令

## 审阅

**批注（Note）**：
用户对原文某处内容的一条文字意见，属于原文并随原文保存；它不是原文内容的一部分，也不属于 GenUI 视图。
_Avoid_: 注释、评论、备注、批注块
