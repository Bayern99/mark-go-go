#!/usr/bin/env python3
import argparse
import json
import os
import plistlib
import subprocess
import time
from pathlib import Path

import AppKit
import Quartz as cg

from mac_window import Window, app_window, clock, fixture, ink_change, sha, wait_for


def run_trial(args, folder, name, trial):
    first, second = fixture(folder / "documents")
    log_file = folder / "events.jsonl"

    def record(metric, **data):
        with log_file.open("a") as out:
            print(json.dumps(dict(metric=metric, trial=trial, at=clock(), **data)), file=out)

    def capture(window, label, image=None):
        path = folder / f"{window.width}-{label}.png"
        (image if image is not None else window.image()).save(path)
        record("screenshot", file=path.name, sha256=sha(path))

    def visible_change(window, region, act, threshold):
        before = window.image(region)
        started = clock()
        previous = started
        act()
        for _ in range(180):
            after = window.image(region)
            observed = clock()
            if ink_change(before, after) >= threshold:
                return dict(ms=1000 * (observed - started), gapMs=1000 * (observed - previous), observed=observed)
            previous = observed
        raise RuntimeError("No visible change after input")

    def edit(window, region):
        old = first.read_bytes()
        count = old.count(b"7")
        measurement = visible_change(window, region, lambda: window.key(26), 60)
        window.key(1, cg.kCGEventFlagMaskCommand)
        wait_for(lambda: first.read_bytes().count(b"7") == count + 1, "saved numeral 7")
        updated = first.read_bytes()
        insertion = len(os.path.commonprefix([old, updated]))
        if updated != old[:insertion] + b"7" + old[insertion:]:
            raise RuntimeError("Input replaced unrelated document content")
        return measurement

    def switch(window, target, region):
        measurement = visible_change(window, region, lambda: window.key(48, cg.kCGEventFlagMaskControl), 180)
        wait_for(lambda: target.name in window.refresh(), "active native title " + target.name)
        record("switch", width=window.width, target=target.name, **measurement)

    def scroll(window, region, direction):
        initial = prev = window.image(region)
        times = []
        changes = []
        start = clock()
        for tick in range(60):
            due = start + tick / 60
            time.sleep(max(0, due - clock()))
            window.wheel(direction * 14)
            image = window.image(region)
            now = clock() - start
            times.append(now * 1000)
            if ink_change(prev, image) > 80:
                changes.append(now * 1000)
            prev = image
        moved = len(changes) >= 2 and ink_change(initial, prev) >= 200
        record("scroll", width=window.width, direction=direction, frames=times, changed=changes, moved=moved)

    record("fixtures", firstBytes=first.stat().st_size, firstSHA256=sha(first), secondSHA256=sha(second))
    started = clock()
    subprocess.run(["open", "-na", str(args.app), str(first)], check=True)
    info = wait_for(lambda: app_window(name), "application window")
    window = Window(info)
    AppKit.NSRunningApplication.runningApplicationWithProcessIdentifier_(window.pid).activateWithOptions_(
        AppKit.NSApplicationActivateIgnoringOtherApps)

    def anchor():
        return args.wide_x if window.width >= 900 else args.narrow_x

    def heading():
        return (anchor() - 8, args.heading_y - 15, min(300, window.width - anchor()), 55)

    def focus():
        window.click(anchor() + 8, args.heading_y)
        time.sleep(0.04)

    def wait_heading():
        def has_title_ink():
            image = window.image((anchor(), args.heading_y - 11, 150, 24))
            upper = image.crop((0, 0, 100, 5))
            return sum(upper.histogram()[:130]) > 20 and sum(image.histogram()[:130]) > 550

        wait_for(has_title_ink, "painted document heading")

    try:
        if args.layout:
            wait_for(lambda: first.name in window.refresh(), "first native title")
            wait_for(window.frontmost, "foreground window")
            time.sleep(1)
            for width, height in [(1280, 820), (1180, 820), (1179, 820), (641, 600), (640, 600), (480, 360)]:
                window.resize(width, height)
                time.sleep(0.5)
                capture(window, "layout")
                if width in (1280, 480):
                    window.key(44, cg.kCGEventFlagMaskCommand)
                    time.sleep(0.5)
                    capture(window, "markdown")
                    window.key(44, cg.kCGEventFlagMaskCommand)
                    time.sleep(0.5)
            return
        width, height = args.width, 820 if args.width == 1280 else 360
        window.resize(1280, 820)
        wait_heading()
        wait_for(window.frontmost, "foreground window")
        pristine = window.image()
        focus()
        initial = edit(window, heading())
        record("startup", width=window.width, height=window.height,
               ms=1000 * (initial["observed"] - started), inputMs=initial["ms"])
        if width == 480:
            window.resize(480, 360)
            wait_heading()
            pristine = window.image()
            focus()
        capture(window, "preview", pristine)
        for _ in range(5):
            result = edit(window, heading())
            record("input", width=width, **result)
        capture(window, "edited")
        region = (anchor() - 6, args.heading_y - 18, min(280, width - anchor() - 2), min(96, height - args.heading_y))
        scroll_region = region if width == 480 else (anchor() - 6, 135, 350, 644)
        for direction in [-1, 1, -1]:
            scroll(window, scroll_region, direction)
            time.sleep(0.15)
        capture(window, "scrolled")
        subprocess.run(["open", "-a", str(args.app), str(second)], check=True)
        wait_for(lambda: second.name in window.refresh(), "second document")
        time.sleep(0.5)
        for _ in range(3):
            switch(window, first, region)
            time.sleep(0.18)
            switch(window, second, region)
            time.sleep(0.18)
        capture(window, "second")
        switch(window, first, region)
        record("memory", width=width, processes=window.memory(), load=os.getloadavg())
        if args.soak_minutes and trial == args.trials:
            start = clock()
            for tick in range(args.soak_minutes * 2 + 1):
                time.sleep(max(0, start + tick * 30 - clock()))
                record("soak", elapsed=clock() - start, processes=window.memory(), load=os.getloadavg())
                if tick == args.soak_minutes * 2:
                    break
                window.key(48, cg.kCGEventFlagMaskControl)
                wait_for(lambda: second.name in window.refresh(), "second document during soak")
                window.key(48, cg.kCGEventFlagMaskControl)
                wait_for(lambda: first.name in window.refresh(), "first document during soak")
                focus()
                window.wheel((-1 if tick % 2 == 0 else 1) * 210)
        output = subprocess.run(["vmmap", "-w", str(window.pid)], text=True, capture_output=True, check=True)
        mappings = sorted({name for name in ["WebKit.framework", "JavaScriptCore.framework", "AppKit.framework"]
                           if name in output.stdout})
        record("frameworks", mapped=mappings)
    except Exception as error:
        record("failed-action", reason=str(error))
        raise
    finally:
        AppKit.NSRunningApplication.runningApplicationWithProcessIdentifier_(window.pid).terminate()
        wait_for(lambda: not app_window(name), "application quit")
        wait_for(lambda: AppKit.NSRunningApplication.runningApplicationWithProcessIdentifier_(window.pid) is None,
                 "host process exit")


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser()
    parser.add_argument("app", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("--width", type=int, choices=[1280, 480], default=1280)
    parser.add_argument("--layout", action="store_true", help="capture settled Preview and Markdown windows")
    parser.add_argument("--trials", type=int, default=3)
    parser.add_argument("--soak-minutes", type=int, default=10)
    parser.add_argument("--wide-x", type=int, default=470)
    parser.add_argument("--narrow-x", type=int, default=73)
    parser.add_argument("--heading-y", type=int, default=231)
    args = parser.parse_args()
    if args.layout:
        args.trials = 1
    args.app = args.app.resolve()
    args.output = args.output.resolve()
    if Path(__file__).resolve().parents[2] in args.output.parents:
        parser.error("Store evidence outside the checkout")
    config = plistlib.loads((args.app / "Contents/Info.plist").read_bytes())
    name = config["CFBundleName"]
    if not (name.startswith("Mark Go Go Portrait ") and config["CFBundleIdentifier"].startswith("dev.markgogo.portrait.")):
        parser.error("Build an isolated Portrait application using build.sh")
    if AppKit.NSRunningApplication.runningApplicationsWithBundleIdentifier_(config["CFBundleIdentifier"]):
        parser.error("Quit the isolated application before measuring")
    if args.trials < 1 or args.soak_minutes < 0:
        parser.error("Require a positive trial count and a nonnegative soak duration")
    args.output.mkdir(mode=0o700)
    profile = Path.home() / "Library/Application Support" / name
    for trial in range(1, args.trials + 1):
        folder = args.output / f"trial-{trial}"
        folder.mkdir()
        if profile.exists():
            profile.rename(folder / "previous-profile")
        try:
            run_trial(args, folder, name, trial)
            (folder / "status").write_text("passed\n")
        except Exception as error:
            (folder / "status").write_text(f"failed: {error}\n")
            raise


if __name__ == "__main__":
    main()
