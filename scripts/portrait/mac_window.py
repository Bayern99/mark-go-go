import ctypes
import hashlib
import json
import subprocess
import time
from pathlib import Path

import AppKit
import Quartz as cg
import CoreFoundation as cf
from PIL import Image, ImageChops

clock = time.monotonic
system = ctypes.CDLL(None)


class RUsageInfoV0(ctypes.Structure):
    _fields_ = [
        ("uuid", ctypes.c_ubyte * 16),
        ("user_time", ctypes.c_uint64),
        ("system_time", ctypes.c_uint64),
        ("package_idle_wakeups", ctypes.c_uint64),
        ("interrupt_wakeups", ctypes.c_uint64),
        ("pageins", ctypes.c_uint64),
        ("wired_size", ctypes.c_uint64),
        ("resident_size", ctypes.c_uint64),
        ("physical_footprint", ctypes.c_uint64),
        ("start_abstime", ctypes.c_uint64),
        ("exit_abstime", ctypes.c_uint64),
    ]


def wait_for(check, description, seconds=12):
    deadline = clock() + seconds
    while clock() < deadline:
        result = check()
        if result:
            return result
        time.sleep(0.02)
    raise RuntimeError("Timed out waiting for " + description)


def app_window(name):
    windows = cg.CGWindowListCopyWindowInfo(cg.kCGWindowListOptionOnScreenOnly, 0)
    for window in windows:
        if window.get("kCGWindowOwnerName") == name and window.get("kCGWindowLayer") == 0:
            return window
    return None


def fixture(folder):
    folder.mkdir()
    first = folder / "Portrait.md"
    second = folder / "Second.md"
    lines = ["# Portrait report", "", "The first line identifies this reading sample.", ""]
    for index in range(1, 257):
        lines.extend([f"## Observation {index:03d}", "",
                      f"At station {index}, the reader compares the morning and evening observations. "
                      "The fixed prose gives each section enough lines to move through the window. "
                      "Weather, light, and colour change across the page.", ""])
    first.write_bytes(("\n".join(lines) + "\n").encode())
    second.write_bytes(b"# Second document\n\nA short independent reading sample.\n")
    return first, second


def ink_change(a, b):
    difference = ImageChops.difference(a, b)
    return sum(difference.histogram()[25:])


class Window:
    def __init__(self, info):
        self.pid = int(info["kCGWindowOwnerPID"])
        self.window_id = int(info["kCGWindowNumber"])
        self.session = f"portrait-{self.pid}"
        self.refresh()

    def refresh(self):
        rows = cg.CGWindowListCopyWindowInfo(cg.kCGWindowListOptionIncludingWindow, self.window_id)
        if not rows:
            raise RuntimeError("Measured window closed")
        self.title = str(rows[0].get("kCGWindowName", ""))
        frame = rows[0]["kCGWindowBounds"]
        self.x, self.y, self.width, self.height = (int(frame[k]) for k in ("X", "Y", "Width", "Height"))
        return self.title

    def resize(self, width, height):
        args = dict(session=self.session, pid=self.pid, window_id=self.window_id,
                    x=250, y=60, width=width, height=height)
        self.refresh()
        if self.width != width and abs(self.width - width) <= 2:
            intermediate = dict(args, width=width + 16)
            subprocess.run(["cua-driver", "call", "set_window_frame", json.dumps(intermediate)],
                           stdout=subprocess.DEVNULL, check=True)
        subprocess.run(["cua-driver", "call", "set_window_frame", json.dumps(args)],
                       stdout=subprocess.DEVNULL, check=True)
        time.sleep(0.25)

        def exact_size():
            self.refresh()
            return (self.width, self.height) == (width, height)

        wait_for(exact_size, f"{width} by {height} window")

    def image(self, region=None):
        rect = cg.CGRectNull
        if region:
            x, y, width, height = region
            rect = cg.CGRectMake(self.x + x, self.y + y, width, height)
        shot = cg.CGWindowListCreateImage(rect, cg.kCGWindowListOptionIncludingWindow, self.window_id,
                                          cg.kCGWindowImageBoundsIgnoreFraming | cg.kCGWindowImageNominalResolution)
        if not shot:
            raise RuntimeError("WindowServer capture failed")
        data = cg.CGDataProviderCopyData(cg.CGImageGetDataProvider(shot))
        image = Image.frombytes("RGB", (cg.CGImageGetWidth(shot), cg.CGImageGetHeight(shot)),
                               bytes(data), "raw", "BGRX", cg.CGImageGetBytesPerRow(shot))
        return image.convert("L") if region else image

    def frontmost(self):
        cf.CFRunLoopRunInMode(cf.kCFRunLoopDefaultMode, 0.001, False)
        return AppKit.NSWorkspace.sharedWorkspace().frontmostApplication().processIdentifier() == self.pid

    def post(self, event):
        if not self.frontmost():
            raise RuntimeError("Measured application lost foreground")
        cg.CGEventPost(cg.kCGHIDEventTap, event)

    def click(self, x, y):
        for kind in [cg.kCGEventMouseMoved, cg.kCGEventLeftMouseDown, cg.kCGEventLeftMouseUp]:
            event = cg.CGEventCreateMouseEvent(None, kind, (self.x + x, self.y + y), cg.kCGMouseButtonLeft)
            cg.CGEventSetFlags(event, 0)
            cg.CGEventSetIntegerValueField(event, cg.kCGMouseEventClickState, 1)
            self.post(event)
            time.sleep(0.012)

    def key(self, code, modifiers=0):
        for pressed in [True, False]:
            event = cg.CGEventCreateKeyboardEvent(None, code, pressed)
            cg.CGEventSetFlags(event, modifiers)
            self.post(event)

    def wheel(self, delta):
        event = cg.CGEventCreateScrollWheelEvent(None, cg.kCGScrollEventUnitPixel, 1, delta)
        cg.CGEventSetFlags(event, 0)
        self.post(event)

    def memory(self):
        rows = []
        lines = subprocess.check_output(["ps", "-axo", "pid=,rss=,comm="], text=True).splitlines()
        for line in lines:
            pid_text, rss, command = line.split(None, 2)
            pid = int(pid_text)
            if pid != self.pid and system.responsibility_get_pid_responsible_for_pid(pid) != self.pid:
                continue
            usage = RUsageInfoV0()
            if system.proc_pid_rusage(pid, 0, ctypes.byref(usage)) != 0:
                raise RuntimeError(f"Could not read physical footprint of {pid}")
            rows.append(dict(role=Path(command).name, footprint=usage.physical_footprint, rss=int(rss) * 1024))
        if not any(row["role"].startswith("Mark Go Go Portrait ") for row in rows):
            raise RuntimeError("Host process missing from memory sample")
        return rows


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()
