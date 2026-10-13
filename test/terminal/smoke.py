#!/usr/bin/env python3
"""Exercise the built CLI in a real PTY. Docker mutations never occur: use demo."""

from __future__ import annotations

import codecs
import fcntl
import os
from pathlib import Path
import pty
import re
import select
import signal
import struct
import subprocess
import sys
import termios
import time

import pyte


class Screen(pyte.Screen):
    """Support xterm SU/SD used by Bubble Tea's differential renderer."""

    def scroll_up(self, count: int = 1):
        self.scroll_region(count, upward=True)

    def scroll_down(self, count: int = 1):
        self.scroll_region(count, upward=False)

    def scroll_region(self, count: int, upward: bool):
        top, bottom = self.margins or (0, self.lines - 1)
        saved_y = self.cursor.y
        edge = bottom if upward else top
        try:
            self.cursor.y = edge
            for _ in range(min(count or 1, bottom - top + 1)):
                if upward:
                    self.index()
                else:
                    self.reverse_index()
                # xterm erases new cells with the active background (BCE).
                for x in range(self.columns):
                    self.buffer[edge][x] = self.cursor.attrs
        finally:
            self.cursor.y = saved_y


class Stream(pyte.Stream):
    csi = {**pyte.Stream.csi, "S": "scroll_up", "T": "scroll_down"}
    events = pyte.Stream.events | {"scroll_up", "scroll_down"}


class Terminal:
    def __init__(self, binary: str, args: list[str], cols: int = 120, rows: int = 32, color: bool = False):
        self.master, self.slave = pty.openpty()
        self.original = termios.tcgetattr(self.slave)
        self.screen = Screen(cols, rows)
        self.stream = Stream(self.screen)
        self.decoder = codecs.getincrementaldecoder("utf-8")("replace")
        self.raw = bytearray()
        self.resize(cols, rows)
        env = dict(os.environ, TERM="xterm-256color", COLORTERM="truecolor")
        display_args = [] if color else ["--ascii", "--no-color"]
        if color:
            env.pop("NO_COLOR", None)
        else:
            env["NO_COLOR"] = "1"
        self.process = subprocess.Popen(
            [binary, *display_args, *args],
            stdin=self.slave, stdout=self.slave, stderr=self.slave,
            env=env, start_new_session=True,
        )

    def resize(self, cols: int, rows: int):
        fcntl.ioctl(self.slave, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))
        self.screen.resize(rows, cols)
        if hasattr(self, "process"):
            self.process.send_signal(signal.SIGWINCH)

    def pump(self, timeout: float = 0.1):
        if select.select([self.master], [], [], timeout)[0]:
            try:
                data = os.read(self.master, 65536)
            except OSError:
                return
            self.raw.extend(data)
            self.stream.feed(self.decoder.decode(data))

    def text(self) -> str:
        return "\n".join(self.screen.display)

    def wait(self, text: str, timeout: float = 10):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if text in self.text():
                return
            if self.process.poll() is not None:
                raise AssertionError(f"CLI exited {self.process.returncode} while waiting for {text!r}\n{self.text()}")
            self.pump()
        raise AssertionError(f"Timed out waiting for {text!r}\n{self.text()}")

    def key(self, text: str):
        os.write(self.master, text.encode())

    def wait_absent(self, text: str, timeout: float = 10):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if text not in self.text():
                return
            self.pump()
        raise AssertionError(f"Stale terminal text {text!r}\n{self.text()}")

    def wait_pattern(self, pattern: str, timeout: float = 10):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if re.search(pattern, self.text()):
                return
            self.pump()
        raise AssertionError(f"Missing terminal pattern {pattern!r}\n{self.text()}")

    def background(self, row: int, start: int, end: int, expected: str):
        observed = {self.screen.buffer[row][col].bg for col in range(start, end)}
        assert observed == {expected}, f"Background lost on row {row}, cells {start}:{end}: {observed}, expected {expected}\n{self.text()}"

    def phrase_background(self, phrase: str, expected: str):
        for row, line in enumerate(self.screen.display):
            col = line.find(phrase)
            if col >= 0:
                self.background(row, col, col + len(phrase), expected)
                return
        raise AssertionError(f"Missing phrase {phrase!r}\n{self.text()}")

    def exit(self, key: str | None = "q", sig: int | None = None):
        if sig is not None:
            self.process.send_signal(sig)
        elif key is not None:
            self.key(key)
        deadline = time.monotonic() + 5
        while self.process.poll() is None and time.monotonic() < deadline:
            self.pump()
        assert self.process.poll() == 0, f"CLI did not exit cleanly: {self.process.poll()}\n{self.text()}"
        self.pump(0)
        restored = termios.tcgetattr(self.slave)
        mask = termios.ECHO | termios.ICANON | termios.ISIG
        assert restored[3] & mask == self.original[3] & mask, "Terminal input modes were not restored"
        assert b"\x1b[?1049l" in self.raw, "Alternate terminal screen was not restored"

    def close(self):
        if self.process.poll() is None:
            self.process.kill()
            self.process.wait()
        os.close(self.master)
        os.close(self.slave)


def exercise_demo(binary: str):
    t = Terminal(binary, ["--demo", "--refresh", "1s"])
    try:
        t.wait("CONNECTED")
        t.wait("YOUR DOCKER WORKSPACE")
        t.key("2\t")
        t.wait("exolan-api-1")
        t.key("a")
        t.wait("CONTAINER ACTIONS")
        t.key("j\r")
        t.wait("CONFIRM STOP")
        t.wait("[ CANCEL ]")
        t.key("\r")  # Default choice cancels; reopening still permits Stop.
        t.key("a")
        t.wait("CONTAINER ACTIONS")
        t.key("j\r")
        t.wait("CONFIRM STOP")
        t.key("\t\r")
        t.wait("stop completed")
        t.wait("exited")
        t.key("l")
        t.wait("request completed")
        t.key(" ")
        t.wait("PAUSED")
        t.key("f")
        t.wait("LIVE")
        t.key("v")
        t.wait("stdout")
        t.key("v")
        t.wait("stderr")
        t.key("t")
        t.wait("last 15m")
        t.key("/")
        t.wait("SEARCH LOADED LOGS")
        t.key("upstream\r")
        t.wait("upstream retry")
        t.key("\x1b")
        t.wait("Enter details")
        t.key("a")
        t.wait("CONTAINER ACTIONS")
        t.key("jjj\r")
        t.wait("CONFIRM REMOVE")
        t.wait("demo-api-001")
        t.key("\t\r")
        t.wait("remove completed")
        t.key("\x0b")  # Ctrl+k
        t.wait("FIND A RESOURCE")
        t.key("signinforge")
        t.wait("signinforge-auth-1")  # Results appear while typing.
        t.key("\r")
        t.wait("signinforge-auth-1")
        t.key("s")
        t.wait("CPU history")
        t.wait("1.20%")
        t.key("i")
        t.wait("Environment")
        t.wait("[hidden]")
        t.key(":")
        t.wait("COMMAND PALETTE")
        t.key("System")
        t.wait("Go to System")
        t.key("\r")
        t.wait("DOCKER DAEMON")
        t.key("8d")
        t.wait("Measured")
        t.resize(60, 18)
        t.key("2")
        t.wait("CONTAINERS")
        t.resize(42, 12)
        t.wait("Terminal too small")
        t.exit()
    finally:
        t.close()


def exercise_readonly_and_signals(binary: str):
    for sig in [signal.SIGTERM, signal.SIGINT]:
        t = Terminal(binary, ["--demo", "--readonly"])
        try:
            t.wait("READ ONLY")
            t.key("2a")
            t.wait("Read-only mode")
            t.key("\r")
            t.pump()
            assert "CONFIRM START" not in t.text(), "Read-only controls opened a mutation"
            t.exit(sig=sig)
        finally:
            t.close()


def exercise_disconnected(binary: str):
    t = Terminal(binary, ["--host", "unix:///tmp/exodock-nonexistent-smoke.sock"])
    try:
        t.wait("DISCONNECTED")
        t.wait("retry")
        t.key("?")
        t.wait("KEYBOARD GUIDE")
        t.key("\x1b")
        t.wait("YOUR DOCKER WORKSPACE")
        t.exit()
    finally:
        t.close()


def exercise_color_themes(binary: str):
    for theme in ["dark", "light"]:
        t = Terminal(binary, ["--demo", "--theme", theme, "--reduced-motion"], color=True)
        try:
            t.wait("CONNECTED")
            t.wait("HEALTH DISTRIBUTION")
            t.wait("signinforge-auth-1")
            assert re.search(rb"\x1b\[[0-9;:]*38[;:]2[;:][0-9;:]+m", t.raw), "Missing truecolor foreground"
            assert re.search(rb"\x1b\[[0-9;:]*48[;:]2[;:][0-9;:]+m", t.raw), "Missing truecolor surface background"
            header, canvas, card, selected = (
                ("15111e", "0b0910", "27222f", "543680") if theme == "dark"
                else ("f5f2f9", "ece5f5", "ece5f5", "decdf2")
            )
            t.background(0, 0, t.screen.columns, header)
            t.background(2, 0, t.screen.columns, canvas)
            t.phrase_background("CONTAINERS", card)
            t.phrase_background("loaded inventory", card)
            card_row = next(row for row, line in enumerate(t.screen.display) if "CONTAINERS" in line and "RUNNING" in line)
            card_line = t.screen.display[card_row]
            card_left = card_line.index("CONTAINERS") - 1
            card_right = card_line.index("RUNNING") - 2  # One space separates the cards.
            card_bottom = next(row for row, line in enumerate(t.screen.display) if row > card_row and "loaded inventory" in line)
            for row in range(card_row, card_bottom + 1):
                t.background(row, card_left, card_right, card)
            t.key("\x0b")
            t.wait("FIND A RESOURCE")
            t.key("exolan")
            t.wait("› exolan_")
            t.wait("Up/Down select")
            top = next(row for row, line in enumerate(t.screen.display) if "FIND A RESOURCE" in line)
            bottom = next(row for row, line in enumerate(t.screen.display) if row > top and "Up/Down select" in line)
            left = t.screen.display[top].index("╭")
            right = t.screen.display[top].index("╮") + 1
            results = "\n".join(line[left:right] for line in t.screen.display[top:bottom+1])
            assert "exolan-api-1" in results and "exolan_db" in results
            assert "signinforge" not in results, "Filtered search retained old result rows"
            t.key("\x1b")
            t.wait_absent("FIND A RESOURCE")
            t.key("2\t")
            t.wait("items · all resources")
            # A PTY read can end halfway through a differential frame.
            t.wait_pattern(r"exolan-api-1 +running +healthy")
            for row, line in enumerate(t.screen.display):
                name_col = line.find("exolan-api-1")
                if name_col >= 0 and "running" in line:
                    left, right = line.rfind("│", 0, name_col), line.find("│", name_col)
                    assert left >= 0 and right > left, "Cannot locate selected row panel"
                    t.background(row, left + 1, right, selected)
                    break
            else:
                raise AssertionError("Missing selected container status row\n" + t.text())
            t.key("a")
            t.wait("CONTAINER ACTIONS")
            t.key("j\r")
            t.wait("CONFIRM STOP")
            t.resize(60, 18)
            t.wait("demo-api-001")
            t.wait("CANCEL")
            t.wait("isolated sandbox")
            t.resize(42, 12)
            t.wait("Terminal too small")
            t.key("\r")
            t.resize(120, 32)
            t.wait("CONNECTED")
            assert "CONFIRM STOP" not in t.text(), "Hidden confirmation survived resize"
            t.exit()
        finally:
            t.close()


def exercise_emulator_scroll_regions():
    screen = Screen(5, 5)
    stream = Stream(screen)
    stream.feed("A\r\nB\r\nC\r\nD\r\nE")
    stream.feed("\x1b[2;4r\x1b[5;3H\x1b[48;2;21;17;30m\x1b[S")
    assert [line.strip() for line in screen.display] == ["A", "C", "D", "", "E"]
    assert (screen.cursor.x, screen.cursor.y) == (2, 4), "SU changed cursor position"
    assert {screen.buffer[3][x].bg for x in range(5)} == {"15111e"}, "SU lost erase background"
    stream.feed("\x1b[T")
    assert [line.strip() for line in screen.display] == ["A", "", "C", "D", "E"]
    assert (screen.cursor.x, screen.cursor.y) == (2, 4), "SD changed cursor position"


if __name__ == "__main__":
    binary = str(Path(sys.argv[1] if len(sys.argv) > 1 else "dist/exodock").resolve())
    exercise_emulator_scroll_regions()
    exercise_demo(binary)
    exercise_readonly_and_signals(binary)
    exercise_disconnected(binary)
    exercise_color_themes(binary)
    print("PTY journeys passed: actions, default cancellation, logs, live search/palette, disk, resize, dark/light truecolor, read-only, disconnected state, SIGINT/SIGTERM and terminal restoration.")
