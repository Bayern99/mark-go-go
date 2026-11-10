# ADR 0003：经 ACP 调用用户自己的 Agent 生成 GenUI 视图

- 状态：已采纳（2026-11-07，#7）
- 依据：ADR 0002（视图文件是 app 与外部 Agent 之间的接口）、#5 规格中“生成”一节、2026-11-07 对 `agy acp` 1.5.2 的实测

Generate 默认通过 Agent Client Protocol（ACP，JSON-RPC 2.0，按行读写标准输入输出）启动用户已经装好并登录的 Antigravity（`agy acp`）。这条默认路径复用用户的账号、额度与模型配置，app 不保存其上游密钥。Agent 在原文副本的临时目录里写出 `out.json`；宿主检查外层结构、补上元数据和来源哈希，把完整候选交给页面校验，通过后才原子写入视图文件（旧视图成为撤回备份）。原文从不被这个功能写入。ACP 是默认生成执行路径，不限制其他独立智能能力选择更合适的接口。

## 决定

- **协议与命令。** 宿主用 Go 标准库实现最小 ACP 客户端。每个进程只做一次 `initialize`：报告客户端身份与版本，不声明 fs 与 terminal 能力，并核对 Agent 协商的协议版本；该次应答的 `agentInfo`、`authMethods` 与 capabilities（含 `sessionCapabilities.close`）随进程保留，复用进程时沿用这份真实握手结果，不重复 initialize，也不伪造 ACP 行。每次运行在独占租约上，以新的运行目录调用 `session/new`，得到非空且不同于该进程此前用过的 Session ID；再按本次运行捕获的设置 `session/set_config_option`（模型非空时 `configId: "model"`；思考级别只在 `session/new` 的 `configOptions` 确实提供该值时才发）→ `session/prompt`。处理 `session/update` 通知和 `session/request_permission` 请求，取消时发 `session/cancel`；Agent 发来的其他请求一律回 method-not-found。取消在提示轮应答时结束；1 秒内没有应答则有界杀死进程，不等固定 2 秒或进程自然退出。只有 Agent 声明 `sessionCapabilities.close` 且 `session/close` 在限期内应答的会话才算关闭，进程才可能回到空闲缓存。命令是设置里的 `agentCommand`（默认 `agy`），只加一个参数 `acp`，从不加 `--dangerously-skip-permissions` 之类的参数。
- **隔离。** 每次运行一个新的临时目录，里面只有 `source.md`（点 Generate 那一刻页面上的文本，含未保存的修改；局部更新另有当时视图的 `view.json`），`session/new` 的 `cwd` 就是它。ACP 进程在缓存自己的稳定私有目录（`markgogo-acp-*`）里启动；进程目录不随运行创建或删除，也不与任何运行的来源快照共用。提示词把这份完整文本（局部更新再加当前视图原文）直接内联一次，说明两个快照文件只是副本、不要读，并给出经符号链接解析后的绝对输出路径：只准写该 `out.json`、不联网、不用其他工具。2026-11-10 的真实 AGY 运行曾把 `./source.md` 当作 `/source.md` 报告；绝对路径消除这处契约歧义，权限检查仍使用同一规范化路径规则。运行结束（成功、失败、取消）后删除整个运行目录；进程目录保留到缓存关闭（应用退出、生成任务清理完成后）时删除。
- **两道防线，同一条规则。** 允许的只有：读 `source.md` 或这个目录本身，写 `out.json`，以及思考。规则对路径先规范化（`../`、目录符号链接、指向外部的文件符号链接都解析后再比）。
  1. `session/request_permission`：符合规则选 allow_once，否则选 reject_once。
  2. 每个 `tool_call` / `tool_call_update` 通知都过一遍同一规则；一旦不符合，立刻 `session/cancel` 并结束 Agent 进程，这次运行失败，视图文件不变。
- **宿主只信任自己算出的东西。** `out.json` 必须是普通文件（不是符号链接）、不超过 1 MiB、是 JSON 对象，有非空的 `a2ui` 数组和合法的 `sources`。宿主丢掉 Agent 写的其他顶层字段，自己写 `version`、`generatedAt`、`model`、`request`，并用发给 Agent 的那份文本按页面给出的章节范围（UTF-16 偏移）计算每个来源的 `fnv1a32` 哈希。找不到对应章节的来源不写哈希（页面上显示为“原文找不到”）。宿主不做 A2UI 与组件目录的深层校验，也不复制这套规则：它在写入前把完整候选交给页面，由页面的 `Catalog` 与 `MessageProcessor` 判定，失败即不写。
- **原子写入。** 写入前先做候选校验：宿主把将要落盘的完整候选（整份文件字节，含末尾换行与局部更新合并后的历史）作为单个字符串参数交给页面 `window.__mgoValidateGeneratedView`（`mygoHost.ts` 安装的 `validateGeneratedView`，Go 的值经 JSON 引号转义，模型文本不会变成代码）。页面拒绝、桥未安装、窗口或页面不可用、页面抛错或调用取消时，视图与撤回备份都不变，失败按 `invalid-output` 报告。通过后先写同目录的临时文件，`fsync` 后 `rename` 到 `.<name>.md.genui.json`，并把被替换的视图留作备份；页面经已有的视图文件监听看到新视图。
- **提示词的内容。** 页面把模型契约（消息与来源形状）后接组件目录的实际 JSON（`catalogDefinition()` 的紧凑形式）作为格式数据交给宿主，宿主把它接在固定的隔离指令后面；文档的完整文本、局部更新时的当前视图，都内联一次并用标记分隔。读者生成要求单独标出，并存进视图文件的 `request`。提示词带通用内容规则：保留名称与拼写/大小写、日期、数值、单位、条件与 if/else 分支优先级，覆盖跨章节的全部相关材料，共享属性用 ComparisonTable、有序阶段用 Timeline，Checklist 只写文档明说的完成状态，来源 heading 照抄原文（含编号与标点）。`view-format.md` 是读者可见的格式文档，不再整份发给 Agent。
- **设置。** `{model, agentCommand}` 存在 app 用户数据目录的 `settings.json`。`model` 为空表示不发 `set_config_option`，用 Agent 自己的默认模型；写错的模型名报错时列出 Agent 提供的模型。
- **从 Finder 启动的 PATH。** 打包后的 app 从 Finder 启动时 PATH 很短，命令先按 PATH 找，再找 `~/.local/bin`、`/opt/homebrew/bin`、`/usr/local/bin`；也可以在设置里填绝对路径。
- **局部重新生成（#12）。** 临时目录里多放一份 `view.json`（当前视图原样），只读。Agent 只写一条 `updateComponents`；合并由宿主完成：追加到 `a2ui` 末尾，来源只增不改，哈希全部重算。这是宿主的外层与合并检查，不是第二套组件规则：宿主核对更新只动了被要求替换的 id 和新 id、是视图的 surface。否则 Agent 可以借「局部」之名改掉整个视图，而页面已经无法区分。组件属性与目录的深层校验仍由页面完成：宿主把合并后的整份候选交给同一个校验器，通过才提交。
- **撤回一次（#12）。** 候选校验通过后、写入新视图前，把当前视图保留为 `.<name>.genui.prev.json`（权限与视图相同），撤回是一次 `rename`。校验失败或写入失败时备份保持原样。局部更新在写入前核对视图文件还是 Agent 开始时的那份，变了就不写。

## 进程复用（2026-11-10 实测）

应用内最多保留一个空闲 ACP 进程，每次运行以独占租约使用一个进程。忙碌时已有文档可各自启动临时独立进程；每次运行结束时缓存仍只保留一个空闲进程。空闲默认两分钟，没有后台保活、心跳或可复用对话。可执行文件或环境改变（按环境指纹比较）、进程已退出、会话 ID 为空或重复、会话关闭未获确认、工具调用越界或其他不确定状态都会丢弃进程，不做静默重试。

缓存只保留进程和它的真实握手。每次运行仍建立自己的会话、运行目录与来源快照，模型与思考级别按该运行捕获下发；主要实测收益来自新 Session 的热连接准备。观测记录 `agent.reused` 与 `initialize.cached`（带原进程 PID、启动时间与真实握手答案），本次 `session/new`、配置与 `session/prompt` 仍是实际线上交换。

选择依据（同一机器、同一 `agy acp`、真实 ACP 链路）：三个新进程的首次会话为 3.16–3.51 s，同一进程上的六次全新会话为 0.25–0.70 s；`session/close` 被声明并在约 0.18–0.20 ms 内应答，且不结束进程。取消同样不等待进程退出：提示轮应答后，只有会话关闭被证明才把进程放回缓存；1 秒内无应答则有界杀死进程。每个运行的临时目录无论结果都删除，进程目录到缓存关闭时删除。详见[完成报告](../acp-genui-completion.md)。

最终三题两次对照中，实际冷连接 7 次、热连接 5 次的设置阶段均值为 3.347 s 与 0.974 s；但包含首次冷启动的复用策略六次完整生成均值为 21.673 s，高于逐次冷启动六次的 16.383 s。本决定保留缓存以降低已重复测得的连接准备代价，不据此宣称整体生成更快；LLM/工具阶段的耗时需独立判断。该次真实取消未及时应答，1.015 s 后丢弃进程，符合不复用不确定会话的规则。

## 2026-11-10 重评：执行方式与智能能力分开

保留默认 ACP 整份生成、原文权威、宿主提交和 React/A2UI 渲染。先在现有生成与来源接口内改善任务要求、证据选择和定位；不因标题来源 ID 覆盖率低而建立独立 EvidenceIndex、文档图谱或 ViewPlan 运行时。规划提示实验不能当成已实现的增量编排。

Jev 等决策模型适合对固定候选证据或布局选项作判断，LLM 负责开放式阅读、综合与内容生成。若采用外部选择器，其输入是任务与候选、输出是候选 ID 和判断元数据；不让供应商格式进入文档模型或渲染器。保留本地选择实现。直接模型 API 可以成为明确配置的替代生成实现，但必须复用同一来源、校验、版本与提交契约；本轮 OpenDesign API 对照不计作 ACP 生产成功，也没有新增生产 Provider 平台。

A2UI Express 属于可替换的推理输出格式，经确定性编译后仍进入同一 A2UI 视图契约。是否采用由同任务、目录、预算下的真实生成和端到端结果决定；不把少 token、编译重放或公开 demo 倍数当作收益证明。当前保留现有应用格式为默认。

依据：[ReadBench 执行契约](../research/readbench/execution-contract.md)、[失败原因复核](../research/readbench/failure-analysis.md)、[决策模型消融原始结果](../research/readbench/results/jev-v2/ablation.json)。

## 局限（必须知道的）

- **隔离不是沙箱。** 实测（2026-11-07）：本机 `~/.gemini/antigravity-cli/settings.json` 里 `allowNonWorkspaceAccess: true`、权限预设为 TURBO 时，`agy acp` 写工作区外的文件、执行 shell 命令都**不会**先发 `request_permission`。第二道防线只能在 Agent 报告这次工具调用时停下整次运行，那一次调用可能已经发生。真正的限制取决于用户在 Antigravity 里的权限设置；app 不改那些设置，也不绕过它们。
- 临时目录只是让 Agent 默认看不到别的文件，并不阻止它访问绝对路径。
- Agent 的输出被当成数据：只渲染组件目录里的组件，不执行任何 HTML 或脚本（ADR 0002）。
- 超时、错误分类、退出清理在 #8。撤回只有一份备份：连续生成两次后，只能回到第二次之前。

## 放弃的方案

| 方案 | 放弃原因 |
|---|---|
| app 直接调用模型 API | 要管理用户的 API key 和额度；用户已经有 Antigravity 账号 |
| 让 Agent 直接写视图文件 | 要给 Agent 原文所在目录的写权限；哈希和元数据会由 Agent 随意填写 |
| 声明 ACP fs 能力，由宿主代读代写 | `agy` 用自己的工具读写文件，不走客户端 fs；声明了也不能阻止它 |
| 只靠 `request_permission` 判断 | 实测在常见的用户设置下根本不发这个请求 |
| 用 macOS 沙箱（`sandbox-exec`）包住 Agent | 已废弃的接口；`agy` 要访问网络和自己的配置目录，配置规则不稳定 |
