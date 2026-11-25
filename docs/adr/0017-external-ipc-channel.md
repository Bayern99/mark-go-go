# ADR 0017 · 外部 Agent 的本机通道与独立 CLI

Status: accepted

外部 Harness 需要在用户自己的终端里读写当前工作区，而 Feynman 的本地 HTTP + 会话 Token 通道只服务它自己的研究会话：Token 按研究会话发放，工作区绑定在注册时写入，外部进程拿不到。此 ADR 记录为外部 Agent 增加的第二条通道，以及随之拆出的独立 CLI。

**Decision**

- 通道是 `<UserData>/ipc.sock` 上的 Unix socket，父目录 `0700`、socket `0600`，在设置里显式启用前不监听；关闭时停止监听并删除 socket，启动时清理上一次崩溃留下的 socket，若仍有实例在应答则拒绝接管。
- 每行一条 JSON-RPC 消息，复用与 Feynman HTTP 相同的工具分发。连接的第一帧是客户端的工作目录 `{"cwd": ...}`，服务端把它规范化后选出包含它的最深已打开工作区根目录，绑定这条连接；没有匹配就返回 `WORKSPACE_NOT_OPEN`。stdio 垫片用自己的工作目录写这一帧，客户端只看到标准 MCP 消息。
- 工具面收敛为七项：`read_document`、`get_workspace`、`propose_edit`、`create_document`、`open_document`、`write_genui_view`、`create_checkpoint`。结果与审阅队列解耦：`propose_edit` 只回提案 id、修订号、hunk 数与一句摘要，`resources/list` 不再扫盘，文件清单有上限并报告 `truncated`。提案与检查点的来源由服务端按通道写入。
- 同时支持两个 MCP 时代：`server/discover` 与逐请求 `_meta` 版本（`2026-07-28`），不支持的版本回 `-32022` 并列出支持版本；旧 `initialize` 握手协商，未知版本回落到 `2025-11-25`。
- `cmd/markgogo` 是独立二进制，只链接纯 Go 包（`internal/ipc`、`internal/markdown`、`internal/genuikit`、`internal/appversion`），不链接 MyGo 桌面运行时。应用未运行时 `read` 与 `genui format` 走本地只读路径，写操作返回 `APP_NOT_RUNNING`，不静默改盘。

**Considered Options**
- 复用 Feynman 的 HTTP 端口与 Token：外部进程没有 Token 发放路径，而且把本机通道暴露成端口。
- 让 CLI 链接 `internal/workspace` 直接操作文件：会把 MyGo 桌面运行时拖进 CLI，并让 CLI 绕过应用内的审阅队列与检查点。

**Consequences**
- 章节与笔记解析下沉到无 GUI 的 `internal/markdown`，`internal/workspace` 与 `internal/genui` 保留原有名字作为薄别名。
- 通道的启用开关先落在 `settings.json` 的 `externalIPC` 字段；Settings 界面属于后续票。
- 写操作向 `<UserData>/mcp-audit.jsonl` 追加一条 JSONL 记录（时间、通道、根目录、工具、目标、结果码）。
