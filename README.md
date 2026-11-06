# Mark Go Go

本地优先的 macOS Markdown 编辑器原型：MyGo（Go 宿主）+ Milkdown CrepeBuilder（编辑器），保存时按块保留原始字节。

- 架构决策：`docs/adr/0001-architecture-baseline.md`
- 功能与接口：`docs/interfaces.md`
- 验收结果：`docs/acceptance.md`

## 开发

```sh
npm --prefix web install
npx --prefix web playwright install chromium-headless-shell   # 测试用，一次
go tool mygo dev        # Vite :5173 + 开发版应用
go tool mygo generate   # 重新生成 web/src/host/generated/mygo.ts
```

纯浏览器调试（内存宿主、示例文档）：`npm --prefix web run dev`。

## 检查与打包

```sh
go vet ./...
npm --prefix web run typecheck
npm --prefix web test
go tool mygo build      # build/darwin-arm64/Mark Go Go.app 与 .dmg
```
