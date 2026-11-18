# 验收

## 可复验入口

CI 以 [工作流](../.github/workflows/ci.yml) 为准。原生窗口、菜单和重启在 macOS 上按 [Automation 指南](agents/native-automation.md) 验证。提交前选择覆盖实际改动的检查。

## MyGo v0.10.0 升级证据

- 最新升级合并在 `9559625`。Go、Web 构建、两组 Web 测试和 Windows 交叉编译通过。 [main CI](https://github.com/Bayern99/mark-go-go/actions/runs/44240578323)
- `5b68b5c` 的隔离应用在 macOS 完整跑了两次原生 smoke，均退出 `0`。四个文档同窗、切换、磁盘保存、原生撤销重做和重启恢复通过。空正文退出 `1`，应用退出 `23` 时脚本返回 `23`。QA 数据与 Vite 临时缓存随运行目录清理。 [升级 PR](https://github.com/Bayern99/mark-go-go/pull/41)
- 升级后的 macOS 包另行验证了 Save As、Rename、Move To、未命名原文关闭确认、外部冲突取消，以及真实 `Ctrl+Tab`。 [升级 PR](https://github.com/Bayern99/mark-go-go/pull/41)

## 原生覆盖缺口

升级后窄窗几何、精确光标与滚动位置、VoiceOver 和 Linux/Windows 界面仍需相应平台的复验。ComparisonTable 的键盘排序由 [Chromium 用例](../web/test/genui-catalog.test.tsx) 覆盖。历史验收与范围见 [工作区集成 #32](https://github.com/Bayern99/mark-go-go/issues/32) 和仓库历史。
