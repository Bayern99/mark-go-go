# ADR 0003：经 ACP 调用用户自己的 Agent 生成 GenUI 视图

- 状态：已采纳（2026-11-07，#7）
- 依据：ADR 0002（视图文件是 app 与外部 Agent 之间的接口）、#5 规格中“生成”一节、2026-11-07 对 `agy acp` 1.5.2 的实测

Generate 不在 app 里接任何模型 API，而是通过 Agent Client Protocol（ACP，JSON-RPC 2.0，按行读写标准输入输出）启动用户已经装好并登录的 Antigravity（`agy acp`）。账号、额度、模型都是用户自己的，app 不保存任何密钥。Agent 在一个只放了原文副本的临时目录里工作，写出 `out.json`；宿主检查外层结构、补上元数据和来源哈希，再原子写入视图文件。原文从不被这个功能写入。

## 决定

- **协议与命令。** 宿主用 Go 标准库实现最小 ACP 客户端：`initialize`（不声明 fs 与 terminal 能力）→ `session/new` → 设置了模型时 `session/set_config_option`（`configId: "model"`）→ `session/prompt`；处理 `session/update` 通知和 `session/request_permission` 请求，取消时发 `session/cancel`；Agent 发来的其他请求一律回 method-not-found。命令是设置里的 `agentCommand`（默认 `agy`），只加一个参数 `acp`，从不加 `--dangerously-skip-permissions` 之类的参数。
- **隔离。** 每次运行一个新的临时目录，里面只有 `source.md`（点 Generate 那一刻页面上的文本，含未保存的修改），工作目录就是它。提示词要求只读 `./source.md`、只写 `./out.json`、不联网、不用其他工具。运行结束（成功、失败、取消）后删除整个目录。
- **两道防线，同一条规则。** 允许的只有：读 `source.md` 或这个目录本身，写 `out.json`，以及思考。规则对路径先规范化（`../`、目录符号链接、指向外部的文件符号链接都解析后再比）。
  1. `session/request_permission`：符合规则选 allow_once，否则选 reject_once。
  2. 每个 `tool_call` / `tool_call_update` 通知都过一遍同一规则；一旦不符合，立刻 `session/cancel` 并结束 Agent 进程，这次运行失败，视图文件不变。
- **宿主只信任自己算出的东西。** `out.json` 必须是普通文件（不是符号链接）、不超过 1 MiB、是 JSON 对象，有非空的 `a2ui` 数组和合法的 `sources`。宿主丢掉 Agent 写的其他顶层字段，自己写 `version`、`generatedAt`、`model`、`request`，并用发给 Agent 的那份文本按页面给出的章节范围（UTF-16 偏移）计算每个来源的 `fnv1a32` 哈希。找不到对应章节的来源不写哈希（页面上显示为“原文找不到”）。A2UI 与组件目录的深层校验只在页面上做。
- **原子写入。** 先写同目录的临时文件，`fsync` 后 `rename` 到 `.<name>.md.genui.json`；页面经已有的视图文件监听看到新视图。
- **提示词的内容。** 页面把格式文档（`view-format.md`）和由组件目录生成的组件定义 JSON 一起交给宿主，宿主把它接在固定的隔离指令后面；读者的生成要求单独标出，并存进视图文件的 `request`。
- **设置。** `{model, agentCommand}` 存在 app 用户数据目录的 `settings.json`。`model` 为空表示不发 `set_config_option`，用 Agent 自己的默认模型；写错的模型名报错时列出 Agent 提供的模型。
- **从 Finder 启动的 PATH。** 打包后的 app 从 Finder 启动时 PATH 很短，命令先按 PATH 找，再找 `~/.local/bin`、`/opt/homebrew/bin`、`/usr/local/bin`；也可以在设置里填绝对路径。

## 局限（必须知道的）

- **隔离不是沙箱。** 实测（2026-11-07）：本机 `~/.gemini/antigravity-cli/settings.json` 里 `allowNonWorkspaceAccess: true`、权限预设为 TURBO 时，`agy acp` 写工作区外的文件、执行 shell 命令都**不会**先发 `request_permission`。第二道防线只能在 Agent 报告这次工具调用时停下整次运行，那一次调用可能已经发生。真正的限制取决于用户在 Antigravity 里的权限设置；app 不改那些设置，也不绕过它们。
- 临时目录只是让 Agent 默认看不到别的文件，并不阻止它访问绝对路径。
- Agent 的输出被当成数据：只渲染组件目录里的组件，不执行任何 HTML 或脚本（ADR 0002）。
- 超时、错误分类、退出清理在 #8；局部重新生成与撤回在 #12。

## 放弃的方案

| 方案 | 放弃原因 |
|---|---|
| app 直接调用模型 API | 要管理用户的 API key 和额度；用户已经有 Antigravity 账号 |
| 让 Agent 直接写视图文件 | 要给 Agent 原文所在目录的写权限；哈希和元数据会由 Agent 随意填写 |
| 声明 ACP fs 能力，由宿主代读代写 | `agy` 用自己的工具读写文件，不走客户端 fs；声明了也不能阻止它 |
| 只靠 `request_permission` 判断 | 实测在常见的用户设置下根本不发这个请求 |
| 用 macOS 沙箱（`sandbox-exec`）包住 Agent | 已废弃的接口；`agy` 要访问网络和自己的配置目录，配置规则不稳定 |
