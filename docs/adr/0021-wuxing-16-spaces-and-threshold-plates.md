# ADR 0021 · 五行十六境、门槛摄影图版与语义矩阵入口

Status: accepted (amends ADR 0013, ADR 0014, ADR 0015 and ADR 0020; builds on ADR 0012 and ADR 0019)

## Context

[ADR 0014](0014-wuxing-spaces-per-space-accent-and-dark-mode.md) 确立了八个五行 Space 与 16 套浅深外观的色彩系统，但在进一步深化五行体系与视觉图版时暴露出四类结构边界：

1. **五行层次不完整且存在反局色**：原八境将五单行与三组组合并列，未展开十天干阴阳与完整五相生环；生产代码中水行（`Water`）与 `MetalWater` 深色仍含紫调（`#EDE3E7` / `#D8BFD8` / `#2E317C` / `#302F4B` / `#B8A5D2` / `#22202E`），偏离先秦「水黑（玄青/靛/墨）」正色；早期 742 色扩展草稿中存在染色浅色纸底（在生产共享墨色 `Ink3 #6E6E73` 下 9–13 套跌破 WCAG 4.5:1）、彩色 `Ink3` 与反局色（戊土/土生金深色燥红 `#482522`、丁火过粉 `#F6DCCE`、己土刺眼 `#C9E079`）。
2. **摄影图版与真实编辑窗口的空间方向相反**：定稿的 16 境 × 浅深双版 = 32 张摄影母图采用「左 ~58% 低频静区 + 右 ~42% 视觉主体」的自然大画幅构图；而 Mark Go Go 真实默认窗口（`1280×820`）为左停靠侧栏（`248pt`，`19.4%`）+ 右悬浮不透明纸页（`1019×804`，`78.1%`），四周干净布景仅 `2.5%`（右缘 `8pt`），且宽窗开启 Feynman 时右侧停靠第二张研究纸页。若将照片铺作全窗布景，主体会被纸页盖住，镜像则把复杂照片垫在每日必读的文件树文字背后，直接破坏 C3 布景对比度契约；若把右缘拉宽成 `74pt` 书口，既与研究分屏冲突，又只露出画面最静的 6% 边条。
3. **扩容即破坏旧工作区与底栏布局**：`SpaceFor(root)` 使用 `FNV-1a % len(Spaces)`，直接把数组从 8 扩到 16 或重排会导致部分未手动保存的工作区静默换境；移除旧 5 单行 ID 且不做别名映射会让已保存偏好失效；侧栏底栏 8 颗色点扩展到 16 颗后会在默认宽度折 2 行、最窄宽度折 3 行，挤压文件树高度。此外存在两处现存体验缺口：欢迎页「最近工作区」色标直接调用 `SpaceFor(r.Root)` 而忽略用户手选偏好（`internal/nativeui/document.go`）；命令栏 `Space:` 仅索引英文 `Label` 而无法按中文气势名检索（`internal/nativeui/command.go`）。
4. **原生图像与动效框架边界**：MyGo `DecodeBitmap` 支持 PNG/JPEG/WebP/GIF/BMP（不支持 AVIF），`DecodeBitmapScaled` 先全量解码再缩放且不保留源数据（对其调用 `ReleasePixels` 不释放像素）；32 张母图 PNG 合计约 55.7 MiB（展开 RGBA 约 192 MiB），严禁 `go:embed` 或在打开设置时同步解码全部原图；Retina 2× 下 `1672×941` 按 `Cover` 铺满窗口或整纸分别放大 1.74× 与 1.71×（扣 40pt 顶栏约 1.62×）。动效上，全局 `Motion.Switch (160ms)` 由测试锁定并驱动控件，MyGo `animateColors` 不插值 `LinearGradient` 停点也不动画文字色，且 `portrait.Space` 必须保持可比较的纯值结构体、主题缓存键仅为 `(Name, dark)`。

> 文档同步原则：按 `AGENTS.md`「以代码、测试和配置确认当前行为」，[DESIGN.md](../../DESIGN.md) 与 [GLOSSARY.md](../../GLOSSARY.md) 的现在时产品描述不提前在 Phase 0 改写，而是随各实施切片落地同步更新。

## Decision

### 1. 封闭集合：五行十六境（32 套浅深外观）与 5 列语义矩阵顺序

`portrait.Spaces` 由 8 境扩充为 **16 境封闭集合**，按「太和 + 阳干行 + 阴干行 + 相生行（按子行所在列对齐）」排序：

- **Row 0 · 太和（1）**：`FiveElements`
- **Row 1 · 五阳干（木火土金水）**：`JiaWood`、`BingFire`、`WuEarth`、`GengMetal`、`RenWater`
- **Row 2 · 五阴干（木火土金水）**：`YiWood`、`DingFire`、`JiEarth`、`XinMetal`、`GuiWater`
- **Row 3 · 五相生环（按所生之「子」列对齐：木火土金水）**：`WaterWood`、`WoodFire`、`FireEarth`、`EarthMetal`、`MetalWater`

`Spaces[0]` 固定为 `FiveElements`（`DefaultSpace`）。字段保持 `Name`、`Label`、`Element` 三个既有显示/标识字段，并新增不可变纯值分类字段 `Polarity`（`Harmony` / `Yang` / `Yin` / `Cycle`，用于语义分组、搜索过滤及 Phase 3 可选缓动曲线）：

- `Name`：稳定持久化 ID（沿用 PascalCase，永不直接展示），保留 `FiveElements`、`MetalWater`、`WoodFire` 原样不变。
- `Label`：简短英文界面名（如 `Jia Wood`、`Metal–Water`），遵守 ADR 0014 英文主标签规则。
- `Element`：中文「干名/相生名 · 气势双字名」（如 `甲木 · 植立`、`金生水 · 淬清`，保留「生」字表达相生方向；`FiveElements` 为四字特例 `太和 · 五行全备`）；具体物象名（参天直干、苍松、灯烛等）仅留在资产清单中，不进入任何产品界面。
- 搜索与无障碍：命令栏 `Space:` 与 Settings 搜索将 `Element`、拼音及阴阳极性写入既有 `keywords` 字段并打通外层行过滤，支持中英文与气势名直搜。

### 2. 冻结旧 8 槽自动分配池、5 个只读阳干别名与精确解析优先级

- **5 个只读旧 ID 别名**：退休的五个单行 ID 通过封闭别名表解析至同元素阳干（阳干为本气）：
  - `Wood → JiaWood`、`Fire → BingFire`、`Earth → WuEarth`、`Metal → GengMetal`、`Water → RenWater`。
  - `SpaceNamed(name)` 先查 16 境规范 `Name`，再查 5 个别名并返回规范 `Space`；未知名字仍返回 `false`。
  - **不启动批量回写偏好**：读取时透明解析，仅当用户主动改选 Space 时写入新规范 ID。未改选的旧偏好在降级回旧版时零漂移；改选过新境的工作区若回退旧版本，旧版不识别新 ID 会自动退回其 8 槽哈希默认。
- **冻结旧 8 槽自动分配池**：`SpaceFor(root)` 不再对 `len(Spaces)` 取模，而是对永久冻结的旧 8 槽顺序 `[FiveElements, MetalWater, WoodFire, Earth, Metal, Wood, Water, Fire]`（长度固定为 8，与 `len(Spaces)` 解耦）执行 `FNV-1a(root) % 8`，命中后经同一别名表解析；空根 `""` 走提前返回 `DefaultSpace`（`FiveElements`）。
  - 黄金测试锁定 **8 条非空根路径 + 1 条空根**（写死预期规范 ID，并断言池长恒为 8）：
    - slot 0 (`FiveElements`)：`"/Users/ada/Books"`（非空路径实算 `% 8 == 0`）
    - slot 1 (`MetalWater`)：`"/Users/ada/Journal"`
    - slot 2 (`WoodFire`)：`"/Users/ada/Manuscript"`
    - slot 3 (`Earth → WuEarth`)：`"/Users/ada/Field Notes"`
    - slot 4 (`Metal → GengMetal`)：`"/Users/ada/Studio"`
    - slot 5 (`Wood → JiaWood`)：`"/Users/ada/Atlas"`
    - slot 6 (`Water → RenWater`)：`"/Users/ada/Handbook"`
    - slot 7 (`Fire → BingFire`)：`"/Users/ada/Ledger"`
    - 空根：`"" → FiveElements`（直返 `DefaultSpace`，不进哈希）
- **精确解析优先级（保住空根偏好 `mgo.space:`）**：
  - 带根路径的界面与 UI 导出统一按以下顺序解析 `(root, savedName)`：
    1. 若 `savedName` 经 `SpaceNamed` 有效（规范 16 ID 或 5 别名），直接返回该境——**确保无文件夹窗口手动保存的 `mgo.space:` 偏好不被空根提前短路吃掉**；
    2. 否则若 `root != ""`，调用 `SpaceFor(root)` 走冻结旧 8 槽；
    3. 否则（`root == ""` 且无有效已保存偏好），返回 `DefaultSpace`。
  - Reset 清空对应 `mgo.space:<root>` 偏好后重新走上述顺序。
  - 独立无根导出（Markdown / GenUI / Pandoc Mermaid）接受规范 ID 与 5 别名，未知名回退 `DefaultSpace`；PDF 继续保留既有显式深色导出选项（`Keep Dark Theme for PDF`）。
  - 同步修复欢迎页「最近工作区」色标仅调 `SpaceFor(r.Root)` 的现存缺陷：窗口模型填充最近工作区根路径对应的已保存偏好，经同一解析函数渲染。

### 3. 浅色纯白纸页、P0 反局清洗与 32 套 742 传统色表

- **浅色纸页锁死 `#FFFFFF`，保留共享墨色阶**：浅色 Ink `#1D1B1C` / `#48484D` / `#6E6E73`、OnAccent `#FFFFFF`；深色 Ink `#F2F3F4` / `#CDD1D3` / `#959DA2`、OnAccent `#0E100F`。不微染浅色纸页、不压深 `Ink3`，保护浅色代码面对比度（最低 4.640）与三级墨色层次。深色维持逐境深色纸页与代码面。
- **来源与取色规则**：布景两端（`From`/`To`）、强调色（`Accent`）、标记色（`Mark`）与元素色（`Success`/`Warn`/`Danger`）均取自本地整理表 `先秦基准_五行742色重组_v0.2.json` 具名条目（按视觉色相与明度择优吸附，不要求 742 表的古籍主归属标签与本境五行严格同名，不复制源库代码与资源）。`FiveElements` 与 `WoodFire` 与旧值逐字节一致，沿用 `wuxing@adr-0014`；其余 14 境（含去紫重调的 `MetalWater`）登记为 `wuxing@adr-0021`。`From`/`To` 从各图版静区采样后吸附至满足 C3 对比度的 742 具名色，令日常渐变（「气」）与门槛图版（「形」）视觉贯通。
- **P0 反局禁令（可测守卫规则，仅限定在 Space 自有字段，绝不误伤全局色）**：
  - **水行禁紫**：`RenWater`、`GuiWater`、`WaterWood`、`MetalWater` 的 `From`/`To`/`Accent`/`Mark`/`Paper`/`Code` 中，凡彩度 `HSV.S > 0.12` 者色相严禁落入 `[250°, 330°]`，并显式禁止旧水行/金生水退役紫值（`#EDE3E7`、`#D8BFD8`、`#2E317C`、`#302F4B`、`#B8A5D2`、`#22202E`、`#CCCCD6`、`#F2E7E5`）。
  - **戊土与土生金禁燥红**：`WuEarth`、`EarthMetal` 所有字段禁止 `#482522`。
  - **丙火与丁火禁过粉**：`BingFire`、`DingFire` 浅色禁止 `#F6DCCE`。
  - **己土禁刺眼荧光黄**：`JiEarth` 所有字段禁止 `#C9E079`。
  - **区分度与可接受近似记录**：16 境 `(Light.Accent, Dark.Accent)` 全部 **16 对精确互异**。已明确接受以下同类色相近对（均通过 C3，并列为弹层视觉走查项，借双语名称与选中态辨识）：浅色 `GengMetal`/`GuiWater`（OKLab ΔE×100 ≈ 2.3）、`BingFire`/`WoodFire`（3.3）、`MetalWater`/`FiveElements`（4.1）；深色 `RenWater`/`GuiWater`（2.0）、`BingFire`/`WoodFire`/`DingFire`（2.3–2.6）。
- **语义色与全局脱钩色**：`Success`/`Warn`/`Danger` 维持共享默认值，仅在对比度或 30° 色相间距需要时以 `.elements()` 覆写（`WoodFire`、`MetalWater`）。已明确接受 `JiEarth` 浅色 Accent 沿用 `#815F25`（同默认 Warn，继承旧 `Earth`）、`WoodFire` Accent 沿用 `#AB372F`（同默认 Danger，按钮按既有 `ButtonWith` 规则在色相差 ≤ 35° 时自动补预警图标）。`Reader.Purple`/`Pink`（GenUI 目录具名色）、`Important`（固定语义紫混 12% Accent）与六色 `Charts` **全部保持不动**，并在 32 套外观上复核可读性与辨识度。

#### 冻结色表（16 境 × 浅深双态 = 32 套，已按生产公式复算通过 0 失败；Go 合约测试随 Slice 1 落地锁值）

> **表注**：浅色纸页均为 `#FFFFFF`；**仅列一个 `Accent / Mark` 色值时，两者取同值（`Mark = Accent`）**；未注元素色者使用共享默认值（浅 `#1A6840` / `#815F25` / `#AB372F`，深 `#A4C2A8` / `#D8C37C` / `#F6AD8F`）；`Code` 与深色 `Paper` 为派生中性面（`portrait-derived`）。

| # | `Name` (`Label`) | `Element` | 极性 | 浅色：`From → To`；`Code`；`Accent / Mark` | 深色：`From → To`；`Paper / Code`；`Accent / Mark` |
|---|---|---|---|---|---|
| 0 | `FiveElements` (`Five Elements`) | 太和 · 五行全备 | harmony | `#057 荔肉白 #F2E6CE → #680 凝脂莲青 #C6E3E1`；`#FBF7EE`；`#734 石青 #2E5D8C` / `#232 鹅血石红 #AB372F` | `#555 玄黑 #0E100F → #741 青黛 #1A3A5F`；`#1A1A1A / #25211C`；`#561 雪青 #B0D8E8` / `#186 淡藏花红 #F6AD8F` |
| 1 | `JiaWood` (`Jia Wood`) | 甲木 · 植立 | yang | `#679 白雪藤 #E8F4F0 → #680 凝脂莲青 #C6E3E1`；`#F3F7F5`；`#471 苍绿 #223E36` | `#496 云杉绿 #15231B → #471 苍绿 #223E36`；`#151B18 / #1D2622`；`#433 粉绿 #83CBAC` |
| 2 | `BingFire` (`Bing Fire`) | 丙火 · 朗照 | yang | `#057 荔肉白 #F2E6CE → #185 介壳淡粉红 #F7CFBA`；`#FBF6F1`；`#730 汉绣红 #9E2F2F` | `#656 墨色 #1D1B1C → #180 玫瑰灰 #4B2E2B`；`#1B1717 / #251F1E`；`#122 桃红 #F0ADA0` |
| 3 | `WuEarth` (`Wu Earth`) | 戊土 · 固重 | yang | `#077 米色 #F9E9CD → #083 浅驼色 #E2C17C`；`#FAF6EE`；`#112 驼色 #66462A` | `#619 玄色 #1A1A1A → #094 橄榄灰 #503E2A`；`#1A1816 / #24201C`；`#083 浅驼色 #E2C17C` |
| 4 | `GengMetal` (`Geng Metal`) | 庚金 · 刚断 | yang | `#550 霜白 #F2F3F4 → #597 霜华 #D0E7F9`；`#F4F6F8`；`#704 黛色 #3B4F5C` | `#555 玄黑 #0E100F → #420 青灰 #2B333E`；`#16181A / #1F2226`；`#597 霜华 #D0E7F9` |
| 5 | `RenWater` (`Ren Water`) | 壬水 · 通流 | yang | `#584 月魄 #E0E7EF → #411 鸥蓝 #C7D2D4`；`#F3F6F9`；`#549 夜筵青 #1D3F5A` | `#396 钢蓝 #0F1423 → #421 鸽蓝 #1C2938`；`#14181E / #1C222B`；`#375 秋波蓝 #8ABCD1` |
| 6 | `YiWood` (`Yi Wood`) | 乙木 · 舒蔓 | yin | `#424 月白 #EEF7F2 → #498 艾背绿 #DFECD5`；`#F4F8F4`；`#728 苍筤 #2E5D4A` | `#475 莽丛绿 #141E1B → #454 苍蓝 #134857`；`#151B1A / #1D2625`；`#432 竹篁绿 #B9DEC9` |
| 7 | `DingFire` (`Ding Fire`) | 丁火 · 昭融 | yin | `#350 银白 #F1F0ED → #731 藕丝秋 #D9C6B3`；`#F9F6F2`；`#162 淡豆沙 #873D24` | `#555 玄黑 #0E100F → #182 豆沙 #481E1C`；`#1A1615 / #241E1C`；`#155 玉粉红 #E8B49A` |
| 8 | `JiEarth` (`Ji Earth`) | 己土 · 润蓄 | yin | `#350 银白 #F1F0ED → #057 荔肉白 #F2E6CE`；`#F8F6F1`；`#091 莱阳梨黄 #815F25` | `#474 苷蓝绿 #1F2623 → #554 鸦青 #424B50`；`#181A1A / #212525`；`#646 素鞸 #D6C9B8` |
| 9 | `XinMetal` (`Xin Metal`) | 辛金 · 清莹 | yin | `#516 汉白玉 #F8F4ED → #519 珍珠灰 #E4DFD7`；`#F7F6F4`；`#049 燕羽灰 #685E48` | `#619 玄色 #1A1A1A → #404 水牛灰 #2F2F35`；`#18181A / #222225`；`#519 珍珠灰 #E4DFD7` |
| 10 | `GuiWater` (`Gui Water`) | 癸水 · 润微 | yin | `#602 霜色 #E8F4F8 → #368 远天蓝 #D0DFE6`；`#F4F7F9`；`#393 鲸鱼灰 #475164` | `#422 钢青 #142334 → #704 黛色 #3B4F5C`；`#15191E / #1E242C`；`#369 星蓝 #93B5CF` |
| 11 | `WaterWood` (`Water–Wood`) | 水生木 · 滋生 | cycle | `#602 霜色 #E8F4F8 → #680 凝脂莲青 #C6E3E1`；`#F2F7F7`；`#451 玉鈫蓝 #126E82` | `#736 墨黑 #0F1A20 → #471 苍绿 #223E36`；`#141A1A / #1C2524`；`#705 松花绿 #A0D6B4` |
| 12 | `WoodFire` (`Wood–Fire`) | 木生火 · 敷荣 | cycle | `#424 月白 #EEF7F2 → #185 介壳淡粉红 #F7CFBA`；`#FBF6EE`；`#232 鹅血石红 #AB372F`（Warn `#147 岩石棕 #964D22`） | `#475 莽丛绿 #141E1B → #182 豆沙 #481E1C`；`#1B1817 / #25201E`；`#186 淡藏花红 #F6AD8F`（Success `#705 松花绿 #A0D6B4`、Warn `#083 浅驼色 #E2C17C`） |
| 13 | `FireEarth` (`Fire–Earth`) | 火生土 · 温煦 | cycle | `#584 月魄 #E0E7EF → #057 荔肉白 #F2E6CE`；`#F9F5F0`；`#147 岩石棕 #964D22` | `#406 燕颔蓝 #131824 → #648 缁絇繶纯 #2E2014`；`#1A1715 / #24201C`；`#013 香水玫瑰黄 #F7DA94` |
| 14 | `EarthMetal` (`Earth–Metal`) | 土生金 · 润莹 | cycle | `#643 素衣 #F2F0E6 → #699 杏子灰 #D9CBB8`；`#F7F6F3`；`#094 橄榄灰 #503E2A` | `#555 玄黑 #0E100F → #474 苷蓝绿 #1F2623`；`#171817 / #202221`；`#350 银白 #F1F0ED` |
| 15 | `MetalWater` (`Metal–Water`) | 金生水 · 淬清 | cycle | `#584 月魄 #E0E7EF → #368 远天蓝 #D0DFE6`；`#F3F5F7`；`#401 海涛蓝 #15559A`（Warn `#049 燕羽灰 #685E48`） | `#736 墨黑 #0F1A20 → #422 钢青 #142334`；`#15181C / #1D2229`；`#579 缥色 #A8C9E4`（Warn `#067 银鼠灰 #B5AA90`） |

### 4. 门槛摄影图版（方案 B 增强版）与离线资产管线

严守「窗口是一个房间，纸页是其中唯一的实体」立场：32 张摄影图版仅作为**「开卷、选境」门槛影像**，绝不铺作日常编辑纸页、侧栏文件树或研究分屏的背景：

1. **无文档欢迎舞台（`doc.ID == ""`）**：
   - 保持纸页圆角矩形几何不变，顶部 `40pt` 页栏保持不透明纸色作为「天头」（拖窗区、按钮与路径对比度零变动）。
   - 页栏下方铺设当前 `(Space, Light/Dark)` 图版（`Fit(Cover)` + 按境登记的 `ObjectPosition` 焦点），绘制图版时强制不透明（绕过 `shell.go` 半透明分支）。
   - 欢迎文案、操作按钮与最近工作区列（最大 `460pt`）放进一张不透明纸卡作为「签条」，停靠在左静区一侧——所有文字对比度仅取决于纸色，无需逐像素验算 32 张照片。
   - **响应式、分屏互斥与可访问性退化**：直接由外壳几何算出扣除 `40pt` 天头后的舞台可用尺寸（避免 `Bounds()` + `Invalidate` 造成一帧闪烁）。仅当 `!researchDocked` 且可用尺寸满足 `availW >= 720 && availH >= 520` 且未开启 `HighContrast` 且图版就绪时启用摄影舞台；任一条件不满足（含超宽窗停靠研究面板、窄窗或缺失图版）一律**退回既有居中不透明纯纸欢迎页**（纸页外的窗口布景继续显示当前 Space 渐变）。
   - 已有 `doc.ID` 的未命名原文继续直接进入纯纸编辑器。
2. **Settings / Appearance 5 列语义画廊**：
   - 将 [space_card.go](../../internal/portrait/space_card.go) 的 `32pt` 渐变预览升级为接近 16:9 比例的摄影缩略图画幅（保留纸色片、标记点与强调色胶囊；名称、选中勾留在图外）。
   - 明确布局：**顶部 Row 0 为 `FiveElements` 通栏卡片**，其下为 **3 行 × 5 列语义矩阵**（阳/阴/生 × 木火土金水）。将现有列数上限从 `min(4, …)`（`internal/nativeui/settings_native.go:317`）提升至 `5`；由于设置内容列 `720pt` 扣两侧 `32pt` 外边距后为 `656pt`，再进入 `SettingsGroup` 扣除发丝边框与 `SettingsRow` 两侧各 `16pt` 内边距后，网格实际可用净宽约为 **`622pt`**，因此将卡片最小宽与列距调整为满足实际净宽的阈值（如单卡最小 `116pt`、列距 `8pt`，`5×116 + 4×8 = 612pt ≤ 622pt`；或沿用 `10pt` 列距配 `114pt` 单卡，`5×114 + 4×10 = 610pt ≤ 622pt`），确保默认 `1040×720` sheet 真正渲染 5 列，窄 sheet 按组自适应折行。
3. **侧栏底栏语义弹层（修订 ADR 0015 chrome 例外）**：
   - 废除 8 颗色点横排；底栏收为 **1 颗「当前境」色块按钮**（无障碍 `Label("Space")` + 描述 `<Label> · <Element>`，避免与弹层和设置卡片的 `Space: <Label>` 选项同名冲突）+ 昼夜切换按钮，为文件树释放稳定高度。
   - 点击当前境按钮打开 Portrait `Popover`（作为「打开后直接选境」的高频快捷切换例外，搜索与 Reset 唯一归属 `Settings / Appearance`）：第一行太和，下方 `3×5` 语义色块网格，采用 MyGo 原生 `RoleRadioGroup` / `RoleRadio`（每行分组，选项提供英文名、中文气势名与选中态）。
   - **完整键盘契约**：打开时初始焦点落在当前境；`← / →` 同行移动，`↑ / ↓` 同列移动（太和行按 `↓` 落入上次或默认第 1 列，阳干行按 `↑` 落回太和）；`Home / End` 跳至首末项；`Enter / Space` 提交切换并关闭弹层、归还焦点至触发按钮；`Esc` 取消且不写偏好、关闭弹层并归还焦点至触发按钮。
4. **资产管线与内存硬约束**：
   - **格式与打包（锁定 JPEG q88，零外部编码依赖）**：离线资产脚本将 32 张锁版母图统一规范为 16:9（其中 `ding_fire` 原图 `1536×1024` 保全宽、上下均衡各裁 `80px` 得 `1536×864` 保留完整竖直光隙并经人工目检签收；`wood_fire_dark` 原图 `1671×941` 补齐至 `1672×941`），输出 **64 个衍生文件**——32 张大图（`<base>_{light,dark}.jpg`，JPEG q88）+ 32 张预生成缩略图（`<base>_{light,dark}_thumb.jpg`，`640×360`，JPEG q85），放入 `resources/plates/`（随 MyGo `resources/` 机制打包，图像载荷随外部资源分发而非 `go:embed` 进二进制），并在 `resources/licenses/` 记录图像生成与授权说明；仓库内总增量控制在 ~5–8 MiB（免 LFS）。
   - **架构隔离**：`portrait.Space` 保持纯数据可比较值结构体，仅登记资源相对名（由 `snake_case(Name)` 推导）与每境 `ObjectPosition(x, y)` 焦点；图版解码与缓存器放在 `portrait` 之外（`internal/plates` 包），运行时从 `PathResources/plates` 读取，单元测试通过 `fs.FS` 注入（CI 资源完备性测试显式定位仓库 `resources/plates`）。
   - **内存与主线程守卫**：画廊只读取预生成 `640×360` 缩略图（16 张解码约 14.7 MiB），严禁主线程同步解码全尺寸原图；大图经 `DecodeBitmap` 加载，显示上传后在主线程调用 `ReleasePixels()`（注意不可对 `DecodeBitmapScaled` 依赖 `ReleasePixels`）。常驻预算：**全进程大图槽位 ≤ 2 张/每窗（上限 4 张）**，离开欢迎态、切境或关窗时释放引用，以**已解码位图计数**写入自动化预算测试。

#### 每境图版裁切焦点清单（运行时 `ObjectPosition(x, y)`，浅深成对校验）

| `Name` | 资产基名 | `ObjectPosition(x, y)` | 离线规整与构图保护要点 |
|---|---|---|---|
| `FiveElements` | `five_elements` | `(0.58, 0.62)` | 1672×941；保中右双青峰与云海纵深 |
| `JiaWood` | `jia_wood` | `(0.75, 0.45)` | 1672×941；保右侧五株笔直杉干 |
| `BingFire` | `bing_fire` | `(0.65, 0.55)` | 1672×941；保中右雕塑感日光积云/暖褐天光 |
| `WuEarth` | `wu_earth` | `(0.58, 0.68)` | 1672×941；保平顶沉积台地地平线与顶缘微光 |
| `GengMetal` | `geng_metal` | `(0.75, 0.50)` | 1672×941；保右侧斧凿铁灰巨崖 |
| `RenWater` | `ren_water` | `(0.55, 0.72)` | 1672×941；保下缘深海涌浪与海雾层次 |
| `YiWood` | `yi_wood` | `(0.72, 0.40)` | 1672×941；保右上柔弧细竹 |
| `DingFire` | `ding_fire` | `(0.68, 0.55)` | 1536×1024 → 上下各裁 80px 得 1536×864（人工签收光隙完整） |
| `JiEarth` | `ji_earth` | `(0.58, 0.70)` | 1672×941；保下缘雨后浅滩与映天水网 |
| `XinMetal` | `xin_metal` | `(0.72, 0.45)` | 1672×941；保右侧珠母贝壳微弧与虹彩 |
| `GuiWater` | `gui_water` | `(0.62, 0.68)` | 1672×941；保右下静水微漪与空濛雨雾 |
| `WaterWood` | `water_wood` | `(0.75, 0.45)` | 1672×941；保右侧水影与直干 |
| `WoodFire` | `wood_fire` | `(0.64, 0.65)` | 1672×941（深色 1671→1672）；保右下层叠松岭与暮霞 |
| `FireEarth` | `fire_earth` | `(0.52, 0.68)` | 1672×941；保冷天暖地地平线过渡 |
| `EarthMetal` | `earth_metal` | `(0.70, 0.72)` | 1672×941；保右下湿润盐滩与薄银矿壳 |
| `MetalWater` | `metal_water` | `(0.68, 0.62)` | 1672×941；保右侧冷岩岬与静水镜面 |

### 5. 动效边界（分期交付，拒绝伪区分与撕裂）

- **Phase 1 不做切境动效**：保持既有瞬切，优先稳定交付 16 境数据、色表、迁移与新入口。
- **Phase 2 统一布景/图版交叉淡入**：
  - 严禁修改锁定的全局 `Motion.Switch (160ms)`；严禁使用 `Transition{Colors: true}` 做换境；严禁构造同名中间 `Space`。
  - 新增独立语义变量 `Motion.Scenery`（`200ms`，`portrait-derived`；虽与 `Motion.Enter` 等值但独立命名，以便布景淡入与弹层入场解耦演进）。窗口外壳各自维护上一境背景/图版不透明度淡出层（`1 → 0`），前景 Token 照旧同帧落地。
  - **触发来源与瞬态对比度护栏**：扩展 `SetSpace` 传入触发来源（`keyboard bool`），经 `portrait.Animates(c, keyboard)` 门控：键盘切换、命令栏回车、`ReduceMotion`、跨浅深外观切换或新图未就绪时直接 `Jump` 瞬切；快速连切丢弃过期异步加载结果，关窗立即释放引用。针对浅色跨境交叉淡出期 `Accent on 旧布景` 的瞬态低对比路径（渐变中段 `27/240`、底端 `To` 色标 `49/240`，最差 `WuEarth → JiEarth` 首帧对中段 `3.8597:1`、对底端 `#E2C17C` 为 `3.3786:1`），侧栏常规文字仅用 `Ink`/`Ink2`（全 240 路径 ≥ 7 / ≥ 4.5），凡在布景上使用强调色的选中态或图标控件必须落于纸色/不透明承托面内，并在 Slice 5 显式验收 `WuEarth → JiEarth` 覆盖渐变中段与底端 `To` 的首帧、中间帧与终帧。
- **Phase 3（可选）阴阳缓动曲线**：否决 `140ms / 190ms / 160ms` 三档时长（30ms 低于人眼可感知阈值且跨阴阳切换归属模糊）。若需表达「阳健阴顺」，仅在三期按**目标境**极性切换布景淡入的缓动曲线（阳干 `Motion.EaseOut`、阴干 `Motion.EaseInOut`、太和/相生守默认），时长维持统一。

## Contracts

沿用并扩展 [ADR 0012](0012-portrait-2-coss-reference-and-contracts.md) 五类契约：

- **C1 来源**：允许列表新增 `wuxing@adr-0021`（与既有 `wuxing@adr-0014` 并存）；`Spaces.FiveElements` 与 `Spaces.WoodFire` 保持 `wuxing@adr-0014`，其余 14 个 `Spaces.<Name>`（含原在 `provenance_test.go:174` 登记于 `0014` 的 `Spaces.MetalWater`）登记为 `wuxing@adr-0021`；删除已退休 5 境登记及 `provenance_test.go:174` 中旧 `Spaces.Earth` 断言；`Colors.Paper` 维持 `wuxing@adr-0014`；Slice 5 新增 `Motion.Scenery` 时同步将 `Motion` 纳入 `provenance_test.go` 扫描根，并为全部既有字段（`Press`/`Switch`/`Expand`/`Enter`/`Menu`/`Jump`/`Morph` 及 `EaseOut`/`EaseInOut` 子字段）与新 `Scenery` 登记完整来源（同时在 `portrait_test.go` 锁定 `200ms`，保留 `Motion.Switch` 原锁值）。（`Reader` 非现有 provenance 扫描根，其具名色语义留作文档记录，不虚增未扫描条目。）
- **C2 锁定取值**：`TestSpacesMatchTheirLockedValues` 升级锁定 16 境 × 2 外观 = 32 套完整 `Scheme`；既有 `FiveElements` 默认浅深配色、材质与玻璃锁定测试继续通过。
- **C3 对比度**：全部 32 套外观全量通过 `wuxing_test.go`、`tone_test.go`、`look_contrast_test.go`（按钮三态 + 设置控件）与 `TestSceneryKeepsSidebarInkReadable`；`appearance_test.go` 收紧为断言精确 **16 对** `(Light.Accent, Dark.Accent)` 互异；新增 P0 反局色彩守卫测试（水行 HSV `[250°, 330°]` 禁区 + 退役 HEX 断言，仅作用于 Space 自有字段）。
- **C4 封闭阶梯**：既有 `Radii`、`Sizes`、`Type` 封闭阶梯不变；编辑页中心 `(255,255,255)` 白纸像素测试保持不变。
- **C5 不变式与迁移契约**：既有 `Info == Accent`、`Note == Accent`、`Success/Danger` 与 `Success/Warn` 色相 ≥ 30°、`Note/Important` 距离 ≥ 48 覆满 32 套；新增 8 条非空黄金路径 + 1 条空根 + 池长恒为 8 测试、5 别名解析测试（含「根路径落土槽但已保存 `Water`」跨入口测试）、按名查境替代下标查境、64 文件资源完备性测试与解码位图计数内存预算测试。

## Consequences

- 实施按 Parent Spec `#221` 下的 7 张垂直切片工单（`#222`–`#228`）在独立工作树推进，统一合入集成开发分支后执行 Standards + Spec 双轴审查（显式依赖 DAG：`#222 → #223`、`#222 → #224`、`#222 → #225`、`#222 → #226`、`#224 + #225 + #226 → #227`、`#227 → #228`，其中 `#223` 只依赖 `#222`、不阻塞 `#227`）；每张合入切片均以 `scripts/verify.sh`（`go test -race ./...`、`go tool mygo vet`、Windows 交叉编译）全绿为门槛，且 `DESIGN.md` 与 `GLOSSARY.md` 随对应功能切片同步更新。
- 自动分配仍收敛在旧 8 槽经别名后的 8 个默认落点（太和 + 金生水 + 木生火 + 五阳干），五阴干与其余三组相生境需用户手动挑选，避免静默打乱存量用户工作区身份。

## Alternatives considered

- **方案 A · 全窗摄影布景 + 侧栏线性遮罩**：日常编辑时 78% 面积被纸页覆盖，底图主体完全被挡；水平镜像则把复杂照片垫在每日阅读的文件树背后，破坏 C3 布景对比度契约，故否决。
- **方案 C · 宽窗条件性 56–74pt 右书口**：与 ≥1180pt 停靠的 Feynman 研究面板直接冲突，破坏锁定外壳尺寸 `PageInset = 8`，且 74pt 仅露出构图中刻意留静的 6% 边条，主体仍在纸下，故否决。
- **将 55.7 MiB PNG 直接 `go:embed` 或运行时 `DecodeBitmapScaled`**：拖慢三平台编译链接、膨胀二进制，并在首次打开画廊时触发主线程大内存解码峰值，故否决。
- **浅色纸页微染或压深 `Ink3`**：草稿染色纸导致 9–13 套浅色跌破 WCAG 4.5:1，而压深 `Ink3` 会压扁全应用三级墨色层次并压缩代码面余量，故否决。
- **阳健 140ms / 阴顺 190ms / 和合 160ms 三档切境时长**：30ms 差异低于人眼可感知阈值，跨阴阳切换归属模糊，故否决。
