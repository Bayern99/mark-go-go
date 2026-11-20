# Portrait 正式构建对照

同一台 Mac 上，用两个独立标识的正式应用运行相同的文档和操作。基线来自迁移起点 `81522cb48ec9718bffbda0255df1039692a2f6cd`。构建、文档、窗口截图、应用配置和逐笔日志保存在本机。仓库保留驱动和汇总记录。

## 复跑

需要屏幕录制与辅助功能权限、Go、Python、PyObjC 的 Cocoa 和 Quartz 框架、Pillow 以及 `cua-driver`。旧版构建另需 Node 和 npm。使用项目的 macOS 桌面，接通电源，保持显示器分辨率、刷新率、输入法、系统设置和后台负载尽可能一致。测量期间将键盘与指针交给测试应用，驱动在每个输入事件前核对前台 PID，通过 Quartz 向 macOS HID 事件流派发鼠标、键盘与滚轮操作。

从仓库根目录指定目标提交和全新的绝对目录。命令中的 Python 解释器须能导入 `AppKit`、`Quartz`、`CoreFoundation` 和 `PIL`。构建脚本把指定提交导出到证据目录，并把解析后的完整提交 SHA 写入 `revision.txt`。基线固定为 `81522cb48ec9718bffbda0255df1039692a2f6cd`；该提交的 `mygo.json` 请求 Web 构建，因此导出的旧版源码会执行 `npm ci`。新提交按自身配置构建原生界面。

```sh
evidence="$HOME/Library/Application Support/Mark Go Go Benchmarks/new-baseline"
scripts/portrait/build.sh 81522cb "$evidence"
app=$(find "$evidence/bundle" -name '*.app' -maxdepth 3)
python3 scripts/portrait/measure.py "$app" "$evidence/wide" --width 1280 --trials 3 --soak-minutes 10
python3 scripts/portrait/measure.py "$app" "$evidence/narrow" --width 480 --trials 3 --soak-minutes 0
python3 scripts/portrait/measure.py "$app" "$evidence/layout" --layout
python3 scripts/portrait/summarize.py "$evidence/wide"
python3 scripts/portrait/summarize.py "$evidence/narrow"
```

对候选提交更换 revision 和输出目录。构建脚本从 Git 对象导出源码，为包名、slug、bundle identifier 分配独立值。正式编译使用 `go tool mygo build -skip-dmg -skip-notarize`。驱动按应用默认宽窗启动并验证首键。窄窗流程随后调整到 480 × 360。每次启动都生成一对文档，把上一次 QA 专用的 Application Support 目录移入本次记录，再从 Launch Services 打开一份新路径的原文。每次退出都等待应用进程结束。系统文件缓存及该测试包的 WebKit 缓存可跨次保留。

夹具包含一份 256 节、56,791 字节的英文长文和一份 55 字节短文。长文 SHA-256 为 `72325788cd3a0272af9aa78fee7f9dca80dec623a03c7470cf88f3ea93266e61`，短文为 `ab72d34013b27a3349574349f67ecaccf76768e3821b81cc3417549784a5be21`。原文由 [夹具与窗口驱动](../../scripts/portrait/mac_window.py) 创建。若候选布局移动标题，先查看其稳定窗口，用 `--wide-x`、`--narrow-x` 和 `--heading-y` 指向可编辑标题；窄窗标题垂直位置不同时，用 `--narrow-heading-y` 单独指定，默认沿用 `--heading-y`。记录所用坐标。

## 读数含义

| 读数 | 操作与观察 |
| --- | --- |
| 启动到首个输入可见 | 从调用 `open -na` 前计时。取得真实窗口，用辅助功能设置大小，核对 WindowServer 实际尺寸，以标题行的墨迹判断正文已显示，捕获一帧，点击标题并按数字 7。看到标题区域变化时停表。⌘S 后核对整篇原文仅插入一个 7。耗时包含驱动设置窗口的 250 ms 等待、截图、点击和派发事件。 |
| 输入到显示 | 保持焦点，再按五次 7。每次从发键到首个有明显像素变化的窗口采样，随后保存并逐次核对写盘内容。 |
| 滚动 | 在长文内向下、向上、再向下滚动。每段计划在一秒内派发 60 个 14 像素的滚轮事件。每个事件后抓取可读正文区域，分别记录相邻采样和相邻画面变化之间的间隔。只有观察到至少两次内容变化，且起止画面有变化，才纳入画面变化统计。 |
| 标签切换 | 通过 Launch Services 的打开文件事件打开短文，再以系统合成的 Ctrl+Tab 在两份文档间切换七次。每次观察正文区域变化，并核对窗口标题切换至目标文件。 |
| 进程组内存 | 在切回长文后枚举宿主和 macOS 将其责任 PID 指向宿主的进程。用 `proc_pid_rusage(RUSAGE_INFO_V0)` 读取各进程 physical footprint，另存 `ps` 的 RSS。按进程列出 WebContent、GPU、Networking 等，并累加 footprint。系统共享资源的归属按 macOS 计账。 |
| 长会话 | 最后一次宽窗试验持续 10 分钟，每 30 秒记录一组内存、实际经过时间与系统平均负载。在两次采样间切到短文、切回长文、点击正文并交替向上或向下滚动。 |
| 系统框架与窗口 | 每轮以 `vmmap -w` 检查宿主的 WebKit、JavaScriptCore 与 AppKit 映射。进程表另记专属 WebKit 辅助进程。可用独立的 `--layout` 流程另行采集稳定后的 Preview、Markdown，以及 640/641 和 1179/1180 点的窗口。 |

时间来自单调时钟。渲染观察以窗口独立截图相邻两次采样的返回时间为界。像素阈值排除轻微噪声，但区域中其他变化也可能触发。合成器可在两次采样间提交多帧；统计描述采样器可见的画面变化。启动包含前述自动化开销，样本来自新进程以及已经使用过的系统缓存。P95 采用最近秩，样本量为 3 时等于最大值。失败、中断与已完成的原始记录均可从独立输出目录复查。 [操作驱动](../../scripts/portrait/measure.py) · [汇总算法](../../scripts/portrait/summarize.py)

## 2026-11-20 旧版实测

机器为 Apple M4 Max、128 GiB、macOS 27.2（26B123）、arm64，使用交流电。外接 3840 × 2160 显示器以 1920 × 1080 点、60 Hz 运行。工具为 Xcode 27.0（27A266a）、Go 1.27.3、Node 26.13.0、npm 11.20.0、MyGo v0.10.0 及仓库补丁、cua-driver 0.38.0、系统 Python 3.9.6、PyObjC 11.1 和 Pillow 11.3.0。系统输入源为简体拼音，本组输入使用数字键。

测量包从迁移起点的 Git 对象构建，复用本机已有的旧版前端依赖副本。正式 arm64 可执行文件 SHA-256 为 `19af8a43c522808f6201e267f7946c3c54320303ea55e569854949b2050bcd73`，可执行文件包含构建标签 `mygo_nodownload,mygo_noinspector`，CGO 已关闭。包经过 ad hoc 签名和 `codesign --verify --strict` 校验。另用提交的构建脚本从干净 Git 导出执行 npm ci，成功打包并复验签名。宿主和 WebKit 的数据均归属专用测试包。原始记录归档名为 `2026-11-20-81522cb`，在采样 Mac 的 `~/Library/Application Support/Mark Go Go Benchmarks/` 下。

迁移前同一提交的四个 CI 作业均通过。Ubuntu 24.04 跑过 Go 竞态测试、Go 和 MyGo vet、Web 构建和 Chromium 的两组测试；Windows amd64 的宿主交叉编译通过。Windows 和 Linux 实际桌面操作没有纳入本次 macOS 采样。 [迁移前 CI](https://github.com/Bayern99/mark-go-go/actions/runs/44730420373)

宽窗正式序列 3/3 次完成。窄窗完成了两次编辑、滚动和切换流程。其中一次从宽窗打开后缩到 480 × 360，另一次在启动中缩窗。在启动中直接缩窗的三批试验共计划 9 次，实际开始 4 次、完成 1 次。一次因前台 PID 改变退出，两次未能把首键保存到夹具，余下 5 次未启动。下面分列两份窄窗记录，便于候选构建采用相同操作对照。

| 观测项 | 1280 × 820，3 轮 | 打开后缩窗，1 轮 | 启动中缩窗，1 轮完成 |
| --- | ---: | ---: | ---: |
| 启动至首个输入可见，中位数，ms | 1188.6（1173.0–1201.3） | 1208.4（首键时仍在宽窗） | 1094.1 |
| 按键至显示，中位数 / P95，ms | 49.9 / 58.0（15 次） | 46.7 / 83.0（5 次） | 48.0 / 50.9（5 次） |
| 切换至显示，中位数 / P95，ms | 97.5 / 114.4（21 次） | 94.3 / 118.8（7 次） | 91.5 / 126.5（7 次） |
| 输入或切换命中时的采样间隔，中位数 / P95，ms | 17.1 / 37.8 | 16.3 / 50.4 | 17.8 / 48.0 |
| 滚动时可见变化的间隔，中位数 / P95，ms | 16.65 / 18.57（505 个） | 17.03 / 19.60（166 个） | 16.65 / 19.14（168 个） |
| 截图采样间隔，中位数 / P95，ms | 16.62 / 18.43 | 16.68 / 19.01 | 16.42 / 19.02 |
| 切回长文后的进程组 physical footprint，中位数，MiB | 929.9（912.7–941.0） | 793.2 | 790.5 |

成功流程的 9/9、3/3、3/3 段滚动均观察到正文移动。两份窄窗记录用 280 × 96 点观察区域，宽窗正式序列用 350 × 644 点。当前驱动沿用这两个采样区域。公开比较时应匹配尺寸、采样区域、打开方式和前台状态。窄窗单次成功启动时间仅作个案记录。

| 切回长文时的进程 | 宽窗中位数，MiB | 打开后缩窗，MiB | 启动中缩窗，MiB |
| --- | ---: | ---: | ---: |
| 应用宿主 | 45.8 | 46.6 | 48.6 |
| WebKit WebContent | 602.6 | 467.2 | 468.5 |
| WebKit GPU | 268.0 | 266.0 | 259.6 |
| WebKit Networking | 5.9 | 5.7 | 5.8 |
| 其他归属宿主的系统辅助进程 | SetStoreUpdateService、音频 SandboxHelper | 同左 | 同左 |

宽窗长会话在 600 秒内完成 20 组操作、21/21 次采样。进程组从 901.8 MiB 到 568.2 MiB，期间最低 560.7 MiB。成功的五轮均单独观察到 WebKit 内容、GPU 和网络进程，宿主也映射了 WebKit、JavaScriptCore 和 AppKit。采样期间同机还有开发任务运行，逐次负载随内存记录保存。

本机归档的 `wide-series`、`pilot3` 和 `narrow-measured/trial-1` 分别保留上表三列的窗口 PNG、文件和逐项记录。宽窗可见停靠文件栏、标签、居中的正文和独立滚动区域；480 点宽的工作区显示文件栏入口，正文仍可编辑。直接缩窗的一张启动截图捕获到 Welcome，之后的编辑和保存核对成功。窄窗稳定的长文阅读截图在 `exploration/narrow.png`，可用 `pilot3/trial-1/480-edited.png`、`480-scrolled.png` 和 `480-second.png` 对照状态。`layout-boundaries/trial-1` 保存 1280、1180、1179、641、640 和 480 点宽的 Preview，以及默认和最小窗口的 Markdown 截图。初次尝试在从 1180 调到 1179 点时超时；驱动先调整 16 点再设定目标，随后以 WindowServer 返回的精确尺寸通过全部六档。

连同校准，本机保留了 12 次带状态的尝试，5 次完成、7 次失败。除三次直接缩窗失败外，校准中各有一次多字符输入未写入、菜单打开时退出超时、窗口动画期间尺寸偏差 2 点，以及滚动观察区域未检测到移动。失败记录保留原始输入与截图；上表采用完成相应流程的试验。归档时逐字节复查了 5 轮的全部 10 份文件，长文均只插入 6 个数字 7，短文与原夹具一致。20 张流程截图均与记录的窗口尺寸相符。
