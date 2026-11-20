# ADR 0003 · 通过 ACP 生成 GenUI

默认经 ACP 使用用户已配置的 Agent，复用其账号、模型和额度。宿主按运行准备原文与视图快照，由 Agent 产出候选，宿主校验后提交。取消或失败时保留现有视图。主要实现见 [generate.go](../../internal/acp/generate.go) 和 [update.go](../../internal/genui/update.go)。

复用已确认关闭会话的空闲进程，以减少握手耗时。每次生成建立新会话并使用独立运行目录。复用规则见 [process_cache.go](../../internal/acp/process_cache.go)。

临时目录、ACP 权限回调和越界检测依赖 Agent 配合。Agent 的文件和网络访问权限由本机配置决定。
