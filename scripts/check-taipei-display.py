#!/usr/bin/env python3
"""Guard Taipei time display in a Flutter app's lib/ (ADR multi-locale D5).

The compiler already keeps wall-clock accessors out of the apps (`hawkr_api` makes them library-private and
exposes only `TaipeiDisplay`). What it cannot see is `DateTime.toLocal()`, whose output follows the phone's
timezone, so this script fails on a `toLocal(` call in any .dart file under the given lib directory.

Comments, string contents (including raw and triple-quoted strings) are ignored; code inside `${...}` string
interpolation is checked.
"""

import re
import sys
from pathlib import Path

TO_LOCAL = re.compile(r"\btoLocal\(")


def code_only(text: str) -> str:
    """Return `text` with comments and string contents replaced by spaces (newlines kept)."""
    out = []
    i = 0
    n = len(text)
    stack: list = []

    def blank(c: str) -> str:
        return c if c == "\n" else " "

    while i < n:
        c = text[i]
        top = stack[-1] if stack else None
        if isinstance(top, tuple):
            quote, raw = top
            if c == "\\" and not raw:
                out.append(blank(c))
                if i + 1 < n:
                    out.append(blank(text[i + 1]))
                i += 2
                continue
            if text.startswith(quote, i):
                out.append(" " * len(quote))
                i += len(quote)
                stack.pop()
                continue
            if c == "$" and not raw and text.startswith("${", i):
                out.append("  ")
                i += 2
                stack.append("{")
                continue
            out.append(blank(c))
            i += 1
            continue
        if text.startswith("//", i):
            while i < n and text[i] != "\n":
                out.append(" ")
                i += 1
            continue
        if text.startswith("/*", i):
            depth = 0
            while i < n:
                if text.startswith("/*", i):
                    depth += 1
                    out.append("  ")
                    i += 2
                elif text.startswith("*/", i):
                    depth -= 1
                    out.append("  ")
                    i += 2
                    if depth == 0:
                        break
                else:
                    out.append(blank(text[i]))
                    i += 1
            continue
        raw = False
        j = i
        if c == "r" and i + 1 < n and text[i + 1] in "'\"" and not (
            i > 0 and (text[i - 1].isalnum() or text[i - 1] == "_")
        ):
            raw = True
            j = i + 1
        if text[j] in "'\"":
            quote = text[j] * 3 if text.startswith(text[j] * 3, j) else text[j]
            out.append(" " * (j - i + len(quote)))
            i = j + len(quote)
            stack.append((quote, raw))
            continue
        if c == "{" and stack:
            stack.append("{")
        elif c == "}" and stack and top == "{":
            stack.pop()
            out.append(" ")
            i += 1
            continue
        out.append(c)
        i += 1
    return "".join(out)


def violations(lib: Path):
    for path in sorted(lib.rglob("*.dart")):
        code = code_only(path.read_text(encoding="utf-8"))
        for n, line in enumerate(code.splitlines(), 1):
            if TO_LOCAL.search(line):
                yield f"{path}:{n}: toLocal( is banned; show time through TaipeiDisplay"


def main(argv):
    if len(argv) != 2:
        print("usage: check-taipei-display.py <app>/lib", file=sys.stderr)
        return 2
    lib = Path(argv[1])
    if not lib.is_dir():
        print(f"not a directory: {lib}", file=sys.stderr)
        return 2
    found = list(violations(lib))
    for v in found:
        print(v)
    return 1 if found else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
