#!/usr/bin/env python3
"""Capture actual demo PTY cells as SVG; requires test/terminal/requirements.txt."""
import argparse
import importlib.util
from html import escape
from pathlib import Path
import time

module = importlib.util.spec_from_file_location(
    "terminal_smoke", Path(__file__).resolve().parents[1] / "test/terminal/smoke.py"
)
smoke = importlib.util.module_from_spec(module)
module.loader.exec_module(smoke)

ANSI = {"black": "#000000", "red": "#cd3131", "green": "#0dbc79",
        "brown": "#e5e510", "blue": "#2472c8", "magenta": "#bc3fbc",
        "cyan": "#11a8cd", "white": "#e5e5e5"}


def color(value, default):
    if value == "default":
        return default
    return ANSI.get(value, "#" + value)


def capture(binary, destination, theme, scene, cols=120, rows=36):
    terminal = smoke.Terminal(binary, ["--demo", "--theme", theme, "--reduced-motion"], cols, rows, color=True)
    try:
        terminal.wait("YOUR DOCKER WORKSPACE")
        if scene == "containers":
            terminal.key("2\t")
            terminal.wait("exolan-api-1")
        elif scene == "logs":
            terminal.key("2\tl")
            terminal.wait("request completed")
        elif scene == "search":
            terminal.key("\x0b")
            terminal.wait("FIND A RESOURCE")
            terminal.key("exolan")
        deadline = time.monotonic() + 0.5
        while time.monotonic() < deadline:
            terminal.pump()
        cells = terminal.screen.buffer
        bg = "#ece5f5" if theme == "light" else "#0b0910"
        fg = "#15111e" if theme == "light" else "#f5f2f9"
        svg = [f'<svg xmlns="http://www.w3.org/2000/svg" width="{cols*10+32}" height="{rows*20+32}" viewBox="0 0 {cols*10+32} {rows*20+32}">',
               f'<rect width="100%" height="100%" fill="{bg}"/>',
               '<g font-family="DejaVu Sans Mono,monospace" font-size="16" xml:space="preserve">']
        for y in range(rows):
            for x in range(cols):
                cell = cells[y][x]
                cellfg, cellbg = color(cell.fg, fg), color(cell.bg, bg)
                if cell.reverse:
                    cellfg, cellbg = cellbg, cellfg
                px, py = 16 + x*10, 16 + y*20
                svg.append(f'<rect x="{px}" y="{py}" width="10" height="20" fill="{cellbg}"/>')
                if cell.data.strip():
                    weight = "bold" if cell.bold else "normal"
                    svg.append(f'<text x="{px}" y="{py+16}" fill="{cellfg}" font-weight="{weight}">{escape(cell.data)}</text>')
        svg.extend(["</g>", "</svg>"])
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_text("\n".join(svg) + "\n")
        terminal.exit(key="\x03")  # Ctrl+c also quits while a text overlay is open.
    finally:
        terminal.close()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary")
    parser.add_argument("output", type=Path)
    parser.add_argument("--theme", choices=["dark", "light"], default="dark")
    parser.add_argument("--scene", choices=["overview", "containers", "logs", "search"], default="overview")
    parser.add_argument("--cols", type=int, default=120)
    parser.add_argument("--rows", type=int, default=36)
    args = parser.parse_args()
    capture(args.binary, args.output, args.theme, args.scene, args.cols, args.rows)
