# ADR 0012 · Portrait 2.0 的基准来源与五类自动化契约

Status: accepted (amends ADR 0010's single WebUI baseline)

> Note: 分支 `t3/arc-inspired-editor-redesign` 上的 [ADR 0013](0013-arc-experiment-scenery-page-command-bar.md) 是一次未合并的实验：五类契约照旧，锁定的取值、色调别名与圆角阶梯按该 ADR 更新，并新增来源 `arc@adr-0013`。

Portrait 的设计变量不再只以退役 WebUI 为唯一基准。每个导出变量登记三种来源之一，并由五类自动化测试守护；WebUI 来源的变量仍锁定 c6504af，新增的组件语义参考 coss ui，Portrait 自行推导的变量以不变量约束。整体仍是纯 Go 与 MyGo Native UI，[internal/portrait](../../internal/portrait/) 仍是设计变量的唯一来源。 PRD 见 #169。

## 设计变量来源

| 来源标记 | 含义 |
|---|---|
| `webui@c6504af` | 退役 WebUI 固定提交 [c6504af 的 web/src](https://github.com/Bayern99/mark-go-go/tree/c6504af20b2129486bc05ee4db314f8e5fb555d2/web/src) 中的 Portrait 浅色变量，取值照旧。 |
| `coss@b2336c4` | [cosscom/ui@b2336c45f2b7355c42678ce786b6f14d62b819be](https://github.com/cosscom/ui/tree/b2336c45f2b7355c42678ce786b6f14d62b819be)（MIT）。只吸收组件语义、状态模型与尺寸档位，不引入 React、Tailwind 或 WebView，不拷贝源码。 |
| `portrait-derived` | 由上两类变量按固定规则推导，或为 Portrait 自定的值；必须满足 C3 与 C5。 |

## 五类自动化契约

新设计变量加入 Portrait 模块时必须登记来源并通过以下测试，它们取代原来单一的 WebUI 对照表。

- **C1 来源登记** `TestEveryTokenDeclaresProvenance`：反射遍历 `Palette`、`Tones`、`Radii`、`Sizes`、`TypeScale` 的全部导出字段，任一字段未登记上表三种来源之一即失败。
- **C2 取值锁定** `TestTokensMatchTheirSource`：`webui@c6504af` 变量与 c6504af 的取值比较；`coss@b2336c4` 与 `portrait-derived` 变量与来源表中的锁定值比较。
- **C3 WCAG AA 实算** `TestTonesMeetWCAGAA`：半透明色先合成到所在底面再计算对比度。正文：`Tone.Ink` 对 Paper 与 Wash、`OnSolid` 对 `Solid` ≥ 4.5；非文字：`Border`、`Ring` 对 Paper ≥ 3.0；`Solid` 的 rest、hover、press 三态都须达标。
- **C4 封闭阶梯** `TestScalesAreClosed`：
  - 控件尺寸只有 `Sizes{Compact: 22, Regular: 28, Large: 32}`。
  - 圆角只有 `Radii{XS: 4, S: 6, M: 8, L: 12, XL: 14, Pill: 999}`，外加唯一的嵌套奇数对 `Track: 9 / Segment: 7`。现存的面板 13 归入 XL 14，零散的 7、9 归入 M 8，5 归入 S 6。
  - 反射检查所有 `TypeStyle` 与 `Reader` 的字号 ≥ 12；`Reader.Tick` 由 11 调为 12。
  - 字号表保持封闭，只新增别名 `Type.Heading = Type.Reading.In(FontDisplay)`，不新增 18/24 档。
- **C5 派生不变量** `TestDerivedTokenInvariants`：`Tones.Success == Tones.Accent`；`Soft`、`Wash` 由 `Solid` 加固定 alpha 推导；`Segment == Track - 2`；`Frame` 外圆角 == 内圆角 + 4；`ModalScrim.A <= 0.16`；`Info` 与 `Accent` 的色相差 ≥ 30°。

## 存量字面量与 Visual Gallery

- [守护测试](../../internal/portrait/contract_test.go) 对原生包中的 `Radius`、`Height` 等数字字面量使用按文件、按取值计数的双向 ratchet allowlist：allowlist 由守护正则生成，新增字面量失败，删掉字面量而未同步下调 allowlist 也失败，计数只降不升。
- 视觉验收由 Portrait Visual Gallery 承担：每个组件一个文件，CI 只跑结构与关系型像素断言（例如 Frame 放在非 Paper 底面时失败），不提交 golden PNG；本地预览与 PNG 导出用独立命令，不进入产品二进制。

## Tone 色彩

每个 `Tone` 有 `Solid`、`Ink`、`Soft`、`Wash`、`Border`、`Ring` 六槽，共五态：

| Tone | 取值 | 用途 |
|---|---|---|
| `Neutral` | `Ink #1d1d1f`、`Ink2 #48484d` 与 `Fill` | 默认文字与中性控件。 |
| `Accent` | Teal `#2f6f62`；`Success` 是它的别名 | 强调与成功。`Reader.Success` 只留给 GenUI。 |
| `Info` | 钢蓝 Solid `#2b6cb0`、Ink `#1e4e8c` | 只用于 Alert 与 Toast，不用于链接。 |
| `Warning` | Solid 与 Ink 都是深琥珀 `Warn #9a4d12` | 警告。 |
| `Danger` | 砖红 `#a83a3a`（白字对比度 6.30） | 破坏性操作与错误。 |

- 分类色 `Tags.*` 不进入 Phase 1，且与 `Reader.Charts` 解耦。
- 窄文档标签页的状态用 6px Tone 圆点与 Tooltip 表达，完整状态保留在无障碍标签中，避免挤占文件名；面板头部、列表等宽空间使用带文字的 Tone Badge。

## 材质与交互

- 按钮按控件扩展既有 `Look`：`Raised`、`Accent`、`Quiet`，新增 `Danger`、`DangerOutline`、`Link`。工具条内一律 `Quiet`。不新造 `Outline` 按钮、`InputLine` 或 `FlatCard`。
- 容器沿用 0.5px 阴影 hairline。`Card` 复用 `Materials.Sheet`，可放在 Glass 底面上（例如停靠的 Feynman 面板）；`Frame` 托盘只允许放在 Paper 底面上。
- 不可逆操作 Clear API key、Remove MCP server、Delete conversation 弹出 `Confirm`：基于 `DialogBase`，默认聚焦 Cancel，按钮右对齐为 Cancel（Raised）与确认（Danger）；Esc 或点击背景等同 Cancel；`Return *ui.Handle` 归还焦点；遮罩 `ModalScrim = RGBA(29, 29, 31, 0.12)`，不模糊。Delete note 不弹确认，由编辑器 Undo 撤销。
- 右键菜单保留 macOS 原生 `SystemMenus`。

## 包归属

- `internal/portrait/`：零领域依赖、零 IO 与 goroutine、零跨帧语义状态（只用调用方传入的 `*bool`、`*string` 与 `ui.Local`、`AnimateTo`）。设计变量、按钮、`InputFrame`、Alert、Badge、`Card`、`Frame`、`Confirm` 等外观组件放这里。
- `internal/nativekit/`：被两个及以上原生包复用的有状态组件，自动受 [守护测试](../../internal/portrait/contract_test.go) 约束。
- `internal/native<feature>/`：只有一个原生包使用的有状态组件，留在该包。

**Considered Options**
- 继续以 WebUI 为唯一基准：WebUI 没有 Alert、Badge、Confirm、多尺寸按钮等组件，新组件无处取值，只能在各包里写私有字面量。
- 照搬 coss ui 的取值与 Tailwind 色板：其语义与状态模型完整，但色彩与 Portrait 的墨色、材质不同系；只取语义与档位，色值由 Portrait 推导并以 C3、C5 约束。
- 通过 WebView 或 React 渲染 coss ui 组件：与原生界面方向相反（见 ADR 0010、ADR 0011）。
- 只靠人工视觉评审：来源、对比度和阶梯无法持续保证，改为可反射的自动化契约加关系型像素断言。
