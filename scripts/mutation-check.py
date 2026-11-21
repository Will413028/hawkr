#!/usr/bin/env python3
"""Mutation manifests: each listed source edit must make its test command fail.

Manifest (`**/mutations/*.json`): a JSON list of
  {"id", "file", "old", "new", "cmd", "expect"}
or, for a multi-file mutation, "edits": [{"file", "old", "new"}, ...] in place of
file/old/new. `file` is repo-relative, `old` must occur exactly once, `cmd` is a
shell command run from `backend/`, `expect` names the test that should kill it.

Each mutation is applied to the checkout, `cmd` runs, the files are restored from
memory, and the mutation counts as killed when `cmd` exits non-zero. Exit 1 when
a mutation survives or an edit does not apply.

Usage: mutation-check.py [--root DIR] [--check-only] [MANIFEST_OR_DIR ...]
  --check-only  validate that every edit applies; run nothing, write nothing.
"""
import argparse
import json
import subprocess
import sys
from pathlib import Path

CMD_DIR = "backend"


def load(manifest):
    items = json.loads(Path(manifest).read_text(encoding="utf-8"))
    if not isinstance(items, list):
        raise ValueError(f"{manifest}: top level must be a list")
    out = []
    for i, m in enumerate(items):
        for k in ("id", "cmd"):
            if not m.get(k):
                raise ValueError(f"{manifest}[{i}]: missing {k}")
        edits = m.get("edits") or [{k: m.get(k) for k in ("file", "old", "new")}]
        for e in edits:
            if not e.get("file") or not e.get("old") or e.get("new") is None:
                raise ValueError(f"{manifest}[{i}] {m['id']}: edit needs file, old, new")
        out.append({"id": m["id"], "cmd": m["cmd"], "expect": m.get("expect", ""), "edits": edits})
    return out


def apply_edits(root, edits):
    """Return ({path: original}, {path: mutated}); raise ValueError if an edit does not apply."""
    originals, mutated = {}, {}
    for e in edits:
        p = root / e["file"]
        if not p.is_file():
            raise ValueError(f"{e['file']}: no such file")
        text = mutated.get(p)
        if text is None:
            text = p.read_text(encoding="utf-8")
            originals[p] = text
        n = text.count(e["old"])
        if n != 1:
            raise ValueError(f"{e['file']}: old occurs {n} times, want exactly 1")
        mutated[p] = text.replace(e["old"], e["new"], 1)
    return originals, mutated


def run_one(root, m, check_only, runner=subprocess.run):
    """Return 'killed', 'survived' or 'error: ...'."""
    try:
        originals, mutated = apply_edits(root, m["edits"])
    except ValueError as err:
        return f"error: {err}"
    if check_only:
        return "applies"
    try:
        for p, text in mutated.items():
            p.write_text(text, encoding="utf-8")
        r = runner(m["cmd"], shell=True, cwd=root / CMD_DIR)
        result = "survived" if r.returncode == 0 else "killed"
    finally:
        for p, text in originals.items():
            p.write_text(text, encoding="utf-8")
    for p, text in originals.items():
        if p.read_text(encoding="utf-8") != text:
            return f"error: {p} not restored"
    if (root / ".git").exists() and subprocess.run(["git", "-C", str(root), "diff", "--quiet"]).returncode != 0:
        return "error: git diff not clean after restore"
    return result


def collect(root, args):
    if not args:
        return sorted(p for p in root.glob("**/mutations/*.json") if ".git" not in p.parts and "node_modules" not in p.parts)
    found = []
    for a in args:
        p = Path(a)
        found += sorted(p.glob("**/*.json")) if p.is_dir() else [p]
    return found


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--root", default=str(Path(__file__).resolve().parent.parent))
    ap.add_argument("--check-only", action="store_true")
    ap.add_argument("paths", nargs="*")
    a = ap.parse_args(argv)
    root = Path(a.root).resolve()
    manifests = collect(root, a.paths)
    if not manifests:
        print("no mutation manifests found")
        return 0
    bad = 0
    print(f"{'id':<8} {'result':<10} manifest / detail")
    for mf in manifests:
        for m in load(mf):
            res = run_one(root, m, a.check_only)
            ok = res in ("killed", "applies")
            bad += not ok
            print(f"{m['id']:<8} {res:<10} {Path(mf).name}" + (f"  (expect {m['expect']})" if m["expect"] and not ok else ""), flush=True)
    print("FAIL" if bad else "OK", f"{bad} problem(s)" if bad else "")
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
