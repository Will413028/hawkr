#!/usr/bin/env python3
"""Prove a test or guard can fail, without touching the working tree.

Every run happens in a throwaway copy of the repository (tracked and untracked,
non-ignored files; ``--link`` adds ignored paths such as a venv as symlinks). The
working tree is only read. Two modes:

Mutation (``--edit``): each edit is its own mutant in its own copy, so one mutant can
never hide another. Before the mutants, a baseline copy must pass and, for each edited
file, a canary copy must fail: first with a junk line put in front of the file (code
that parses it fails), then, if that passes, with the whole content replaced by the junk
line (a test that only looks for some lines, in YAML or prose, fails); the file always
stays in place, so a check that only needs it to exist proves nothing. If both pass, the
command is not reading the copy (an editable install pointing at the original, for
example) and no verdict would mean anything. A mutant is killed when the command fails
and its output matches ``--expect`` (the failing test's name; a mutant that breaks the
import also fails, which proves nothing), survived when it passes; ``--any-failure``
waives the match on purpose.

Revision (``--rev``): the command must pass on the working tree and fail on a copy of
the revision with the ``--overlay`` paths (the new test) taken from the working tree:
the "red before the fix, green after" proof for a regression test.

Exit codes: 0 every mutant killed (or red at the revision); 1 a mutant survived (or
green at the revision); 2 no verdict (baseline failing, canary passing, anchor not
unique, a path outside the repository). Rules and cases: scripts/README.md
"red-proof.py".
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
from dataclasses import dataclass
from pathlib import Path

CANARY = "@@ red-proof canary: this line must not parse @@ )(\n"
TAIL = 3000


class NoVerdict(Exception):
    """The runner cannot judge: exit code 2."""


@dataclass(frozen=True)
class Edit:
    path: str
    old: str
    new: str
    label: str


@dataclass(frozen=True)
class Run:
    code: int
    output: str


def repo_root(start: Path) -> Path:
    out = subprocess.run(["git", "-C", str(start), "rev-parse", "--show-toplevel"],
                         capture_output=True, text=True, check=False)
    if out.returncode != 0:
        raise NoVerdict(f"not a git repository: {start}")
    return Path(out.stdout.strip())


def working_files(repo: Path) -> list[str]:
    out = subprocess.run(["git", "-C", str(repo), "ls-files", "-z", "-co", "--exclude-standard"],
                         capture_output=True, check=True).stdout.decode()
    return sorted({name for name in out.split("\0") if name})


def copy_working_tree(repo: Path, dest: Path, links: list[str]) -> None:
    for name in working_files(repo):
        source = repo / name
        target = dest / name
        if source.is_symlink():
            target.parent.mkdir(parents=True, exist_ok=True)
            target.symlink_to(os.readlink(source))
        elif source.is_file():
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(source, target)
    link_paths(repo, dest, links)
    init_git(dest)


def copy_revision(repo: Path, rev: str, dest: Path, links: list[str], overlays: list[str]) -> None:
    dest.mkdir(parents=True, exist_ok=True)
    archive = subprocess.run(["git", "-C", str(repo), "archive", "--format=tar", rev],
                             capture_output=True, check=False)
    if archive.returncode != 0:
        raise NoVerdict(f"git archive {rev} failed: {archive.stderr.decode().strip()}")
    subprocess.run(["tar", "-x", "-C", str(dest)], input=archive.stdout, check=True)
    for name in overlays:
        source = inside(repo, name)
        if not source.is_file():
            raise NoVerdict(f"overlay is not a file in the working tree: {name}")
        target = dest / name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(source, target)
    link_paths(repo, dest, links)
    init_git(dest)


def init_git(dest: Path) -> None:
    """Make the copy its own repository (one snapshot commit), so code that runs git in it
    works without reaching the original's .git; no hooks, no signing."""
    quiet = {"capture_output": True, "check": True}
    subprocess.run(["git", "-C", str(dest), "init", "-q"], **quiet)
    subprocess.run(["git", "-C", str(dest), "add", "-A"], **quiet)
    subprocess.run(["git", "-C", str(dest), "-c", "core.hooksPath=/dev/null",
                    "-c", "commit.gpgsign=false", "-c", "user.name=red-proof",
                    "-c", "user.email=red-proof@localhost", "commit", "-q", "--no-verify",
                    "--allow-empty", "-m", "red-proof snapshot"], **quiet)


def link_paths(repo: Path, dest: Path, links: list[str]) -> None:
    for name in links:
        source = inside(repo, name)
        if not source.exists() and not source.is_symlink():
            raise NoVerdict(f"--link path does not exist: {name}")
        target = dest / name
        if target.exists() or target.is_symlink():
            continue  # tracked in the copy already
        target.parent.mkdir(parents=True, exist_ok=True)
        target.symlink_to(source)


def inside(repo: Path, name: str) -> Path:
    path = Path(os.path.normpath(repo / name))
    if not path.is_relative_to(repo) or path == repo:
        raise NoVerdict(f"path outside the repository: {name}")
    return path


def check_anchor(copy: Path, edit: Edit) -> None:
    target = copy / edit.path
    if not target.is_file():
        raise NoVerdict(f"{edit.label}: no such file {edit.path}")
    if not target.resolve().is_relative_to(copy.resolve()):  # a --link'd or absolute symlink
        raise NoVerdict(f"{edit.label}: {edit.path} resolves outside the copy; editing it would "
                        "change the original")
    count = target.read_text().count(edit.old)
    if count != 1:
        raise NoVerdict(f"{edit.label}: anchor found {count} times in {edit.path}; it must be unique")


def apply_edit(copy: Path, edit: Edit) -> None:
    check_anchor(copy, edit)
    target = copy / edit.path
    target.write_text(target.read_text().replace(edit.old, edit.new))


def run(copy: Path, cwd: str, command: list[str], timeout: int) -> Run:
    env = {**os.environ, "RED_PROOF": "1", "PYTHONDONTWRITEBYTECODE": "1"}
    try:
        done = subprocess.run(command, cwd=copy / cwd, env=env, capture_output=True, text=True,
                              timeout=timeout, check=False)
    except subprocess.TimeoutExpired:
        raise NoVerdict(f"command timed out after {timeout}s in {copy / cwd}") from None
    return Run(done.returncode, done.stdout + done.stderr)


def digest(paths: list[Path]) -> str:
    h = hashlib.sha256()
    for path in paths:
        h.update(str(path).encode())
        h.update(path.read_bytes() if path.is_file() else b"<missing>")
    return h.hexdigest()


def failed_line(output: str, expect: re.Pattern[str] | None) -> str | None:
    """The line that proves the failure: the ``--expect`` match, else the last non-empty line."""
    if expect is not None:
        # The whole matching line: with pytest's -rf it carries the test and the reason.
        for line in output.splitlines():
            if expect.search(line):
                return line
        return None
    lines = [line for line in output.splitlines() if line.strip()]
    return lines[-1] if lines else "(no output)"


def parse_edits(args: argparse.Namespace) -> list[Edit]:
    edits = [Edit(p, o, n, f"M{i}") for i, (p, o, n) in enumerate(args.edit or [], 1)]
    for source in args.edits:
        for item in json.loads(Path(source).read_text()):
            i = len(edits) + 1
            edits.append(Edit(item["path"], item["old"], item["new"], item.get("id", f"M{i}")))
    seen: set[str] = set()
    for edit in edits:
        if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]*", edit.label):
            raise NoVerdict(f"mutant id {edit.label!r}: use letters, digits, '_', '.', '-' "
                            "(it names the copy's directory)")
        key = edit.label.casefold()
        if key in seen:
            raise NoVerdict(f"duplicate mutant id {edit.label!r}: each mutant needs its own id "
                            "(ids differing only in case share a directory on macOS)")
        seen.add(key)
    return edits


def mutation_mode(repo: Path, args: argparse.Namespace, edits: list[Edit], work: Path,
                  expect: re.Pattern[str] | None) -> int:
    for edit in edits:
        inside(repo, edit.path)
    print(f"{'id':<6} {'verdict':<9} detail")

    base = work / "baseline"
    copy_working_tree(repo, base, args.link)
    for edit in edits:  # every anchor is checked before anything runs
        check_anchor(base, edit)
    result = run(base, args.cwd, args.command, args.timeout)
    if result.code != 0:
        print(result.output[-TAIL:])
        raise NoVerdict(f"baseline fails without any edit (exit {result.code})")
    print(f"{'base':<6} {'pass':<9} unchanged copy passes")

    # One canary per edited file: each may be loaded by a different path (src/ vs scripts/).
    for n, path in enumerate(dict.fromkeys(edit.path for edit in edits), 1):
        canary = work / f"canary{n}"
        copy_working_tree(repo, canary, args.link)
        target = canary / path
        original = target.read_text()
        for mode, content in (("junk line first", CANARY + original), ("content replaced", CANARY)):
            target.write_text(content)
            if run(canary, args.cwd, args.command, args.timeout).code != 0:
                print(f"{'canary':<6} {'fail':<9} the command reads {path} from the copy ({mode})")
                break
        else:
            raise NoVerdict(f"canary passed: the command does not read {path} from the copy, with "
                            "a junk line first or the content replaced (an install pointing at the "
                            "original, a test that never loads it, or a tool that tolerates "
                            "unparsable files, such as zizmor without --strict-collection, "
                            "dependency-cruiser, knip, or a text-only count test; for those, prove "
                            "the violation in a manual copy instead)")

    survived = 0
    for edit in edits:
        copy = work / edit.label
        copy_working_tree(repo, copy, args.link)
        apply_edit(copy, edit)
        result = run(copy, args.cwd, args.command, args.timeout)
        line = failed_line(result.output, expect)
        if result.code != 0 and line is not None:
            print(f"{edit.label:<6} {'killed':<9} exit {result.code}: {line.strip()[:300]}")
        elif result.code != 0:
            print(result.output[-TAIL:])
            raise NoVerdict(f"{edit.label}: exit {result.code} but the output does not match "
                            f"--expect {expect.pattern if expect else ''}")
        else:
            survived += 1
            print(f"{edit.label:<6} {'SURVIVED':<9} {edit.path}: the command still passes")
    return 1 if survived else 0


def revision_mode(repo: Path, args: argparse.Namespace, work: Path,
                  expect: re.Pattern[str] | None) -> int:
    now = work / "now"
    copy_working_tree(repo, now, args.link)
    result = run(now, args.cwd, args.command, args.timeout)
    if result.code != 0:
        print(result.output[-TAIL:])
        raise NoVerdict(f"the command fails on the working tree (exit {result.code})")
    print(f"{'now':<6} {'pass':<9} working tree passes")
    then = work / "revision"
    copy_revision(repo, args.rev, then, args.link, args.overlay)
    result = run(then, args.cwd, args.command, args.timeout)
    line = failed_line(result.output, expect)
    if result.code != 0 and line is not None:
        print(f"{'rev':<6} {'red':<9} {args.rev} exit {result.code}: {line.strip()[:200]}")
        return 0
    if result.code != 0:
        print(result.output[-TAIL:])
        raise NoVerdict(f"{args.rev}: exit {result.code} but the output does not match --expect")
    print(f"{'rev':<6} {'GREEN':<9} {args.rev} passes too: the test does not catch the old code")
    return 1


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description="Prove a test can fail, in throwaway copies of the repository.",
        epilog="Example: red-proof.py --repo . --cwd backend --link backend/.venv "
               "--edit backend/src/x.py 'await lock()' 'pass' "
               "--expect 'FAILED .*test_lock' -- .venv/bin/python -m pytest -q tests/test_x.py")
    parser.add_argument("--repo", default=".", help="any path inside the repository")
    parser.add_argument("--cwd", default=".", help="directory, relative to the repo root, to run in")
    parser.add_argument("--link", action="append", default=[],
                        help="ignored path to symlink into each copy (a venv, node_modules, .env)")
    parser.add_argument("--edit", action="append", nargs=3, metavar=("PATH", "OLD", "NEW"),
                        help="one mutant: replace the unique OLD with NEW in PATH")
    parser.add_argument("--edits", action="append", default=[],
                        help="JSON list of {path, old, new, id?}; repeatable, every file's mutants run")
    parser.add_argument("--rev", help="revision mode: the command must fail at this revision")
    parser.add_argument("--overlay", action="append", default=[],
                        help="with --rev: working-tree file put over the revision (the new test)")
    parser.add_argument("--expect", help="regex the failing output must match to count as a kill "
                        "(the failing test's name): a mutant that only breaks the import is no kill")
    parser.add_argument("--any-failure", action="store_true",
                        help="count any non-zero exit as a kill instead of --expect")
    parser.add_argument("--timeout", type=int, default=1800, help="seconds per command run")
    parser.add_argument("--keep", action="store_true", help="keep the copies and print where")
    parser.add_argument("command", nargs=argparse.REMAINDER, help="-- then the test command")
    args = parser.parse_args(argv)
    if args.command and args.command[0] == "--":
        args.command = args.command[1:]
    try:
        if not args.command:
            raise NoVerdict("no command: put the test command after --")
        edits = parse_edits(args)
        if bool(edits) == bool(args.rev):
            raise NoVerdict("give mutants (--edit/--edits) or --rev, not both or neither")
        if args.overlay and not args.rev:
            raise NoVerdict("--overlay needs --rev")
        if args.command[0] == "pnpm" and any(Path(link).name == "node_modules" for link in args.link):
            # see scripts/README.md red-proof.py
            raise NoVerdict("pnpm in a copy with a linked node_modules checks dependencies and tries to "
                            "reinstall into the original; run the tool from node_modules/.bin instead "
                            "(e.g. node_modules/.bin/vitest run ...)")
        if bool(args.expect) == args.any_failure:
            raise NoVerdict("give --expect <failing test regex>, or --any-failure on purpose")
        repo = repo_root(Path(args.repo).resolve())
        expect = re.compile(args.expect) if args.expect else None
        watched = [repo / e.path for e in edits] + [repo / o for o in args.overlay]
        before = digest(watched)
        work = Path(tempfile.mkdtemp(prefix="red-proof-"))
        try:
            code = (revision_mode(repo, args, work, expect) if args.rev
                    else mutation_mode(repo, args, edits, work, expect))
        finally:
            if args.keep:
                print(f"copies kept in {work}")
            else:
                shutil.rmtree(work, ignore_errors=True)
        if digest(watched) != before:
            print("warning: an edited file changed in the working tree during the run "
                  "(another process); the verdicts are for the content copied at each start")
        return code
    except NoVerdict as exc:
        print(f"red-proof: no verdict: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
