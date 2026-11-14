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
_path = rf"(?:backend/)?docs/(?:{_dirs})/"
_stop = r"\s`'\"<>()\[\]{}|，。、；：「」『』（）！？"
BARE = re.compile(rf"(?<![\w./\-])({_path}[^{_stop}]*)(.?)")
CODE = re.compile(r"`([^`\n]+)`")
CODE_PATH = re.compile(rf"^{_path}")
LINK = re.compile(r"\]\(([^)\s]+)(?:\s+\"[^\"]*\")?\)")
BASE_ROW = re.compile(r"^\|\s*路徑基準\s*\|\s*`?([^`|\s]+)`?\s*\|", re.M)
PLACEHOLDER = re.compile(r"YYYY|<|\{|\*|\.\.\.|…")
TEMPLATE_NEXT = set("<{*")
HEADER_LINES = 20


class Tree:
    """Tracked files and their parent directories; the only notion of "exists"."""

    def __init__(self, files: list[str]):
        self.files = set(files)
        self.dirs = {posixpath.dirname(f) for f in files}
        for d in list(self.dirs):
            while d:
                d = posixpath.dirname(d)
                self.dirs.add(d)

    def has(self, repo_path: str) -> bool:
        norm = posixpath.normpath(repo_path)
        return not norm.startswith("..") and (norm in self.files or norm in self.dirs)


def tracked_files(root: Path) -> list[str]:
    out = subprocess.run(
        ["git", "-C", str(root), "ls-files", "-z"],
        check=True, capture_output=True,
    ).stdout
    return [p for p in out.decode("utf-8").split("\0") if p]


def readme_lists(root: Path, tree: Tree) -> tuple[list[str], list[str]]:
    """Return (legacy, transitional) path prefixes from docs/README.md; fail closed."""
    path = root / README
    if not path.exists():
        raise SystemExit(f"{README} is missing; it defines the 遺留基準 and 過渡基準 lists")
    sections: dict[str, list[str]] = {}
    current = None
    for line in path.read_text(encoding="utf-8").splitlines():
        if line.startswith("## "):
            current = line[3:].strip()
            sections.setdefault(current, [])
            continue
        if current and line.startswith("- "):
            sections[current].extend(CODE.findall(line))
    lists = []
    for name in ("遺留基準", "過渡基準"):
        if name not in sections:
            raise SystemExit(f"{README}: heading '## {name}' not found")
        for entry in sections[name]:
            if not tree.has(entry.rstrip("/")):
                raise SystemExit(f"{README}: {name} entry `{entry}` is not a tracked path")
        lists.append(sections[name])
    return lists[0], lists[1]


def under(path: str, prefixes: list[str]) -> bool:
    return any(path == p.rstrip("/") or path.startswith(p.rstrip("/") + "/") for p in prefixes)


def clean(token: str) -> str:
    token = token.split("#", 1)[0]
    token = re.sub(r":L?\d[\d,L\-]*$", "", token)
    return token.rstrip(".,:;*")


def is_checked(repo_path: str) -> bool:
    return any(repo_path.startswith(p) or repo_path.startswith("backend/" + p) for p in CHECKED_DIRS)


def path_tokens(line: str) -> list[str]:
    """Plain-text paths outside Markdown links; a code span is taken whole."""
    line = LINK.sub(" ", line)
    tokens = []
    def code(m: re.Match) -> str:
        content = m.group(1).strip()
        if CODE_PATH.match(content):
            if not PLACEHOLDER.search(content):
                tokens.append(content)
            return " "
        return m.group(0)
    line = CODE.sub(code, line)
    for tok, nxt in BARE.findall(line):
        if nxt not in TEMPLATE_NEXT and not PLACEHOLDER.search(tok):
            tokens.append(tok)
    return tokens


def check_file(tree: Tree, rel: str, text: str, transitional: bool) -> list[str]:
    bases = [""]
    if rel.startswith(("docs/adr/", "backend/docs/adr/")):
        m = BASE_ROW.search("\n".join(text.splitlines()[:HEADER_LINES]))
        if m:
            bases.insert(0, m.group(1).strip("/") + "/")
    if transitional:
        bases.append("backend/")
    errors = []
    for lineno, line in enumerate(text.splitlines(), 1):
        for tok in path_tokens(line):
            tok = clean(unquote(tok))
            if not tok or tok.removeprefix("backend/") in CHECKED_DIRS:
                continue
            if not any(tree.has(b + tok) for b in bases):
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
            if is_checked(resolved) and not tree.has(resolved):
                errors.append(f"{rel}:{lineno}: ]({target})")
    return errors


def run(root: Path) -> list[str]:
    files = tracked_files(root)
    tree = Tree(files)
    legacy, transitional = readme_lists(root, tree)
    errors = []
    for rel in files:
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
        errors += check_file(tree, rel, text, under(rel, transitional))
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
