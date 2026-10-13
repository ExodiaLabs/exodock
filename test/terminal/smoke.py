#!/usr/bin/env python3
"""Exercise the built CLI in a real PTY. Docker mutations never occur: use demo."""

from __future__ import annotations

import codecs
import fcntl
import os
from pathlib import Path
import pty
import select
import signal
import struct
import subprocess
import sys
import termios
import time

import pyte


class Terminal:
    def __init__(self, binary: str, args: list[str], cols: int = 120, rows: int = 32):
        self.master, self.slave = pty.openpty()
        self.original = termios.tcgetattr(self.slave)
        self.screen = pyte.Screen(cols, rows)
        self.stream = pyte.Stream(self.screen)
        self.decoder = codecs.getincrementaldecoder("utf-8")("replace")
        self.raw = bytearray()
        self.resize(cols, rows)
        env = dict(os.environ, TERM="xterm-256color", NO_COLOR="1")
        self.process = subprocess.Popen(
            [binary, "--ascii", "--no-color", *args],
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
        t.key("signinforge\r")
        t.wait("SEARCH RESULTS")
        t.key("\r")
        t.wait("signinforge-auth-1")
        t.key("s")
        t.wait("CPU   1.20%")
        t.key("i")
        t.wait("Environment")
        t.wait("[hidden]")
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


if __name__ == "__main__":
    binary = str(Path(sys.argv[1] if len(sys.argv) > 1 else "dist/exodock").resolve())
    exercise_demo(binary)
    exercise_readonly_and_signals(binary)
    exercise_disconnected(binary)
    print("PTY journeys passed: actions, default cancellation, logs, search, disk, resize, read-only, disconnected state, SIGINT/SIGTERM and terminal restoration.")
