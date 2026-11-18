# 编辑器排版基准

在仓库根目录安装依赖并启动 Vite。测量使用 [实际 App 的基准页](../../web/bench/typeset.html)。

```sh
npm --prefix web ci
npx --prefix web playwright install webkit chromium
npm --prefix web run dev
```

另开终端运行 `node docs/editor-typeset/measure.mjs --label baseline --engine both`。输出在 `docs/research/editor-typeset/results/`。对照运行保持引擎、夹具、视口和等待条件一致。夹具、变体与探针以 [脚本](measure.mjs) 为准。
