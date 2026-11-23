# ADR 0010 · Portrait 在 MyGo 基础组件上自建

Status: accepted

> Note: 以 WebUI 为唯一视觉基准的规则已由 [ADR 0012](0012-portrait-2-coss-reference-and-contracts.md) 修订：设计变量可来自 `webui@c6504af`、`coss@b2336c4` 或 `portrait-derived`，并由五类自动化契约守护；WebUI 来源的变量仍锁定 c6504af。在 MyGo 基础组件上自建、沿用 Phosphor 与官方 glass 插件的决策继续有效。

原生端的 Portrait 以退役 WebUI 的设计变量为视觉基准（固定提交 [c6504af 的 web/src](https://github.com/Bayern99/mark-go-go/tree/c6504af20b2129486bc05ee4db314f8e5fb555d2/web/src)），目标是观感与节奏等价，允许像素差异。Portrait 在 MyGo 不带外观的基础组件上自建，图标沿用已打包的 Phosphor；半透明材质使用与框架同一上游提交的官方 glass 插件。框架做不到的效果先用最接近的做法替代，并逐项记成 issue，写明自行实现的成本与收益。

**Considered Options**
- 采用 lobe-mygo、Onevra 等第三方组件库：组件和动效现成，但各自带有一套设计语言（Lobe UI、shadcn、Nuxt UI）和 Lucide 图标，会取代 Portrait 与 Phosphor。可以阅读其 MIT 源码作参考，不引入依赖。
- 修改 vendored 框架补齐缺口：维护成本落在本仓库，且偏离上游。只有在 issue 评估认为效果值得时才做。
