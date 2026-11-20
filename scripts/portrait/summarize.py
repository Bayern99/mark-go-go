#!/usr/bin/env python3
import argparse
import json
from math import ceil
from statistics import median
from pathlib import Path


def distribution(values):
    if not values:
        return None
    ordered = sorted(values)
    return dict(n=len(values), min=round(ordered[0], 2), median=round(median(ordered), 2),
                p95=round(ordered[ceil(0.95 * len(ordered)) - 1], 2), max=round(ordered[-1], 2))


def intervals(samples):
    return [right - left for left, right in zip(samples, samples[1:])]


def footprint(row):
    return sum(process["footprint"] for process in row["processes"]) / 2 ** 20


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("series", type=Path)
    args = parser.parse_args()
    trials = sorted(args.series.glob("trial-*"))
    if not trials:
        parser.error("No trial directories in series")
    status = {}
    records = []
    for trial in trials:
        status_file = trial / "status"
        status[trial.name] = status_file.read_text().strip() if status_file.exists() else "incomplete"
        records.extend(json.loads(line) for line in (trial / "events.jsonl").read_text().splitlines())

    def metric(key):
        return [r for r in records if r["metric"] == key]

    scroll = metric("scroll")
    soak = metric("soak")
    memory = metric("memory")
    results = dict(status=status)
    for key in ["startup", "input", "switch"]:
        results[key + "Ms"] = distribution([r["ms"] for r in metric(key)])
    results.update(
        pollGapMs=distribution([r["gapMs"] for r in metric("input") + metric("switch")]),
        scrollBursts=len(scroll), scrollConfirmed=sum(r.get("moved", len(r["changed"]) >= 2) for r in scroll),
        captureIntervalsMs=distribution([v for r in scroll for v in intervals(r["frames"])]),
        changedIntervalsMs=distribution([v for r in scroll if r.get("moved", len(r["changed"]) >= 2) for v in intervals(r["changed"])]),
        groupMiB=distribution([footprint(r) for r in memory]),
        processMiB={role: distribution([p["footprint"] / 2 ** 20 for r in memory for p in r["processes"]
                                        if p["role"] == role])
                    for role in sorted({p["role"] for r in memory for p in r["processes"]})},
        soakMiB=[round(footprint(r), 2) for r in soak],
        soakSeconds=[round(r["elapsed"], 2) for r in soak],
        frameworkSamples=[r["mapped"] for r in metric("frameworks")])
    print(json.dumps(results, indent=2))


if __name__ == "__main__":
    main()
