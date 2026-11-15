#!/usr/bin/env python3
"""List the packages whose test binary builds differently under the integration tag.

A package needs an integration run when its own files, or the files of any package
linked into its test binary, change under `-tags integration`. Every other package
builds the same test binary with and without the tag, so the plain `go test` run
already covers it.

Run from backend/ with GOWORK=off. Prints one import path per line; exits non-zero
when `go list` fails or nothing qualifies (the filter would be broken).
"""

import json
import subprocess
import sys


def go_list(args: list[str]) -> list[dict]:
    out = subprocess.run(
        ["go", "list", "-e=false", "-json", *args],
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    dec = json.JSONDecoder()
    pkgs, i = [], 0
    while i < len(out):
        while i < len(out) and out[i].isspace():
            i += 1
        if i == len(out):
            break
        obj, i = dec.raw_decode(out, i)
        pkgs.append(obj)
    return pkgs


def base(path: str) -> str:
    return path.split(" ", 1)[0]


def file_sets(tags: list[str]) -> dict[str, tuple]:
    sets = {}
    for p in go_list([*tags, "-deps", "-test", "./..."]):
        key = (
            tuple(sorted(p.get("GoFiles", []))),
            tuple(sorted(p.get("CgoFiles", []))),
            tuple(sorted(p.get("TestGoFiles", []))),
            tuple(sorted(p.get("XTestGoFiles", []))),
        )
        sets.setdefault(base(p["ImportPath"]), set()).add(key)
    return {k: tuple(sorted(v)) for k, v in sets.items()}


def main() -> int:
    plain = file_sets([])
    tagged = file_sets(["-tags", "integration"])
    changed = {p for p in plain.keys() | tagged.keys() if plain.get(p) != tagged.get(p)}

    selected = set()
    for p in go_list(["-tags", "integration", "-test", "./..."]):
        path = p["ImportPath"]
        if not path.endswith(".test"):
            continue
        under_test = path.removesuffix(".test")
        linked = {base(d) for d in p.get("Deps", [])} | {under_test}
        if linked & changed:
            selected.add(under_test)

    if not selected:
        print("integration-packages: no package changes under the integration tag; the filter is broken", file=sys.stderr)
        return 1
    print("\n".join(sorted(selected)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
