#!/usr/bin/env python3
"""Check local destinations and heading fragments in repository Markdown."""

from __future__ import annotations

import re
import sys
from pathlib import Path
from urllib.parse import unquote, urlsplit

ROOT = Path(__file__).resolve().parents[1]
LINK = re.compile(r"!?\[[^\]]*\]\(\s*(?:<([^>]+)>|([^\s)]+))(?:\s+[^)]*)?\)")
HEADING = re.compile(r"^#{1,6}\s+(.+?)\s*#*\s*$")
HTML_ID = re.compile(r"\bid\s*=\s*['\"]([^'\"]+)['\"]", re.IGNORECASE)


def heading_anchors(markdown: str) -> set[str]:
    anchors: set[str] = set()
    counts: dict[str, int] = {}
    for line in markdown.splitlines():
        heading = HEADING.match(line)
        if heading:
            text = re.sub(r"[`*_~]", "", heading.group(1)).lower()
            slug = re.sub(r"[^\w -]", "", text, flags=re.UNICODE)
            slug = re.sub(r"\s", "-", slug.strip()).replace("--", "-")
            duplicate = counts.get(slug, 0)
            counts[slug] = duplicate + 1
            anchors.add(slug if duplicate == 0 else f"{slug}-{duplicate}")
        anchors.update(HTML_ID.findall(line))
    return anchors


def markdown_files() -> list[Path]:
    return sorted(path for path in ROOT.rglob("*.md") if ".git" not in path.parts)


def check() -> list[str]:
    errors: list[str] = []
    markdown_paths = markdown_files()
    text_by_path = {path: path.read_text(encoding="utf-8") for path in markdown_paths}
    anchors_by_path = {path: heading_anchors(text) for path, text in text_by_path.items()}

    for source, text in text_by_path.items():
        for match in LINK.finditer(text):
            destination = match.group(1) or match.group(2)
            parsed = urlsplit(destination)
            if parsed.scheme or parsed.netloc:
                continue

            relative_path = unquote(parsed.path)
            target = (source.parent / relative_path).resolve() if relative_path else source
            line = text.count("\n", 0, match.start()) + 1
            if not target.exists():
                errors.append(f"{source.relative_to(ROOT)}:{line}: missing {destination}")
                continue

            fragment = unquote(parsed.fragment)
            if fragment and target in anchors_by_path and fragment not in anchors_by_path[target]:
                errors.append(
                    f"{source.relative_to(ROOT)}:{line}: missing anchor #{fragment} in "
                    f"{target.relative_to(ROOT)}"
                )

    return errors


if __name__ == "__main__":
    problems = check()
    if problems:
        print("\n".join(problems), file=sys.stderr)
        raise SystemExit(1)
    print(f"Checked local links in {len(markdown_files())} Markdown files.")
