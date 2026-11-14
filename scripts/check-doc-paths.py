#!/usr/bin/env python3
"""Check that references to repo-level documents resolve to real files.

Rules (docs/README.md is the source of truth):
- Paths are resolved from the repo root. Checked prefixes are CHECKED_DIRS,
  either at the repo root or under backend/.
- Files under a "遺留基準" entry in docs/README.md are not scanned.
- Files under a "過渡基準" entry fall back to backend/ when the repo-root
  resolution fails.
- A file whose header table has a "路徑基準" row resolves against that base
  first, then the repo root.
- Markdown links: relative links resolve against the file's directory,
  links starting with "/" against the repo root; both are URL-decoded.

Usage: check-doc-paths.py [--root DIR]
"""

from __future__ import annotations

import argparse
import posixpath
import re
import subprocess
import sys
from pathlib import Path
from urllib.parse import unquote

CHECKED_DIRS = ("docs/adr/", "docs/共通規範/", "docs/plans/")
README = "docs/README.md"
# This checker and its tests hold sample paths that intentionally do not exist.
SELF = {"scripts/check-doc-paths.py", "scripts/test_check_doc_paths.py"}
MAX_BYTES = 2_000_000

_dirs = "|".join(re.escape(d.removeprefix("docs/").rstrip("/")) for d in CHECKED_DIRS)
_stop = r"\s`'\"<>()\[\]{}|，。、；：「」『』（）！？"
BARE = re.compile(rf"(?<![\w./\-])((?:backend/)?docs/(?:{_dirs})/[^{_stop}]*)")
LINK = re.compile(r"\]\(([^)\s]+)(?:\s+\"[^\"]*\")?\)")
BASE_ROW = re.compile(r"^\|\s*路徑基準\s*\|\s*`?([^`|\s]+)`?\s*\|", re.M)
PLACEHOLDER = re.compile(r"YYYY|<|\{|\*|\.\.\.|…")


def tracked_files(root: Path) -> list[str]:
    out = subprocess.run(
        ["git", "-C", str(root), "ls-files", "-z"],
        check=True, capture_output=True,
    ).stdout
    return [p for p in out.decode("utf-8").split("\0") if p]


def readme_lists(root: Path) -> tuple[list[str], list[str]]:
    """Return (legacy, transitional) path prefixes from docs/README.md."""
    path = root / README
    if not path.exists():
        return [], []
    sections: dict[str, list[str]] = {}
    current = None
    for line in path.read_text(encoding="utf-8").splitlines():
        if line.startswith("## "):
            current = line[3:].strip()
            continue
        if current and line.lstrip().startswith("- "):
            sections.setdefault(current, []).extend(re.findall(r"`([^`]+)`", line))
    def pick(name: str) -> list[str]:
        return next((v for k, v in sections.items() if k.startswith(name)), [])
    return pick("遺留基準"), pick("過渡基準")


def under(path: str, prefixes: list[str]) -> bool:
    return any(path == p.rstrip("/") or path.startswith(p.rstrip("/") + "/") for p in prefixes)


def clean(token: str) -> str:
    token = token.split("#", 1)[0]
    token = re.sub(r":L?\d[\d,L\-]*$", "", token)
    return token.rstrip(".,:;*")


def is_checked(repo_path: str) -> bool:
    return any(repo_path.startswith(p) or repo_path.startswith("backend/" + p) for p in CHECKED_DIRS)


def exists(root: Path, repo_path: str) -> bool:
    norm = posixpath.normpath(repo_path)
    return not norm.startswith("..") and (root / norm).exists()


def check_file(root: Path, rel: str, text: str, transitional: bool) -> list[str]:
    bases = [""]
    m = BASE_ROW.search(text)
    if m:
        bases.insert(0, m.group(1).strip("/") + "/")
    if transitional:
        bases.append("backend/")
    errors = []
    for lineno, line in enumerate(text.splitlines(), 1):
        for tok in BARE.findall(line):
            tok = clean(tok)
            if not tok or PLACEHOLDER.search(tok) or tok.removeprefix("backend/") in CHECKED_DIRS:
                continue
            if not any(exists(root, b + tok) for b in bases):
                errors.append(f"{rel}:{lineno}: {tok}")
        for target in LINK.findall(line):
            if re.match(r"^[a-z][a-z0-9+.-]*:", target) or target.startswith("#"):
                continue
            target = clean(unquote(target))
            if not target or PLACEHOLDER.search(target):
                continue
            if target.startswith("/"):
                resolved = posixpath.normpath(target.lstrip("/"))
            else:
                resolved = posixpath.normpath(posixpath.join(posixpath.dirname(rel), target))
            if is_checked(resolved) and not exists(root, resolved):
                errors.append(f"{rel}:{lineno}: ]({target})")
    return errors


def run(root: Path) -> list[str]:
    legacy, transitional = readme_lists(root)
    errors = []
    for rel in tracked_files(root):
        if rel in SELF or under(rel, legacy):
            continue
        path = root / rel
        try:
            if not path.is_file() or path.stat().st_size > MAX_BYTES:
                continue
            data = path.read_bytes()
        except OSError:
            continue
        if b"\0" in data:
            continue
        try:
            text = data.decode("utf-8")
        except UnicodeDecodeError:
            continue
        errors += check_file(root, rel, text, under(rel, transitional))
    return errors


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parent.parent)
    args = parser.parse_args()
    errors = run(args.root)
    for e in errors:
        print(e)
    if errors:
        print(f"{len(errors)} unresolved document reference(s); rules in {README}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
