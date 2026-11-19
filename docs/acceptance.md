# 验收

## 可复验入口

CI 以 [工作流](../.github/workflows/ci.yml) 为准。原生窗口、菜单和重启在 macOS 上按 [Automation 指南](agents/native-automation.md) 验证。提交前选择覆盖实际改动的检查。

## Feynman 与 Git 插件

2026-11-19 在隔离的 macOS arm64 应用中，使用 Feynman 0.5.71、pi-acp 0.0.33 和本地固定响应模型验证了工作区对话。空文件夹可以发起研究；同一窗口内两个工作区各自保留对话和产物。生成文件可打开为文档标签，重启后可恢复文档和对话。

480 px 宽窗口中，收到修改建议后继续编辑正文会使旧比较失效。重新比较、应用和原生 Undo 均通过。删除一份已完成报告后，从右栏显式恢复取得相同字节。[改名期间的修改建议](../web/test/feynman-edit.test.tsx) 由浏览器用例覆盖。单文档 Git 检查点在已有暂存内容的仓库中完成创建、比较与恢复。实测 HEAD、当前分支和暂存条目前后一致。[ACP 与权限用例](../research_test.go) · [产物用例](../artifact_test.go) · [Git 用例](../checkpoints_test.go) · [编辑器用例](../web/test/source-mode.test.ts)

## MyGo v0.10.0 升级证据

- 最新升级合并在 `9559625`。Go、Web 构建、两组 Web 测试和 Windows 交叉编译通过。 [main CI](https://github.com/Bayern99/mark-go-go/actions/runs/44240578323)
- `5b68b5c` 的隔离应用在 macOS 完整跑了两次原生 smoke，均退出 `0`。四个文档同窗、切换、磁盘保存、原生撤销重做和重启恢复通过。空正文退出 `1`，应用退出 `23` 时脚本返回 `23`。QA 数据与 Vite 临时缓存随运行目录清理。 [升级 PR](https://github.com/Bayern99/mark-go-go/pull/41)
- 升级后的 macOS 包另行验证了 Save As、Rename、Move To、未命名原文关闭确认、外部冲突取消，以及真实 `Ctrl+Tab`。 [升级 PR](https://github.com/Bayern99/mark-go-go/pull/41)

## 窄窗键盘操作

[PR #43](https://github.com/Bayern99/mark-go-go/pull/43) 的 macOS arm64 隔离 QA 构建在 480 px 窗口宽度下确认文件栏可展开和收起。正文内按 `⌥⌘0` 后焦点进入右栏，输入字母时正文保持原样，`Esc` 关闭浮层并将焦点还给开关。扩宽窗口后文件栏恢复停靠。保存、撤销、重做和重启恢复均通过。

## 原生覆盖缺口

精确光标与滚动位置、VoiceOver 和 Linux/Windows 界面仍需相应平台的复验。ComparisonTable 的键盘排序由 [Chromium 用例](../web/test/genui-catalog.test.tsx) 覆盖。历史验收与范围见 [工作区集成 #32](https://github.com/Bayern99/mark-go-go/issues/32) 和仓库历史。
