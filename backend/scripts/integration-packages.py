#!/usr/bin/env python3
"""List the packages whose test binary builds differently under the integration tag.

A package needs an integration run when its own files, or the files of any package
linked into its test binary, change under `-tags integration`. Every other package
builds the same test binary with and without the tag, so the plain `go test` run
already covers it.

Run from backend/ with GOWORK=off. Prints one import path per line.
"""

import argparse
import fnmatch
import json
import posixpath
import subprocess
import sys
from pathlib import Path, PurePosixPath


BACKEND_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = BACKEND_ROOT.parent
FORCE_FULL_PATHS = {
    "backend/go.mod",
    "backend/go.sum",
    "backend/.go-version",
    "backend/scripts/integration-packages.py",
    ".github/workflows/backend-monolith.yml",
}
FORCE_FULL_PATH_PATTERNS = (
    "api/**",
    "backend/cmd/hawkr/modules.go",
    "backend/cmd/new-module/templates/**",
    "backend/gateway/internal/authz/capability.go",
    "backend/internal/arch/check.go",
    "backend/internal/arch/modules.go",
    "backend/internal/modules/*/db/schema.sql",
    "backend/internal/modules/*/db/migrations/**",
    "backend/internal/modules/buyer/internal/model/model.go",
    "backend/internal/modules/notification/internal/model/testdata/**",
    "backend/internal/modules/social/internal/pubsql/**",
    "backend/internal/platform/bizcalendar/testdata/**",
    "backend/internal/platform/stripe/testdata/**",
)


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


def changed_package_scope(
    changed_paths: list[str],
    packages: list[dict],
    repo_root: Path = REPO_ROOT,
    backend_root: Path = BACKEND_ROOT,
) -> tuple[set[str], bool]:
    repo_root = Path(repo_root).resolve()
    backend_root = Path(backend_root).resolve()
    packages_by_dir: dict[str, set[str]] = {}
    for package in packages:
        import_path = base(package.get("ImportPath", ""))
        directory = package.get("Dir")
        if not import_path or import_path.endswith(".test") or not directory:
            continue
        package_dir = Path(directory)
        if not package_dir.is_absolute():
            package_dir = backend_root / package_dir
        try:
            relative_dir = package_dir.resolve().relative_to(repo_root)
        except ValueError:
            continue
        if not relative_dir.parts or relative_dir.parts[0] != "backend":
            continue
        packages_by_dir.setdefault(relative_dir.as_posix(), set()).add(import_path)

    changed_packages = set()
    for changed_path in changed_paths:
        path = posixpath.normpath(changed_path.replace("\\", "/"))
        if path in FORCE_FULL_PATHS or any(
            fnmatch.fnmatchcase(path, pattern)
            for pattern in FORCE_FULL_PATH_PATTERNS
        ):
            return set(), True
        if path != "backend" and not path.startswith("backend/"):
            continue

        directory = PurePosixPath(path)
        if path != "backend":
            directory = directory.parent
        mapped = set()
        while True:
            mapped.update(packages_by_dir.get(directory.as_posix(), set()))
            if directory == PurePosixPath("backend"):
                break
            directory = directory.parent
            if directory == PurePosixPath("."):
                break
        if not mapped:
            return set(), True
        changed_packages.update(mapped)

    return changed_packages, False


def test_binary_links(packages: list[dict]) -> list[tuple[str, set[str]]]:
    test_packages = []
    for package in packages:
        import_path = base(package.get("ImportPath", ""))
        if not import_path.endswith(".test"):
            continue
        under_test = import_path.removesuffix(".test")
        linked = {base(dependency) for dependency in package.get("Deps", [])}
        linked.add(under_test)
        test_packages.append((under_test, linked))
    return test_packages


def select_packages(
    plain_file_sets: dict[str, tuple],
    tagged_file_sets: dict[str, tuple],
    packages: list[dict],
    changed_paths: list[str] | None = None,
    repo_root: Path = REPO_ROOT,
    backend_root: Path = BACKEND_ROOT,
) -> set[str]:
    tag_changed = {
        package
        for package in plain_file_sets.keys() | tagged_file_sets.keys()
        if plain_file_sets.get(package) != tagged_file_sets.get(package)
    }
    tests = test_binary_links(packages)
    full_selection = {
        package for package, linked in tests if linked & tag_changed
    }
    if changed_paths is None:
        return full_selection

    changed_packages, force_full = changed_package_scope(
        changed_paths, packages, repo_root, backend_root
    )
    if force_full:
        return full_selection
    return {
        package
        for package, linked in tests
        if package in full_selection and linked & changed_packages
    }


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--changed-files", type=Path)
    args = parser.parse_args(argv)

    plain = file_sets([])
    tagged = file_sets(["-tags", "integration"])
    packages = go_list(["-tags", "integration", "-test", "./..."])
    changed_paths = (
        args.changed_files.read_text(encoding="utf-8").splitlines()
        if args.changed_files
        else None
    )
    full_selection = select_packages(plain, tagged, packages)
    selected = select_packages(plain, tagged, packages, changed_paths)

    if not selected:
        if args.changed_files and full_selection:
            return 0
        print("integration-packages: no package changes under the integration tag; the filter is broken", file=sys.stderr)
        return 1
    print("\n".join(sorted(selected)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
