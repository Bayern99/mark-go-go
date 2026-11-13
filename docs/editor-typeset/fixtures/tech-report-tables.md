# MarkSync 同步引擎 2026-Q1 性能基准报告

## 1 概述

本报告汇总了 MarkSync 同步引擎 2.4.1 版本在三种部署形态（单机、双副本、五副本集群）下的性能基准结果。测试于 2026-02-11 至 2026-02-18 在专用基准集群上执行，硬件与软件环境见附录。所有数字均为三次独立运行的中位数，抖动范围在 ±3% 以内时不再单独标注。

测试负载分为两类：**bulk-import**（一次性导入 120 万份 Markdown 文档，平均 4.2 KB/份）与 **incremental-sync**（模拟 4,000 个并发客户端的小增量更新，平均每次变更 312 B）。报告的主体由五张数据表构成，分别覆盖环境信息（表 1）、核心时延指标（表 2）、集群节点清单（表 3）、已知限制与失败模式（表 4）、以及配置项速查（表 5）。

## 2 测试环境

表 1 给出基准环境的摘要信息。三个测试单元中，unit-b 同时承担客户端负载生成器的角色，因此其数据仅用于参考，不参与横向对比。

| 单元    | CPU 核数 | 内存    |
|---------|---------:|--------:|
| unit-a  |       16 |  64 GB  |
| unit-b  |        8 |  32 GB  |
| unit-c  |       16 |  64 GB  |
| unit-d  |       32 | 128 GB  |
| unit-e  |       32 | 128 GB  |

需要说明的是，unit-d 与 unit-e 仅在五副本集群形态下启用；单机与双副本形态下全部负载落在 unit-a 上。

## 3 核心指标

### 3.1 时延与吞吐

表 2 是本次基准的核心结果：三种部署形态下 bulk-import 与 incremental-sync 的时延分位数与吞吐量。可以看到，双副本形态的 p99 写时延比单机高 38%，这是同步复制（synchronous replication）的固有代价；五副本形态通过 quorum=3 的折中，将代价控制在 51%。

| 指标              | 部署形态 |    p50 |    p90 |    p99 |   吞吐 |
|:------------------|:--------:|-------:|-------:|-------:|-------:|
| bulk-import 时延 (ms)  | 单机     |   8.21 |  14.03 |  31.77 | 18,204 |
| bulk-import 时延 (ms)  | 双副本   |  11.42 |  19.85 |  43.92 | 13,977 |
| bulk-import 时延 (ms)  | 五副本   |  12.86 |  22.14 |  48.03 | 12,650 |
| inc-sync 写时延 (ms)   | 单机     |   2.04 |   4.61 |  11.28 | 41,530 |
| inc-sync 写时延 (ms)   | 双副本   |   3.11 |   6.92 |  15.60 | 34,862 |
| inc-sync 写时延 (ms)   | 五副本   |   3.58 |   7.74 |  17.02 | 31,905 |
| inc-sync 读时延 (ms)   | 单机     |   0.87 |   1.93 |   4.66 | 92,114 |
| inc-sync 读时延 (ms)   | 双副本   |   0.91 |   2.05 |   5.01 | 88,730 |
| inc-sync 读时延 (ms)   | 五副本   |   0.94 |   2.12 |   5.18 | 91,206 |
| compaction 耗时 (s)    | 单机     |  41.30 |  44.12 |  47.95 |      — |
| compaction 耗时 (s)    | 双副本   |  43.77 |  46.51 |  50.20 |      — |
| compaction 耗时 (s)    | 五副本   |  45.02 |  48.33 |  52.64 |      — |

### 3.2 集群节点清单

表 3 列出了参与基准的全部节点，包括其内部资产编号、角色、固件版本与监控面板地址。这张表较宽，在窄列布局下需要横向滚动才能完整阅读。

| 节点资产编号         | 主机名                | 角色          | 加入日期     | 固件版本   | 机架位置        | BMC 地址                                  | 监控面板 URL                                                        | 备注                     |
|----------------------|-----------------------|---------------|--------------|------------|-----------------|-------------------------------------------|---------------------------------------------------------------------|--------------------------|
| ASSET-2025-00871     | bench-node-01.dc1     | primary       | 2025-09-03   | 4.17.2     | R12-U04         | https://bmc.dc1.internal/ASSET-2025-00871 | https://grafana.dc1.internal/d/nodes/bench-node-01                  | unit-a                   |
| ASSET-2025-00872     | bench-node-02.dc1     | loadgen       | 2025-09-03   | 4.17.2     | R12-U05         | https://bmc.dc1.internal/ASSET-2025-00872 | https://grafana.dc1.internal/d/nodes/bench-node-02                  | unit-b（负载生成）       |
| ASSET-2025-00904     | bench-node-03.dc1     | replica       | 2025-10-21   | 4.17.2     | R12-U09         | https://bmc.dc1.internal/ASSET-2025-00904 | https://grafana.dc1.internal/d/nodes/bench-node-03                  | unit-c                   |
| ASSET-2025-00931     | bench-node-04.dc1     | replica       | 2025-11-12   | 4.18.0-rc1 | R14-U02         | https://bmc.dc1.internal/ASSET-2025-00931 | https://grafana.dc1.internal/d/nodes/bench-node-04                  | unit-d                   |
| ASSET-2025-00932     | bench-node-05.dc1     | replica       | 2025-11-12   | 4.18.0-rc1 | R14-U03         | https://bmc.dc1.internal/ASSET-2025-00932 | https://grafana.dc1.internal/d/nodes/bench-node-05                  | unit-e                   |
| ASSET-2024-01190     | bench-storage-01.dc1  | nvme-tier     | 2024-05-30   | 3.9.8      | R09-U11         | https://bmc.dc1.internal/ASSET-2024-01190 | https://grafana.dc1.internal/d/storage/bench-storage-01             | 共享 NVMe 存储池         |
| ASSET-2024-01208     | bench-switch-tor-02   | tor-switch    | 2024-06-14   | 9.2.1      | R12-TOR         | https://bmc.dc1.internal/ASSET-2024-01208 | https://grafana.dc1.internal/d/network/bench-switch-tor-02          | 100GbE ToR               |
| ASSET-2025-01012     | bench-ctrl-01.dc1     | orchestrator  | 2026-01-08   | 4.18.0     | R14-U07         | https://bmc.dc1.internal/ASSET-2025-01012 | https://grafana.dc1.internal/d/control/bench-ctrl-01                | 基准编排与控制节点       |

## 4 已知限制

### 4.1 失败模式

表 4 汇总了在压力窗口内观测到的失败模式。需要强调的是，这些失败均已在 2.4.2 的里程碑中排期，不构成当前版本的阻塞项，但在生产部署时需要配合相应的降级预案。

| 失败模式        | 触发条件            | 现象与影响                                                                                          |
|-----------------|---------------------|-----------------------------------------------------------------------------------------------------|
| split-brain     | 网络分区 > 30 s     | 双副本形态下若 witness 不可达，两副本各自进入 read-write 状态；恢复后需要人工选择 winner，冲突数据进入 quarantine 目录。 |
| retry-storm     | 客户端超时 < 500 ms | 过短的 client timeout 会在 p99 抖动时触发大面积重试，retry amplification 可达 6×，进一步恶化尾时延。  |
| compaction-stall | 磁盘水位 > 85%      | L0 文件数超过阈值时 compaction 被迫串行化，写入吞吐下降约 40%；水位回落后自动恢复，无需干预。          |
| clock-skew      | NTP 偏差 > 250 ms   | 副本间心跳判定失效，触发不必要的 leader 切换；集群要求 chrony 的 maxoffset 配置不超过 100 ms。        |
| index-bloat     | 文档数 > 200 万     | 倒排索引内存占用随文档数近似线性增长，单机 64 GB 配置在 200 万文档后开始出现 swap 抖动。              |
| lease-loss      | leader CPU 抢占     | 与宿主机其他租户争抢 CPU 时，leader 续约间隔超时，造成 200–800 ms 的不可用窗口。                     |

### 4.2 复现方式

上表中的 split-brain 场景可用以下命令在隔离环境中复现：

```bash
#!/usr/bin/env bash
# 在 unit-a 与 unit-c 之间注入 45 s 网络分区
sudo tc qdisc add dev bond0 root netem loss 100%
sleep 45
sudo tc qdisc del dev bond0 root
# 检查 quarantine 目录中的冲突条目
marksync-cli quarantine list --since 10m
```

## 5 配置速查

表 5 给出与上述指标直接相关的关键配置项。生产部署建议以 `marksync.toml` 中的注释为准，本表仅列默认值与调优方向。

| 配置项                | 默认值        | 说明                                       | 参考文档                                           |
|-----------------------|---------------|--------------------------------------------|----------------------------------------------------|
| `sync.quorum`         | `2`           | 写入确认所需副本数；五副本建议设为 **3**   | [docs/replication.md](https://example.com/repl)    |
| `client.timeout_ms`   | `2000`        | 低于 **500** 会触发 retry-storm            | [docs/client.md](https://example.com/client)       |
| `wal.segment_bytes`   | `67108864`    | WAL 段大小，64 MiB；调大可降低 fsync 频率  | [docs/storage.md](https://example.com/storage)     |
| `compaction.l0_limit` | `8`           | 触发 compaction 的 L0 文件数阈值           | [docs/compaction.md](https://example.com/compact)  |
| `index.cache_mb`      | `4096`        | 索引页缓存；文档数大时应设为内存的 **25%** | [docs/index.md](https://example.com/index)         |
| `rpc.max_inflight`    | `1024`        | 单连接最大在途请求，超出即背压             | [docs/rpc.md](https://example.com/rpc)             |

## 6 结论

- 单机形态仍是单位成本下吞吐最高的选择，适合**可容忍分钟级数据丢失**的场景；
- 双副本在 p99 写时延上付出约 38% 的代价，换取了单点故障下的数据安全；
- 五副本通过 quorum=3 取得了可用性与时延的折中，但成本接近单机的 3.4 倍；
- 已知限制集中在分区恢复与资源争抢两类场景，运维侧需要为 split-brain 与 lease-loss 准备预案。

综上，2.4.1 版本的性能画像满足 GA 标准；下一里程碑的重点是 compaction 并行化与索引分片，预期在五副本形态上再获得 15% 左右的写吞吐提升。
