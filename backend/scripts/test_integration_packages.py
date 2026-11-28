import contextlib
import importlib.util
import io
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


SCRIPT_PATH = Path(__file__).with_name("integration-packages.py")
SPEC = importlib.util.spec_from_file_location("integration_packages", SCRIPT_PATH)
integration_packages = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(integration_packages)

REPO_ROOT = Path(__file__).resolve().parents[2]
BACKEND_ROOT = REPO_ROOT / "backend"
MODULE_ROOT = "example.com/hawkr/backend"
CATALOG = f"{MODULE_ROOT}/internal/modules/catalog"
CATALOG_DESCRIPTION = f"{CATALOG}/internal/description"
SELLER = f"{MODULE_ROOT}/internal/modules/seller"
PLATFORM_DB = f"{MODULE_ROOT}/internal/platform/db"


def package(import_path, relative_dir, deps=()):
    return {
        "ImportPath": import_path,
        "Dir": str(REPO_ROOT / relative_dir),
        "Deps": list(deps),
    }


PACKAGES = [
    package(CATALOG, "backend/internal/modules/catalog"),
    package(CATALOG_DESCRIPTION, "backend/internal/modules/catalog/internal/description"),
    package(SELLER, "backend/internal/modules/seller"),
    package(PLATFORM_DB, "backend/internal/platform/db"),
    package(
        f"{CATALOG}.test",
        "backend/internal/modules/catalog",
        [CATALOG, CATALOG_DESCRIPTION, PLATFORM_DB],
    ),
    package(f"{SELLER}.test", "backend/internal/modules/seller", [SELLER]),
]


PLAIN_FILE_SETS = {
    CATALOG: (("catalog.go", "", "catalog_test.go", ""),),
    SELLER: (("seller.go", "", "seller_test.go", ""),),
}

TAGGED_FILE_SETS = {
    CATALOG: (("catalog.go", "", "catalog_test.go", "catalog_integration_test.go"),),
    SELLER: (("seller.go", "", "seller_test.go", "seller_integration_test.go"),),
}


class IntegrationPackagesTest(unittest.TestCase):
    def selected(self, changed_paths):
        return integration_packages.select_packages(
            PLAIN_FILE_SETS,
            TAGGED_FILE_SETS,
            PACKAGES,
            changed_paths,
            REPO_ROOT,
            BACKEND_ROOT,
        )

    def test_changed_descendant_maps_to_ancestor_package(self):
        self.assertEqual(
            self.selected(
                ["backend/internal/modules/catalog/internal/description/data/catalog.json"]
            ),
            {CATALOG},
        )

    def test_forced_full_path_keeps_all_integration_packages(self):
        self.assertEqual(self.selected(["backend/go.mod"]), {CATALOG, SELLER})

    def test_unmapped_backend_path_keeps_all_integration_packages(self):
        self.assertEqual(
            self.selected(["backend/infra/seed-data/01-account.sql"]),
            {CATALOG, SELLER},
        )

    def test_unmapped_non_ascii_backend_path_keeps_all_integration_packages(self):
        self.assertEqual(
            self.selected(["backend/docs/codemeta/x.md"]),
            {CATALOG, SELLER},
        )

    def test_test_input_paths_keep_all_integration_packages(self):
        for path in (
            "api/hawkr/procurement/v1alpha1/procurement.proto",
            "backend/internal/modules/catalog/db/schema.sql",
            "backend/internal/modules/catalog/db/migrations/001.sql",
            "backend/cmd/hawkr/modules.go",
        ):
            with self.subTest(path=path):
                self.assertEqual(self.selected([path]), {CATALOG, SELLER})

    def test_unrelated_outside_path_is_ignored(self):
        self.assertEqual(self.selected(["docs/README.md"]), set())

    def test_changed_dependency_selects_dependent_test_package(self):
        self.assertEqual(
            self.selected(["backend/internal/platform/db/pool.go"]),
            {CATALOG},
        )

    def test_empty_filtered_selection_exits_successfully_without_output(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            changed_files = Path(temp_dir) / "changed-files"
            changed_files.write_text("docs/README.md\n", encoding="utf-8")
            plain = [
                {
                    "ImportPath": CATALOG,
                    "GoFiles": ["catalog.go"],
                    "TestGoFiles": ["catalog_test.go"],
                    "XTestGoFiles": [],
                    "CgoFiles": [],
                },
                {
                    "ImportPath": SELLER,
                    "GoFiles": ["seller.go"],
                    "TestGoFiles": ["seller_test.go"],
                    "XTestGoFiles": [],
                    "CgoFiles": [],
                },
            ]
            tagged = [
                {
                    **plain[0],
                    "TestGoFiles": ["catalog_test.go", "catalog_integration_test.go"],
                },
                {
                    **plain[1],
                    "TestGoFiles": ["seller_test.go", "seller_integration_test.go"],
                },
            ]
            stdout = io.StringIO()
            stderr = io.StringIO()
            with patch.object(integration_packages, "go_list", side_effect=[plain, tagged, PACKAGES]):
                with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
                    result = integration_packages.main(["--changed-files", str(changed_files)])

        self.assertEqual(result, 0)
        self.assertEqual(stdout.getvalue(), "")
        self.assertEqual(stderr.getvalue(), "")

    def test_empty_unfiltered_selection_remains_a_failure(self):
        stdout = io.StringIO()
        stderr = io.StringIO()
        with patch.object(integration_packages, "go_list", side_effect=[[], [], []]):
            with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
                result = integration_packages.main([])

        self.assertEqual(result, 1)
        self.assertEqual(stdout.getvalue(), "")
        self.assertIn("the filter is broken", stderr.getvalue())

    def test_empty_full_selection_in_changed_files_mode_is_a_failure(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            changed_files = Path(temp_dir) / "changed-files"
            changed_files.write_text("docs/README.md\n", encoding="utf-8")
            stdout = io.StringIO()
            stderr = io.StringIO()
            with patch.object(integration_packages, "go_list", side_effect=[[], [], PACKAGES]):
                with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
                    result = integration_packages.main(["--changed-files", str(changed_files)])

        self.assertEqual(result, 1)
        self.assertEqual(stdout.getvalue(), "")
        self.assertIn("the filter is broken", stderr.getvalue())


if __name__ == "__main__":
    unittest.main()
