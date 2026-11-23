#!/usr/bin/env python3
"""Guard Taipei time display in a Flutter app's lib/ (ADR multi-locale D5).

Fails when a .dart file under the given lib directory
  - calls `toLocal(` (output would follow the phone's timezone), or
  - calls or tear-offs (`.map(formatTaipeiDate)`) formatTaipeiTimestamp/DateTime/Date outside shared/time/taipei_display.dart.
Whole-line and trailing `//` comments are ignored (a `//` inside a string literal is not a comment).
"""

import re
import sys
from pathlib import Path

HELPER = Path("shared/time/taipei_display.dart")
TO_LOCAL = re.compile(r"\btoLocal\(")
FORMAT = re.compile(r"\bformatTaipei(?:Timestamp|DateTime|Date)\b")


def strip_comment(line: str) -> str:
    quote = None
    i = 0
    while i < len(line):
        c = line[i]
        if quote:
            if c == "\\":
                i += 1
            elif c == quote:
                quote = None
        elif c in "'\"":
            quote = c
        elif line.startswith("//", i):
            return line[:i]
        i += 1
    return line


def violations(lib: Path):
    for path in sorted(lib.rglob("*.dart")):
        rel = path.relative_to(lib)
        is_helper = rel == HELPER
        for n, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
            line = strip_comment(line)
            if TO_LOCAL.search(line):
                yield f"{path}:{n}: toLocal( is banned; show time through TaipeiDisplay"
            if not is_helper and FORMAT.search(line):
                yield f"{path}:{n}: call formatTaipei* only inside {HELPER}; use context.taipeiDisplay"


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
