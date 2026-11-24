# ADR 0013 · Arc 实验：布景、纸页与命令栏

Status: experimental (branch `t3/arc-inspired-editor-redesign`; amends ADR 0004, ADR 0010 and ADR 0012 only on this branch, pending evaluation)

> 布景、固定的蓝色强调色与"只有浅色外观"已由 [ADR 0014](0014-wuxing-spaces-per-space-accent-and-dark-mode.md) 改为八个五行 Space、每个 Space 自有的强调色与完整的深色外观。

把 Mark Go Go 当作 Arc 浏览器设计团队从头设计的原生 Markdown 编辑器，重做窗口构成、导航、材质和字体，用来直接体验和评估。产品原则不变：Markdown 是主要产物，本地优先，一切皆插件，MyGo 原生界面，简单、快、读写体验好，已有功能全部保留。[internal/portrait](../../internal/portrait/) 仍是设计变量的唯一来源，[ADR 0012](0012-portrait-2-coss-reference-and-contracts.md) 的五类契约照旧生效，只是锁定的取值换了。界面说明见 [DESIGN.md](../../DESIGN.md)。

## 参考来源

来源标记 `arc@adr-0013` 指 2026 年 11 月 23 日读取的三份参考，只吸收设计语言与模式，不拷贝源码：

- [open-design 的 Arc 设计系统](https://github.com/nexu-io/open-design/blob/main/design-systems/arc/DESIGN.md)：磨砂面加一条主题渐变、侧栏优先、8 与 12 至 16 的圆角、少用边线、命令栏输入框。
- [shadcn.io 的 Arc 设计系统](https://www.shadcn.io/design/arc)：品牌蓝 `#3139fb`，以及"电压只出现一次"的克制用法。
- [Blake Crosley 的 Arc 设计解析](https://blakecrosley.com/guides/design/arc)：侧栏取代标签栏、Spaces、命令栏、界面重量与任务相称。

## 决定

| 方面 | 原 Portrait | 本实验 |
|---|---|---|
| 窗口 | 白色窗口，左右是玻璃面板 | 带颜色的布景上浮着一张白色纸页；侧栏直接在布景上 |
| 文档标签 | 顶部一行，放不下时进溢出菜单 | 侧栏里的竖排标签 |
| 文档界面 | 标签行、工具行、状态行共 104 点 | 纸页顶部一行 40 点 |
| 查找与命令 | 溢出菜单里的标签搜索；功能分散在各处按钮 | 命令栏 `⌘K` / `⌘T` 搜索原文、文件、标题和命令 |
| 工作区 | 一个名字和一个菜单 | 每个工作区一种布景，底部色点切换 |
| 研究面板 | 右侧玻璃面板 | 并排的第二张纸；窄窗仍是玻璃抽屉 |
| 标题栏 | 系统标题栏 | macOS 隐藏标题栏，红绿灯落在侧栏顶行 |
| 材质 | 渐变凸起面、多层内外阴影、带高光的玻璃 | 平面、发丝线、单层柔和投影；玻璃只用于浮层 |
| 字体 | IBM Plex Sans，标题 Barlow Semi Condensed ×1.05 | Inter，标题 Inter Display，字间距随字号收紧 |
| 强调色 | Teal `#2f6f62`，同时表示成功 | 蓝 `#3139fb`；成功是独立的绿 `#197a49`，Info 并入强调色 |
| 圆角 | 4、6、8、12、14，轨道 9 / 分段 7 | 4、6、8、12、16，轨道 10 / 分段 8 |

契约随之更新：C1 增加来源 `arc@adr-0013` 并登记 `Spaces`；C2 锁定新的颜色、材质、玻璃、圆角与布景取值；C3 增加 `Success` 色调，并检查每种布景两端上 Ink ≥ 7、Ink2 ≥ 4.5、选中行上 Ink3 ≥ 4.5；C4 的圆角集合换成新阶梯；C5 改为 `Info == Accent`、`Success` 与 `Accent` 色相差 ≥ 60°、纸页圆角等于 `Radii.L`、侧栏行高等于 `Sizes.Large`、同一根路径总得到同一种彩色布景。原生包的字面量 allowlist 因外壳改用设计变量而下调。

## 从第一性原理保留或拒绝的部分

- 墨色不换成 Arc 的灰度：`#8c8c93` 在白底上只有 3.3，达不到正文对比度。
- 强调色不随布景变化：正文链接、焦点环和主按钮必须在每种布景上都通过 C3，一种经过验算的蓝比六套配色稳妥。
- 布景上不用 Ink3：它在六种布景上只有 3.5 至 4.7。
- 控件高度不放大到 Arc 的尺寸：编辑器的信息密度比浏览器侧栏高。
- 不做悬停滑出侧栏、双原文分屏和标签自动归档：它们改变文档模型或需要框架没有的指针追踪，超出一次视觉与交互实验的范围。
- 右键菜单与应用菜单继续用 macOS 原生菜单。

**Considered Options**
- 只换颜色和圆角，保留顶部标签：成本低，但 Arc 的核心是把导航移到侧面并让内容成为唯一实体，换皮体验不到这一点。
- 让强调色跟随布景：更像 Arc，但 Portrait 的色调、材质和契约都在包初始化时定值，改成按窗口取值要重写全部控件，并让对比度检查乘以布景数量。
- 用系统字体代替打包的 Inter：最贴近 macOS，但 Linux 与 Windows 上字形和度量不同，测试也不再确定。
- 把命令栏做成独立窗口或原生菜单项：`ui.DialogBase` 加玻璃材质已能做到浮层、键盘导航和焦点归还，不需要新窗口。
