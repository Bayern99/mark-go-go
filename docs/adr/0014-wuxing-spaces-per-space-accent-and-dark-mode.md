# ADR 0014 · 五行 Space：每个 Space 自有强调色，完整的浅色与深色外观

Status: experimental (branch on top of `t3/arc-inspired-editor-redesign`; amends ADR 0013 on this branch, pending evaluation)

> 五行 Space 已由 [ADR 0021](0021-wuxing-16-spaces-and-threshold-plates.md) 修订并扩充为**五行十六境（32 套浅深外观）**：含太和、十天干与五相生环、冻结旧 8 槽自动分配池与 5 个只读阳干别名、清洗后的 742 传统色表、明确浅色纸页固定 `#FFFFFF` 以及门槛摄影图版边界。

ADR 0013 的布景只是一条渐变，强调色固定为 Arc 的蓝 `#3139fb`，并且只有浅色外观。本次把布景换成八个五行（Wu Xing）Space：每个 Space 在浅色与深色两种外观下各有一套布景、纸页、代码面、强调色、标记色和元素色（Success、Warn、Danger），像 Arc 的 Space 一样，窗口取它所显示的工作区的颜色。外观（Appearance）可选 System、Light、Dark，跟随系统或固定，记在偏好文件里。界面说明见 [DESIGN.md](../../DESIGN.md)，取值见 [space.go](../../internal/portrait/space.go)。

## 参考来源

来源标记 `wuxing@adr-0014` 指本地整理表 `先秦基准_五行742色重组_v0.2.json`（742 条传统色，按五行主归属、先秦依据等级 `[P]` 等分层；它本身重组自 `nevertoday/zhongguo-traditional-colors` 的色名总表）。只取用表中具名颜色的 HEX 值和编号作为出处，不复制该仓库的任何代码、数据文件或资源；该仓库以 GPL-3.0 发布，本项目不引入其内容。纸页（Paper）与代码面（Code）是按各 Space 色相调出的中性面，登记为 `portrait-derived`。

## 决定

八个 Space 是封闭集合，界面以英文命名：

| Space | 名称 | 浅色：布景 → 强调色 | 深色：布景 → 强调色 |
|---|---|---|---|
| `FiveElements`（默认） | Five Elements · 五行全备 | #057 荔肉白 `#F2E6CE` → #680 凝脂莲青 `#C6E3E1`；石青 `#2E5D8C`，标记鹅血石红 `#AB372F` | #555 玄黑 `#0E100F` → #741 青黛 `#1A3A5F`，纸 #619 玄色；雪青 `#B0D8E8`，标记淡藏花红 `#F6AD8F` |
| `MetalWater` | Metal–Water · 金生水 | #597 霜华 `#D0E7F9` → #352 远山紫 `#CCCCD6`；石青 `#2E5D8C`，Warn 燕羽灰 `#685E48` | #736 墨黑 `#0F1A20` → #363 暗龙胆紫 `#22202E`；缥色 `#A8C9E4`，Success 缥缃、Warn 银鼠灰 `#B5AA90` |
| `WoodFire` | Wood–Fire · 木生火 | #424 月白 `#EEF7F2` → #185 介壳淡粉红 `#F7CFBA`；鹅血石红 `#AB372F`，Warn 岩石棕 `#964D22` | #475 莽丛绿 `#141E1B` → #182 豆沙 `#481E1C`；淡藏花红 `#F6AD8F`，Success 松花绿、Warn 浅驼色 |
| `Earth` | Earth · 中土 | #077 米色 `#F9E9CD` → #012 酪黄 `#F6DEAD`；莱阳梨黄 `#815F25` | #656 墨色 `#1D1B1C` → #094 橄榄灰 `#503E2A`；浅驼色 `#E2C17C` |
| `Metal` | Metal · 秋西正白 | #550 霜白 `#F2F3F4` → #351 芡食白 `#E2E1E4`；绀碧 `#4B6C8C` | #555 玄黑 `#0E100F` → 青灰 `#2B333E`；霜华 `#D0E7F9` |
| `Wood` | Wood · 春东正青 | #424 月白 `#EEF7F2` → #680 凝脂莲青 `#C6E3E1`；玉鈫蓝 `#126E82` | #475 莽丛绿 `#141E1B` → #454 苍蓝 `#134857`；松花绿 `#A0D6B4` |
| `Water` | Water · 冬北玄墨 | #309 马鞭草紫 `#EDE3E7` → #723 浅苋菜紫 `#D8BFD8`；满天星紫 `#2E317C` | #555 玄黑 `#0E100F` → #360 野葡萄紫 `#302F4B`；霜紫 `#B8A5D2` |
| `Fire` | Fire · 夏南正赤 | #258 草珠红 `#F8EBE6` → #209 淡绯 `#F2CAC9`；汉绣红 `#9E2F2F` | #656 墨色 `#1D1B1C` → #182 豆沙 `#481E1C`；桃红 `#F0ADA0` |

各外观共用的墨色：浅色 Ink #656 墨色 `#1D1B1C`、Ink2 `#48484D`、Ink3 `#6E6E73`，强调色上的字是白色；深色 Ink #550 霜白 `#F2F3F4`、Ink2 #390 银鱼白 `#CDD1D3`、Ink3 `#959DA2`，强调色上的字是 #555 玄黑 `#0E100F`。未单独给出元素色的 Space 用默认值：浅色 Success #490 荷叶绿、Warn #091 莱阳梨黄、Danger #232 鹅血石红；深色 Success #574 缥缃、Warn #578 槐黄、Danger #186 淡藏花红。完整的 HEX 与编号锁在 [五行契约用例](../../internal/portrait/wuxing_test.go)。

实现上，Portrait 的 `Colors`、`Materials`、`Tones`、`Diff`、`GenUI`、`Reader`、`Highlights` 与按钮外观都由 `(Space, 深浅)` 推导，每种组合只算一次并缓存。窗口外壳在每帧开始时调用 `portrait.Use(space, dark)`，该帧构建和绘制的一切都取这个 Space 的颜色；原生各包照旧读取 `portrait.Colors` 等变量，不必改签名。`portrait.Theme()` 在深色下以 MyGo 的 `DarkTheme` 为底，编辑器、Markdown、表格和弹层随之变深。所有窗口都在主线程上逐个构建与绘制，所以多窗口各取各的 Space 不会互相串色。

外观选择在命令栏（`Toggle Dark Mode`、`Appearance: System / Light / Dark`）和工作区菜单的 **Appearance** 子菜单里；Space 在命令栏（`Space: …`）和工作区菜单的 **Space** 子菜单里按工作区选择。System 跟随系统外观；当所选外观与系统不同时，布景画成不透明，避免系统侧栏材质透出另一种明暗。

## 契约

- **C1 来源**：增加 `wuxing@adr-0014`，登记八个 Space、强调色、纸页、Ink、OnAccent 与元素色；Chrome、Code、发丝线、填充、柔和色与焦点环是 `portrait-derived`。
- **C2 锁定取值**：八个 Space × 两种外观共 16 套布景、纸页、代码面、强调色、标记色和元素色逐值锁定；默认外观下的完整 `Colors`、材质与玻璃也锁定。
- **C3 对比度**：在全部 16 套配色上检查 Ink ≥ 7、Ink2 与 Ink3 在纸页上 ≥ 4.5、Ink3 在代码面上 ≥ 4.5、强调色在纸页、代码面和强调柔和色上 ≥ 4.5、OnAccent 在强调色的静止、悬停和按下三态上 ≥ 4.5、各色调的 Ink 在纸页与 Wash 上 ≥ 4.5、Border 与 Ring ≥ 3、布景两端上 Ink ≥ 7 与 Ink2 ≥ 4.5、选中行上 Ink3 ≥ 4.5，以及按钮外观的三态。燕羽灰 `#685E48` 与银鼠灰 `#B5AA90` 作为 Warn 在各自外观下验算通过，未改动表中取值。
- **C4 封闭阶梯**：圆角、控件高度和字号不变。
- **C5 不变式**：`Info == Accent`、`Note == Accent`；浅色下 ModalScrim ≤ 0.16、深色下 ≤ 0.5；Success 与 Danger、Warn 色相相距 ≥ 30°。原生包的字面量 allowlist 只降不升：表格的 `Height(31)` 与 `Height(36)` 已移除。

## 从第一性原理保留或拒绝的部分

- **放弃 Success 与 Accent 色相相距 ≥ 60° 的不变式。** 木的正色是青绿，Wood 的强调色与成功色天然相近；成功状态始终带对勾图标或文字，不只靠色相区分。
- **深色下选中行用纸页色而不是白色薄层。** 白色薄层叠在豆沙、橄榄灰这类较亮的深色布景上，Ink3 达不到 4.5；纸页色 85% 让选中的标签像一小片纸，与浅色外观的含义一致。
- **深色外观不是浅色取反。** 发丝线与填充改用白色薄层，投影加深，凹面改为比纸更暗，凸起与弹层比纸更亮。
- **不随系统强调色变化。** 强调色属于 Space，才能在每种组合上都通过 C3。

**Considered Options**
- 保留 ADR 0013 的固定蓝强调色，只加深色外观：改动小，但 Space 只剩一条渐变，体验不到 Arc 那样"换一个 Space 就换一种颜色"。
- 把颜色放进每个控件的参数或 `ui.Context`：最显式，但要改动几百处调用和所有控件签名；MyGo 的主题已经按子树传递，Portrait 在帧开始时切换整套变量即可覆盖全部原生包。
- 为每个 Space 只定一套颜色、深色时自动变暗：省掉一半取值，但自动变暗得不到表中具名的传统色，也无法逐套验算对比度。
