# 界面改动

先从目标交互定位现有组件和实际状态。复用 [Portrait Token](../../web/src/theme/portrait-tokens.ts)、[应用样式](../../web/src/styles.css) 和 [基础组件](../../web/src/components/)。三栏与原地输入的边界见 [ADR 0004](../adr/0004-three-column-layout-and-genui-panel.md)。特殊块和内容语义色见 [ADR 0006](../adr/0006-preserved-blocks-and-semantic-colors.md)。

验证入口触发和结果，覆盖改动涉及的键盘焦点、窄窗、空态与失败。页面用例在 [web/test](../../web/test/)。原生菜单、文件面板和窗口生命周期走 [macOS 验证](native-automation.md)。报告实际跑过的检查与待验项。
