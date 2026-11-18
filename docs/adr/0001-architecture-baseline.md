# ADR 0001 · 本地宿主与原文保真

原文是内容的唯一权威。Go/MyGo 管理磁盘与原生生命周期，WebView 编辑器提供当前缓冲区。Preview 用 Crepe，Markdown 用 CodeMirror，方便同时支持可视编辑和原始语法。

按块组合保存，未编辑的块及其间隔保持原字节。这样读者可以继续用其他编辑器打开同一份原文，现有格式也得以保留。回归依据为 [往返用例](../../web/test/roundtrip.test.ts) 和 [撤销保真用例](../../web/test/undo-fidelity.test.ts)。
